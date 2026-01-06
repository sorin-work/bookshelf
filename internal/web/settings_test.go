package web

import (
	"net/http"
	"net/url"
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
		"pickup_days": {"5"}, "max_holds": {"2"}}
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

	tests := []struct {
		name string
		form url.Values
		want string
	}{
		{"not a number", settingsForm("loan_days", "two weeks"), "Enter a whole number."},
		{"out of range", settingsForm("loan_days", "0"), "Loan period must be between 1 and 365 days."},
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
		"pickup_days": {"7"}, "max_holds": {"5"}}
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
