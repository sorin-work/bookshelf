package web

import (
	"net/http"
	"testing"
)

func TestHoldFlow(t *testing.T) {
	c := newCirc(t)
	ana := c.env.member("Ana Reader", "ana@example.com")

	// While a copy is on the shelf there is no hold button.
	r := ana.get(c.title)
	if want := "Place a hold"; contains(r.body, want) {
		t.Errorf("offered a hold with copies on the shelf")
	}
	wantStatus(t, ana.post(c.title+"/hold", nil), http.StatusConflict)

	c.checkout("C01")
	c.checkout("C02")
	wantBody(t, ana.get(c.title), "Place a hold", "All copies are out", "probably within 3 weeks")

	r = ana.post(c.title+"/hold", nil)
	wantRedirect(t, r, c.title)
	wantBody(t, ana.get(c.title), "number 1 in the queue, which usually means about 3 weeks")
	wantBody(t, ana.get("/account"), "Number 1 in the queue")
	wantStatus(t, ana.post(c.title+"/hold", nil), http.StatusConflict)

	// Mo can't renew while Ana waits.
	wantBody(t, c.member.get("/account"), "Someone else is waiting for it")

	// The returned copy is set aside for Ana.
	r = c.lib.post("/desk/return", titleValues("barcode", "C02"))
	wantBody(t, r, "Put this copy on the hold shelf", "Ana Reader")
	wantBody(t, ana.get("/account"), "Ready at the desk")
	wantBody(t, c.lib.get("/holds"), "C02", "Ana Reader")
	// Scanning it again by mistake doesn't send it back to the shelf.
	wantBody(t, c.lib.post("/desk/return", titleValues("barcode", "C02")), "waiting on the hold shelf for Ana Reader")

	// Mo can't borrow it again; Ana can.
	r = c.checkout("C02")
	wantStatus(t, r, http.StatusUnprocessableEntity)
	wantBody(t, r, "on the hold shelf for another member")

	anaUser, _ := c.env.store.UserByEmail(t.Context(), "ana@example.com")
	wantBody(t, c.lib.get("/members/"+itoa(anaUser.ID)), "Ready: copy C02")
	r = c.lib.post("/desk/checkout", titleValues("card", anaUser.CardNumber, "barcode", "C02"))
	wantStatus(t, r, http.StatusOK)
	wantBody(t, ana.get("/account"), "No holds.")
}

func TestCancelHold(t *testing.T) {
	c := newCirc(t)
	c.checkout("C01")
	c.checkout("C02")
	ana := c.env.member("Ana Reader", "ana@example.com")
	ana.post(c.title+"/hold", nil)
	anaUser, _ := c.env.store.UserByEmail(t.Context(), "ana@example.com")
	holds, _ := c.env.store.OpenHolds(t.Context(), anaUser.ID)
	path := "/holds/" + itoa(holds[0].ID) + "/cancel"

	other := c.env.member("Other", "other@example.com")
	wantStatus(t, other.post(path, nil), http.StatusNotFound)

	wantRedirect(t, ana.post(path, nil), "/account")
	wantBody(t, ana.get("/account"), "No holds.")
	wantStatus(t, ana.post(path, nil), http.StatusConflict)
}

func TestHoldsNeedLogin(t *testing.T) {
	c := newCirc(t)
	c.checkout("C01")
	c.checkout("C02")
	anon := c.env.client()
	wantBody(t, anon.get(c.title), "to place a hold")
	r := anon.post(c.title+"/hold", nil)
	if r.status != http.StatusSeeOther || !contains(r.location, "/login") {
		t.Errorf("anonymous hold: %d %s", r.status, r.location)
	}
}

// htmx buttons outside a form send the CSRF token in a header set on the
// body, not as a form field.
func TestCancelHoldWithHTMXHeader(t *testing.T) {
	c := newCirc(t)
	c.checkout("C01")
	c.checkout("C02")
	ana := c.env.member("Ana Reader", "ana@example.com")
	ana.post(c.title+"/hold", nil)
	anaUser, _ := c.env.store.UserByEmail(t.Context(), "ana@example.com")
	holds, _ := c.env.store.OpenHolds(t.Context(), anaUser.ID)

	page := ana.get("/account")
	wantBody(t, page, `hx-headers='{"X-CSRF-Token": "`+ana.csrf()+`"}'`)

	req, _ := http.NewRequest(http.MethodPost, c.env.srv.URL+"/holds/"+itoa(holds[0].ID)+"/cancel", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("X-CSRF-Token", ana.csrf())
	r := ana.do(req)
	wantStatus(t, r, http.StatusOK)
	if r.body != "" {
		t.Errorf("expected an empty body, got %q", r.body)
	}
	wantBody(t, ana.get("/account"), "No holds.")
}

func TestPlaceHoldForMember(t *testing.T) {
	c := newCirc(t)
	c.checkout("C01")
	c.checkout("C02")
	c.env.member("Ana Reader", "ana@example.com")
	ana, _ := c.env.store.UserByEmail(t.Context(), "ana@example.com")

	wantBody(t, c.lib.get(c.title), "Place a hold for a member")
	wantStatus(t, c.lib.post(c.title+"/hold-for", titleValues("card", "123")), http.StatusUnprocessableEntity)
	wantRedirect(t, c.lib.post(c.title+"/hold-for", titleValues("card", ana.CardNumber)), c.title+"#queue")
	wantBody(t, c.lib.get(c.title), "Hold placed for Ana Reader, number 1 in the queue.")

	r := c.lib.post(c.title+"/hold-for", titleValues("card", ana.CardNumber))
	wantStatus(t, r, http.StatusConflict)
	wantBody(t, r, "Ana Reader already has a hold")
	// Mo has it on loan.
	wantBody(t, c.lib.post(c.title+"/hold-for", titleValues("card", c.mo.CardNumber)), "Mo Member already has this title")
	wantStatus(t, c.member.post(c.title+"/hold-for", titleValues("card", ana.CardNumber)), http.StatusForbidden)
}

func TestWithdrawingLastCopyCancelsHolds(t *testing.T) {
	c := newCirc(t)
	c2, _ := c.env.store.CopyByBarcode(t.Context(), "C02")
	c.lib.post("/copies/"+itoa(c2.ID)+"/status", titleValues("status", "withdrawn"))
	c.checkout("C01")
	ana := c.env.member("Ana Reader", "ana@example.com")
	wantRedirect(t, ana.post(c.title+"/hold", nil), c.title)

	// Mo loses the only copy; Ana is still waiting for it.
	loans, _ := c.env.store.ActiveLoans(t.Context(), c.mo.ID)
	c.lib.post("/loans/"+itoa(loans[0].ID)+"/lost", nil)
	if q, _ := c.env.store.HoldQueue(t.Context(), 1); len(q) != 1 {
		t.Fatalf("queue %+v", q)
	}

	// Once it's written off for good, nobody can get a copy any more.
	c1, _ := c.env.store.CopyByBarcode(t.Context(), "C01")
	wantRedirect(t, c.lib.post("/copies/"+itoa(c1.ID)+"/status", titleValues("status", "withdrawn")), c.title+"#copies")
	wantBody(t, c.lib.get(c.title), "That was the last copy, so 1 waiting hold was cancelled.")
	wantBody(t, ana.get("/account"), "No holds.")
}
