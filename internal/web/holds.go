package web

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

func holdMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrHoldOnShelf):
		return "A copy is on the shelf right now, so there's no need to place a hold. Ask at the desk."
	case errors.Is(err, domain.ErrHoldNoCopies):
		return "The library doesn't have a copy of this yet."
	case errors.Is(err, domain.ErrHoldDuplicate):
		return "You already have a hold on this title."
	case errors.Is(err, domain.ErrHoldBorrowing):
		return "You already have this title on loan."
	case errors.Is(err, domain.ErrTooManyHolds):
		return "You have as many holds as the library allows. Cancel one to place another."
	case errors.Is(err, domain.ErrHoldsSwitchedOff):
		return "This library doesn't take holds."
	case errors.Is(err, domain.ErrCardExpired):
		return "Your library card has expired. Ask at the desk to renew it."
	}
	return ""
}

func (s *Server) placeHold(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	user := currentUser(r)
	h, err := s.store.PlaceHold(r.Context(), user.ID, id, s.today(), s.now())
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if msg := holdMessage(err); msg != "" {
		http.Error(w, msg, http.StatusConflict)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.log.Info("hold placed", "hold_id", h.ID, "title_id", id, "member_id", user.ID)
	s.setFlash(w, fmt.Sprintf("Hold placed. You're number %d in the queue; we'll set a copy aside when one comes back.", h.Position))
	http.Redirect(w, r, fmt.Sprintf("/titles/%d", id), http.StatusSeeOther)
}

// cancelHold lets members cancel their own holds and librarians any hold.
func (s *Server) cancelHold(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	user := currentUser(r)
	h, err := s.store.HoldByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && !user.IsLibrarian() && h.UserID != user.ID) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	err = s.store.CancelHold(r.Context(), id, s.today(), s.now())
	if errors.Is(err, store.ErrHoldClosed) {
		http.Error(w, "This hold is no longer open.", http.StatusConflict)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.log.Info("hold cancelled", "hold_id", id, "by", user.ID)
	if isHTMX(r) {
		// The list item is replaced with nothing.
		w.WriteHeader(http.StatusOK)
		return
	}
	s.setFlash(w, "Hold on "+h.Title+" cancelled.")
	back := "/account"
	if user.IsLibrarian() && h.UserID != user.ID {
		back = "/holds"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

type holdsPage struct {
	Shelf   []store.HoldRow
	Waiting []store.HoldRow
	Today   domain.Date
}

// holds is the librarian's view of the hold shelf and the queues.
func (s *Server) holds(w http.ResponseWriter, r *http.Request) {
	shelf, err := s.store.HoldShelf(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	waiting, err := s.store.WaitingHolds(r.Context(), 100)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "holds.html", page{Title: "Holds", Data: holdsPage{Shelf: shelf, Waiting: waiting, Today: s.today()}})
}
