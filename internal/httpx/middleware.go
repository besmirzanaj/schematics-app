package httpx

import (
	"context"
	"io/fs"
	"net/http"

	"schematics-app/internal/auth"
	"schematics-app/internal/store"
)

type ctxKey int

const ctxUser ctxKey = 0

func (s *Server) currentUser(r *http.Request) (store.User, bool) {
	u, ok := r.Context().Value(ctxUser).(store.User)
	return u, ok
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("skemat_session")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		u, err := s.St.UserByToken(auth.SHA256Hex(c.Value))
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxUser, u)))
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.currentUser(r)
		if !ok || u.Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "skemat_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// noDirs hides directories from FileServerFS so /static never renders a
// directory listing. Requests for a directory (with or without trailing
// slash) surface as 404 instead.
type noDirs struct{ fs fs.FS }

func (n noDirs) Open(name string) (fs.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if st.IsDir() {
		f.Close()
		return nil, fs.ErrNotExist
	}
	return f, nil
}

var securityHeaders = map[string]string{
	"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	"Content-Security-Policy": "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; worker-src 'self' blob:; frame-ancestors 'self'; base-uri 'self'; form-action 'self'",
	"X-Content-Type-Options":    "nosniff",
	"X-Frame-Options":           "SAMEORIGIN",
	"Referrer-Policy":           "strict-origin-when-cross-origin",
	"Permissions-Policy":        "geolocation=(), camera=(), microphone=()",
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range securityHeaders {
			w.Header().Set(k, v)
		}
		next.ServeHTTP(w, r)
	})
}