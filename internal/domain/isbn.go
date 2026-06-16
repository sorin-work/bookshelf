// Package domain holds the library rules: ISBNs, card numbers, due dates,
// renewals, holds and fines. Nothing in here touches the database.
package domain

import (
	"errors"
	"strings"
)

var (
	ErrISBNLength   = errors.New("an ISBN has 10 or 13 digits")
	ErrISBNChars    = errors.New("an ISBN may only contain digits, hyphens and spaces (and X as the last ISBN-10 digit)")
	ErrISBNChecksum = errors.New("the ISBN check digit does not match")
	ErrISBNPrefix   = errors.New("an ISBN-13 starts with 978 or 979")
)

// NormalizeISBN strips hyphens and spaces from s, checks that what is left
// is a valid ISBN-10 or ISBN-13 and returns it as a 13 digit ISBN. Storing
// everything as ISBN-13 means the same book can't be added twice under its
// two numbers.
func NormalizeISBN(s string) (string, error) {
	digits := stripISBN(s)
	switch len(digits) {
	case 10:
		if err := checkISBN10(digits); err != nil {
			return "", err
		}
		return ISBN10To13(digits), nil
	case 13:
		if err := checkISBN13(digits); err != nil {
			return "", err
		}
		return digits, nil
	default:
		for _, r := range digits {
			if (r < '0' || r > '9') && r != 'X' {
				return "", ErrISBNChars
			}
		}
		return "", ErrISBNLength
	}
}

func stripISBN(s string) string {
	s = strings.TrimSpace(s)
	// Pasted from a book's imprint page: "ISBN 978-...", "ISBN-13: 978-...".
	upper := strings.ToUpper(s)
	for _, prefix := range []string{"ISBN-13:", "ISBN-10:", "ISBN13:", "ISBN10:", "ISBN:", "ISBN"} {
		if strings.HasPrefix(upper, prefix) {
			s = s[len(prefix):]
			break
		}
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '-', ' ':
			continue
		case 'x':
			// Typed by hand the check digit is often lower case.
			r = 'X'
		}
		b.WriteRune(r)
	}
	return b.String()
}

func checkISBN10(d string) error {
	sum := 0
	for i := 0; i < 10; i++ {
		c := d[i]
		var v int
		switch {
		case c >= '0' && c <= '9':
			v = int(c - '0')
		case c == 'X' && i == 9:
			v = 10
		default:
			return ErrISBNChars
		}
		sum += v * (10 - i)
	}
	if sum%11 != 0 {
		return ErrISBNChecksum
	}
	return nil
}

func checkISBN13(d string) error {
	for i := 0; i < 13; i++ {
		if d[i] < '0' || d[i] > '9' {
			return ErrISBNChars
		}
	}
	if !strings.HasPrefix(d, "978") && !strings.HasPrefix(d, "979") {
		return ErrISBNPrefix
	}
	if isbn13CheckDigit(d[:12]) != d[12] {
		return ErrISBNChecksum
	}
	return nil
}

// isbn13CheckDigit returns the check digit for the first 12 digits of an
// ISBN-13 (weights alternate 1 and 3).
func isbn13CheckDigit(first12 string) byte {
	sum := 0
	for i := 0; i < 12; i++ {
		v := int(first12[i] - '0')
		if i%2 == 1 {
			v *= 3
		}
		sum += v
	}
	return byte('0' + (10-sum%10)%10)
}

// ISBN10To13 converts a valid ISBN-10 (digits only) to its ISBN-13 by
// prefixing 978 and recomputing the check digit.
func ISBN10To13(isbn10 string) string {
	first12 := "978" + isbn10[:9]
	return first12 + string(isbn13CheckDigit(first12))
}

// ISBN13To10 returns the ISBN-10 for a 978-prefixed ISBN-13, or "" when
// there isn't one (979 numbers have no ISBN-10).
func ISBN13To10(isbn13 string) string {
	if len(isbn13) != 13 || !strings.HasPrefix(isbn13, "978") {
		return ""
	}
	core := isbn13[3:12]
	sum := 0
	for i := 0; i < 9; i++ {
		sum += int(core[i]-'0') * (10 - i)
	}
	check := (11 - sum%11) % 11
	if check == 10 {
		return core + "X"
	}
	return core + string(byte('0'+check))
}
