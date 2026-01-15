package domain

import (
	"errors"
	"fmt"
	"strings"
)

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

// FormatMoney prints cents as an amount with the policy's currency symbol:
// €1.25, -€0.50.
func (p Policy) FormatMoney(cents int) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%s%d.%02d", sign, p.Currency, cents/100, cents%100)
}

// ParseMoney reads an amount typed into a form ("2", "2.5", "2,50") as
// cents.
func ParseMoney(s string) (int, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return 0, errors.New("enter an amount")
	}
	whole, frac, _ := strings.Cut(s, ".")
	if len(frac) > 2 {
		return 0, errors.New("use at most two decimals")
	}
	for len(frac) < 2 {
		frac += "0"
	}
	var w, f int
	if _, err := fmt.Sscanf(whole+" "+frac, "%d %d", &w, &f); err != nil || w < 0 || strings.HasPrefix(frac, "-") {
		return 0, fmt.Errorf("%q is not an amount", s)
	}
	return w*100 + f, nil
}
