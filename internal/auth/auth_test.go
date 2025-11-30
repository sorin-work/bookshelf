package auth

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func init() { Cost = bcrypt.MinCost }

func TestHashAndCheckPassword(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "correct horse battery") {
		t.Error("right password rejected")
	}
	if CheckPassword(h, "correct horse") {
		t.Error("wrong password accepted")
	}
	if CheckPassword("not a hash", "anything") {
		t.Error("garbage hash accepted")
	}
}

func TestHashPasswordTooLong(t *testing.T) {
	_, err := HashPassword(strings.Repeat("a", 73))
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Errorf("err = %v", err)
	}
}

func TestTokens(t *testing.T) {
	a, b := NewToken(), NewToken()
	if a == b {
		t.Error("tokens repeat")
	}
	if len(a) != 43 {
		t.Errorf("token length %d", len(a))
	}
	if HashToken(a) == a || HashToken(a) != HashToken(a) || len(HashToken(a)) != 64 {
		t.Error("unexpected token hash")
	}
}
