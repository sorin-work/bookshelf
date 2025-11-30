// Package auth hashes passwords and creates session tokens.
package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLength is the shortest password accepted at sign up.
const MinPasswordLength = 8

// bcrypt ignores everything after 72 bytes, so longer passwords are
// rejected instead of silently truncated.
const maxPasswordBytes = 72

var ErrPasswordTooLong = errors.New("password is longer than 72 bytes")

// Cost is the bcrypt cost used for new hashes. Tests lower it.
var Cost = 12

func HashPassword(password string) (string, error) {
	if len(password) > maxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), Cost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// CheckPassword reports whether password matches the stored hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
