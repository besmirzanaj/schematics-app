package httpx

import (
	"net/http"
	"strconv"

	"schematics-app/internal/store"
)

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	s.render(w, r, "home.tmpl", map[string]any{"User": u, "Years": []int{2005, 2008, 2013}, "Q": ""})
}

func (s *Server) makes(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	e, _ := s.St.EntitlementsForUser(u.ID)
	ms, _ := s.St.Makes(e, u.Role != "customer")
	year := r.URL.Query().Get("year")
	out := make([]store.Make, 0, len(ms))
	for _, m := range ms {
		if year == "" {
			out = append(out, m)
			continue
		}
		mods, _ := s.St.Models(m.ID, e, u.Role != "customer")
		for _, mod := range mods {
			if strconv.Itoa(int(mod.DatasetYear)) == year {
				out = append(out, m)
				break
			}
		}
	}
	s.render(w, r, "makes.tmpl", map[string]any{"Makes": out, "Year": year})
}

func (s *Server) models(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	e, _ := s.St.EntitlementsForUser(u.ID)
	mkID, _ := strconv.ParseInt(r.URL.Query().Get("make_id"), 10, 64)
	mods, _ := s.St.Models(mkID, e, u.Role != "customer")
	s.render(w, r, "models.tmpl", map[string]any{"Models": mods, "MakeID": mkID})
}

func (s *Server) modelPage(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	m, err := s.modelByIDAndCheck(w, r, id)
	if err != nil {
		return
	}
	sys, _ := s.St.Systems(m.ID)
	s.render(w, r, "model.tmpl", map[string]any{
		"Model": m, "User": u, "Systems": sys, "Q": "",
	})
}

func (s *Server) systemPage(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	ref, err := s.checkSystem(w, r, id)
	if err != nil {
		return
	}
	objs, _ := s.St.Objects(ref.Sys.ID)
	s.render(w, r, "system.tmpl", map[string]any{
		"User": u, "Ref": ref, "Objects": objs, "Q": "",
	})
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	e, _ := s.St.EntitlementsForUser(u.ID)
	q := r.URL.Query().Get("q")
	hits, _ := s.St.Search(q, e, u.Role != "customer")
	s.render(w, r, "search.tmpl", map[string]any{"Hits": hits, "Q": q, "User": u})
}