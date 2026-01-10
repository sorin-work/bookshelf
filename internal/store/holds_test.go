package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
)

// holdFixture has one title with a single copy, out on loan to the
// fixture member, and two more members to queue for it.
type holdFixture struct {
	fixture
	ana, bob User
}

func newHoldFixture(t *testing.T) holdFixture {
	t.Helper()
	f := newFixture(t)
	ctx := context.Background()
	// Keep only B1 in circulation.
	for _, b := range []string{"B2", "B3", "B4", "B5", "B6"} {
		c, _ := f.s.CopyByBarcode(ctx, b)
		if err := f.s.SetCopyStatus(ctx, c.ID, domain.CopyWithdrawn); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.checkout(t, "B1"); err != nil {
		t.Fatal(err)
	}
	mk := func(email string) User {
		u, err := f.s.CreateUser(ctx, NewUser{Email: email, Name: email, CardExpires: f.today.AddYears(1)}, now)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	return holdFixture{fixture: f, ana: mk("ana@example.com"), bob: mk("bob@example.com")}
}

func TestHoldQueueFirstComeFirstServed(t *testing.T) {
	f := newHoldFixture(t)
	ctx := context.Background()

	bobHold, err := f.s.PlaceHold(ctx, f.bob.ID, f.titleID, f.today, now)
	if err != nil {
		t.Fatal(err)
	}
	anaHold, err := f.s.PlaceHold(ctx, f.ana.ID, f.titleID, f.today, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if bobHold.Position != 1 || anaHold.Position != 2 {
		t.Errorf("positions bob %d, ana %d", bobHold.Position, anaHold.Position)
	}
	if _, err := f.s.PlaceHold(ctx, f.ana.ID, f.titleID, f.today, now); !errors.Is(err, domain.ErrHoldDuplicate) {
		t.Errorf("second hold: %v", err)
	}
	if _, err := f.s.PlaceHold(ctx, f.member.ID, f.titleID, f.today, now); !errors.Is(err, domain.ErrHoldBorrowing) {
		t.Errorf("hold by the borrower: %v", err)
	}

	res, err := f.s.Return(ctx, "B1", f.today, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Hold == nil || res.Hold.UserID != f.bob.ID {
		t.Fatalf("copy went to %+v, want bob", res.Hold)
	}
	if want := domain.DefaultPolicy().PickupDeadline(f.today); res.Hold.ExpiresOn != want {
		t.Errorf("expires %s, want %s", res.Hold.ExpiresOn, want)
	}
	c, _ := f.s.CopyByBarcode(ctx, "B1")
	if c.Status != domain.CopyOnHoldShelf {
		t.Errorf("copy status %s", c.Status)
	}

	// Ana can't take Bob's copy; Bob can.
	if _, err := f.s.Checkout(ctx, CheckoutRequest{UserID: f.ana.ID, Barcode: "B1", Today: f.today, Now: now}); !errors.Is(err, ErrCopyHeldForOther) {
		t.Errorf("ana took bob's copy: %v", err)
	}
	if _, err := f.s.Checkout(ctx, CheckoutRequest{UserID: f.bob.ID, Barcode: "B1", Today: f.today, Now: now}); err != nil {
		t.Fatalf("bob collecting: %v", err)
	}
	h, _ := f.s.HoldByID(ctx, bobHold.ID)
	if h.Status != domain.HoldFulfilled {
		t.Errorf("bob's hold is %s", h.Status)
	}
	h, _ = f.s.HoldByID(ctx, anaHold.ID)
	if h.Status != domain.HoldWaiting || h.Position != 1 {
		t.Errorf("ana's hold is %s at %d", h.Status, h.Position)
	}
}

func TestPlaceHoldWhenCopyOnShelf(t *testing.T) {
	f := newHoldFixture(t)
	ctx := context.Background()
	if _, err := f.s.Return(ctx, "B1", f.today, now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.PlaceHold(ctx, f.ana.ID, f.titleID, f.today, now); !errors.Is(err, domain.ErrHoldOnShelf) {
		t.Errorf("hold with a copy on the shelf: %v", err)
	}
	if _, err := f.s.PlaceHold(ctx, f.ana.ID, 999, f.today, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("hold on a missing title: %v", err)
	}
}

func TestCancelReadyHoldPassesCopyOn(t *testing.T) {
	f := newHoldFixture(t)
	ctx := context.Background()
	bobHold, _ := f.s.PlaceHold(ctx, f.bob.ID, f.titleID, f.today, now)
	anaHold, _ := f.s.PlaceHold(ctx, f.ana.ID, f.titleID, f.today, now.Add(time.Minute))
	if _, err := f.s.Return(ctx, "B1", f.today, now); err != nil {
		t.Fatal(err)
	}

	if err := f.s.CancelHold(ctx, bobHold.ID, f.today, now); err != nil {
		t.Fatal(err)
	}
	if err := f.s.CancelHold(ctx, bobHold.ID, f.today, now); !errors.Is(err, ErrHoldClosed) {
		t.Errorf("cancel twice: %v", err)
	}
	h, _ := f.s.HoldByID(ctx, anaHold.ID)
	if h.Status != domain.HoldReady || h.Barcode != "B1" {
		t.Errorf("ana's hold is %s with copy %q", h.Status, h.Barcode)
	}

	// With nobody left in the queue the copy goes back on the shelf.
	if err := f.s.CancelHold(ctx, anaHold.ID, f.today, now); err != nil {
		t.Fatal(err)
	}
	c, _ := f.s.CopyByBarcode(ctx, "B1")
	if c.Status != domain.CopyAvailable {
		t.Errorf("copy status %s", c.Status)
	}
}

func TestExpireHolds(t *testing.T) {
	f := newHoldFixture(t)
	ctx := context.Background()
	bobHold, _ := f.s.PlaceHold(ctx, f.bob.ID, f.titleID, f.today, now)
	anaHold, _ := f.s.PlaceHold(ctx, f.ana.ID, f.titleID, f.today, now.Add(time.Minute))
	res, err := f.s.Return(ctx, "B1", f.today, now)
	if err != nil {
		t.Fatal(err)
	}
	deadline := res.Hold.ExpiresOn

	// Nothing expires on the last pickup day itself.
	if n, err := f.s.ExpireHolds(ctx, deadline, now); err != nil || n != 0 {
		t.Fatalf("on the deadline: %d, %v", n, err)
	}
	n, err := f.s.ExpireHolds(ctx, deadline.AddDays(1), now)
	if err != nil || n != 1 {
		t.Fatalf("after the deadline: %d, %v", n, err)
	}
	h, _ := f.s.HoldByID(ctx, bobHold.ID)
	if h.Status != domain.HoldExpired {
		t.Errorf("bob's hold is %s", h.Status)
	}
	h, _ = f.s.HoldByID(ctx, anaHold.ID)
	if h.Status != domain.HoldReady || h.ReadyOn != deadline.AddDays(1) {
		t.Errorf("ana's hold is %s, ready %s", h.Status, h.ReadyOn)
	}
	shelf, _ := f.s.HoldShelf(ctx)
	if len(shelf) != 1 || shelf[0].ID != anaHold.ID {
		t.Errorf("hold shelf %+v", shelf)
	}
}

func TestReadyHoldDoesNotBlockRenewals(t *testing.T) {
	f := newHoldFixture(t)
	ctx := context.Background()
	// Bring B2 back so the title has two copies, both out.
	c, _ := f.s.CopyByBarcode(ctx, "B2")
	if err := f.s.SetCopyStatus(ctx, c.ID, domain.CopyAvailable); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Checkout(ctx, CheckoutRequest{UserID: f.ana.ID, Barcode: "B2", Today: f.today, Now: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.PlaceHold(ctx, f.bob.ID, f.titleID, f.today, now); err != nil {
		t.Fatal(err)
	}
	loans, _ := f.s.ActiveLoans(ctx, f.member.ID)
	if _, err := f.s.Renew(ctx, loans[0].ID, f.today); !errors.Is(err, domain.ErrRenewHeld) {
		t.Fatalf("renewal with bob waiting: %v", err)
	}

	// Ana's copy goes to Bob, so nobody is waiting any more.
	if _, err := f.s.Return(ctx, "B2", f.today, now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Renew(ctx, loans[0].ID, f.today); err != nil {
		t.Errorf("renewal with bob's copy on the hold shelf: %v", err)
	}
}
