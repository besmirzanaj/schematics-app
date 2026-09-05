package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func seed(t *testing.T, s *Store) (Make, Model) {
	t.Helper()
	if _, err := s.db.Exec(`INSERT INTO makes (name) VALUES ('Ford')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO models (make_id, name, display_name, year, dataset_year, region) VALUES (1, 'Escape', 'Escape', 2012, 2013, '')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO models (make_id, name, display_name, year, dataset_year, region, internal_only) VALUES (1, 'Internal', 'Internal', 0, 0, '', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO systems (model_id, code) VALUES (1, '1114')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO objects (system_id, filename, display, kind, rel_path, sort_order) VALUES (1, '1114_1.pdf', '1114_1', 'pdf', '2013/Ford/Escape/1114/1114_1.pdf', 1)`); err != nil {
		t.Fatal(err)
	}
	return Make{ID: 1, Name: "Ford"}, Model{ID: 1, MakeID: 1, Name: "Escape", DisplayName: "Escape", Year: 2012, DatasetYear: 2013}
}

func TestVisibleMatrix(t *testing.T) {
	mk := Make{ID: 1, Name: "Ford"}
	m := Model{ID: 1, MakeID: 1, Name: "Escape", DatasetYear: 2013}
	cases := []struct {
		name  string
		e     Entitlements
		admin bool
		want  bool
	}{
		{"admin sees all", Entitlements{}, true, true},
		{"nothing denied", Entitlements{Global: false, Makes: map[int64]bool{}, Models: map[int64]bool{}, Years: map[int64]bool{}}, false, false},
		{"global ok", Entitlements{Global: true}, false, true},
		{"make scope", Entitlements{Makes: map[int64]bool{1: true}}, false, true},
		{"model scope", Entitlements{Models: map[int64]bool{1: true}}, false, true},
		{"year scope", Entitlements{Years: map[int64]bool{2013: true}}, false, true},
		{"wrong make", Entitlements{Makes: map[int64]bool{99: true}}, false, false},
	}
	for _, c := range cases {
		if got := c.e.Visible(m, mk, c.admin); got != c.want {
			t.Errorf("%s: Visible = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestStoreCRUD(t *testing.T) {
	s := newTestStore(t)
	mk, m := seed(t, s)

	gets, err := s.Makes(Entitlements{Global: true}, false)
	if err != nil || len(gets) != 1 || gets[0].Name != "Ford" {
		t.Fatalf("Makes = %v, err=%v", gets, err)
	}
	// internal make hidden from customer global
	if _, err := s.db.Exec(`UPDATE makes SET internal_only=1 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	gets, _ = s.Makes(Entitlements{Global: true}, false)
	if len(gets) != 0 {
		t.Fatalf("internal make leaked: %v", gets)
	}
	if _, err := s.db.Exec(`UPDATE makes SET internal_only=0 WHERE id=1`); err != nil {
		t.Fatal(err)
	}

	mods, err := s.Models(mk.ID, Entitlements{Global: true}, false)
	if err != nil || len(mods) != 1 {
		t.Fatalf("Models = %v, err=%v", mods, err)
	}
	if mods[0].ID != m.ID {
		t.Fatalf("wrong model")
	}

	sys, err := s.Systems(m.ID)
	if err != nil || len(sys) != 1 || sys[0].Code != "1114" {
		t.Fatalf("Systems = %v, err=%v", sys, err)
	}

	objs, err := s.Objects(sys[0].ID)
	if err != nil || len(objs) != 1 || objs[0].Kind != "pdf" {
		t.Fatalf("Objects = %v, err=%v", objs, err)
	}

	ref, err := s.Object(objs[0].ID)
	if err != nil || ref.Mk.Name != "Ford" || ref.Obj.ID != objs[0].ID {
		t.Fatalf("Object = %+v, err=%v", ref, err)
	}
	if _, err := s.Object(9999); err != ErrNotFound {
		t.Fatalf("missing object err = %v, want ErrNotFound", err)
	}
}

func TestMakesReachability(t *testing.T) {
	s := newTestStore(t)
	seed(t, s)

	// year-only customer: make appears because it owns a 2013 model
	gets, err := s.Makes(Entitlements{Years: map[int64]bool{2013: true}}, false)
	if err != nil || len(gets) != 1 || gets[0].Name != "Ford" {
		t.Fatalf("year-only Makes = %v, err=%v", gets, err)
	}
	// model-only customer on model 1 (Ford Escape)
	gets, err = s.Makes(Entitlements{Models: map[int64]bool{1: true}}, false)
	if err != nil || len(gets) != 1 || gets[0].Name != "Ford" {
		t.Fatalf("model-only Makes = %v, err=%v", gets, err)
	}
	// wrong year -> nothing
	gets, err = s.Makes(Entitlements{Years: map[int64]bool{2005: true}}, false)
	if err != nil || len(gets) != 0 {
		t.Fatalf("wrong-year Makes = %v, err=%v", gets, err)
	}
	// admin sees internal makes too
	gets, err = s.Makes(Entitlements{}, true)
	if err != nil || len(gets) != 1 {
		t.Fatalf("admin Makes = %v, err=%v", gets, err)
	}
}

func TestUsersSessionsEntitlements(t *testing.T) {
	s := newTestStore(t)
	u, err := s.CreateUser("a@b.c", "hash", "customer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser("a@b.c", "x", "staff"); err == nil {
		t.Fatal("expected unique email violation")
	}
	if err := s.SetEntitlement(u.ID, "year", 2013); err != nil {
		t.Fatal(err)
	}
	e, err := s.EntitlementsForUser(u.ID)
	if err != nil || !e.Years[2013] || e.Global {
		t.Fatalf("Entitlements = %+v err=%v", e, err)
	}
	if err := s.CreateSession(u.ID, "abcd", "2099-01-01 00:00:00"); err != nil {
		t.Fatal(err)
	}
	got, err := s.UserByToken("abcd")
	if err != nil || got.Email != "a@b.c" {
		t.Fatalf("UserByToken = %v err=%v", got, err)
	}
	if _, err := s.UserByToken("nope"); err != ErrNotFound {
		t.Fatalf("bad token err = %v", err)
	}
}

func TestSearchBuildMatch(t *testing.T) {
	if got := buildMatch("a3 2012"); got != `"a3" AND "2012"` {
		t.Fatalf("buildMatch = %q", got)
	}
	if got := buildMatch(" */š@ 4 "); got != `"4"` {
		t.Fatalf("buildMatch dirty = %q", got)
	}
}