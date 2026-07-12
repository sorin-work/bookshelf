package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSettings(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	m := env.member("Mo", "mo@example.com")

	wantStatus(t, m.get("/settings"), http.StatusForbidden)
	wantBody(t, lib.get("/settings"), `value="21"`)

	form := url.Values{"loan_days": {"14"}, "max_renewals": {"1"}, "max_loans": {"3"}, "closed": {"0", "6"},
		"pickup_days": {"5"}, "max_holds": {"2"}, "grace_days": {"1"},
		"fine_per_day": {"0.10"}, "fine_cap": {"3"}, "block_at": {"7,50"}, "currency": {"£"}}
	wantRedirect(t, lib.post("/settings", form), "/settings")
	wantBody(t, lib.get("/settings"), "Saved. New checkouts")
	p, err := env.store.Policy(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if p.LoanDays != 14 || p.MaxRenewals != 1 || p.MaxLoans != 3 || len(p.ClosedDays) != 2 || p.ClosedDays[1] != time.Saturday ||
		p.HoldPickupDays != 5 || p.MaxHolds != 2 {
		t.Errorf("saved %+v", p)
	}
	if p.FinePerDay != 10 || p.FineCap != 300 || p.BlockAt != 750 || p.FineGraceDays != 1 || p.Currency != "£" {
		t.Errorf("saved %+v", p)
	}

	tests := []struct {
		name string
		form url.Values
		want string
	}{
		{"not a number", settingsForm("loan_days", "two weeks"), "Enter a whole number."},
		{"out of range", settingsForm("loan_days", "0"), "Loan period must be between 1 and 365 days."},
		{"bad money", settingsForm("fine_per_day", "ten cents"), "Enter an amount, like 0.25."},
		{"pickup too long", settingsForm("pickup_days", "90"), "Hold pickup time must be between 1 and 60 days"},
		{"always closed", settingsForm("closed", "0", "closed", "1", "closed", "2", "closed", "3",
			"closed", "4", "closed", "5", "closed", "6"), "open at least one day"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := lib.post("/settings", tt.form)
			wantStatus(t, r, http.StatusUnprocessableEntity)
			wantBody(t, r, tt.want)
		})
	}
}

// settingsForm is a valid settings form with the given fields replaced.
func settingsForm(kv ...string) url.Values {
	v := url.Values{"loan_days": {"14"}, "max_renewals": {"1"}, "max_loans": {"3"},
		"pickup_days": {"7"}, "max_holds": {"5"}, "grace_days": {"0"},
		"fine_per_day": {"0.25"}, "fine_cap": {"5"}, "block_at": {"10"}, "currency": {"€"}}
	replaced := map[string]bool{}
	for i := 0; i+1 < len(kv); i += 2 {
		if !replaced[kv[i]] {
			v.Del(kv[i])
			replaced[kv[i]] = true
		}
		v.Add(kv[i], kv[i+1])
	}
	return v
}

func TestParseClosedDates(t *testing.T) {
	today := testDate(t, "2026-03-10")
	got, err := parseClosedDates("2026-12-25\n\n2026-03-01\n 2026-04-03 to 2026-04-06 \n2026-12-25", today)
	if err != nil {
		t.Fatal(err)
	}
	var s []string
	for _, d := range got {
		s = append(s, d.String())
	}
	want := "2026-04-03 2026-04-04 2026-04-05 2026-04-06 2026-12-25"
	if strings.Join(s, " ") != want {
		t.Errorf("got %v, want %s", s, want)
	}
	for _, bad := range []string{"25/12/2026", "2026-04-06 to 2026-04-03", "2026-01-01 to 2026-12-31"} {
		if _, err := parseClosedDates(bad, testDate(t, "2026-01-01")); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

func TestClosureDatesMoveDueDates(t *testing.T) {
	c := newCirc(t)
	wantRedirect(t, c.lib.post("/settings", settingsForm("loan_days", "21", "closed", "0", "closed_dates", "2026-03-31 to 2026-04-01")), "/settings")
	// Due on Tuesday 31 March, which is now closed along with the 1st.
	wantBody(t, c.checkout("C01"), "Due Thu 2 Apr")
	wantBody(t, c.lib.get("/settings"), "2026-03-31\n2026-04-01")
}
