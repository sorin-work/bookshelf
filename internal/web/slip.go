package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/sorin-work/bookshelf/internal/store"
)

type slipPage struct {
	Member store.User
	Loans  []store.LoanRow
}

// slip is a printable list of what a member has out and when it is due,
// to tuck into the top book. ?loans=1,2,3 limits it to those loans, which
// is what the desk links to after a checkout.
func (s *Server) slip(w http.ResponseWriter, r *http.Request) {
	m, ok := s.memberFromPath(w, r)
	if !ok {
		return
	}
	loans, err := s.store.ActiveLoans(r.Context(), m.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if only := r.URL.Query().Get("loans"); only != "" {
		want := map[int64]bool{}
		for _, part := range strings.Split(only, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
				want[id] = true
			}
		}
		var kept []store.LoanRow
		for _, l := range loans {
			if want[l.ID] {
				kept = append(kept, l)
			}
		}
		loans = kept
	}
	if len(loans) == 0 {
		s.render(w, r, http.StatusNotFound, "error.html", page{Title: "Nothing to print", Data: errorPage{
			Heading: "Nothing to print",
			Message: m.Name + " has nothing on loan.",
		}})
		return
	}
	s.execute(w, r, http.StatusOK, "slip.html", "slip", page{Title: "Due date slip", Data: slipPage{Member: m, Loans: loans}})
}

// card is a printable library card, for members who don't get a plastic
// one: cut it out and laminate it.
func (s *Server) card(w http.ResponseWriter, r *http.Request) {
	m, ok := s.memberFromPath(w, r)
	if !ok {
		return
	}
	s.execute(w, r, http.StatusOK, "slip.html", "card", page{Title: "Library card", Data: slipPage{Member: m}})
}
