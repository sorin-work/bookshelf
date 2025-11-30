// Package web serves the HTML pages. Pages are rendered on the server with
// html/template; htmx swaps in fragments where a full reload would be
// clumsy.
package web

import (
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/sorin-work/bookshelf/internal/store"
)

type Options struct {
	Store         *store.Store
	Logger        *slog.Logger
	SecureCookies bool
	SessionTTL    time.Duration
	// Now returns the current time; tests replace it.
	Now func() time.Time
}

type Server struct {
	store         *store.Store
	log           *slog.Logger
	secureCookies bool
	sessionTTL    time.Duration
	now           func() time.Time
	pages         map[string]*template.Template
}

func New(o Options) (*Server, error) {
	s := &Server{
		store:         o.Store,
		log:           o.Logger,
		secureCookies: o.SecureCookies,
		sessionTTL:    o.SessionTTL,
		now:           o.Now,
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.sessionTTL == 0 {
		s.sessionTTL = 14 * 24 * time.Hour
	}
	pages, err := parsePages()
	if err != nil {
		return nil, err
	}
	s.pages = pages
	return s, nil
}

// Handler returns the router with every route and middleware attached.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticHandler()))
	mux.HandleFunc("GET /{$}", s.home)

	mux.HandleFunc("GET /signup", s.signupPage)
	mux.HandleFunc("POST /signup", s.signup)
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.login)

	return s.csrf(s.loadUser(mux))
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if err := s.store.Ping(r.Context()); err != nil {
		s.log.Error("health check failed", "err", err)
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "home.html", page{Title: "Welcome"})
}
