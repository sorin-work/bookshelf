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
}

func DefaultPolicy() Policy {
	return Policy{
		LoanDays:    21,
		MaxRenewals: 2,
		MaxLoans:    5,
		ClosedDays:  []time.Weekday{time.Sunday},

		HoldPickupDays: 7,
		MaxHolds:       5,
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
