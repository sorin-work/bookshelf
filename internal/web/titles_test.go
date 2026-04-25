package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// librarian signs up the first account, which makes it the librarian.
func (e *testEnv) librarian() *client {
	e.t.Helper()
	c := e.client()
	wantRedirect(e.t, c.signup("Lena Librarian", "lena@example.com", "long enough"), "/")
	return c
}

func (e *testEnv) member(name, email string) *client {
	e.t.Helper()
	c := e.client()
	wantRedirect(e.t, c.signup(name, email, "long enough"), "/")
	return c
}

func titleValues(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return v
}

func (c *client) addTitle(kv ...string) string {
	c.env.t.Helper()
	r := c.post("/titles", titleValues(kv...))
	if r.status != http.StatusSeeOther || !strings.HasPrefix(r.location, "/titles/") {
		c.env.t.Fatalf("add title: %d %s\n%s", r.status, r.location, r.body)
	}
	return r.location
}

func TestAddAndEditTitle(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()

	loc := lib.addTitle("title", "The Left Hand of Darkness", "isbn", "0-441-47812-3",
		"authors", "Ursula K. Le Guin", "subjects", "Science fiction, Classics", "year", "1969")
	r := lib.get(loc)
	wantStatus(t, r, http.StatusOK)
	wantBody(t, r, "The Left Hand of Darkness", "9780441478125", "Ursula K. Le Guin",
		`<a href="/catalogue?subject=Classics">Classics</a>, <a href="/catalogue?subject=Science%20fiction">Science fiction</a>`)

	r = lib.post(loc, titleValues("title", "The Left Hand of Darkness", "isbn", "9780441478125",
		"subtitle", "50th anniversary edition", "authors", "Ursula K. Le Guin"))
	wantRedirect(t, r, loc)
	wantBody(t, lib.get(loc), "50th anniversary edition")
}

func TestTitleValidation(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	lib.addTitle("title", "Dune", "isbn", "9780441172719")

	tests := []struct {
		name string
		form url.Values
		want string
	}{
		{"missing title", titleValues("isbn", "9780441172719"), "A title is required."},
		{"bad isbn", titleValues("title", "X", "isbn", "978-0-441-17271-0"), "check digit does not match"},
		{"duplicate isbn", titleValues("title", "Dune", "isbn", "0441172717"), "already in the catalogue"},
		{"bad year", titleValues("title", "X", "year", "3000"), "Enter a year between 1450"},
		{"bad cover", titleValues("title", "X", "cover_url", "javascript:alert(1)"), "starting with https://"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := lib.post("/titles", tt.form)
			wantStatus(t, r, http.StatusUnprocessableEntity)
			wantBody(t, r, tt.want)
		})
	}
}

func TestMembersCannotEditCatalogue(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	loc := lib.addTitle("title", "Dune")
	m := env.member("Mo Member", "mo@example.com")

	wantStatus(t, m.get("/titles/new"), http.StatusForbidden)
	wantStatus(t, m.post("/titles", titleValues("title", "Sneaky")), http.StatusForbidden)
	wantStatus(t, m.post(loc+"/copies", titleValues("barcode", "B001")), http.StatusForbidden)

	// Anonymous visitors are sent to log in.
	r := env.client().get("/titles/new")
	wantRedirect(t, r, "/login?next=%2Ftitles%2Fnew")

	// Everyone can read the catalogue.
	wantBody(t, env.client().get("/catalogue"), "Dune")
	wantStatus(t, m.get(loc), http.StatusOK)
}

func TestCopies(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	loc := lib.addTitle("title", "Matilda")

	wantRedirect(t, lib.post(loc+"/copies", titleValues("barcode", " b-0001 ", "shelf", "Children D")), loc+"#copies")
	wantRedirect(t, lib.post(loc+"/copies", titleValues("barcode", "B-0002")), loc+"#copies")

	r := lib.post(loc+"/copies", titleValues("barcode", "b-0001"))
	wantStatus(t, r, http.StatusUnprocessableEntity)
	wantBody(t, r, "Another copy already has this barcode.")

	r = lib.get(loc)
	wantBody(t, r, "B-0001", "Children D", "2 of 2 on the shelf")

	c, err := env.store.CopyByBarcode(t.Context(), "B-0002")
	if err != nil {
		t.Fatal(err)
	}
	wantRedirect(t, lib.post("/copies/"+itoa(c.ID)+"/status", titleValues("status", "lost")), loc+"#copies")
	wantBody(t, lib.get(loc), "1 of 2 on the shelf", "Lost")
	wantBody(t, lib.get("/catalogue"), "1 available")
}

func TestCatalogueSearchAsYouType(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	lib.addTitle("title", "Matilda", "authors", "Roald Dahl")
	lib.addTitle("title", "Dune", "authors", "Frank Herbert")

	req, _ := http.NewRequest(http.MethodGet, env.srv.URL+"/catalogue?q=dahl", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "results")
	r := env.client().do(req)
	wantStatus(t, r, http.StatusOK)
	wantBody(t, r, "Matilda", "1 title matching")
	if strings.Contains(r.body, "Dune") || strings.Contains(r.body, "<form") {
		t.Errorf("unexpected fragment:\n%s", r.body)
	}
}

func TestBrowseBySubjectAndAuthor(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	lib.addTitle("title", "Matilda", "authors", "Roald Dahl", "subjects", "Children")
	lib.addTitle("title", "The BFG", "authors", "Roald Dahl", "subjects", "Children, Giants")
	lib.addTitle("title", "Dune", "authors", "Frank Herbert", "subjects", "Science fiction")

	r := env.client().get("/catalogue?subject=Giants")
	wantBody(t, r, "The BFG", "Subject: <strong>Giants</strong>", "1 title")
	if strings.Contains(r.body, "Matilda") {
		t.Error("subject filter let Matilda through")
	}
	r = env.client().get("/catalogue?author=Roald+Dahl")
	wantBody(t, r, "Matilda", "The BFG", "2 titles")
	// The subject list shows counts.
	wantBody(t, env.client().get("/catalogue"), `Children</a> <span class="muted">2</span>`)
}

func TestNavMarksCurrentPage(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	wantBody(t, lib.get("/members"), `<a href="/members" class="active" aria-current="page">Members</a>`,
		`<a href="/desk">Desk</a>`)
}

func TestDeleteTitle(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	loc := lib.addTitle("title", "Typo Titel", "authors", "Nobody Else")
	keep := lib.addTitle("title", "Matilda")
	wantStatus(t, lib.post(keep+"/copies", titleValues("barcode", "M-01")), http.StatusSeeOther)

	wantStatus(t, lib.post(keep+"/delete", nil), http.StatusConflict)
	wantRedirect(t, lib.post(loc+"/delete", nil), "/catalogue")
	wantStatus(t, lib.get(loc), http.StatusNotFound)
	if strings.Contains(lib.get("/catalogue?author=Nobody+Else").body, "Typo") {
		t.Error("deleted title still listed")
	}
}

func TestEditCopy(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	loc := lib.addTitle("title", "Matilda")
	lib.post(loc+"/copies", titleValues("barcode", "M-01", "shelf", "Children"))
	c, _ := env.store.CopyByBarcode(t.Context(), "M-01")
	wantRedirect(t, lib.post("/copies/"+itoa(c.ID), titleValues("shelf", "Children D", "note", "Spine taped")), loc+"#copies")
	wantBody(t, lib.get(loc), "Children D · Spine taped")
}
