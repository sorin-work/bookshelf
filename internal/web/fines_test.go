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
