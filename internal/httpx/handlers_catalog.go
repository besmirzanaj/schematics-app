package httpx

import (
	"net/http"
	"slices"
	"strconv"

	"schematics-app/internal/store"
)

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	e, _ := s.St.EntitlementsForUser(u.ID)
	brands, _ := s.St.Brands(e, u.Role != "customer")
	s.render(w, r, "home.tmpl", map[string]any{"User": u, "Brands": brands, "Q": ""})
}

func (s *Server) brand(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	e, _ := s.St.EntitlementsForUser(u.ID)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	mk, err := s.St.MakeByID(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !s.makeVisible(mk, e, u.Role != "customer") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	mods, _ := s.St.Models(id, e, u.Role != "customer")

	var editions []int64
	seen := map[int64]bool{}
	for _, m := range mods {
		if !seen[m.DatasetYear] {
			seen[m.DatasetYear] = true
			editions = append(editions, m.DatasetYear)
		}
	}
	slices.Sort(editions)

	year := r.URL.Query().Get("year")
	var shown []store.Model
	if year == "" {
		shown = mods
	} else {
		for _, m := range mods {
			if strconv.Itoa(int(m.DatasetYear)) == year {
				shown = append(shown, m)
			}
		}
	}
	s.render(w, r, "brand.tmpl", map[string]any{
		"User": u, "Make": mk, "Models": shown, "Editions": editions, "Year": year, "Q": "",
	})
}

func (s *Server) modelPage(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	m, err := s.modelByIDAndCheck(w, r, id)
	if err != nil {
		return
	}
	sys, _ := s.St.Systems(m.ID)
	mk, _ := s.St.MakeByID(m.MakeID)
	s.render(w, r, "model.tmpl", map[string]any{
		"Model": m, "User": u, "Systems": sys, "Make": mk,
		"CargoYear": store.CargoYear(m.Name), "Q": "",
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
		"CargoYear": store.CargoYear(ref.Mod.Name),
	})
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	e, _ := s.St.EntitlementsForUser(u.ID)
	q := r.URL.Query().Get("q")
	hits, _ := s.St.Search(q, e, u.Role != "customer")
	s.render(w, r, "search.tmpl", map[string]any{"Hits": hits, "Q": q, "User": u})
}