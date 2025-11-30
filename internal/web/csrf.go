package web

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/sorin-work/bookshelf/internal/auth"
)

const (
	csrfCookie = "bookshelf_csrf"
	csrfField  = "csrf_token"
	csrfHeader = "X-CSRF-Token"
)

// csrf implements the double submit cookie pattern: every visitor gets a
// random token in a cookie, and every state changing request has to echo it
// back in a form field or header. Another site can make the browser send the
// cookie but cannot read it, so it cannot fill in the field.
func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if c, err := r.Cookie(csrfCookie); err == nil && len(c.Value) >= 32 {
			token = c.Value
		} else {
			token = auth.NewToken()
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookie,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				Secure:   s.secureCookies,
				SameSite: http.SameSiteLaxMode,
			})
		}

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			sent := r.Header.Get(csrfHeader)
			if sent == "" {
				sent = r.PostFormValue(csrfField)
			}
			if sent == "" || subtle.ConstantTimeCompare([]byte(sent), []byte(token)) != 1 {
				s.log.Warn("csrf check failed", "path", r.URL.Path)
				http.Error(w, "This form has expired. Go back, reload the page and try again.", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), csrfKey, token)))
	})
}

func csrfToken(r *http.Request) string {
	t, _ := r.Context().Value(csrfKey).(string)
	return t
}
