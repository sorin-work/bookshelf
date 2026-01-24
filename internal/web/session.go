package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/sorin-work/bookshelf/internal/auth"
	"github.com/sorin-work/bookshelf/internal/store"
)

const sessionCookie = "bookshelf_session"

type ctxKey int

const (
	userKey ctxKey = iota
	csrfKey
)

func currentUser(r *http.Request) *store.User {
	u, _ := r.Context().Value(userKey).(*store.User)
	return u
}

// loadUser looks up the session cookie and puts the signed in user, if any,
// on the request context.
func (s *Server) loadUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		u, err := s.store.SessionUser(r.Context(), auth.HashToken(c.Value), s.now())
		switch {
		case errors.Is(err, store.ErrNotFound):
			s.clearSessionCookie(w)
		case err != nil:
			s.serverError(w, r, err)
			return
		default:
			r = r.WithContext(context.WithValue(r.Context(), userKey, &u))
		}
		next.ServeHTTP(w, r)
	})
}

// requireUser sends anonymous visitors to the login page and back again
// afterwards.
func (s *Server) requireUser(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r) == nil {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		h(w, r)
	}
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	token := auth.NewToken()
	if err := s.store.CreateSession(r.Context(), auth.HashToken(token), userID, s.now(), s.sessionTTL); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(s.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// safeNext only allows redirects to paths on this site.
func safeNext(next string) string {
	if next == "" || next[0] != '/' || (len(next) > 1 && (next[1] == '/' || next[1] == '\\')) {
		return "/"
	}
	return next
}

// requireLibrarian is requireUser plus a 403 for members.
func (s *Server) requireLibrarian(h http.HandlerFunc) http.HandlerFunc {
	return s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).IsLibrarian() {
			s.forbidden(w, r)
			return
		}
		h(w, r)
	})
}
