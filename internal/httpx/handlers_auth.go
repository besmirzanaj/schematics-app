package httpx

import (
	"net/http"
	"time"

	"schematics-app/internal/auth"
)

func (s *Server) getLogin(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "login.tmpl", map[string]any{"Error": "", "Q": ""})
}

func (s *Server) postLogin(w http.ResponseWriter, r *http.Request) {
	email := r.PostFormValue("email")
	pw := r.PostFormValue("password")
	u, err := s.St.UserByEmail(email)
	if err != nil || !auth.CheckPassword(u.PasswordHash, pw) {
		s.render(w, r, "login.tmpl", map[string]any{"Error": "Invalid email or password"})
		return
	}
	token, err := auth.GenerateToken()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := s.St.CreateSession(u.ID, auth.SHA256Hex(token), time.Now().Add(auth.SessionExpiry).Format("2006-01-02 15:04:05")); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	s.setSessionCookie(w, token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) postLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("skemat_session"); err == nil {
		_ = s.St.DeleteSession(auth.SHA256Hex(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "skemat_session", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}