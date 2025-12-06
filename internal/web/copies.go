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
	Copies    []store.Copy
	Available int
	CopyForm  copyForm
}

type copyForm struct {
	Barcode string
	Shelf   string
	Note    string
	Errors  formErrors
}

func (s *Server) loadTitleView(r *http.Request, id int64) (titleView, error) {
	t, err := s.store.TitleByID(r.Context(), id)
	if err != nil {
		return titleView{}, err
	}
	copies, err := s.store.CopiesForTitle(r.Context(), id)
	if err != nil {
		return titleView{}, err
	}
	v := titleView{Title: t, Copies: copies}
	for _, c := range copies {
		if c.Status == domain.CopyAvailable {
			v.Available++
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
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
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
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.store.SetCopyStatus(r.Context(), id, status); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "This copy is on loan or waiting for pickup. Check it in first.", http.StatusConflict)
			return
		}
		s.serverError(w, r, err)
		return
	}
	s.log.Info("copy status changed", "copy_id", id, "status", status, "by", currentUser(r).ID)
	http.Redirect(w, r, fmt.Sprintf("/titles/%d#copies", c.TitleID), http.StatusSeeOther)
}
