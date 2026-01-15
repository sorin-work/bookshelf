// Package web serves the HTML pages. Pages are rendered on the server with
// html/template; htmx swaps in fragments where a full reload would be
// clumsy.
package web

import (
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

type Options struct {
	Store         *store.Store
	Logger        *slog.Logger
	SecureCookies bool
	SessionTTL    time.Duration
	// Location is the library's time zone; defaults to UTC.
	Location *time.Location
	// Now returns the current time; tests replace it.
	Now func() time.Time
}

type Server struct {
	store         *store.Store
	log           *slog.Logger
	secureCookies bool
	sessionTTL    time.Duration
	loc           *time.Location
	now           func() time.Time
	pages         map[string]*template.Template
}

func New(o Options) (*Server, error) {
	s := &Server{
		store:         o.Store,
		log:           o.Logger,
		secureCookies: o.SecureCookies,
		sessionTTL:    o.SessionTTL,
		loc:           o.Location,
		now:           o.Now,
	}
	if s.loc == nil {
		s.loc = time.UTC
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
	mux.HandleFunc("POST /logout", s.logout)

	mux.HandleFunc("GET /account", s.requireUser(s.account))
	mux.HandleFunc("GET /catalogue", s.catalogue)
	mux.HandleFunc("GET /titles/new", s.requireLibrarian(s.newTitlePage))
	mux.HandleFunc("POST /titles", s.requireLibrarian(s.createTitle))
	mux.HandleFunc("GET /titles/{id}", s.titlePage)
	mux.HandleFunc("GET /titles/{id}/edit", s.requireLibrarian(s.editTitlePage))
	mux.HandleFunc("POST /titles/{id}", s.requireLibrarian(s.updateTitle))
	mux.HandleFunc("POST /titles/{id}/copies", s.requireLibrarian(s.addCopy))
	mux.HandleFunc("POST /copies/{id}/status", s.requireLibrarian(s.setCopyStatus))
	mux.HandleFunc("POST /titles/{id}/hold", s.requireUser(s.placeHold))
	mux.HandleFunc("POST /holds/{id}/cancel", s.requireUser(s.cancelHold))
	mux.HandleFunc("GET /holds", s.requireLibrarian(s.holds))
	mux.HandleFunc("GET /overdue", s.requireLibrarian(s.overdue))
	mux.HandleFunc("GET /dashboard", s.requireLibrarian(s.dashboard))

	mux.HandleFunc("GET /desk", s.requireLibrarian(s.desk))
	mux.HandleFunc("POST /desk/checkout", s.requireLibrarian(s.deskCheckout))
	mux.HandleFunc("POST /desk/return", s.requireLibrarian(s.deskReturn))
	mux.HandleFunc("POST /loans/{id}/renew", s.requireUser(s.renewLoan))

	mux.HandleFunc("GET /settings", s.requireLibrarian(s.settings))
	mux.HandleFunc("POST /settings", s.requireLibrarian(s.saveSettings))

	mux.HandleFunc("GET /members", s.requireLibrarian(s.members))
	mux.HandleFunc("GET /members/new", s.requireLibrarian(s.newMemberPage))
	mux.HandleFunc("POST /members", s.requireLibrarian(s.createMember))
	mux.HandleFunc("GET /members/{id}", s.requireLibrarian(s.memberPage))
	mux.HandleFunc("GET /members/{id}/edit", s.requireLibrarian(s.editMemberPage))
	mux.HandleFunc("POST /members/{id}", s.requireLibrarian(s.updateMember))
	mux.HandleFunc("POST /members/{id}/renew-card", s.requireLibrarian(s.renewCard))
	mux.HandleFunc("POST /members/{id}/role", s.requireLibrarian(s.setRole))
	mux.HandleFunc("POST /members/{id}/charges", s.requireLibrarian(s.addCharge))
	mux.HandleFunc("POST /fines/{id}/resolve", s.requireLibrarian(s.resolveFine))

	return s.recoverPanics(s.logRequests(s.csrf(s.loadUser(mux))))
}

// today is the current calendar day at the library.
func (s *Server) today() domain.Date {
	return domain.DateOf(s.now().In(s.loc))
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
	if u := currentUser(r); u != nil && u.IsLibrarian() {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	s.render(w, r, http.StatusOK, "home.html", page{Title: "Welcome"})
}
