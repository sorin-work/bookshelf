package web

import (
	"net/mail"
	"strings"
)

// formErrors maps a field name to what is wrong with it.
type formErrors map[string]string

func (e formErrors) any() bool { return len(e) > 0 }

func validEmail(s string) bool {
	if len(s) > 254 || !strings.Contains(s, "@") {
		return false
	}
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s
}
