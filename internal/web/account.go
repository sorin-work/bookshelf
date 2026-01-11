package web

import (
	"net/http"

	"github.com/sorin-work/bookshelf/internal/store"
)

type accountPage struct {
	CardExpired bool
	Loans       []loanLine
	Holds       []store.HoldRow
	History     []store.LoanRow
	Fines       []store.Fine
	Owed        int
	Currency    string
}

// account is the member's own page: their card, what they have out and
// what they returned recently.
func (s *Server) account(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	d := accountPage{CardExpired: u.CardExpired(s.today())}
	active, err := s.store.ActiveLoans(r.Context(), u.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if d.Loans, err = s.loanLines(r, active); err != nil {
		s.serverError(w, r, err)
		return
	}
	if d.Holds, err = s.store.OpenHolds(r.Context(), u.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	if d.History, err = s.store.LoanHistory(r.Context(), u.ID, 10); err != nil {
		s.serverError(w, r, err)
		return
	}
	if d.Fines, err = s.store.Fines(r.Context(), u.ID, 10); err != nil {
		s.serverError(w, r, err)
		return
	}
	if d.Owed, err = s.store.FinesOwed(r.Context(), u.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	policy, err := s.store.Policy(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	d.Currency = policy.Currency
	s.render(w, r, http.StatusOK, "account.html", page{Title: "My account", Data: d})
}
