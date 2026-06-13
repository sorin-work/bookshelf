package web

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverPanics(t *testing.T) {
	var logs bytes.Buffer
	s := &Server{log: slog.New(slog.NewTextHandler(&logs, nil))}
	h := s.recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("shelf collapsed")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/desk", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status %d", rec.Code)
	}
	if !strings.Contains(logs.String(), "shelf collapsed") {
		t.Errorf("panic not logged: %s", logs.String())
	}
}

func TestLogRequestsRecordsStatus(t *testing.T) {
	var logs bytes.Buffer
	s := &Server{log: slog.New(slog.NewTextHandler(&logs, nil))}
	h := s.logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/catalogue", nil))
	if !strings.Contains(logs.String(), "status=418") || !strings.Contains(logs.String(), "path=/catalogue") {
		t.Errorf("log line: %s", logs.String())
	}

	// Health checks are only logged at debug level.
	logs.Reset()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if logs.Len() != 0 {
		t.Errorf("health check logged at info: %s", logs.String())
	}
}

func TestSafeNext(t *testing.T) {
	tests := map[string]string{
		"":                 "/",
		"/account":         "/account",
		"//evil.example":   "/",
		`/\evil.example`:   "/",
		"https://evil.com": "/",
		"account":          "/",
	}
	for in, want := range tests {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}
