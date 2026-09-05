package httpx

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

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