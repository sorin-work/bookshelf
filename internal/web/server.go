// Package web serves the HTML pages. Pages are rendered on the server with
// html/template; htmx swaps in fragments where a full reload would be
// clumsy.
package web

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/openlibrary"
	"github.com/sorin-work/bookshelf/internal/store"
)

type Options struct {
	Store         *store.Store
	Logger        *slog.Logger
	SecureCookies bool
	SessionTTL    time.Duration
	// DisableSignup hides the sign up page; members are then registered
	// at the desk.
	DisableSignup bool
	// LibraryName is shown in the header and on slips.
	LibraryName string
	// Books looks titles up by ISBN. Nil switches the lookup off.
	Books BookLookup
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
	logins        *loginLimiter
	books         BookLookup
	allowSignup   bool
	libraryName   string
}

// BookLookup finds book details by ISBN-13.
type BookLookup interface {
	Lookup(ctx context.Context, isbn string) (openlibrary.Book, error)
}

func New(o Options) (*Server, error) {
	s := &Server{
		store:         o.Store,
		log:           o.Logger,
		secureCookies: o.SecureCookies,
		sessionTTL:    o.SessionTTL,
		loc:           o.Location,
		now:           o.Now,
		logins:        newLoginLimiter(5, 15*time.Minute),
		books:         o.Books,
		allowSignup:   !o.DisableSignup,
		libraryName:   o.LibraryName,
	}
	if s.libraryName == "" {
		s.libraryName = "Bookshelf"
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
	pages, err := parsePages(s.loc)
	if err != nil {
		return nil, err
	}
	s.pages = pages
	return s, nil
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
