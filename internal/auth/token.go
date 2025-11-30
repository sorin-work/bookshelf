package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// NewToken returns a random URL-safe token with 256 bits of entropy.
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand only fails if the OS has no randomness to give.
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken is what gets stored for a session token, so a leaked database
// does not hand out working cookies.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
