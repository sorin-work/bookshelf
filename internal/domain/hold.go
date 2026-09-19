package domain

import (
	"errors"
	"sort"
	"time"
)

// HoldStatus tracks a hold from the queue to the member's hands.
type HoldStatus string

const (
	// HoldWaiting is in the queue for the next copy.
	HoldWaiting HoldStatus = "waiting"
	// HoldReady has a copy set aside on the hold shelf.
	HoldReady HoldStatus = "ready"
	// HoldFulfilled was collected and turned into a loan.
	HoldFulfilled HoldStatus = "fulfilled"
	// HoldCancelled was cancelled by the member or a librarian.
	HoldCancelled HoldStatus = "cancelled"
	// HoldExpired was not collected in time.
	HoldExpired HoldStatus = "expired"
)

func (s HoldStatus) Open() bool { return s == HoldWaiting || s == HoldReady }

func (s HoldStatus) Label() string {
	switch s {
	case HoldWaiting:
		return "In the queue"
	case HoldReady:
		return "Ready to collect"
	case HoldFulfilled:
		return "Collected"
	case HoldCancelled:
		return "Cancelled"
	case HoldExpired:
		return "Not collected in time"
	}
	return string(s)
}

var (
	ErrHoldOnShelf      = errors.New("a copy is on the shelf, so there is no need to place a hold")
	ErrHoldNoCopies     = errors.New("the library has no copies of this title yet")
	ErrHoldDuplicate    = errors.New("you already have a hold on this title")
	ErrHoldBorrowing    = errors.New("you already have this title on loan")
	ErrTooManyHolds     = errors.New("you have as many holds as the library allows")
	ErrHoldsSwitchedOff = errors.New("this library does not take holds")
)

// HoldRequest is what the hold rules need to know when a member asks for
// a title.
type HoldRequest struct {
	CardExpires     Date
	Copies          int // copies that can circulate (not lost or withdrawn)
	AvailableCopies int // copies on the shelf right now
	HasOpenHold     bool
	HasLoan         bool
	OpenHolds       int // the member's holds on any title
}

// CanPlaceHold checks whether a member may join the queue for a title.
func (p Policy) CanPlaceHold(r HoldRequest, today Date) error {
	switch {
	case p.MaxHolds == 0:
		return ErrHoldsSwitchedOff
	case !r.CardExpires.IsZero() && r.CardExpires.Before(today):
		return ErrCardExpired
	case r.Copies == 0:
		return ErrHoldNoCopies
	case r.HasOpenHold:
		return ErrHoldDuplicate
	case r.HasLoan:
		return ErrHoldBorrowing
	case r.AvailableCopies > 0:
		return ErrHoldOnShelf
	case r.OpenHolds >= p.MaxHolds:
		return ErrTooManyHolds
	}
	return nil
}

// PickupDeadline is the last day a copy set aside on ready waits on the
// hold shelf. It is always a day the library is open.
func (p Policy) PickupDeadline(ready Date) Date {
	return p.nextOpenDay(ready.AddDays(p.HoldPickupDays))
}

// QueuedHold is a waiting hold in a title's queue.
type QueuedHold struct {
	ID       int64
	PlacedAt time.Time
}

// SortQueue orders holds first come, first served. Holds placed in the
// same instant go by id, which is the order they were saved in.
func SortQueue(q []QueuedHold) {
	sort.SliceStable(q, func(i, j int) bool {
		if !q[i].PlacedAt.Equal(q[j].PlacedAt) {
			return q[i].PlacedAt.Before(q[j].PlacedAt)
		}
		return q[i].ID < q[j].ID
	})
}

// NextInQueue returns the hold that should get the next copy.
func NextInQueue(q []QueuedHold) (QueuedHold, bool) {
	if len(q) == 0 {
		return QueuedHold{}, false
	}
	sorted := append([]QueuedHold(nil), q...)
	SortQueue(sorted)
	return sorted[0], true
}

// EstimatedWait guesses how many days a member at position in the queue
// will wait, assuming loans run their full period and are not renewed:
// each circulating copy serves one member per loan period. It returns 0
// when there is nothing to estimate.
func (p Policy) EstimatedWait(position, copies int) int {
	if position <= 0 || copies <= 0 {
		return 0
	}
	rounds := (position + copies - 1) / copies
	return rounds * p.LoanDays
}
