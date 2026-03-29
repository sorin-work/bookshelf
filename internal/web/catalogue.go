package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sorin-work/bookshelf/internal/store"
)

const cataloguePageSize = 20

type cataloguePage struct {
	Query   string
	Titles  []store.TitleSummary
	Total   int
	Page    int
	HasPrev bool
	HasNext bool
	PrevURL string
	NextURL string
}

func (s *Server) catalogue(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	pageNum, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	titles, total, err := s.store.ListTitles(r.Context(), store.TitleFilter{
		Query:  q,
		Limit:  cataloguePageSize,
		Offset: (pageNum - 1) * cataloguePageSize,
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	data := cataloguePage{
		Query:   q,
		Titles:  titles,
		Total:   total,
		Page:    pageNum,
		HasPrev: pageNum > 1,
		HasNext: pageNum*cataloguePageSize < total,
	}
	data.PrevURL = catalogueURL(q, pageNum-1)
	data.NextURL = catalogueURL(q, pageNum+1)
	if isHTMX(r) && r.Header.Get("HX-Target") == "results" {
		// Search as you type: only the results are replaced.
		s.execute(w, r, http.StatusOK, "catalogue.html", "results", page{Data: data})
		return
	}
	s.render(w, r, http.StatusOK, "catalogue.html", page{Title: "Catalogue", Data: data})
}

func catalogueURL(q string, pageNum int) string {
	v := url.Values{}
	if q != "" {
		v.Set("q", q)
	}
	if pageNum > 1 {
		v.Set("page", strconv.Itoa(pageNum))
	}
	if len(v) == 0 {
		return "/catalogue"
	}
	return "/catalogue?" + v.Encode()
}
