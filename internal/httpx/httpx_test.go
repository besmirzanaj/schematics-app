package httpx

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"schematics-app/internal/auth"
	"schematics-app/internal/config"
	"schematics-app/internal/store"
)

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	db := filepath.Join(t.TempDir(), "t.db")
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(st, config.Config{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	t.Cleanup(func() { st.Close() })
	return ts, st
}

func TestHealthz(t *testing.T) {
	ts, _ := newTestServer(t)
	res, err := ts.Client().Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("healthz = %d", res.StatusCode)
	}
}

func seedStore(t *testing.T, st *store.Store) {
	t.Helper()
	for _, q := range []string{
		`INSERT INTO makes (name) VALUES ('Ford')`,
		`INSERT INTO models (make_id, name, display_name, year, dataset_year, region, internal_only) VALUES (1,'Escape','Escape',2012,2013,'',0)`,
		`INSERT INTO models (make_id, name, display_name, year, dataset_year, region, internal_only) VALUES (1,'Mystery','Mystery',0,2005,'',1)`,
		`INSERT INTO systems (model_id, code) VALUES (1,'1114'),(2,'2222')`,
		`INSERT INTO objects (system_id, filename, display, kind, rel_path, sort_order) VALUES (1,'1114_1.pdf','1114_1','pdf','2013/Ford/Escape/1114/1114_1.pdf',1)`,
		`INSERT INTO objects (system_id, filename, display, kind, rel_path, sort_order) VALUES (2,'2222_1.jpg','2222_1','jpg','2005/Ford/Mystery/2222/2222_1.jpg',1)`,
		`INSERT INTO catalog_fts (rowid, model_name, make_name, system_code) VALUES (1,'Escape','Ford','1114')`,
	} {
		if _, err := st.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
}

func login(t *testing.T, base, email, pw string) (*http.Client, error) {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	form := map[string][]string{"email": {email}, "password": {pw}}
	res, err := c.PostForm(base+"/login", form)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 303 {
		return nil, fmt.Errorf("login %s -> %d", email, res.StatusCode)
	}
	return c, nil
}

func TestAccessControlMatrix(t *testing.T) {
	db := filepath.Join(t.TempDir(), "m.db")
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	seedStore(t, st)

	hash, _ := auth.HashPassword("pw")
	accts := []struct{ email, role string }{
		{"admin@x", "admin"}, {"staff@x", "staff"},
		{"cust-global@x", "customer"}, {"cust-make@x", "customer"},
		{"cust-year@x", "customer"}, {"nobody@x", "customer"},
	}
	for _, a := range accts {
		u, err := st.CreateUser(a.email, hash, a.role)
		if err != nil {
			t.Fatal(err)
		}
		switch a.email {
		case "cust-global@x":
			st.SetEntitlement(u.ID, "global", 0)
		case "cust-make@x":
			st.SetEntitlement(u.ID, "make", 1)
		case "cust-year@x":
			st.SetEntitlement(u.ID, "year", 2013)
		}
	}

	// model 1 (Ford Escape, public) exists on disk for the file test
	dataRoot := t.TempDir()
	dir := filepath.Join(dataRoot, "2013", "Ford", "Escape", "1114")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1114_1.pdf"), []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(st, config.Config{DataRoot: dataRoot})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)

	// anonymous -> redirect to login on both pages and files
	anon := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, p := range []string{"/", "/file/1"} {
		res, err := anon.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 303 {
			t.Errorf("anon %s = %d want 303", p, res.StatusCode)
		}
		res.Body.Close()
	}

	// customer expectations for public model 1: entitled -> 200, nobody -> 403
	custWant := map[string]int{"cust-global@x": 200, "cust-make@x": 200, "cust-year@x": 200, "nobody@x": 403}

	for _, a := range accts {
		c, err := login(t, ts.URL, a.email, "pw")
		if err != nil {
			t.Fatal(err)
		}

		for _, path := range []string{"/model/1", "/system/1"} {
			res, err := c.Get(ts.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			got := res.StatusCode
			res.Body.Close()
			want := 200
			if a.role == "customer" {
				want = custWant[a.email]
			}
			if got != want {
				t.Errorf("%s %s = %d want %d", a.email, path, got, want)
			}
		}

		// internal model 2: only staff/admin
		res, err := c.Get(ts.URL + "/system/2")
		if err != nil {
			t.Fatal(err)
		}
		code := res.StatusCode
		res.Body.Close()
		if a.role == "customer" && code != 403 {
			t.Errorf("%s /system/2 = %d want 403", a.email, code)
		}
		if (a.role == "admin" || a.role == "staff") && code != 200 {
			t.Errorf("%s /system/2 = %d want 200", a.email, code)
		}

		// file ACL + no-store header on paid content
		res, err = c.Get(ts.URL + "/file/1")
		if err != nil {
			t.Fatal(err)
		}
		code = res.StatusCode
		cc := res.Header.Get("Cache-Control")
		res.Body.Close()
		if a.role == "customer" {
			if want := custWant[a.email]; code != want {
				t.Errorf("%s /file/1 = %d want %d", a.email, code, want)
			}
		} else if code != 200 {
			t.Errorf("%s /file/1 = %d want 200", a.email, code)
		}
		if code == 200 && cc != "private, no-store" {
			t.Errorf("%s /file/1 Cache-Control = %q", a.email, cc)
		}
	}

	// search filters results by entitlement
	for _, tc := range []struct{ email, want string }{
		{"cust-global@x", "Ford — Escape"},
		{"nobody@x", "No matches"},
	} {
		c, err := login(t, ts.URL, tc.email, "pw")
		if err != nil {
			t.Fatal(err)
		}
		res, err := c.Get(ts.URL + "/search?q=Escape")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Errorf("%s search = %d want 200", tc.email, res.StatusCode)
		}
		if !strings.Contains(string(body), tc.want) {
			t.Errorf("%s search body missing %q", tc.email, tc.want)
		}
	}
}