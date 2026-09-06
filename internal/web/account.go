package web

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

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
	pager
	Loans []store.LoanRow
	Total int
}

// history lists everything the member has borrowed and returned.
func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	pageNum := pageParam(r)
	loans, total, err := s.store.LoanHistoryPage(r.Context(), u.ID, historyPageSize, (pageNum-1)*historyPageSize)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	d := historyPage{Loans: loans, Total: total}
	d.pager = newPager(pageNum, historyPageSize, total, func(n int) string {
		return "/account/history?page=" + strconv.Itoa(n)
	})
	s.render(w, r, http.StatusOK, "history.html", page{Title: "Reading history", Data: d})
}

type detailsForm struct {
	Name   string
	Phone  string
	Errors formErrors
}

func (s *Server) detailsPage(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	s.render(w, r, http.StatusOK, "details.html", page{Title: "Your details", Data: detailsForm{Name: u.Name, Phone: u.Phone}})
}

// updateDetails lets members fix their name and phone number. The email
// is their login and their card is the library's, so those stay with the
// desk.
func (s *Server) updateDetails(w http.ResponseWriter, r *http.Request) {
	f := detailsForm{
		Name:   strings.TrimSpace(r.PostFormValue("name")),
		Phone:  strings.TrimSpace(r.PostFormValue("phone")),
		Errors: formErrors{},
	}
	if f.Name == "" {
		f.Errors["name"] = "Tell us your name."
	} else if utf8.RuneCountInString(f.Name) > 100 {
		f.Errors["name"] = "Keep it under 100 characters."
	}
	if utf8.RuneCountInString(f.Phone) > 30 {
		f.Errors["phone"] = "That phone number is too long."
	}
	if f.Errors.any() {
		s.render(w, r, http.StatusUnprocessableEntity, "details.html", page{Title: "Your details", Data: f})
		return
	}
	if err := s.store.UpdateContact(r.Context(), currentUser(r).ID, f.Name, f.Phone); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.setFlash(w, "Saved your details.")
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}
