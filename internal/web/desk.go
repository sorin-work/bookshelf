package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

// deskResult is the outcome of one checkout or check in, shown above the
// forms so the librarian can keep scanning.
type deskResult struct {
	OK      bool
	Message string
	Loan    *store.LoanRow
	// Hold is set when a returned copy has to go on the hold shelf.
	Hold *store.HoldRow
}

type deskPage struct {
	Card   string
	Result *deskResult
	Member *memberSummary
	// RefreshMember sends the member summary along with an htmx result so
	// the loan count and fines stay current.
	RefreshMember bool
}

// memberSummary is shown at the desk once a card has been scanned, so the
// librarian knows about expired cards, fines and waiting holds before
// scanning books.
type memberSummary struct {
	Member      store.User
	CardExpired bool
	Loans       int
	Overdue     int
	Owed        int
	Blocked     bool
	Currency    string
	Ready       []store.HoldRow
	NotFound    bool
}

// memberSummary returns nil until card is a complete, valid number.
func (s *Server) memberSummary(r *http.Request, card string) (*memberSummary, error) {
	card, cardErr := domain.NormalizeCardNumber(card)
	if cardErr != nil {
		return nil, nil //nolint:nilerr // a half typed number is not an error
	}
	m, err := s.store.UserByCard(r.Context(), card)
	if errors.Is(err, store.ErrNotFound) {
		return &memberSummary{NotFound: true}, nil
	}
	if err != nil {
		return nil, err
	}
	sum := &memberSummary{Member: m, CardExpired: m.CardExpired(s.today())}
	loans, err := s.store.ActiveLoans(r.Context(), m.ID)
	if err != nil {
		return nil, err
	}
	sum.Loans = len(loans)
	for _, l := range loans {
		if l.Rules().Overdue(s.today()) {
			sum.Overdue++
		}
	}
	if sum.Owed, err = s.store.FinesOwed(r.Context(), m.ID); err != nil {
		return nil, err
	}
	policy, err := s.store.Policy(r.Context())
	if err != nil {
		return nil, err
	}
	sum.Currency = policy.Currency
	sum.Blocked = policy.BlockAt > 0 && sum.Owed >= policy.BlockAt
	holds, err := s.store.OpenHolds(r.Context(), m.ID)
	if err != nil {
		return nil, err
	}
	for _, h := range holds {
		if h.Status == domain.HoldReady {
			sum.Ready = append(sum.Ready, h)
		}
	}
	return sum, nil
}

// deskMember answers the htmx lookup made when a card is scanned.
func (s *Server) deskMember(w http.ResponseWriter, r *http.Request) {
	sum, err := s.memberSummary(r, r.URL.Query().Get("card"))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.execute(w, r, http.StatusOK, "desk.html", "desk_member", page{Data: deskPage{Member: sum}})
}

func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

func (s *Server) desk(w http.ResponseWriter, r *http.Request) {
	d := deskPage{Card: r.URL.Query().Get("card")}
	var err error
	if d.Member, err = s.memberSummary(r, d.Card); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "desk.html", page{Title: "Circulation desk", Data: d})
}

func (s *Server) deskRespond(w http.ResponseWriter, r *http.Request, d deskPage) {
	status := http.StatusOK
	if d.Result != nil && !d.Result.OK {
		status = http.StatusUnprocessableEntity
	}
	if isHTMX(r) {
		d.RefreshMember = d.Member != nil
		// htmx only swaps 2xx responses by default, so failures are sent
		// as 200 with the error in the fragment.
		s.execute(w, r, http.StatusOK, "desk.html", "desk_result", page{Data: d})
		return
	}
	s.render(w, r, status, "desk.html", page{Title: "Circulation desk", Data: d})
}

