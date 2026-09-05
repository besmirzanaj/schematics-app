package httpx

import (
	"net/http"

	"schematics-app/internal/store"
)

func (s *Server) modelByIDAndCheck(w http.ResponseWriter, r *http.Request, id int64) (store.Model, error) {
	u, _ := s.currentUser(r)
	m, err := s.St.ModelByID(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return store.Model{}, err
	}
	e, _ := s.St.EntitlementsForUser(u.ID)
	mk, _ := s.St.MakeByID(m.MakeID)
	if !e.Visible(m, mk, u.Role != "customer") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return store.Model{}, store.ErrNotFound
	}
	return m, nil
}

// makeVisible reports whether a non-admin user may reach the given make,
// mirroring the filter applied by Makes().
func (s *Server) makeVisible(mk store.Make, e store.Entitlements, admin bool) bool {
	if admin {
		return true
	}
	if mk.InternalOnly {
		return false
	}
	if e.Global || e.Makes[mk.ID] {
		return true
	}
	return s.St.MakeReachable(mk.ID, e)
}

func (s *Server) checkSystem(w http.ResponseWriter, r *http.Request, systemID int64) (store.ObjectRef, error) {
	u, _ := s.currentUser(r)
	ref, err := s.St.SystemWithContext(systemID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return store.ObjectRef{}, err
	}
	e, _ := s.St.EntitlementsForUser(u.ID)
	if !e.Visible(ref.Mod, ref.Mk, u.Role != "customer") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return store.ObjectRef{}, store.ErrNotFound
	}
	return ref, nil
}