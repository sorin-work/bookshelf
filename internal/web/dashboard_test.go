package web

import (
	"net/http"
	"testing"
	"time"
)

func TestDashboard(t *testing.T) {
	c := newCirc(t)
	wantRedirect(t, c.lib.get("/"), "/dashboard")

	c.checkout("C01")
	c.checkout("C02")
	c.lib.post("/desk/return", titleValues("barcode", "C02"))

	r := c.lib.get("/dashboard")
	wantStatus(t, r, http.StatusOK)
	wantBody(t, r,
		`<span class="stat-value">2</span><span class="stat-label">checked out today</span>`,
		`<span class="stat-value">1</span><span class="stat-label">returned today</span>`,
		`<span class="stat-value">1</span><span class="stat-label">on loan</span>`,
		"Mo Member</a> returned",
		"2 loans",
	)
	wantStatus(t, c.member.get("/dashboard"), http.StatusForbidden)
}

func TestTimesShownInLibraryTimeZone(t *testing.T) {
	bucharest, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		t.Skip("no tz database")
	}
	c := newCircIn(t, bucharest)
	c.env.now = time.Date(2026, time.March, 10, 22, 30, 0, 0, time.UTC)
	c.checkout("C01")
	wantBody(t, c.lib.get("/dashboard"), "11 Mar 00:30")
	wantBody(t, c.lib.get("/members/"+itoa(c.mo.ID)), "<dd>10 Mar 2026</dd>")
}
