package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

type circ struct {
	env    *testEnv
	lib    *client
	member *client
	mo     store.User
	title  string
}

// newCirc sets up a librarian, a member called Mo and a title with two
// copies, C01 and C02.
func newCirc(t *testing.T) circ {
	env := newTestEnv(t)
	lib := env.librarian()
	m := env.member("Mo Member", "mo@example.com")
	mo, err := env.store.UserByEmail(t.Context(), "mo@example.com")
	if err != nil {
		t.Fatal(err)
	}
	loc := lib.addTitle("title", "Dune")
	for _, b := range []string{"C01", "C02"} {
		wantStatus(t, lib.post(loc+"/copies", titleValues("barcode", b)), http.StatusSeeOther)
	}
	return circ{env: env, lib: lib, member: m, mo: mo, title: loc}
}

func (c circ) checkout(barcode string) response {
	return c.lib.post("/desk/checkout", titleValues("card", c.mo.CardNumber, "barcode", barcode))
}

func TestDeskCheckoutAndReturn(t *testing.T) {
	c := newCirc(t)

	r := c.checkout("c01")
	wantStatus(t, r, http.StatusOK)
	wantBody(t, r, "Dune checked out to Mo Member.", "Due Tue 31 Mar")

	r = c.checkout("C01")
	wantStatus(t, r, http.StatusUnprocessableEntity)
	wantBody(t, r, "already checked out")

	wantBody(t, c.member.get("/account"), "Dune", "C01", "Tue 31 Mar")
	wantBody(t, c.lib.get(c.title), "1 of 2 on the shelf")

	r = c.lib.post("/desk/return", titleValues("barcode", "C01"))
	wantStatus(t, r, http.StatusOK)
	wantBody(t, r, "Dune checked in from Mo Member.")
	if strings.Contains(r.body, "late") {
		t.Error("on-time return reported as late")
	}

	r = c.lib.post("/desk/return", titleValues("barcode", "C01"))
	wantBody(t, r, "wasn&#39;t checked out")
	wantBody(t, c.member.get("/account"), "Recently returned", "nothing out")
}

func TestDeskErrors(t *testing.T) {
	c := newCirc(t)
	tests := []struct {
		name, card, barcode, want string
	}{
		{"bad card", "1234", "C01", "doesn&#39;t look right"},
		{"unknown card", domain.CardNumber(999), "C01", "No member has card"},
		{"unknown copy", c.mo.CardNumber, "C99", "No copy has barcode C99"},
		{"no barcode", c.mo.CardNumber, "", "Scan or type the barcode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := c.lib.post("/desk/checkout", titleValues("card", tt.card, "barcode", tt.barcode))
			wantStatus(t, r, http.StatusUnprocessableEntity)
			wantBody(t, r, tt.want)
		})
	}
	wantStatus(t, c.member.post("/desk/checkout", titleValues("card", c.mo.CardNumber, "barcode", "C01")), http.StatusForbidden)
}

func TestDeskLateReturn(t *testing.T) {
	c := newCirc(t)
	c.checkout("C01")
	c.env.now = c.env.now.AddDate(0, 0, 24) // due after 21 days
	r := c.lib.post("/desk/return", titleValues("barcode", "C01"))
	wantBody(t, r, "It was 3 days late.")
}

func TestDeskHTMXFragment(t *testing.T) {
	c := newCirc(t)
	req, _ := http.NewRequest(http.MethodPost, c.env.srv.URL+"/desk/checkout",
		strings.NewReader(titleValues("card", c.mo.CardNumber, "barcode", "C99", "csrf_token", c.lib.csrf()).Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	r := c.lib.do(req)
	wantStatus(t, r, http.StatusOK)
	wantBody(t, r, "No copy has barcode C99")
	if strings.Contains(r.body, "<html") {
		t.Error("htmx request got a whole page")
	}
}

func TestRenewals(t *testing.T) {
	c := newCirc(t)
	c.checkout("C01")
	loans, _ := c.env.store.ActiveLoans(t.Context(), c.mo.ID)
	path := "/loans/" + itoa(loans[0].ID) + "/renew"

	// Another member can't renew Mo's loan.
	other := c.env.member("Other", "other@example.com")
	wantStatus(t, other.post(path, nil), http.StatusNotFound)

	wantStatus(t, c.member.post(path, nil), http.StatusSeeOther)
	wantStatus(t, c.lib.post(path, nil), http.StatusSeeOther)
	r := c.member.post(path, nil)
	wantStatus(t, r, http.StatusConflict)
	wantBody(t, r, "maximum number of times")
	wantBody(t, c.member.get("/account"), "Renewed the maximum number of times")
}
