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

type titleView struct {
	Title     store.Title
	Copies    []store.CopyWhereabouts
	Available int
	// Circulating counts copies that are not lost or withdrawn.
	Circulating int
	CopyForm    copyForm
	// Queue is every open hold, shown to librarians.
	Queue   []store.HoldRow
	Waiting int
	// MyHold is the signed in member's open hold on this title.
	MyHold *store.HoldRow
	// CanHold says whether to offer the "place a hold" button.
	CanHold bool
	// WaitWeeks is a rough wait for a new hold, or for MyHold.
	WaitWeeks int
}

type copyForm struct {
	// Suggested is the next barcode in the library's series.
	Suggested string
	Barcode   string
	Shelf     string
	Note      string
	Errors    formErrors
}

func (s *Server) loadTitleView(r *http.Request, id int64) (titleView, error) {
	t, err := s.store.TitleByID(r.Context(), id)
	if err != nil {
		return titleView{}, err
	}
	copies, err := s.store.CopiesWithWhereabouts(r.Context(), id)
	if err != nil {
		return titleView{}, err
	}
	v := titleView{Title: t, Copies: copies}
	for _, c := range copies {
		switch c.Status {
		case domain.CopyAvailable:
			v.Available++
			v.Circulating++
		case domain.CopyOnLoan, domain.CopyOnHoldShelf:
			v.Circulating++
		}
	}
	if v.Queue, err = s.store.HoldQueue(r.Context(), id); err != nil {
		return titleView{}, err
	}
	user := currentUser(r)
	for i, h := range v.Queue {
		if h.Status == domain.HoldWaiting {
			v.Waiting++
		}
		if user != nil && h.UserID == user.ID {
			v.MyHold = &v.Queue[i]
		}
	}
	v.CanHold = user != nil && v.MyHold == nil && v.Available == 0 && v.Circulating > 0
	if v.Available == 0 && v.Circulating > 0 {
		policy, err := s.store.Policy(r.Context())
		if err != nil {
			return titleView{}, err
		}
		position := v.Waiting + 1
		if v.MyHold != nil {
			position = v.MyHold.Position
		}
		days := policy.EstimatedWait(position, v.Circulating)
		v.WaitWeeks = (days + 6) / 7
	}
	if user != nil && user.IsLibrarian() {
		if v.CopyForm.Suggested, err = s.store.SuggestBarcode(r.Context()); err != nil {
			return titleView{}, err
		}
	}
	return v, nil
}

func (s *Server) addCopy(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	f := copyForm{
		Barcode: r.PostFormValue("barcode"),
		Shelf:   strings.TrimSpace(r.PostFormValue("shelf")),
		Note:    strings.TrimSpace(r.PostFormValue("note")),
		Errors:  formErrors{},
	}
	barcode, err := domain.NormalizeBarcode(f.Barcode)
	if err != nil {
		f.Errors["barcode"] = capitalise(err.Error()) + "."
	}
	if utf8.RuneCountInString(f.Shelf) > 60 {
		f.Errors["shelf"] = "Keep the shelf under 60 characters."
	}
	if utf8.RuneCountInString(f.Note) > 500 {
		f.Errors["note"] = "Keep the note under 500 characters."
	}

	if !f.Errors.any() {
		_, err = s.store.CreateCopy(r.Context(), id, store.CopyInput{Barcode: barcode, Shelf: f.Shelf, Note: f.Note}, s.now())
		switch {
		case errors.Is(err, store.ErrConflict):
			f.Errors["barcode"] = "Another copy already has this barcode."
		case err != nil:
			s.serverError(w, r, err)
			return
		default:
			http.Redirect(w, r, fmt.Sprintf("/titles/%d#copies", id), http.StatusSeeOther)
			return
		}
	}

	v, err := s.loadTitleView(r, id)
	if err != nil {
		s.loadFailed(w, r, err)
		return
	}
	f.Suggested = v.CopyForm.Suggested
	v.CopyForm = f
	s.render(w, r, http.StatusUnprocessableEntity, "title.html", page{Title: v.Title.Title, Data: v})
}

func (s *Server) setCopyStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	status := domain.CopyStatus(r.PostFormValue("status"))
	switch status {
	case domain.CopyAvailable, domain.CopyLost, domain.CopyWithdrawn:
	default:
		http.Error(w, "unknown status", http.StatusBadRequest)
		return
	}
	c, err := s.store.CopyByID(r.Context(), id)
	if err != nil {
		s.loadFailed(w, r, err)
		return
	}
	if err := s.store.SetCopyStatus(r.Context(), id, status, s.today()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "This copy is on loan or waiting for pickup. Check it in first.", http.StatusConflict)
			return
		}
		s.serverError(w, r, err)
		return
	}
	s.log.Info("copy status changed", "copy_id", id, "status", status, "by", currentUser(r).ID)
	if status == domain.CopyWithdrawn {
		n, err := s.store.CancelUnfillableHolds(r.Context(), c.TitleID, s.now())
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		if n > 0 {
			s.setFlash(w, fmt.Sprintf("That was the last copy, so %d waiting %s cancelled.", n, plural(n, "hold was", "holds were")))
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/titles/%d#copies", c.TitleID), http.StatusSeeOther)
}

func (s *Server) updateCopy(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	shelf := strings.TrimSpace(r.PostFormValue("shelf"))
	note := strings.TrimSpace(r.PostFormValue("note"))
	if utf8.RuneCountInString(shelf) > 60 || utf8.RuneCountInString(note) > 500 {
		http.Error(w, "Keep the shelf under 60 characters and the note under 500.", http.StatusUnprocessableEntity)
		return
	}
	c, err := s.store.CopyByID(r.Context(), id)
	if err != nil {
		s.loadFailed(w, r, err)
		return
	}
	if err := s.store.UpdateCopy(r.Context(), id, shelf, note); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/titles/%d#copies", c.TitleID), http.StatusSeeOther)
}
