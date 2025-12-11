package domain

import (
	"errors"
	"fmt"
	"strings"
)

var ErrCardNumber = errors.New("that is not a valid library card number")

// CardNumber returns the library card number for a member: "2", the member
// id padded to eight digits, and a Luhn check digit. The check digit
// catches most typos when a number is keyed in at the desk.
func CardNumber(memberID int64) string {
	body := fmt.Sprintf("2%08d", memberID)
	return body + string(luhnDigit(body))
}

// NormalizeCardNumber strips spaces and hyphens and checks the Luhn digit.
func NormalizeCardNumber(s string) (string, error) {
	s = strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(s))
	if len(s) != 10 {
		return "", ErrCardNumber
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return "", ErrCardNumber
		}
	}
	if luhnDigit(s[:9]) != s[9] {
		return "", ErrCardNumber
	}
	return s, nil
}

// FormatCardNumber groups a card number for printing: 2000 0004 28.
func FormatCardNumber(s string) string {
	if len(s) != 10 {
		return s
	}
	return s[:4] + " " + s[4:8] + " " + s[8:]
}

func luhnDigit(body string) byte {
	sum := 0
	double := true
	for i := len(body) - 1; i >= 0; i-- {
		d := int(body[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return byte('0' + (10-sum%10)%10)
}
