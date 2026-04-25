package web

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

type overdueLine struct {
	store.LoanRow
	DaysOverdue int
	// Fine is what the loan would cost if it came back today.
	Fine int
}

// ReminderURL is a mailto link with a polite reminder already written.
func (l overdueLine) ReminderURL(currency string) string {
	body := fmt.Sprintf("Hello %s,\n\n"+
		"\"%s\" (copy %s) was due back on %s and is now %d %s late.",
		l.MemberName, l.Title, l.Barcode, l.Due.Format("Monday 2 January"), l.DaysOverdue, plural(l.DaysOverdue, "day", "days"))
	if l.Fine > 0 {
		body += " The late fine so far is " + domain.Policy{Currency: currency}.FormatMoney(l.Fine) + "."
	}
	body += "\n\nPlease bring it back when you can, or reply if you think this is a mistake.\n\nThank you!"
	q := url.Values{}
	q.Set("subject", "Overdue library book: "+l.Title)
	q.Set("body", body)
	// mailto wants %20 for spaces, not +.
	return "mailto:" + l.MemberEmail + "?" + strings.ReplaceAll(q.Encode(), "+", "%20")
}

type overduePage struct {
	Loans    []overdueLine
	Currency string
	Total    int
}

func (s *Server) overdueData(r *http.Request) (overduePage, error) {
	today := s.today()
	loans, err := s.store.OverdueLoans(r.Context(), today)
	if err != nil {
		return overduePage{}, err
	}
	policy, err := s.store.Policy(r.Context())
	if err != nil {
		return overduePage{}, err
	}
	d := overduePage{Currency: policy.Currency}
	for _, l := range loans {
		line := overdueLine{
			LoanRow:     l,
			DaysOverdue: l.Rules().DaysOverdue(today),
			Fine:        policy.LateFine(l.Due, today),
		}
		d.Total += line.Fine
		d.Loans = append(d.Loans, line)
	}
	return d, nil
}

func (s *Server) overdue(w http.ResponseWriter, r *http.Request) {
	d, err := s.overdueData(r)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "overdue.html", page{Title: "Overdue", Data: d})
}

// overdueCSV downloads the overdue list for a mail merge or a spreadsheet.
func (s *Server) overdueCSV(w http.ResponseWriter, r *http.Request) {
	d, err := s.overdueData(r)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="overdue-%s.csv"`, s.today()))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"member", "email", "phone", "card", "title", "barcode", "due", "days_overdue", "fine"})
	for _, l := range d.Loans {
		_ = cw.Write([]string{
			csvSafe(l.MemberName), l.MemberEmail, csvSafe(l.MemberPhone), l.CardNumber, csvSafe(l.Title), l.Barcode,
			l.Due.String(), strconv.Itoa(l.DaysOverdue), centsText(l.Fine),
		})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		s.log.Error("write overdue csv", "err", err)
	}
}

// csvSafe stops a spreadsheet from treating a cell as a formula.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@", rune(v[0])) {
		return "'" + v
	}
	return v
}
