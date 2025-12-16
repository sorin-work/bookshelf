package web

import (
	"net/http"
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
	Saved    bool
}

// policyFields maps form fields to the integer policy settings they edit.
func policyFields(p *domain.Policy) map[string]*int {
	return map[string]*int{
		"loan_days":    &p.LoanDays,
		"max_renewals": &p.MaxRenewals,
		"max_loans":    &p.MaxLoans,
	}
}

func settingsData(p domain.Policy) settingsPage {
	d := settingsPage{Values: map[string]string{}, Errors: formErrors{}}
	for name, v := range policyFields(&p) {
		d.Values[name] = strconv.Itoa(*v)
	}
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
	d := settingsData(p)
	d.Saved = r.URL.Query().Get("saved") == "1"
	s.render(w, r, http.StatusOK, "settings.html", page{Title: "Lending rules", Data: d})
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
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}
