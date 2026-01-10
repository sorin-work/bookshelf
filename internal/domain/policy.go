package domain

import (
	"errors"
	"fmt"
	"time"
)

// Policy is the set of lending rules a library chooses. Librarians edit it
// on the settings page.
type Policy struct {
	// LoanDays is how long a checkout or renewal lasts.
	LoanDays int
	// MaxRenewals is how many times one loan can be renewed.
	MaxRenewals int
	// MaxLoans is how many items one member can have out at once.
	MaxLoans int
	// ClosedDays are the weekdays the library is closed. Nothing falls due
	// on a closed day.
	ClosedDays []time.Weekday
	// HoldPickupDays is how long a copy waits on the hold shelf.
	HoldPickupDays int
	// MaxHolds is how many holds one member can have at once.
	MaxHolds int

	// FinePerDay is charged for each day an item is late, in cents.
	FinePerDay int
	// FineCap is the most one late item can cost, in cents.
	FineCap int
	// FineGraceDays are days late that are forgiven: an item returned
	// within them costs nothing.
	FineGraceDays int
	// BlockAt stops checkouts once a member owes this much, in cents.
	// Zero means members are never blocked.
	BlockAt int
	// Currency is printed in front of amounts.
	Currency string
}

func DefaultPolicy() Policy {
	return Policy{
		LoanDays:    21,
		MaxRenewals: 2,
		MaxLoans:    5,
		ClosedDays:  []time.Weekday{time.Sunday},

		HoldPickupDays: 7,
		MaxHolds:       5,

		FinePerDay:    25,
		FineCap:       500,
		FineGraceDays: 0,
		BlockAt:       1000,
		Currency:      "€",
	}
}

func (p Policy) Validate() error {
	var errs []error
	if p.LoanDays < 1 || p.LoanDays > 365 {
		errs = append(errs, fmt.Errorf("loan period must be between 1 and 365 days"))
	}
	if p.MaxRenewals < 0 || p.MaxRenewals > 20 {
		errs = append(errs, fmt.Errorf("renewals must be between 0 and 20"))
	}
	if p.MaxLoans < 1 || p.MaxLoans > 100 {
		errs = append(errs, fmt.Errorf("loan limit must be between 1 and 100"))
	}
	if p.HoldPickupDays < 1 || p.HoldPickupDays > 60 {
		errs = append(errs, fmt.Errorf("hold pickup time must be between 1 and 60 days"))
	}
	if p.MaxHolds < 0 || p.MaxHolds > 50 {
		errs = append(errs, fmt.Errorf("holds per member must be between 0 and 50"))
	}
	if p.FinePerDay < 0 || p.FinePerDay > 10000 {
		errs = append(errs, fmt.Errorf("the daily fine must be between 0 and 100.00"))
	}
	if p.FineCap < 0 || p.FineCap > 100000 {
		errs = append(errs, fmt.Errorf("the fine cap must be between 0 and 1000.00"))
	}
	if p.FineGraceDays < 0 || p.FineGraceDays > 30 {
		errs = append(errs, fmt.Errorf("grace days must be between 0 and 30"))
	}
	if p.BlockAt < 0 {
		errs = append(errs, fmt.Errorf("the borrowing block can't be negative"))
	}
	if len([]rune(p.Currency)) > 4 {
		errs = append(errs, fmt.Errorf("the currency symbol can be at most 4 characters"))
	}
	closed := map[time.Weekday]bool{}
	for _, d := range p.ClosedDays {
		closed[d] = true
	}
	if len(closed) >= 7 {
		errs = append(errs, fmt.Errorf("the library has to be open at least one day a week"))
	}
	return errors.Join(errs...)
}

// IsOpen reports whether the library is open on d.
func (p Policy) IsOpen(d Date) bool {
	wd := d.Weekday()
	for _, c := range p.ClosedDays {
		if c == wd {
			return false
		}
	}
	return true
}

// DueDate is LoanDays after from, moved to the next open day if the
// library is closed then.
func (p Policy) DueDate(from Date) Date {
	return p.nextOpenDay(from.AddDays(p.LoanDays))
}

// nextOpenDay returns d, or the first day after it the library is open.
func (p Policy) nextOpenDay(d Date) Date {
	// Validate guarantees one open day a week; the bound only protects
	// against a policy that skipped validation.
	for i := 0; i < 7 && !p.IsOpen(d); i++ {
		d = d.AddDays(1)
	}
	return d
}
