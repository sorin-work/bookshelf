// Package seed fills an empty database with a small library and a few
// months of made-up circulation, so the app has something to show.
package seed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/sorin-work/bookshelf/internal/auth"
	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

const (
	DemoEmail    = "demo@example.com"
	DemoPassword = "demo-password"
	// MemberEmail logs in with DemoPassword too, to try the member side.
	MemberEmail = "member@example.com"
)

type Options struct {
	Now      time.Time
	Location *time.Location
	Logger   *slog.Logger
	// Days of circulation history to make up. Defaults to 120.
	Days int
}

// Demo creates the demo accounts, the catalogue and the made-up history.
// It does nothing if the demo account already exists.
func Demo(ctx context.Context, st *store.Store, o Options) error {
	if o.Location == nil {
		o.Location = time.UTC
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Days == 0 {
		o.Days = 120
	}
	if _, err := st.UserByEmail(ctx, DemoEmail); err == nil {
		o.Logger.Info("demo data already present")
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	s := &seeder{
		ctx:   ctx,
		st:    st,
		o:     o,
		rnd:   rand.New(rand.NewPCG(42, 2026)),
		today: domain.DateOf(o.Now.In(o.Location)),
	}
	start := s.today.AddDays(-o.Days)
	if err := s.accounts(start); err != nil {
		return fmt.Errorf("accounts: %w", err)
	}
	if err := s.catalogue(start); err != nil {
		return fmt.Errorf("catalogue: %w", err)
	}
	for d := start; !d.After(s.today); d = d.AddDays(1) {
		if err := s.day(d); err != nil {
			return fmt.Errorf("history on %s: %w", d, err)
		}
	}
	if err := s.finishing(); err != nil {
		return err
	}
	o.Logger.Info("demo data created", "email", DemoEmail, "titles", len(books), "members", len(s.members))
	return nil
}

type plannedLoan struct {
	barcode string
	userID  int64
	renewOn domain.Date
	backOn  domain.Date
}

type seeder struct {
	ctx   context.Context
	st    *store.Store
	o     Options
	rnd   *rand.Rand
	today domain.Date

	librarian store.User
	sam       store.User
	members   []store.User
	barcodes  []string
	loans     map[int64]*plannedLoan
}

// at is a random time during opening hours on d.
func (s *seeder) at(d domain.Date) time.Time {
	t := time.Date(d.Year, d.Month, d.Day, 10, 0, 0, 0, s.o.Location)
	t = t.Add(time.Duration(s.rnd.IntN(8*60)) * time.Minute)
	if now := s.o.Now; t.After(now) {
		return now
	}
	return t
}

func (s *seeder) accounts(start domain.Date) error {
	hash, err := auth.HashPassword(DemoPassword)
	if err != nil {
		return err
	}
	joined := s.at(start.AddDays(-30))
	if s.librarian, err = s.st.CreateUser(s.ctx, store.NewUser{
		Email: DemoEmail, Name: "Demo Librarian", PasswordHash: hash,
		Role: store.RoleLibrarian, CardExpires: s.today.AddYears(1),
	}, joined); err != nil {
		return err
	}
	// CreateUser only promotes the very first account, so make sure the
	// demo account is a librarian even in a database that had users.
	if err := s.st.SetRole(s.ctx, s.librarian.ID, store.RoleLibrarian); err != nil {
		return err
	}
	if s.sam, err = s.st.CreateUser(s.ctx, store.NewUser{
		Email: MemberEmail, Name: "Sam Member", PasswordHash: hash,
		Role: store.RoleMember, CardExpires: s.today.AddDays(200),
	}, joined); err != nil {
		return err
	}
	s.members = append(s.members, s.sam)
	for i, p := range people {
		expires := s.today.AddDays(30 + s.rnd.IntN(330))
		if i == 5 {
			expires = s.today.AddDays(-12) // someone whose card ran out
		}
		u, err := s.st.CreateUser(s.ctx, store.NewUser{
			Email: p.email, Name: p.name, Phone: p.phone,
			Role: store.RoleMember, CardExpires: expires,
		}, s.at(start.AddDays(-s.rnd.IntN(300))))
		if err != nil {
			return err
		}
		s.members = append(s.members, u)
	}
	return nil
}

func (s *seeder) catalogue(start domain.Date) error {
	n := 0
	for _, b := range books {
		in := store.TitleInput{
			ISBN: b.isbn, Title: b.title, Subtitle: b.subtitle, Publisher: b.publisher, Year: b.year,
			Description: b.about, Authors: b.authors, Subjects: b.subjects,
		}
		if b.isbn != "" {
			if _, err := domain.NormalizeISBN(b.isbn); err != nil {
				return fmt.Errorf("%s: %w", b.title, err)
			}
			in.CoverURL = "https://covers.openlibrary.org/b/isbn/" + b.isbn + "-M.jpg"
		}
		added := s.at(start.AddDays(-20 + s.rnd.IntN(10)))
		id, err := s.st.CreateTitle(s.ctx, in, added)
		if err != nil {
			return fmt.Errorf("%s: %w", b.title, err)
		}
		for i := 0; i < b.copies; i++ {
			n++
			barcode := fmt.Sprintf("BK-%04d", n)
			note := ""
			if n%9 == 0 {
				note = "Donated"
			}
			if _, err := s.st.CreateCopy(s.ctx, id, store.CopyInput{Barcode: barcode, Shelf: b.shelf, Note: note}, added); err != nil {
				return err
			}
			s.barcodes = append(s.barcodes, barcode)
		}
	}
	s.loans = map[int64]*plannedLoan{}
	return nil
}

// day plays out one day at the desk: returns and renewals that were
// planned for it, people collecting holds, fines being paid and a handful
// of new checkouts.
func (s *seeder) day(d domain.Date) error {
	policy, err := s.st.Policy(s.ctx)
	if err != nil {
		return err
	}
	if !policy.IsOpen(d) {
		return nil
	}
	ids := make([]int64, 0, len(s.loans))
	for id := range s.loans {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		pl := s.loans[id]
		switch {
		case pl.backOn == d:
			if _, err := s.st.Return(s.ctx, pl.barcode, d, s.at(d)); err != nil {
				return err
			}
			delete(s.loans, id)
		case pl.renewOn == d:
			l, err := s.st.Renew(s.ctx, id, d)
			if err == nil && s.rnd.IntN(10) < 8 {
				pl.backOn = l.Due.AddDays(-s.rnd.IntN(5))
			}
		}
	}

	shelf, err := s.st.HoldShelf(s.ctx)
	if err != nil {
		return err
	}
	for _, h := range shelf {
		if s.rnd.IntN(3) == 0 {
			if err := s.borrow(d, h.UserID, h.Barcode, policy); err != nil {
				return err
			}
		}
	}

	if err := s.payFines(d); err != nil {
		return err
	}

	for i, n := 0, 2+s.rnd.IntN(5); i < n; i++ {
		m := s.members[s.rnd.IntN(len(s.members))]
		if s.rnd.IntN(6) == 0 {
			m = s.sam
		}
		barcode := s.barcodes[s.rnd.IntN(len(s.barcodes))]
		c, err := s.st.CopyByBarcode(s.ctx, barcode)
		if err != nil {
			return err
		}
		if c.Status != domain.CopyAvailable {
			if s.rnd.IntN(3) == 0 {
				if _, err := s.st.PlaceHold(s.ctx, m.ID, c.TitleID, d, s.at(d)); err != nil && !isRuleError(err) {
					return err
				}
			}
			continue
		}
		if err := s.borrow(d, m.ID, barcode, policy); err != nil {
			return err
		}
	}

	_, err = s.st.ExpireHolds(s.ctx, d, s.at(d))
	return err
}

// borrow checks a copy out and decides when it will come back.
func (s *seeder) borrow(d domain.Date, userID int64, barcode string, policy domain.Policy) error {
	l, err := s.st.Checkout(s.ctx, store.CheckoutRequest{
		UserID: userID, Barcode: barcode, StaffID: s.librarian.ID, Today: d, Now: s.at(d),
	})
	if isRuleError(err) {
		return nil
	}
	if err != nil {
		return err
	}
	pl := &plannedLoan{barcode: barcode, userID: userID}
	switch r := s.rnd.IntN(100); {
	case r < 60: // back some time before it is due
		pl.backOn = d.AddDays(3 + s.rnd.IntN(policy.LoanDays-3))
	case r < 75: // renewed a couple of days before it is due
		pl.renewOn = l.Due.AddDays(-2)
		pl.backOn = l.Due.AddDays(s.rnd.IntN(4))
	case r < 93: // late
		pl.backOn = l.Due.AddDays(1 + s.rnd.IntN(14))
	default: // very late
		pl.backOn = l.Due.AddDays(30 + s.rnd.IntN(40))
	}
	if !policy.IsOpen(pl.backOn) {
		pl.backOn = pl.backOn.AddDays(1)
	}
	s.loans[l.ID] = pl
	return nil
}

func (s *seeder) payFines(d domain.Date) error {
	for _, m := range s.members {
		if s.rnd.IntN(8) != 0 {
			continue
		}
		fines, err := s.st.Fines(s.ctx, m.ID, 20)
		if err != nil {
			return err
		}
		for _, f := range fines {
			if f.Status != store.FineOpen {
				continue
			}
			status := store.FinePaid
			if s.rnd.IntN(6) == 0 {
				status = store.FineWaived
			}
			if err := s.st.ResolveFine(s.ctx, f.ID, status, s.librarian.ID, s.at(d)); err != nil {
				return err
			}
		}
	}
	return nil
}

// finishing makes sure the member demo account has something to look at.
func (s *seeder) finishing() error {
	holds, err := s.st.OpenHolds(s.ctx, s.sam.ID)
	if err != nil {
		return err
	}
	if len(holds) > 0 {
		return nil
	}
	for _, barcode := range s.barcodes {
		c, err := s.st.CopyByBarcode(s.ctx, barcode)
		if err != nil {
			return err
		}
		if c.Status == domain.CopyAvailable {
			continue
		}
		_, err = s.st.PlaceHold(s.ctx, s.sam.ID, c.TitleID, s.today, s.o.Now)
		if err == nil {
			return nil
		}
		if !isRuleError(err) {
			return err
		}
	}
	return nil
}

// isRuleError reports whether err is the library saying no (limits, holds,
// expired cards) rather than something going wrong.
func isRuleError(err error) bool {
	for _, target := range []error{
		domain.ErrCardExpired, domain.ErrTooManyLoans, domain.ErrFinesOwed,
		domain.ErrHoldOnShelf, domain.ErrHoldNoCopies, domain.ErrHoldDuplicate,
		domain.ErrHoldBorrowing, domain.ErrTooManyHolds, domain.ErrHoldsSwitchedOff,
		store.ErrCopyOnLoan, store.ErrCopyUnavailable, store.ErrCopyHeldForOther,
	} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
