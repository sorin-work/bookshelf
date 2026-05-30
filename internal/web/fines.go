package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

// resolveFine records a payment or waives a fine.
func (s *Server) resolveFine(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	status := store.FineStatus(r.PostFormValue("status"))
	if status != store.FinePaid && status != store.FineWaived {
		http.Error(w, "unknown status", http.StatusBadRequest)
		return
	}
	f, err := s.store.FineByID(r.Context(), id)
	if err != nil {
		s.loadFailed(w, r, err)
		return
	}
	err = s.store.ResolveFine(r.Context(), id, status, currentUser(r).ID, s.now())
	if errors.Is(err, store.ErrFineClosed) {
		http.Error(w, "This fine has already been paid or waived.", http.StatusConflict)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.log.Info("fine resolved", "fine_id", id, "status", status, "by", currentUser(r).ID)
	http.Redirect(w, r, fmt.Sprintf("/members/%d#fines", f.UserID), http.StatusSeeOther)
}

// addCharge adds a fee by hand, for a damaged or lost book.
func (s *Server) addCharge(w http.ResponseWriter, r *http.Request) {
	m, ok := s.memberFromPath(w, r)
	if !ok {
		return
	}
	amount, err := domain.ParseMoney(r.PostFormValue("amount"))
	note := strings.TrimSpace(r.PostFormValue("note"))
	switch {
	case err != nil:
		http.Error(w, "Enter the amount as a number, like 4.50.", http.StatusUnprocessableEntity)
		return
	case amount == 0:
		http.Error(w, "The amount has to be more than zero.", http.StatusUnprocessableEntity)
		return
	case note == "":
		http.Error(w, "Say what the charge is for.", http.StatusUnprocessableEntity)
		return
	case utf8.RuneCountInString(note) > 200:
		http.Error(w, "Keep the note under 200 characters.", http.StatusUnprocessableEntity)
		return
	}
	if _, err := s.store.AddCharge(r.Context(), m.ID, amount, note, s.now()); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.log.Info("charge added", "member_id", m.ID, "amount", amount, "by", currentUser(r).ID)
	http.Redirect(w, r, fmt.Sprintf("/members/%d#fines", m.ID), http.StatusSeeOther)
}
