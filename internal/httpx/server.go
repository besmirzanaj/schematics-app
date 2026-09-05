package httpx

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"schematics-app/internal/config"
	"schematics-app/internal/store"
)

//go:embed templates
var templateFS embed.FS

type Server struct {
	St            *store.Store
	DataRoot      string
	SecureCookies bool
	Templates     *template.Template
	Static        fs.FS
}

func NewServer(st *store.Store, cfg config.Config) (*Server, error) {
	// templates tree is the embed root; static files live under templates/static.
	static, err := fs.Sub(templateFS, "templates")
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}).ParseFS(templateFS, "templates/*.tmpl")
	if err != nil {
		return nil, err
	}
	s := &Server{
		St:            st,
		DataRoot:      cfg.DataRoot,
		SecureCookies: cfg.SecureCookies,
		Templates:     tmpl,
		Static:        static,
	}
	ensureAdmin(st, cfg.AdminEmail)
	return s, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/{file...}", http.FileServerFS(s.Static))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })

	mux.HandleFunc("GET /login", s.getLogin)
	mux.HandleFunc("POST /login", s.postLogin)
	mux.HandleFunc("POST /logout", s.postLogout)

	authed := http.NewServeMux()
	authed.HandleFunc("GET /", s.home)
	authed.HandleFunc("GET /catalog/makes/{id}", s.brand)
	authed.HandleFunc("GET /model/{id}", s.modelPage)
	authed.HandleFunc("GET /system/{id}", s.systemPage)
	authed.HandleFunc("GET /file/{id}", s.fileHandler)
	authed.HandleFunc("GET /search", s.search)
	mux.Handle("/", s.requireAuth(authed))

	admin := http.NewServeMux()
	admin.HandleFunc("GET /admin/users", s.adminUsers)
	admin.HandleFunc("POST /admin/users", s.adminCreateUser)
	admin.HandleFunc("POST /admin/users/{id}/role", s.adminSetRole)
	admin.HandleFunc("POST /admin/users/{id}/password", s.adminSetPassword)
	admin.HandleFunc("POST /admin/users/{id}/entitlements", s.adminSetEntitlements)
	admin.HandleFunc("POST /admin/users/{id}/delete", s.adminDeleteUser)
	mux.Handle("/admin/", s.requireAuth(s.requireAdmin(admin)))

	return mux
}

// render writes a page template.
func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.Templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}