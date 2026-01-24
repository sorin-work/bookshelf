package web

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestHealthz(t *testing.T) {
	env := newTestEnv(t)
	r := env.client().get("/healthz")
	wantStatus(t, r, http.StatusOK)
}

func TestSecurityHeaders(t *testing.T) {
	env := newTestEnv(t)
	r := env.client().get("/")
	if csp := r.header.Get("Content-Security-Policy"); !contains(csp, "default-src 'self'") {
		t.Errorf("Content-Security-Policy = %q", csp)
	}
	if r.header.Get("X-Frame-Options") != "DENY" || r.header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("headers %v", r.header)
	}
}

func TestSignupLoginLogout(t *testing.T) {
	env := newTestEnv(t)
	c := env.client()

	wantRedirect(t, c.signup("Ana Pop", "ana@example.com", "long enough"), "/")
	wantBody(t, c.get("/account"), "My account", "Log out")

	wantRedirect(t, c.post("/logout", nil), "/login")
	r := c.get("/")
	if r.status != http.StatusOK {
		t.Fatalf("home after logout: %d", r.status)
	}
	wantBody(t, r, "Sign up")

	wantRedirect(t, c.login("ANA@example.com", "long enough"), "/")
	wantBody(t, c.get("/account"), "Hello, Ana Pop")
}

func TestSignupValidation(t *testing.T) {
	env := newTestEnv(t)
	tests := []struct {
		name, email, password, want string
	}{
		{"", "a@example.com", "long enough", "Tell us your name."},
		{"Ana", "not-an-email", "long enough", "Enter a valid email address."},
		{"Ana", "a@example.com", "short", "Use at least 8 characters."},
	}
	for _, tt := range tests {
		r := env.client().signup(tt.name, tt.email, tt.password)
		wantStatus(t, r, http.StatusUnprocessableEntity)
		wantBody(t, r, tt.want)
	}
}

func TestSignupDuplicateEmail(t *testing.T) {
	env := newTestEnv(t)
	env.client().signup("Ana", "ana@example.com", "long enough")
	r := env.client().signup("Other Ana", "Ana@Example.com", "long enough")
	wantStatus(t, r, http.StatusUnprocessableEntity)
	wantBody(t, r, "already an account")
}

func TestLoginWrongPassword(t *testing.T) {
	env := newTestEnv(t)
	env.client().signup("Ana", "ana@example.com", "long enough")
	r := env.client().login("ana@example.com", "wrong password")
	wantStatus(t, r, http.StatusUnauthorized)
	wantBody(t, r, "don&#39;t match")
}

func TestLoginRedirectsToNext(t *testing.T) {
	env := newTestEnv(t)
	env.client().signup("Ana", "ana@example.com", "long enough")
	tests := []struct{ next, want string }{
		{"/catalogue?q=dune", "/catalogue?q=dune"},
		{"https://evil.example/", "/"},
		{"//evil.example/", "/"},
		{"", "/"},
	}
	for _, tt := range tests {
		c := env.client()
		r := c.post("/login", url.Values{"email": {"ana@example.com"}, "password": {"long enough"}, "next": {tt.next}})
		wantRedirect(t, r, tt.want)
	}
}

func TestCSRFRequired(t *testing.T) {
	env := newTestEnv(t)
	c := env.client()
	r := c.post("/signup", url.Values{
		"csrf_token": {"forged-token-forged-token-forged-token"},
		"name":       {"Ana"}, "email": {"ana@example.com"}, "password": {"long enough"},
	})
	wantStatus(t, r, http.StatusForbidden)
}

func TestFirstAccountIsLibrarian(t *testing.T) {
	env := newTestEnv(t)
	env.client().signup("First", "first@example.com", "long enough")
	env.client().signup("Second", "second@example.com", "long enough")

	first, err := env.store.UserByEmail(t.Context(), "first@example.com")
	if err != nil {
		t.Fatal(err)
	}
	second, err := env.store.UserByEmail(t.Context(), "second@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !first.IsLibrarian() || second.IsLibrarian() {
		t.Errorf("roles: first %s, second %s", first.Role, second.Role)
	}
}

func TestSessionExpires(t *testing.T) {
	env := newTestEnv(t)
	c := env.client()
	c.signup("Ana", "ana@example.com", "long enough")
	wantStatus(t, c.get("/account"), http.StatusOK)

	// The cookie outlives the session in this test; the server must still
	// refuse it.
	env.now = env.now.Add(testSessionTTL + time.Hour)
	r := c.get("/account")
	wantRedirect(t, r, "/login?next=%2Faccount")
}
