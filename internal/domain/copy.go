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
