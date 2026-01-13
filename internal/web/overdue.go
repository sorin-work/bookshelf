package web

import (
	"net/http"

	"github.com/sorin-work/bookshelf/internal/store"
)

type overdueLine struct {
	store.OverdueLoan
	DaysOverdue int
	// Fine is what the loan would cost if it came back today.
	Fine int
}

type overduePage struct {
	Loans    []overdueLine
	Currency string
	Total    int
}

func (s *Server) overdue(w http.ResponseWriter, r *http.Request) {
	today := s.today()
	loans, err := s.store.OverdueLoans(r.Context(), today)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	policy, err := s.store.Policy(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	d := overduePage{Currency: policy.Currency}
	for _, l := range loans {
		line := overdueLine{
			OverdueLoan: l,
			DaysOverdue: l.Rules().DaysOverdue(today),
			Fine:        policy.LateFine(l.Due, today),
		}
		d.Total += line.Fine
		d.Loans = append(d.Loans, line)
	}
	s.render(w, r, http.StatusOK, "overdue.html", page{Title: "Overdue", Data: d})
}
