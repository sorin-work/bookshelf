package web

import (
	"net/http"
	"time"

	"github.com/sorin-work/bookshelf/internal/store"
)

type reportRow struct {
	store.MonthCount
	Label string
	// Bar is the checkouts as a percentage of the busiest month.
	Bar int
}

type reportPage struct {
	Months []reportRow
	Total  store.MonthCount
}

// report shows the last twelve months of circulation, for the yearly
// report to the parish council or the school.
func (s *Server) report(w http.ResponseWriter, r *http.Request) {
	now := s.now().In(s.loc)
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.loc)
	first := thisMonth.AddDate(0, -11, 0)
	_, offset := now.Zone()

	counts, err := s.store.MonthlyCirculation(r.Context(), first.Format("2006-01"), thisMonth.Format("2006-01"),
		time.Duration(offset)*time.Second)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	byMonth := map[string]store.MonthCount{}
	for _, c := range counts {
		byMonth[c.Month] = c
	}

	var d reportPage
	busiest := 0
	for m := first; !m.After(thisMonth); m = m.AddDate(0, 1, 0) {
		key := m.Format("2006-01")
		c := byMonth[key]
		c.Month = key
		d.Months = append(d.Months, reportRow{MonthCount: c, Label: m.Format("Jan 2006")})
		d.Total.Checkouts += c.Checkouts
		d.Total.Returns += c.Returns
		d.Total.NewUsers += c.NewUsers
		busiest = max(busiest, c.Checkouts)
	}
	if busiest > 0 {
		for i := range d.Months {
			d.Months[i].Bar = d.Months[i].Checkouts * 100 / busiest
		}
	}
	s.render(w, r, http.StatusOK, "report.html", page{Title: "Monthly report", Data: d})
}
