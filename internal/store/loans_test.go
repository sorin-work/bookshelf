package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
)

type fixture struct {
	s       *Store
	today   domain.Date
	staff   User
	member  User
	titleID int64
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	s := newTestStore(t)
	ctx := context.Background()
	today := domain.DateOf(now)
	staff, err := s.CreateUser(ctx, NewUser{Email: "lib@example.com", Name: "Lib", PasswordHash: "x", CardExpires: today.AddYears(1)}, now)
	if err != nil {
		t.Fatal(err)
	}
	member, err := s.CreateUser(ctx, NewUser{Email: "m@example.com", Name: "Member", CardExpires: today.AddYears(1)}, now)
	if err != nil {
		t.Fatal(err)
	}
	titleID, err := s.CreateTitle(ctx, TitleInput{Title: "Dune"}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []string{"B1", "B2", "B3", "B4", "B5", "B6"} {
		if _, err := s.CreateCopy(ctx, titleID, CopyInput{Barcode: b}, now); err != nil {
			t.Fatal(err)
		}
	}
	return fixture{s: s, today: today, staff: staff, member: member, titleID: titleID}
}

func (f fixture) checkout(t *testing.T, barcode string) (LoanRow, error) {
	t.Helper()
	return f.s.Checkout(context.Background(), CheckoutRequest{
		UserID: f.member.ID, Barcode: barcode, StaffID: f.staff.ID, Today: f.today, Now: now,
	})
}

func TestCheckoutAndReturn(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	l, err := f.checkout(t, "B1")
	if err != nil {
		t.Fatal(err)
	}
	if want := domain.DefaultPolicy().DueDate(f.today); l.Due != want {
		t.Errorf("due %s, want %s", l.Due, want)
	}
	if l.Title != "Dune" || l.MemberName != "Member" {
		t.Errorf("got %+v", l)
	}
	c, _ := f.s.CopyByBarcode(ctx, "B1")
	if c.Status != domain.CopyOnLoan {
		t.Errorf("copy status %s", c.Status)
	}

	if _, err := f.checkout(t, "B1"); !errors.Is(err, ErrCopyOnLoan) {
		t.Errorf("second checkout: %v", err)
	}

	returned, err := f.s.Return(ctx, "B1", f.today, now)
	if err != nil {
		t.Fatal(err)
	}
	if !returned.Loan.Returned() || returned.Hold != nil {
		t.Error("loan not marked returned")
	}
	c, _ = f.s.CopyByBarcode(ctx, "B1")
	if c.Status != domain.CopyAvailable {
		t.Errorf("copy status after return %s", c.Status)
	}
	if _, err := f.s.Return(ctx, "B1", f.today, now); !errors.Is(err, ErrNotOnLoan) {
		t.Errorf("double return: %v", err)
	}
	if _, err := f.s.Return(ctx, "NOPE", f.today, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown barcode: %v", err)
	}
}

func TestCheckoutRules(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	c, _ := f.s.CopyByBarcode(ctx, "B6")
	if err := f.s.SetCopyStatus(ctx, c.ID, domain.CopyLost, domain.DateOf(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.checkout(t, "B6"); !errors.Is(err, ErrCopyUnavailable) {
		t.Errorf("lost copy: %v", err)
	}

	for _, b := range []string{"B1", "B2", "B3", "B4", "B5"} {
		if _, err := f.checkout(t, b); err != nil {
			t.Fatalf("checkout %s: %v", b, err)
		}
	}
	// Five is the default limit.
	if _, err := f.s.Return(ctx, "B5", f.today, now); err != nil {
		t.Fatal(err)
	}
	p := domain.DefaultPolicy()
	p.MaxLoans = 4
	if err := f.s.SavePolicy(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.checkout(t, "B5"); !errors.Is(err, domain.ErrTooManyLoans) {
		t.Errorf("over the limit: %v", err)
	}

	if err := f.s.RenewCard(ctx, f.member.ID, f.today.AddDays(-1)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Return(ctx, "B1", f.today, now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.checkout(t, "B1"); !errors.Is(err, domain.ErrCardExpired) {
		t.Errorf("expired card: %v", err)
	}
}

func TestRenew(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	l, err := f.checkout(t, "B1")
	if err != nil {
		t.Fatal(err)
	}

	later := f.today.AddDays(20)
	for i := 0; i < 2; i++ {
		if l, err = f.s.Renew(ctx, l.ID, later); err != nil {
			t.Fatalf("renewal %d: %v", i+1, err)
		}
	}
	if l.Renewals != 2 {
		t.Errorf("renewals = %d", l.Renewals)
	}
	if _, err := f.s.Renew(ctx, l.ID, later); !errors.Is(err, domain.ErrRenewalLimit) {
		t.Errorf("third renewal: %v", err)
	}
}

func TestPolicyDefaultsAndSave(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	p, err := s.Policy(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p.LoanDays != domain.DefaultPolicy().LoanDays {
		t.Errorf("default loan days %d", p.LoanDays)
	}
	p.LoanDays = 14
	if err := s.SavePolicy(ctx, p); err != nil {
		t.Fatal(err)
	}
	p, _ = s.Policy(ctx)
	if p.LoanDays != 14 {
		t.Errorf("saved loan days %d", p.LoanDays)
	}
	p.LoanDays = 0
	if err := s.SavePolicy(ctx, p); err == nil {
		t.Error("invalid policy saved")
	}
}

func TestLateReturnChargesFine(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	l, err := f.checkout(t, "B1")
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.s.Return(ctx, "B1", l.Due.AddDays(60), now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Fine != 500 {
		t.Errorf("fine %d, want the 500 cap", res.Fine)
	}
	owed, _ := f.s.FinesOwed(ctx, f.member.ID)
	if owed != 500 {
		t.Errorf("owed %d", owed)
	}
	fines, _ := f.s.Fines(ctx, f.member.ID, 10)
	if len(fines) != 1 || fines[0].Title != "Dune" || fines[0].Kind != "late" {
		t.Errorf("fines %+v", fines)
	}

	if _, err := f.s.AddCharge(ctx, f.member.ID, 600, "Coffee on pages 20-40", now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.checkout(t, "B2"); !errors.Is(err, domain.ErrFinesOwed) {
		t.Errorf("checkout owing 11.00: %v", err)
	}
	if err := f.s.ResolveFine(ctx, fines[0].ID, FinePaid, f.staff.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := f.s.ResolveFine(ctx, fines[0].ID, FineWaived, f.staff.ID, now); !errors.Is(err, ErrFineClosed) {
		t.Errorf("resolve twice: %v", err)
	}
	if _, err := f.checkout(t, "B2"); err != nil {
		t.Errorf("checkout after paying: %v", err)
	}

	// Returned on time: no fine.
	res, err = f.s.Return(ctx, "B2", f.today, now)
	if err != nil || res.Fine != 0 {
		t.Errorf("on time return: %d, %v", res.Fine, err)
	}
}

func TestDeclareLost(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	l, err := f.checkout(t, "B1")
	if err != nil {
		t.Fatal(err)
	}
	lost, err := f.s.DeclareLost(ctx, l.ID, 1250, now)
	if err != nil {
		t.Fatal(err)
	}
	if !lost.Lost || !lost.Returned() {
		t.Errorf("loan %+v", lost)
	}
	c, _ := f.s.CopyByBarcode(ctx, "B1")
	if c.Status != domain.CopyLost {
		t.Errorf("copy status %s", c.Status)
	}
	owed, _ := f.s.FinesOwed(ctx, f.member.ID)
	if owed != 1250 {
		t.Errorf("owed %d", owed)
	}
	if _, err := f.s.DeclareLost(ctx, l.ID, 0, now); !errors.Is(err, ErrNotOnLoan) {
		t.Errorf("declared lost twice: %v", err)
	}
}

func TestPolicyCacheIsCopied(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	p, _ := s.Policy(ctx)
	p.ClosedDays[0] = time.Monday
	again, _ := s.Policy(ctx)
	if again.ClosedDays[0] != time.Sunday {
		t.Error("changing a returned policy changed the cache")
	}
	p.LoanDays = 7
	if err := s.SavePolicy(ctx, p); err != nil {
		t.Fatal(err)
	}
	if again, _ = s.Policy(ctx); again.LoanDays != 7 {
		t.Errorf("cache not refreshed after saving: %d", again.LoanDays)
	}
}
