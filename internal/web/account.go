package web

import (
	"net/http"
	"strconv"

	"github.com/sorin-work/bookshelf/internal/store"
)

type accountPage struct {
	CardExpired bool
	// CardExpiresSoon is set in the month before the card runs out.
	CardExpiresSoon bool
	Loans           []loanLine
	Holds           []store.HoldRow
	History         []store.LoanRow
	Fines           []store.Fine
	Owed            int
	Currency        string
}

// account is the member's own page: their card, what they have out and
// what they returned recently.
func (s *Server) account(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	d := accountPage{CardExpired: u.CardExpired(s.today())}
	d.CardExpiresSoon = !d.CardExpired && !u.CardExpires.IsZero() && u.CardExpires.Before(s.today().AddDays(31))
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

const historyPageSize = 25

type historyPage struct {
	Loans   []store.LoanRow
	Total   int
	PrevURL string
	NextURL string
}

// history lists everything the member has borrowed and returned.
func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	pageNum, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if pageNum < 1 {
		pageNum = 1
	}
	loans, total, err := s.store.LoanHistoryPage(r.Context(), u.ID, historyPageSize, (pageNum-1)*historyPageSize)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	d := historyPage{Loans: loans, Total: total}
	if pageNum > 1 {
		d.PrevURL = "/account/history?page=" + strconv.Itoa(pageNum-1)
	}
	if pageNum*historyPageSize < total {
		d.NextURL = "/account/history?page=" + strconv.Itoa(pageNum+1)
	}
	s.render(w, r, http.StatusOK, "history.html", page{Title: "Reading history", Data: d})
}
