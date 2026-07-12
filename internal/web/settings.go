package web

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
)

type weekdayOption struct {
	Day    int
	Name   string
	Closed bool
}

type settingsPage struct {
	Values   map[string]string
	Weekdays []weekdayOption
	Errors   formErrors
}

// policyFields maps form fields to the integer policy settings they edit.
func policyFields(p *domain.Policy) map[string]*int {
	return map[string]*int{
		"loan_days":    &p.LoanDays,
		"max_renewals": &p.MaxRenewals,
		"max_loans":    &p.MaxLoans,
		"pickup_days":  &p.HoldPickupDays,
		"max_holds":    &p.MaxHolds,
		"grace_days":   &p.FineGraceDays,
	}
}

// moneyFields are the policy settings entered as amounts and kept in
// cents.
func moneyFields(p *domain.Policy) map[string]*int {
	return map[string]*int{
		"fine_per_day": &p.FinePerDay,
		"fine_cap":     &p.FineCap,
		"block_at":     &p.BlockAt,
	}
}

func centsText(c int) string {
	return strconv.Itoa(c/100) + "." + fmt.Sprintf("%02d", c%100)
}

func settingsData(p domain.Policy) settingsPage {
	d := settingsPage{Values: map[string]string{}, Errors: formErrors{}}
	for name, v := range policyFields(&p) {
		d.Values[name] = strconv.Itoa(*v)
	}
	for name, v := range moneyFields(&p) {
		d.Values[name] = centsText(*v)
	}
	d.Values["currency"] = p.Currency
	var dates []string
	for _, c := range p.ClosedDates {
		dates = append(dates, c.String())
	}
	d.Values["closed_dates"] = strings.Join(dates, "\n")
	closed := map[time.Weekday]bool{}
	for _, c := range p.ClosedDays {
		closed[c] = true
	}
	// Monday first, the way a library week is usually written down.
	for i := 1; i <= 7; i++ {
		wd := time.Weekday(i % 7)
		d.Weekdays = append(d.Weekdays, weekdayOption{Day: int(wd), Name: wd.String(), Closed: closed[wd]})
	}
	return d
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.Policy(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "settings.html", page{Title: "Lending rules", Data: settingsData(p)})
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.Policy(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	errs := formErrors{}
	raw := map[string]string{}
	for name, dst := range policyFields(&p) {
		v := strings.TrimSpace(r.PostForm.Get(name))
		raw[name] = v
		n, err := strconv.Atoi(v)
		if err != nil {
			errs[name] = "Enter a whole number."
			continue
		}
		*dst = n
	}
	for name, dst := range moneyFields(&p) {
		v := strings.TrimSpace(r.PostForm.Get(name))
		raw[name] = v
		n, err := domain.ParseMoney(v)
		if err != nil {
			errs[name] = "Enter an amount, like 0.25."
			continue
		}
		*dst = n
	}
	p.Currency = strings.TrimSpace(r.PostForm.Get("currency"))
	raw["currency"] = p.Currency
	raw["closed_dates"] = r.PostForm.Get("closed_dates")
	if dates, err := parseClosedDates(raw["closed_dates"], s.today()); err != nil {
		errs["closed_dates"] = capitalise(err.Error()) + "."
	} else {
		p.ClosedDates = dates
	}
	p.ClosedDays = nil
	for _, v := range r.PostForm["closed"] {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 0 && n <= 6 {
			p.ClosedDays = append(p.ClosedDays, time.Weekday(n))
		}
	}
	if !errs.any() {
		if err := p.Validate(); err != nil {
			errs["form"] = capitalise(strings.ReplaceAll(err.Error(), "\n", ". ")) + "."
		}
	}
	if errs.any() {
		d := settingsData(p)
		for k, v := range raw {
			d.Values[k] = v
		}
		d.Errors = errs
		s.render(w, r, http.StatusUnprocessableEntity, "settings.html", page{Title: "Lending rules", Data: d})
		return
	}
	if err := s.store.SavePolicy(r.Context(), p); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.log.Info("lending policy changed", "by", currentUser(r).ID)
	s.setFlash(w, "Saved. New checkouts and renewals use these rules; existing due dates don't change.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// parseClosedDates reads one date or date range per line ("2026-12-24" or
// "2026-07-20 to 2026-08-14"). Dates already past are dropped, the rest
// sorted.
func parseClosedDates(text string, today domain.Date) ([]domain.Date, error) {
	seen := map[domain.Date]bool{}
	var out []domain.Date
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		from, to, isRange := strings.Cut(line, " to ")
		first, err := domain.ParseDate(strings.TrimSpace(from))
		if err != nil {
			return nil, fmt.Errorf("line %d: write dates as 2026-12-24, or a range as 2026-07-20 to 2026-08-14", i+1)
		}
		last := first
		if isRange {
			if last, err = domain.ParseDate(strings.TrimSpace(to)); err != nil || last.Before(first) {
				return nil, fmt.Errorf("line %d: the range has to end on or after the day it starts", i+1)
			}
		}
		for d := first; !d.After(last); d = d.AddDays(1) {
			if d.Before(today) || seen[d] {
				continue
			}
			seen[d] = true
			out = append(out, d)
			if len(out) > domain.MaxClosedDates {
				return nil, fmt.Errorf("that's more than %d closure days, list only the coming ones", domain.MaxClosedDates)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out, nil
}
