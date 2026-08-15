package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sorin-work/bookshelf/internal/auth"
	"github.com/sorin-work/bookshelf/internal/store"
)

const membersPageSize = 25

type membersPage struct {
	Query   string
	Show    string
	Members []store.User
	Total   int
	Page    int
	PrevURL string
	NextURL string
}

func (s *Server) members(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	show := r.URL.Query().Get("show")
	pageNum, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if pageNum < 1 {
		pageNum = 1
	}
	members, total, err := s.store.ListMembers(r.Context(), store.MemberFilter{
		Query: q, Show: show, Today: s.today(), Limit: membersPageSize, Offset: (pageNum - 1) * membersPageSize,
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	d := membersPage{Query: q, Show: show, Members: members, Total: total, Page: pageNum}
	if pageNum > 1 {
		d.PrevURL = membersURL(q, show, pageNum-1)
	}
	if pageNum*membersPageSize < total {
		d.NextURL = membersURL(q, show, pageNum+1)
	}
	s.render(w, r, http.StatusOK, "members.html", page{Title: "Members", Data: d})
}

func membersURL(q, show string, pageNum int) string {
	v := url.Values{}
	if q != "" {
		v.Set("q", q)
	}
	if show != "" {
		v.Set("show", show)
	}
	if pageNum > 1 {
		v.Set("page", strconv.Itoa(pageNum))
	}
	return "/members?" + v.Encode()
}

type memberForm struct {
	ID     int64
	Name   string
	Email  string
	Phone  string
	Notes  string
	Errors formErrors
}

func parseMemberForm(r *http.Request) memberForm {
	f := memberForm{
		Name:   strings.TrimSpace(r.PostFormValue("name")),
		Email:  strings.TrimSpace(r.PostFormValue("email")),
		Phone:  strings.TrimSpace(r.PostFormValue("phone")),
		Notes:  strings.TrimSpace(r.PostFormValue("notes")),
		Errors: formErrors{},
	}
	if f.Name == "" {
		f.Errors["name"] = "A name is required."
	} else if utf8.RuneCountInString(f.Name) > 100 {
		f.Errors["name"] = "Keep it under 100 characters."
	}
	if !validEmail(f.Email) {
		f.Errors["email"] = "Enter a valid email address."
	}
	if utf8.RuneCountInString(f.Phone) > 30 {
		f.Errors["phone"] = "That phone number is too long."
	}
	if utf8.RuneCountInString(f.Notes) > 2000 {
		f.Errors["notes"] = "Keep notes under 2000 characters."
	}
	return f
}

func (s *Server) newMemberPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "member_form.html", page{Title: "Register a member", Data: memberForm{}})
}

// createMember registers someone at the desk. The password is optional:
// members without one can borrow but not use the self-service pages until
// a librarian sets one.
func (s *Server) createMember(w http.ResponseWriter, r *http.Request) {
	f := parseMemberForm(r)
	password := r.PostFormValue("password")
	if password != "" && utf8.RuneCountInString(password) < auth.MinPasswordLength {
		f.Errors["password"] = "Use at least 8 characters, or leave it empty."
	}
	if f.Errors.any() {
		s.render(w, r, http.StatusUnprocessableEntity, "member_form.html", page{Title: "Register a member", Data: f})
		return
	}
	hash := ""
	if password != "" {
		var err error
		if hash, err = auth.HashPassword(password); err != nil {
			f.Errors["password"] = "Use at most 72 characters."
			s.render(w, r, http.StatusUnprocessableEntity, "member_form.html", page{Title: "Register a member", Data: f})
			return
		}
	}
	u, err := s.store.CreateUser(r.Context(), store.NewUser{
		Name: f.Name, Email: f.Email, Phone: f.Phone, PasswordHash: hash,
		Role: store.RoleMember, CardExpires: s.today().AddYears(1),
	}, s.now())
	if errors.Is(err, store.ErrConflict) {
		f.Errors["email"] = "Someone is already registered with this email."
		s.render(w, r, http.StatusUnprocessableEntity, "member_form.html", page{Title: "Register a member", Data: f})
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.log.Info("member registered", "member_id", u.ID, "by", currentUser(r).ID)
	http.Redirect(w, r, fmt.Sprintf("/members/%d", u.ID), http.StatusSeeOther)
}

func (s *Server) memberFromPath(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return store.User{}, false
	}
	u, err := s.store.UserByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return store.User{}, false
	}
	if err != nil {
		s.serverError(w, r, err)
		return store.User{}, false
	}
	return u, true
}

type memberView struct {
	Member      store.User
	CardExpired bool
	Loans       []loanLine
	Holds       []store.HoldRow
	History     []store.LoanRow
	Fines       []store.Fine
	Owed        int
	Currency    string
	Blocked     bool
}

