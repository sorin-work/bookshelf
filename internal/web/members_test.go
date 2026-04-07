package web

import (
	"net/http"
	"testing"

	"github.com/sorin-work/bookshelf/internal/domain"
)

func TestRegisterMemberAtDesk(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()

	r := lib.post("/members", titleValues("name", "Ioana Marin", "email", "ioana@example.com", "phone", "0722 000 111"))
	wantStatus(t, r, http.StatusSeeOther)
	m, err := env.store.UserByEmail(t.Context(), "ioana@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if m.CanLogIn() || m.IsLibrarian() {
		t.Errorf("unexpected member: %+v", m)
	}
	if _, err := domain.NormalizeCardNumber(m.CardNumber); err != nil {
		t.Errorf("card %q: %v", m.CardNumber, err)
	}
	if want := domain.DateOf(testNow).AddYears(1); m.CardExpires != want {
		t.Errorf("card expires %s, want %s", m.CardExpires, want)
	}

	page := lib.get(r.location)
	wantBody(t, page, "Ioana Marin", m.FormattedCard(), "No password set")

	// Find by part of the card number, typed with spaces.
	wantBody(t, lib.get("/members?q="+m.CardNumber[4:8]), "Ioana Marin")
	wantBody(t, lib.get("/members?q=marin"), "Ioana Marin")

	r = lib.post("/members", titleValues("name", "Other", "email", "IOANA@example.com"))
	wantStatus(t, r, http.StatusUnprocessableEntity)
	wantBody(t, r, "already registered")
}

func TestRenewCard(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	env.member("Mo", "mo@example.com")
	m, _ := env.store.UserByEmail(t.Context(), "mo@example.com")

	// An expired card is renewed from today.
	if err := env.store.RenewCard(t.Context(), m.ID, domain.DateOf(testNow).AddDays(-10)); err != nil {
		t.Fatal(err)
	}
	wantBody(t, lib.get("/members/"+itoa(m.ID)), "Expired")
	wantStatus(t, lib.post("/members/"+itoa(m.ID)+"/renew-card", nil), http.StatusSeeOther)
	m, _ = env.store.UserByID(t.Context(), m.ID)
	if want := domain.DateOf(testNow).AddYears(1); m.CardExpires != want {
		t.Errorf("expired card renewed to %s, want %s", m.CardExpires, want)
	}

	// A valid card gets a year added to its current expiry.
	wantStatus(t, lib.post("/members/"+itoa(m.ID)+"/renew-card", nil), http.StatusSeeOther)
	m, _ = env.store.UserByID(t.Context(), m.ID)
	if want := domain.DateOf(testNow).AddYears(2); m.CardExpires != want {
		t.Errorf("valid card renewed to %s, want %s", m.CardExpires, want)
	}
}

func TestRoles(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	m := env.member("Mo", "mo@example.com")
	mo, _ := env.store.UserByEmail(t.Context(), "mo@example.com")
	lena, _ := env.store.UserByEmail(t.Context(), "lena@example.com")

	wantStatus(t, m.get("/members"), http.StatusForbidden)

	// The only librarian can't step down.
	r := lib.post("/members/"+itoa(lena.ID)+"/role", titleValues("role", "member"))
	wantStatus(t, r, http.StatusConflict)

	wantStatus(t, lib.post("/members/"+itoa(mo.ID)+"/role", titleValues("role", "librarian")), http.StatusSeeOther)
	wantStatus(t, m.get("/members"), http.StatusOK)
	wantStatus(t, m.post("/members/"+itoa(lena.ID)+"/role", titleValues("role", "member")), http.StatusSeeOther)
	wantStatus(t, lib.get("/members"), http.StatusForbidden)
}

func TestEditMember(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	env.member("Mo", "mo@example.com")
	env.member("Ana", "ana@example.com")
	mo, _ := env.store.UserByEmail(t.Context(), "mo@example.com")
	path := "/members/" + itoa(mo.ID)

	wantRedirect(t, lib.post(path, titleValues("name", "Mo Salah", "email", "mo.s@example.com", "notes", "Year 5, Mrs Pop's class")), path)
	wantBody(t, lib.get(path), "Mo Salah", "mo.s@example.com", "Year 5, Mrs Pop&#39;s class")

	r := lib.post(path, titleValues("name", "Mo", "email", "ana@example.com"))
	wantStatus(t, r, http.StatusUnprocessableEntity)
	wantBody(t, r, "Someone else is registered")

	wantStatus(t, lib.get("/members/9999"), http.StatusNotFound)
}
