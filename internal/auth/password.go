// Package auth provides password hashing, OAuth2 JWT issuance/verification, and
// dashboard session tokens.
package auth

import (
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword returns a bcrypt hash of the password.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword reports whether plain matches the bcrypt hash. It runs in
// constant time relative to the hash to resist timing attacks.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// GenerateSecret returns a URL-safe random secret with the given byte length.
// Used for client secrets and opaque refresh tokens.
func GenerateSecret(nBytes int) (string, error) {
	if nBytes <= 0 {
		nBytes = 32
	}
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
