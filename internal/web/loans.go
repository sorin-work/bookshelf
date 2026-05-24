package web

import (
	"errors"
	"net/http"

	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

// loanLine is a loan as listed on member and account pages.
type loanLine struct {
	store.LoanRow
	Overdue     bool
	DaysOverdue int
	CanRenew    bool
	WhyNot      string
	// Notice is a one-off message shown in the row after an htmx renewal.
	Notice string
}

func (s *Server) loanLines(r *http.Request, loans []store.LoanRow) ([]loanLine, error) {
	policy, err := s.store.Policy(r.Context())
	if err != nil {
		return nil, err
	}
	today := s.today()
	out := make([]loanLine, 0, len(loans))
	for _, l := range loans {
		line := loanLine{
			LoanRow:     l,
			Overdue:     l.Rules().Overdue(today),
			DaysOverdue: l.Rules().DaysOverdue(today),
		}
		if err := policy.CanRenew(l.Rules(), today); err != nil {
			line.WhyNot = renewalMessage(err)
		} else {
			line.CanRenew = true
		}
		out = append(out, line)
	}
	return out, nil
}

func renewalMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrRenewalLimit):
		return "Renewed the maximum number of times"
	case errors.Is(err, domain.ErrRenewOverdue):
		return "Overdue, return it to the desk"
	case errors.Is(err, domain.ErrRenewHeld):
		return "Someone else is waiting for it"
	case errors.Is(err, domain.ErrCardExpired):
		return "Library card has expired"
	}
	return "Can't be renewed"
}

// renewLoan renews one loan. Librarians may renew any loan; members only
// their own.
func (s *Server) renewLoan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	user := currentUser(r)
	l, err := s.store.LoanByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && !user.IsLibrarian() && l.UserID != user.ID) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	back := r.Referer()
	if back == "" {
		back = "/account"
	}
	renewed, err := s.store.Renew(r.Context(), id, s.today())
	ruleErr := errors.Is(err, domain.ErrRenewalLimit) || errors.Is(err, domain.ErrRenewOverdue) ||
		errors.Is(err, domain.ErrRenewHeld) || errors.Is(err, domain.ErrCardExpired) || errors.Is(err, store.ErrNotOnLoan)
	if err != nil && !ruleErr {
		s.serverError(w, r, err)
		return
	}
	if err == nil {
		s.log.Info("loan renewed", "loan_id", id, "by", user.ID)
	}

	if isHTMX(r) {
		// Swap the table row in place.
		if err == nil {
			l = renewed
		}
		lines, lerr := s.loanLines(r, []store.LoanRow{l})
		if lerr != nil {
			s.serverError(w, r, lerr)
			return
		}
		if err == nil {
			lines[0].Notice = "Renewed"
		}
		s.execute(w, r, http.StatusOK, "account.html", "loan_row_fragment", page{Data: lines[0]})
		return
	}
	if ruleErr {
		http.Error(w, renewalMessage(err)+".", http.StatusConflict)
		return
	}
	s.setFlash(w, renewed.Title+" renewed. It's now due "+renewed.Due.Format("Monday 2 January")+".")
	http.Redirect(w, r, back, http.StatusSeeOther)
}
