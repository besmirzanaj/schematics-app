package httpx

import (
	"net/http"
	"strconv"

	"schematics-app/internal/auth"
)

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	users, _ := s.St.ListUsers()
	s.render(w, r, "admin_users.tmpl", map[string]any{"User": u, "Users": users, "Q": ""})
}

func (s *Server) adminCreateUser(w http.ResponseWriter, r *http.Request) {
	email := r.PostFormValue("email")
	role := r.PostFormValue("role")
	pw := r.PostFormValue("password")
	if role != "admin" && role != "staff" && role != "customer" {
		role = "customer"
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		http.Error(w, "bad password", http.StatusBadRequest)
		return
	}
	if _, err := s.St.CreateUser(email, hash, role); err != nil {
		http.Error(w, "create failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (s *Server) adminSetRole(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	s.St.UpdateUserRole(id, r.PostFormValue("role"))
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (s *Server) adminSetPassword(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	pw := r.PostFormValue("password")
	if pw == "" {
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		http.Error(w, "bad password", http.StatusBadRequest)
		return
	}
	s.St.UpdateUserPassword(id, hash)
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (s *Server) adminSetEntitlements(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	s.St.ClearEntitlements(id)
	if v := r.PostFormValue("scope"); v != "" {
		if v == "global" {
			s.St.SetEntitlement(id, "global", 0)
		} else if v == "make" {
			mk, _ := strconv.ParseInt(r.PostFormValue("ref"), 10, 64)
			s.St.SetEntitlement(id, "make", mk)
		} else if v == "model" {
			m, _ := strconv.ParseInt(r.PostFormValue("ref"), 10, 64)
			s.St.SetEntitlement(id, "model", m)
		} else if v == "year" {
			y, _ := strconv.ParseInt(r.PostFormValue("ref"), 10, 64)
			s.St.SetEntitlement(id, "year", y)
		}
	}
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (s *Server) adminDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	s.St.DeleteUser(id)
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}