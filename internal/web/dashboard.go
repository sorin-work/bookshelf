package web

import (
	"net/http"
	"time"

	"github.com/sorin-work/bookshelf/internal/store"
)

type dashboardPage struct {
	Stats    store.Stats
	Popular  []store.PopularTitle
	Activity []store.Activity
	DueSoon  []store.LoanRow
	Currency string
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	today := s.today()
	start := time.Date(today.Year, today.Month, today.Day, 0, 0, 0, 0, s.loc)
	end := start.AddDate(0, 0, 1)

	stats, err := s.store.DashboardStats(r.Context(), today, start, end)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	popular, err := s.store.PopularTitles(r.Context(), s.now().AddDate(0, 0, -30), 5)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	activity, err := s.store.RecentActivity(r.Context(), 10)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	dueSoon, err := s.store.DueBetween(r.Context(), today, today.AddDays(3))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	policy, err := s.store.Policy(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "dashboard.html", page{Title: "Dashboard", Data: dashboardPage{
		Stats: stats, Popular: popular, Activity: activity, DueSoon: dueSoon, Currency: policy.Currency,
	}})
}
