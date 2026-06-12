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

	wantBody(t, c.lib.get("/dashboard"), "Nothing is due back in the next three days.")
	c.env.now = c.env.now.AddDate(0, 0, 19)
	wantBody(t, c.lib.get("/dashboard"), "<td>Tue 31 Mar</td><td><a href=\"/titles/1\">Dune</a></td>")
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

func TestMonthlyReport(t *testing.T) {
	c := newCirc(t)
	c.checkout("C01")
	c.env.now = c.env.now.AddDate(0, 1, 0)
	c.lib.post("/desk/return", titleValues("barcode", "C01"))
	c.checkout("C02")

	r := c.lib.get("/report")
	wantStatus(t, r, http.StatusOK)
	wantBody(t, r,
		"<td>Mar 2026</td>\n      <td>1</td>",
		"<td>Apr 2026</td>\n      <td>1</td>",
		"<td>May 2025</td>",
		"<tr><th>Total</th><th>2</th><th></th><th>1</th><th>1</th></tr>",
	)
	wantStatus(t, c.member.get("/report"), http.StatusForbidden)
}
