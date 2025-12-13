package domain

import "errors"

var (
	ErrRenewalLimit = errors.New("this loan has been renewed as many times as allowed")
	ErrRenewOverdue = errors.New("overdue items can't be renewed; bring it back to the desk")
	ErrCardExpired  = errors.New("the library card has expired")
	ErrTooManyLoans = errors.New("the member already has as many items out as allowed")
)

// Loan is what the renewal rules need to know about a loan.
type Loan struct {
	Due      Date
	Renewals int
}

// Overdue reports whether the loan was due before today.
func (l Loan) Overdue(today Date) bool { return l.Due.Before(today) }

// DaysOverdue is how many days past the due date today is, or 0.
func (l Loan) DaysOverdue(today Date) int {
	if !l.Overdue(today) {
		return 0
	}
	return l.Due.DaysUntil(today)
}

// CanRenew checks the renewal rules for l on today.
func (p Policy) CanRenew(l Loan, today Date) error {
	if l.Overdue(today) {
		return ErrRenewOverdue
	}
	if l.Renewals >= p.MaxRenewals {
		return ErrRenewalLimit
	}
	return nil
}

// RenewedDue is the new due date after a renewal on today. A renewal
// never brings the due date forward.
func (p Policy) RenewedDue(l Loan, today Date) Date {
	due := p.DueDate(today)
	if due.Before(l.Due) {
		return l.Due
	}
	return due
}

// Borrower is what checkout rules need to know about a member.
type Borrower struct {
	CardExpires Date
	ActiveLoans int
}

// CanBorrow checks whether a member may take out one more item today.
func (p Policy) CanBorrow(b Borrower, today Date) error {
	if !b.CardExpires.IsZero() && b.CardExpires.Before(today) {
		return ErrCardExpired
	}
	if b.ActiveLoans >= p.MaxLoans {
		return ErrTooManyLoans
	}
	return nil
}
