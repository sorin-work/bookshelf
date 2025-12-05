package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

// titleForm holds what was typed so the form can be shown again with errors.
type titleForm struct {
	ID          int64
	ISBN        string
	Title       string
	Subtitle    string
	Publisher   string
	Year        string
	Description string
	CoverURL    string
	Authors     string
	Subjects    string
	Errors      formErrors
}

func titleFormFrom(t store.Title) titleForm {
	f := titleForm{
		ID:          t.ID,
		ISBN:        t.ISBN,
		Title:       t.Title,
		Subtitle:    t.Subtitle,
		Publisher:   t.Publisher,
		Description: t.Description,
		CoverURL:    t.CoverURL,
		Authors:     strings.Join(t.Authors, "\n"),
		Subjects:    strings.Join(t.Subjects, ", "),
	}
	if t.Year != 0 {
		f.Year = strconv.Itoa(t.Year)
	}
	return f
}

func parseTitleForm(r *http.Request, currentYear int) (store.TitleInput, titleForm) {
	f := titleForm{
		ISBN:        strings.TrimSpace(r.PostFormValue("isbn")),
		Title:       strings.TrimSpace(r.PostFormValue("title")),
		Subtitle:    strings.TrimSpace(r.PostFormValue("subtitle")),
		Publisher:   strings.TrimSpace(r.PostFormValue("publisher")),
		Year:        strings.TrimSpace(r.PostFormValue("year")),
		Description: strings.TrimSpace(r.PostFormValue("description")),
		CoverURL:    strings.TrimSpace(r.PostFormValue("cover_url")),
		Authors:     r.PostFormValue("authors"),
		Subjects:    r.PostFormValue("subjects"),
		Errors:      formErrors{},
	}
	in := store.TitleInput{
		Title:       f.Title,
		Subtitle:    f.Subtitle,
		Publisher:   f.Publisher,
		Description: f.Description,
		CoverURL:    f.CoverURL,
		Authors:     splitList(f.Authors, "\n"),
		Subjects:    splitList(f.Subjects, ","),
	}

	if f.ISBN != "" {
		isbn, err := domain.NormalizeISBN(f.ISBN)
		if err != nil {
			f.Errors["isbn"] = capitalise(err.Error()) + "."
		}
		in.ISBN = isbn
	}
	switch n := utf8.RuneCountInString(f.Title); {
	case n == 0:
		f.Errors["title"] = "A title is required."
	case n > 300:
		f.Errors["title"] = "Keep the title under 300 characters."
	}
	if utf8.RuneCountInString(f.Subtitle) > 300 {
		f.Errors["subtitle"] = "Keep the subtitle under 300 characters."
	}
	if f.Year != "" {
		y, err := strconv.Atoi(f.Year)
		if err != nil || y < 1450 || y > currentYear+1 {
			f.Errors["year"] = fmt.Sprintf("Enter a year between 1450 and %d.", currentYear+1)
		}
		in.Year = y
	}
	if f.CoverURL != "" {
		u, err := url.Parse(f.CoverURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			f.Errors["cover_url"] = "Enter a full web address starting with https://."
		}
	}
	if utf8.RuneCountInString(f.Description) > 5000 {
		f.Errors["description"] = "Keep the description under 5000 characters."
	}
	if len(in.Authors) > 20 {
		f.Errors["authors"] = "That's a lot of authors. List at most 20."
	}
	if len(in.Subjects) > 20 {
		f.Errors["subjects"] = "List at most 20 subjects."
	}
	return in, f
}

// splitList splits on sep, trims each part and drops empty and repeated
// entries (case-insensitively).
func splitList(s, sep string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(s, sep) {
		part = strings.Join(strings.Fields(part), " ")
		key := strings.ToLower(part)
		if part == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, part)
	}
	return out
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func (s *Server) newTitlePage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "title_form.html", page{Title: "Add a title", Data: titleForm{}})
}

func (s *Server) createTitle(w http.ResponseWriter, r *http.Request) {
	in, f := parseTitleForm(r, s.now().Year())
	if f.Errors.any() {
		s.render(w, r, http.StatusUnprocessableEntity, "title_form.html", page{Title: "Add a title", Data: f})
		return
	}
	id, err := s.store.CreateTitle(r.Context(), in, s.now())
	if errors.Is(err, store.ErrConflict) {
		f.Errors["isbn"] = "A title with this ISBN is already in the catalogue."
		s.render(w, r, http.StatusUnprocessableEntity, "title_form.html", page{Title: "Add a title", Data: f})
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/titles/%d", id), http.StatusSeeOther)
}

func (s *Server) editTitlePage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	t, err := s.store.TitleByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "title_form.html", page{Title: "Edit " + t.Title, Data: titleFormFrom(t)})
}

func (s *Server) updateTitle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	in, f := parseTitleForm(r, s.now().Year())
	f.ID = id
	if f.Errors.any() {
		s.render(w, r, http.StatusUnprocessableEntity, "title_form.html", page{Title: "Edit title", Data: f})
		return
	}
	err := s.store.UpdateTitle(r.Context(), id, in, s.now())
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.notFound(w, r)
		return
	case errors.Is(err, store.ErrConflict):
		f.Errors["isbn"] = "Another title already has this ISBN."
		s.render(w, r, http.StatusUnprocessableEntity, "title_form.html", page{Title: "Edit title", Data: f})
		return
	case err != nil:
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/titles/%d", id), http.StatusSeeOther)
}

func (s *Server) titlePage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	t, err := s.store.TitleByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "title.html", page{Title: t.Title, Data: t})
}
