package web

import (
	"net/http"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
)

type rulesPage struct {
	Policy      domain.Policy
	ClosedDays  []string
	ClosedDates []domain.Date
}

// rules explains the lending rules to members in plain words, using the
// library's actual settings.
func (s *Server) rules(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.Policy(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	d := rulesPage{Policy: p}
	for i := 1; i <= 7; i++ {
		wd := time.Weekday(i % 7)
		for _, c := range p.ClosedDays {
			if c == wd {
				d.ClosedDays = append(d.ClosedDays, wd.String()+"s")
			}
		}
	}
	today := s.today()
	for _, c := range p.ClosedDates {
		if !c.Before(today) && len(d.ClosedDates) < 10 {
			d.ClosedDates = append(d.ClosedDates, c)
		}
	}
	s.render(w, r, http.StatusOK, "rules.html", page{Title: "How borrowing works", Data: d})
}
