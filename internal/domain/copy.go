package domain

import (
	"errors"
	"strings"
)

// CopyStatus is where a physical copy is.
type CopyStatus string

const (
	CopyAvailable   CopyStatus = "available"
	CopyOnLoan      CopyStatus = "on_loan"
	CopyOnHoldShelf CopyStatus = "on_hold_shelf"
	CopyLost        CopyStatus = "lost"
	CopyWithdrawn   CopyStatus = "withdrawn"
)

func (s CopyStatus) Label() string {
	switch s {
	case CopyAvailable:
		return "On the shelf"
	case CopyOnLoan:
		return "On loan"
	case CopyOnHoldShelf:
		return "Waiting for pickup"
	case CopyLost:
		return "Lost"
	case CopyWithdrawn:
		return "Withdrawn"
	}
	return string(s)
}

var ErrBarcode = errors.New("a barcode is 3 to 32 letters, digits or hyphens")

// NormalizeBarcode trims and upper-cases a copy barcode and checks its
// characters. Scanners sometimes add stray whitespace.
func NormalizeBarcode(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) < 3 || len(s) > 32 {
		return "", ErrBarcode
	}
	for _, r := range s {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
			return "", ErrBarcode
		}
	}
	return s, nil
}

// NextBarcode increments the number at the end of a barcode, keeping its
// zero padding: BK-0099 becomes BK-0100. It returns "" if there is no
// number to increment.
func NextBarcode(b string) string {
	end := len(b)
	start := end
	for start > 0 && b[start-1] >= '0' && b[start-1] <= '9' {
		start--
	}
	if start == end {
		return ""
	}
	digits := []byte(b[start:])
	i := len(digits) - 1
	for ; i >= 0; i-- {
		if digits[i] < '9' {
			digits[i]++
			break
		}
		digits[i] = '0'
	}
	if i < 0 {
		digits = append([]byte{'1'}, digits...)
	}
	return b[:start] + string(digits)
}
