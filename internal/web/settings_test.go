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

	form := url.Values{"loan_days": {"14"}, "max_renewals": {"1"}, "max_loans": {"3"}, "closed": {"0", "6"}}
	wantRedirect(t, lib.post("/settings", form), "/settings?saved=1")
	p, err := env.store.Policy(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if p.LoanDays != 14 || p.MaxRenewals != 1 || p.MaxLoans != 3 || len(p.ClosedDays) != 2 || p.ClosedDays[1] != time.Saturday {
		t.Errorf("saved %+v", p)
	}

	tests := []struct {
		name string
		form url.Values
		want string
	}{
		{"not a number", url.Values{"loan_days": {"two weeks"}, "max_renewals": {"1"}, "max_loans": {"3"}}, "Enter a whole number."},
		{"out of range", url.Values{"loan_days": {"0"}, "max_renewals": {"1"}, "max_loans": {"3"}}, "Loan period must be between 1 and 365 days."},
		{"always closed", url.Values{"loan_days": {"14"}, "max_renewals": {"1"}, "max_loans": {"3"},
			"closed": {"0", "1", "2", "3", "4", "5", "6"}}, "open at least one day"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := lib.post("/settings", tt.form)
			wantStatus(t, r, http.StatusUnprocessableEntity)
			wantBody(t, r, tt.want)
		})
	}
}
