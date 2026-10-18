package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
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
	// Renewable counts the loans that can be renewed right now.
	Renewable int
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
	for _, l := range d.Loans {
		if l.CanRenew {
			d.Renewable++
		}
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

// exportData gives members everything the library holds about them, as
// JSON.
func (s *Server) exportData(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	type loanOut struct {
		Title      string `json:"title"`
		Barcode    string `json:"barcode"`
		Borrowed   string `json:"borrowed"`
		Due        string `json:"due"`
		Returned   string `json:"returned,omitempty"`
		Renewals   int    `json:"renewals"`
		ReportLost bool   `json:"reported_lost,omitempty"`
	}
	type holdOut struct {
		Title  string `json:"title"`
		Status string `json:"status"`
		Placed string `json:"placed"`
	}
	type fineOut struct {
		Date   string `json:"date"`
		For    string `json:"for"`
		Amount int    `json:"amount_cents"`
		Status string `json:"status"`
	}
	out := struct {
		Name        string    `json:"name"`
		Email       string    `json:"email"`
		Phone       string    `json:"phone,omitempty"`
		Card        string    `json:"card_number"`
		CardExpires string    `json:"card_expires"`
		Joined      string    `json:"joined"`
		Loans       []loanOut `json:"loans"`
		Holds       []holdOut `json:"holds"`
		Fines       []fineOut `json:"fines"`
	}{
		Name: u.Name, Email: u.Email, Phone: u.Phone, Card: u.CardNumber,
		CardExpires: u.CardExpires.String(), Joined: u.CreatedAt.Format(time.RFC3339),
		Loans: []loanOut{}, Holds: []holdOut{}, Fines: []fineOut{},
	}

	active, err := s.store.ActiveLoans(r.Context(), u.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	history, _, err := s.store.LoanHistoryPage(r.Context(), u.ID, 100000, 0)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	for _, l := range append(active, history...) {
		lo := loanOut{Title: l.Title, Barcode: l.Barcode, Borrowed: l.CheckedOutAt.Format(time.RFC3339),
			Due: l.Due.String(), Renewals: l.Renewals, ReportLost: l.Lost}
		if l.Returned() {
			lo.Returned = l.ReturnedAt.Format(time.RFC3339)
		}
		out.Loans = append(out.Loans, lo)
	}
	holds, err := s.store.OpenHolds(r.Context(), u.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	for _, h := range holds {
		out.Holds = append(out.Holds, holdOut{Title: h.Title, Status: string(h.Status), Placed: h.PlacedAt.Format(time.RFC3339)})
	}
	fines, err := s.store.Fines(r.Context(), u.ID, 100000)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	for _, f := range fines {
		what := f.Note
		if f.Kind == "late" {
			what = "Late return: " + f.Title
		}
		out.Fines = append(out.Fines, fineOut{Date: f.CreatedAt.Format(time.RFC3339), For: what, Amount: f.Amount, Status: string(f.Status)})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="my-library-data.json"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		s.log.Error("write data export", "err", err)
	}
}
