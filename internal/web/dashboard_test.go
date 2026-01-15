package web

import (
	"net/http"
	"testing"
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