func (s *Server) deskCheckout(w http.ResponseWriter, r *http.Request) {
	d := deskPage{Card: r.PostFormValue("card")}
	fail := func(msg string) {
		d.Result = &deskResult{Message: msg}
		s.deskRespond(w, r, d)
	}

	card, err := domain.NormalizeCardNumber(d.Card)
	if err != nil {
		fail("That card number doesn't look right. Check the digits and try again.")
		return
	}
	barcode, err := domain.NormalizeBarcode(r.PostFormValue("barcode"))
	if err != nil {
		fail("Scan or type the barcode on the back of the book.")
		return
	}
	member, err := s.store.UserByCard(r.Context(), card)
	if errors.Is(err, store.ErrNotFound) {
		fail("No member has card " + domain.FormatCardNumber(card) + ".")
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	loan, err := s.store.Checkout(r.Context(), store.CheckoutRequest{
		UserID:  member.ID,
		Barcode: barcode,
		StaffID: currentUser(r).ID,
		Today:   s.today(),
		Now:     s.now(),
	})
	switch {
	case errors.Is(err, store.ErrNotFound):
		fail("No copy has barcode " + barcode + ".")
		return
	case errors.Is(err, store.ErrCopyHeldForOther):
		fail("Copy " + barcode + " is on the hold shelf for another member.")
		return
	case errors.Is(err, store.ErrCopyOnLoan):
		fail("Copy " + barcode + " is already checked out. Check it in first if it's in your hands.")
		return
	case errors.Is(err, store.ErrCopyUnavailable):
		fail("Copy " + barcode + " is marked lost or withdrawn. Put it back on the shelf from the title page first.")
		return
	case errors.Is(err, store.ErrMemberInactive):
		fail(member.Name + "'s account is closed. Reopen it on their member page to lend to them again.")
		return
	case errors.Is(err, domain.ErrCardExpired):
		fail(member.Name + "'s card expired on " + member.CardExpires.Format("2 Jan 2006") + ". Renew it on their member page.")
		return
	case errors.Is(err, domain.ErrFinesOwed):
		fail(member.Name + " has unpaid fines over the borrowing limit. Take a payment on their member page first.")
		return
	case errors.Is(err, domain.ErrTooManyLoans):
		fail(member.Name + " already has as many items out as the library allows.")
		return
	case err != nil:
		s.serverError(w, r, err)
		return
	}
	s.log.Info("checked out", "loan_id", loan.ID, "copy", barcode, "member_id", member.ID)
	d.Result = &deskResult{
		OK:      true,
		Message: fmt.Sprintf("%s checked out to %s.", loan.Title, member.Name),
		Loan:    &loan,
	}
	if d.Member, err = s.memberSummary(r, card); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.deskRespond(w, r, d)
}

func (s *Server) deskReturn(w http.ResponseWriter, r *http.Request) {
	d := deskPage{}
	barcode, err := domain.NormalizeBarcode(r.PostFormValue("barcode"))
	if err != nil {
		d.Result = &deskResult{Message: "Scan or type the barcode on the back of the book."}
		s.deskRespond(w, r, d)
		return
	}
	res, err := s.store.Return(r.Context(), barcode, s.today(), s.now())
	loan := res.Loan
	switch {
	case errors.Is(err, store.ErrNotFound):
		d.Result = &deskResult{Message: "No copy has barcode " + barcode + "."}
	case errors.Is(err, store.ErrNotOnLoan):
		d.Result = &deskResult{Message: s.notOnLoanMessage(r, barcode)}
	case err != nil:
		s.serverError(w, r, err)
		return
	default:
		s.log.Info("checked in", "loan_id", loan.ID, "copy", barcode)
		msg := fmt.Sprintf("%s checked in from %s.", loan.Title, loan.MemberName)
		if late := loan.Rules().DaysOverdue(s.today()); late > 0 {
			msg += fmt.Sprintf(" It was %d %s late.", late, plural(late, "day", "days"))
		}
		if res.Fine > 0 {
			policy, err := s.store.Policy(r.Context())
			if err != nil {
				s.serverError(w, r, err)
				return
			}
			msg += " A late fine of " + policy.FormatMoney(res.Fine) + " was added to their account."
		}
		d.Result = &deskResult{OK: true, Message: msg, Loan: &loan, Hold: res.Hold}
	}
	s.deskRespond(w, r, d)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// notOnLoanMessage says where a copy that was handed in but isn't on loan
// belongs.
func (s *Server) notOnLoanMessage(r *http.Request, barcode string) string {
	c, err := s.store.CopyByBarcode(r.Context(), barcode)
	if err == nil {
		switch c.Status {
		case domain.CopyOnHoldShelf:
			for _, h := range s.holdShelfOrNil(r) {
				if h.CopyID == c.ID {
					return "Copy " + barcode + " is waiting on the hold shelf for " + h.MemberName + ". Keep it there."
				}
			}
		case domain.CopyLost, domain.CopyWithdrawn:
			return "Copy " + barcode + " is marked " + strings.ToLower(c.Status.Label()) + ". Put it back on the shelf from its title page if it's turned up."
		}
	}
	return "Copy " + barcode + " wasn't checked out. It can go straight back on the shelf."
}

func (s *Server) holdShelfOrNil(r *http.Request) []store.HoldRow {
	shelf, err := s.store.HoldShelf(r.Context())
	if err != nil {
		s.log.Warn("load hold shelf", "err", err)
		return nil
	}
	return shelf
}
