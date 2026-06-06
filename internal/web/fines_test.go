package web

import (
	"net/http"
	"testing"
)

func TestFinesAtTheDesk(t *testing.T) {
	c := newCirc(t)
	c.checkout("C01")
	c.env.now = c.env.now.AddDate(0, 3, 0) // months late: the cap applies

	r := c.lib.post("/desk/return", titleValues("barcode", "C01"))
	wantBody(t, r, "A late fine of €5.00 was added")

	member := "/members/" + itoa(c.mo.ID)
	wantBody(t, c.lib.get(member), "Owes <strong>€5.00</strong>", "Late return: Dune")
	wantBody(t, c.member.get("/account"), "You owe <strong>€5.00</strong>")

	// Another 6.00 by hand takes Mo over the 10.00 limit.
	wantRedirect(t, c.lib.post(member+"/charges", titleValues("amount", "6", "note", "Torn cover")), member+"#fines")
	wantBody(t, c.lib.get(member), "€11.00", "can't borrow until they pay", "Torn cover")
	r = c.checkout("C02")
	wantStatus(t, r, http.StatusUnprocessableEntity)
	wantBody(t, r, "unpaid fines over the borrowing limit")

	fines, _ := c.env.store.Fines(t.Context(), c.mo.ID, 10)
	for _, f := range fines {
		wantStatus(t, c.lib.post("/fines/"+itoa(f.ID)+"/resolve", titleValues("status", "paid")), http.StatusSeeOther)
	}
	wantBody(t, c.lib.get(member), "Nothing owed.")
	wantStatus(t, c.checkout("C02"), http.StatusOK)

	wantStatus(t, c.lib.post("/fines/"+itoa(fines[0].ID)+"/resolve", titleValues("status", "waived")), http.StatusConflict)
	wantStatus(t, c.member.post("/fines/"+itoa(fines[0].ID)+"/resolve", titleValues("status", "waived")), http.StatusForbidden)
}

func TestAddChargeValidation(t *testing.T) {
	c := newCirc(t)
	member := "/members/" + itoa(c.mo.ID)
	tests := []struct {
		amount, note string
	}{
		{"abc", "Torn"},
		{"0", "Torn"},
		{"2", ""},
	}
	for _, tt := range tests {
		r := c.lib.post(member+"/charges", titleValues("amount", tt.amount, "note", tt.note))
		wantStatus(t, r, http.StatusUnprocessableEntity)
	}
}

func TestOverdueList(t *testing.T) {
	c := newCirc(t)
	c.checkout("C01")
	wantBody(t, c.lib.get("/overdue"), "Nothing is overdue")

	c.env.now = c.env.now.AddDate(0, 0, 22) // one day late
	r := c.lib.get("/overdue")
	wantBody(t, r, "Dune", "Mo Member", "mo@example.com", "1 day", "€0.25")
	wantStatus(t, c.member.get("/overdue"), http.StatusForbidden)

	r = c.lib.get("/overdue.csv")
	wantStatus(t, r, http.StatusOK)
	if ct := r.header.Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	wantBody(t, r, "member,email,phone,card,title,barcode,due,days_overdue,fine\n",
		"Mo Member,mo@example.com,,"+c.mo.CardNumber+",Dune,C01,2026-03-31,1,0.25\n")
}

func TestCSVSafe(t *testing.T) {
	for in, want := range map[string]string{"Dune": "Dune", "=HYPERLINK()": "'=HYPERLINK()", "+40 722": "'+40 722", "": ""} {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReminderURL(t *testing.T) {
	l := overdueLine{DaysOverdue: 3, Fine: 75}
	l.MemberName, l.MemberEmail, l.Title, l.Barcode = "Mo", "mo@example.com", "Dune & Co", "C01"
	l.Due = testDate(t, "2026-03-31")
	got := l.ReminderURL("€")
	for _, want := range []string{
		"mailto:mo@example.com?",
		"subject=Overdue%20library%20book%3A%20Dune%20%26%20Co",
		"is%20now%203%20days%20late",
		"%E2%82%AC0.75",
	} {
		if !contains(got, want) {
			t.Errorf("%s\ndoes not contain %s", got, want)
		}
	}
}

func TestDeclareLostAtTheDesk(t *testing.T) {
	c := newCirc(t)
	c.checkout("C01")
	loans, _ := c.env.store.ActiveLoans(t.Context(), c.mo.ID)
	path := "/loans/" + itoa(loans[0].ID) + "/lost"
	member := "/members/" + itoa(c.mo.ID)

	wantStatus(t, c.member.post(path, nil), http.StatusForbidden)
	wantStatus(t, c.lib.post(path, titleValues("charge", "twelve")), http.StatusUnprocessableEntity)
	wantRedirect(t, c.lib.post(path, titleValues("charge", "12.50")), member)
	wantBody(t, c.lib.get(member), "is marked lost", "Replacement for lost copy C01", "€12.50", "Reported lost")
	wantStatus(t, c.lib.post(path, nil), http.StatusConflict)
	wantBody(t, c.lib.get(c.title), "1 of 1 on the shelf")
}
