package web

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// logRequests logs one line per request. Static files and health checks are
// logged at debug level so they don't drown everything else.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		level := s.log.Info
		if r.URL.Path == "/healthz" || len(r.URL.Path) > 8 && r.URL.Path[:8] == "/static/" {
			level = s.log.Debug
		}
		level("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// recoverPanics turns a panic in a handler into a 500 and a logged stack.
func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				s.log.Error("panic", "path", r.URL.Path, "err", fmt.Sprint(v), "stack", string(debug.Stack()))
				http.Error(w, "Something went wrong on our side. Please try again.", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// contentSecurityPolicy only allows our own scripts and styles plus htmx
// from unpkg. Covers come from wherever the librarian points them, so
// images may load from any https site.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' https://unpkg.com; " +
	"style-src 'self'; " +
	"img-src 'self' https: data:; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
