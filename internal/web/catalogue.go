package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sorin-work/bookshelf/internal/store"
)

const cataloguePageSize = 20

// catalogueQuery is what the catalogue is filtered by, read from the URL.
type catalogueQuery struct {
	Q         string
	Subject   string
	Author    string
	Available bool
	Page      int
}

func readCatalogueQuery(v url.Values) catalogueQuery {
	q := catalogueQuery{
		Q:         strings.TrimSpace(v.Get("q")),
		Subject:   strings.TrimSpace(v.Get("subject")),
		Author:    strings.TrimSpace(v.Get("author")),
		Available: v.Get("available") == "1",
	}
	q.Page, _ = strconv.Atoi(v.Get("page"))
	if q.Page < 1 {
		q.Page = 1
	}
	return q
}

// URL returns the catalogue URL for q on the given page.
func (q catalogueQuery) URL(pageNum int) string {
	v := url.Values{}
	if q.Q != "" {
		v.Set("q", q.Q)
	}
	if q.Subject != "" {
		v.Set("subject", q.Subject)
	}
	if q.Author != "" {
		v.Set("author", q.Author)
	}
	if q.Available {
		v.Set("available", "1")
	}
	if pageNum > 1 {
		v.Set("page", strconv.Itoa(pageNum))
	}
	if len(v) == 0 {
		return "/catalogue"
	}
	return "/catalogue?" + v.Encode()
}

func (q catalogueQuery) Filtered() bool {
	return q.Q != "" || q.Subject != "" || q.Author != "" || q.Available
}

type cataloguePage struct {
	Query    catalogueQuery
	Titles   []store.TitleSummary
	Total    int
	HasPrev  bool
	HasNext  bool
	PrevURL  string
	NextURL  string
	Subjects []store.SubjectCount
}

func (s *Server) catalogue(w http.ResponseWriter, r *http.Request) {
	q := readCatalogueQuery(r.URL.Query())
	titles, total, err := s.store.ListTitles(r.Context(), store.TitleFilter{
		Query:         q.Q,
		Subject:       q.Subject,
		Author:        q.Author,
		AvailableOnly: q.Available,
		Limit:         cataloguePageSize,
		Offset:        (q.Page - 1) * cataloguePageSize,
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	data := cataloguePage{
		Query:   q,
		Titles:  titles,
		Total:   total,
		HasPrev: q.Page > 1,
		HasNext: q.Page*cataloguePageSize < total,
		PrevURL: q.URL(q.Page - 1),
		NextURL: q.URL(q.Page + 1),
	}
	if isHTMX(r) && r.Header.Get("HX-Target") == "results" {
		// Search as you type: only the results are replaced.
		s.execute(w, r, http.StatusOK, "catalogue.html", "results", page{Data: data})
		return
	}
	if data.Subjects, err = s.store.Subjects(r.Context()); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "catalogue.html", page{Title: "Catalogue", Data: data})
}
