package web

import (
	"net/http"

	"github.com/sorin-work/bookshelf/internal/store"
)

type accountPage struct {
	CardExpired bool
	Loans       []loanLine
	History     []store.LoanRow
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
	if d.History, err = s.store.LoanHistory(r.Context(), u.ID, 10); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "account.html", page{Title: "My account", Data: d})
}