func (s *Server) memberPage(w http.ResponseWriter, r *http.Request) {
	m, ok := s.memberFromPath(w, r)
	if !ok {
		return
	}
	v := memberView{Member: m, CardExpired: m.CardExpired(s.today())}
	active, err := s.store.ActiveLoans(r.Context(), m.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if v.Loans, err = s.loanLines(r, active); err != nil {
		s.serverError(w, r, err)
		return
	}
	if v.Holds, err = s.store.OpenHolds(r.Context(), m.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	if v.History, err = s.store.LoanHistory(r.Context(), m.ID, 20); err != nil {
		s.serverError(w, r, err)
		return
	}
	if v.Fines, err = s.store.Fines(r.Context(), m.ID, 20); err != nil {
		s.serverError(w, r, err)
		return
	}
	if v.Owed, err = s.store.FinesOwed(r.Context(), m.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	policy, err := s.store.Policy(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	v.Currency = policy.Currency
	v.Blocked = policy.BlockAt > 0 && v.Owed >= policy.BlockAt
	s.render(w, r, http.StatusOK, "member.html", page{Title: m.Name, Data: v})
}

func (s *Server) editMemberPage(w http.ResponseWriter, r *http.Request) {
	m, ok := s.memberFromPath(w, r)
	if !ok {
		return
	}
	f := memberForm{ID: m.ID, Name: m.Name, Email: m.Email, Phone: m.Phone, Notes: m.Notes}
	s.render(w, r, http.StatusOK, "member_form.html", page{Title: "Edit " + m.Name, Data: f})
}

func (s *Server) updateMember(w http.ResponseWriter, r *http.Request) {
	m, ok := s.memberFromPath(w, r)
	if !ok {
		return
	}
	f := parseMemberForm(r)
	f.ID = m.ID
	if !f.Errors.any() {
		err := s.store.UpdateMember(r.Context(), m.ID, store.MemberInput{Name: f.Name, Email: f.Email, Phone: f.Phone, Notes: f.Notes})
		switch {
		case errors.Is(err, store.ErrConflict):
			f.Errors["email"] = "Someone else is registered with this email."
		case err != nil:
			s.serverError(w, r, err)
			return
		default:
			http.Redirect(w, r, fmt.Sprintf("/members/%d", m.ID), http.StatusSeeOther)
			return
		}
	}
	s.render(w, r, http.StatusUnprocessableEntity, "member_form.html", page{Title: "Edit " + m.Name, Data: f})
}

// renewCard extends a card by a year from today, or from its current expiry
// if that is still in the future.
func (s *Server) renewCard(w http.ResponseWriter, r *http.Request) {
	m, ok := s.memberFromPath(w, r)
	if !ok {
		return
	}
	from := s.today()
	if m.CardExpires.After(from) {
		from = m.CardExpires
	}
	if err := s.store.RenewCard(r.Context(), m.ID, from.AddYears(1)); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/members/%d", m.ID), http.StatusSeeOther)
}

func (s *Server) setRole(w http.ResponseWriter, r *http.Request) {
	m, ok := s.memberFromPath(w, r)
	if !ok {
		return
	}
	role := store.Role(r.PostFormValue("role"))
	if role != store.RoleLibrarian && role != store.RoleMember {
		http.Error(w, "unknown role", http.StatusBadRequest)
		return
	}
	if m.IsLibrarian() && role == store.RoleMember {
		n, err := s.store.CountLibrarians(r.Context())
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		if n <= 1 {
			http.Error(w, "The library needs at least one librarian.", http.StatusConflict)
			return
		}
	}
	if err := s.store.SetRole(r.Context(), m.ID, role); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.log.Info("role changed", "member_id", m.ID, "role", role, "by", currentUser(r).ID)
	http.Redirect(w, r, fmt.Sprintf("/members/%d", m.ID), http.StatusSeeOther)
}

func (s *Server) setActive(w http.ResponseWriter, r *http.Request) {
	m, ok := s.memberFromPath(w, r)
	if !ok {
		return
	}
	active := r.PostFormValue("active") == "1"
	if !active && m.ID == currentUser(r).ID {
		http.Error(w, "You can't close your own account.", http.StatusConflict)
		return
	}
	err := s.store.SetActive(r.Context(), m.ID, active, s.today(), s.now())
	if errors.Is(err, store.ErrHasLoans) {
		http.Error(w, m.Name+" still has items on loan. Check them in first.", http.StatusConflict)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.log.Info("member active changed", "member_id", m.ID, "active", active, "by", currentUser(r).ID)
	if active {
		s.setFlash(w, m.Name+"'s account is open again.")
	} else {
		s.setFlash(w, m.Name+"'s account is closed. Their history is kept.")
	}
	http.Redirect(w, r, fmt.Sprintf("/members/%d", m.ID), http.StatusSeeOther)
}
