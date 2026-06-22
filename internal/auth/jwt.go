package auth

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AccessClaims are the claims carried by an OAuth2 access token (PRD §11.5).
type AccessClaims struct {
	Scope    string `json:"scope"`
	ClientID string `json:"client_id"`
	jwt.RegisteredClaims
}

// IssueAccessToken signs an RS256 access token for a client with the given scope.
func IssueAccessToken(priv *rsa.PrivateKey, kid, issuer, clientID, scope string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(ttl)
	claims := AccessClaims{
		Scope:    scope,
		ClientID: clientID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   clientID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        "at-" + randHex(8),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	return signed, exp, err
}

// Keyfunc resolves a verification key by kid.
type Keyfunc func(kid string) (*rsa.PublicKey, bool)

// ParseAccessToken validates an RS256 access token's signature, exp and nbf, and
// returns the claims. The key is resolved via the kid header.
func ParseAccessToken(tokenStr string, keyfunc Keyfunc) (*AccessClaims, error) {
	claims := &AccessClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		key, ok := keyfunc(kid)
		if !ok {
			return nil, errors.New("unknown signing key")
		}
		return key, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// HashToken returns the hex sha256 of a token, used to store refresh tokens
// without keeping the plaintext.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randHex(n int) string {
	s, _ := GenerateSecret(n)
	return s
}
