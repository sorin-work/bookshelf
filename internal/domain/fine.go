package domain

import "errors"

var ErrFinesOwed = errors.New("the member owes more in fines than the library allows while borrowing")

// LateFine is what an item due on due and returned on returned costs:
// FinePerDay for every day late, nothing within the grace days, and never
// more than FineCap.
func (p Policy) LateFine(due, returned Date) int {
	if !returned.After(due) {
		return 0
	}
	days := due.DaysUntil(returned)
	if days <= p.FineGraceDays {
		return 0
	}
	fine := days * p.FinePerDay
	if p.FineCap > 0 && fine > p.FineCap {
		fine = p.FineCap
	}
	return fine
}
