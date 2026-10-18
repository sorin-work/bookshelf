package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Amounts of money are whole cents in an int everywhere; these helpers
// turn them into text and back.

// FormatMoney prints cents as an amount with the policy's currency:
// €1.25, -€0.50. Currencies written as letters go after the amount, the
// way they are written locally: 1.25 lei, 3.00 kr.
func (p Policy) FormatMoney(cents int) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	amount := fmt.Sprintf("%d.%02d", cents/100, cents%100)
	if currencyAfter(p.Currency) {
		return sign + amount + " " + p.Currency
	}
	return sign + p.Currency + amount
}

// currencyAfter reports whether a currency is a word rather than a
// symbol: two or more letters.
func currencyAfter(c string) bool {
	letters := 0
	for _, r := range c {
		if !unicode.IsLetter(r) {
			return false
		}
		letters++
	}
	return letters >= 2
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
