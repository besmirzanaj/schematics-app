package httpx

import (
	"net/http"
	"path/filepath"
	"strconv"
)

var contentTypes = map[string]string{
	"pdf": "application/pdf",
	"jpg": "image/jpeg",
	"png": "image/png",
	"swf": "application/x-shockwave-flash",
}

func (s *Server) fileHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	ref, err := s.St.Object(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	e, _ := s.St.EntitlementsForUser(u.ID)
	if !e.Visible(ref.Mod, ref.Mk, u.Role != "customer") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	// ACL integrity: never cache paid content at the edge
	w.Header().Set("Cache-Control", "private, no-store")
	if ct := contentTypes[ref.Obj.Kind]; ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+ref.Obj.Filename+`"`)
	}
	http.ServeFile(w, r, filepath.Join(s.DataRoot, ref.Obj.RelPath))
}