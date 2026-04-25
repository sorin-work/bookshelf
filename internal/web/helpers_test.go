package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/sorin-work/bookshelf/internal/auth"
	"github.com/sorin-work/bookshelf/internal/db"
	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

func init() { auth.Cost = bcrypt.MinCost }

const testSessionTTL = 365 * 24 * time.Hour

// testNow is the fixed "current time" handlers see in tests.
var testNow = time.Date(2026, time.March, 10, 14, 0, 0, 0, time.UTC)

type testEnv struct {
	t     *testing.T
	srv   *httptest.Server
	store *store.Store
	now   time.Time
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	return newTestEnvIn(t, time.UTC)
}

func newTestEnvIn(t *testing.T, loc *time.Location) *testEnv {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := db.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}

	env := &testEnv{t: t, store: store.New(conn), now: testNow}
	app, err := New(Options{
		Store:    env.store,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Location: loc,
		// Long enough for tests that move the clock forward by months.
		SessionTTL: testSessionTTL,
		Now:        func() time.Time { return env.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	env.srv = httptest.NewServer(app.Handler())
	t.Cleanup(env.srv.Close)
	return env
}

// client is a browser stand-in: it keeps cookies and does not follow
// redirects, so tests can check where they point.
type client struct {
	env *testEnv
	hc  *http.Client
}

func (e *testEnv) client() *client {
	jar, _ := cookiejar.New(nil)
	return &client{env: e, hc: &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

type response struct {
	status   int
	body     string
	location string
	header   http.Header
}

func (c *client) do(req *http.Request) response {
	c.env.t.Helper()
	res, err := c.hc.Do(req)
	if err != nil {
		c.env.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return response{status: res.StatusCode, body: string(b), location: res.Header.Get("Location"), header: res.Header}
}

func (c *client) get(path string) response {
	c.env.t.Helper()
	req, _ := http.NewRequest(http.MethodGet, c.env.srv.URL+path, nil)
	return c.do(req)
}

func (c *client) csrf() string {
	u, _ := url.Parse(c.env.srv.URL)
	for _, ck := range c.hc.Jar.Cookies(u) {
		if ck.Name == csrfCookie {
			return ck.Value
		}
	}
	// Any GET hands out the cookie.
	c.get("/healthz")
	for _, ck := range c.hc.Jar.Cookies(u) {
		if ck.Name == csrfCookie {
			return ck.Value
		}
	}
	c.env.t.Fatal("no csrf cookie")
	return ""
}

// post submits a form the way a browser would, including the CSRF token.
func (c *client) post(path string, form url.Values) response {
	c.env.t.Helper()
	if form == nil {
		form = url.Values{}
	}
	if !form.Has(csrfField) {
		form.Set(csrfField, c.csrf())
	}
	req, _ := http.NewRequest(http.MethodPost, c.env.srv.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.do(req)
}

func (c *client) signup(name, email, password string) response {
	c.env.t.Helper()
	return c.post("/signup", url.Values{"name": {name}, "email": {email}, "password": {password}})
}

func (c *client) login(email, password string) response {
	c.env.t.Helper()
	return c.post("/login", url.Values{"email": {email}, "password": {password}})
}

func wantStatus(t *testing.T, r response, status int) {
	t.Helper()
	if r.status != status {
		t.Fatalf("status = %d, want %d\n%s", r.status, status, r.body)
	}
}

func wantRedirect(t *testing.T, r response, location string) {
	t.Helper()
	if r.status != http.StatusSeeOther || r.location != location {
		t.Fatalf("got %d to %q, want 303 to %q\n%s", r.status, r.location, location, r.body)
	}
}

func wantBody(t *testing.T, r response, substrings ...string) {
	t.Helper()
	for _, s := range substrings {
		if !strings.Contains(r.body, s) {
			t.Errorf("body does not contain %q", s)
		}
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func testDate(t *testing.T, s string) domain.Date {
	t.Helper()
	d, err := domain.ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
