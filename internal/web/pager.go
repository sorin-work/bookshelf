package web

import (
	"net/http"
	"strconv"
)

// pager holds the previous and next page links of a paged list. An empty
// URL means there is no such page.
type pager struct {
	PrevURL string
	NextURL string
}

// pageParam reads ?page=, defaulting to the first page.
func pageParam(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("page"))
	return max(n, 1)
}

// newPager builds the links for page n of a list of total items shown
// size at a time; link returns the URL of a given page.
func newPager(n, size, total int, link func(page int) string) pager {
	var p pager
	if n > 1 {
		p.PrevURL = link(n - 1)
	}
	if n*size < total {
		p.NextURL = link(n + 1)
	}
	return p
}
