package auth

import (
	"crypto/rsa"
	"testing"
	"time"
)

func TestAccessTokenValidatesIssuerAudienceAndSkew(t *testing.T) {
	priv, err := GenerateRSAKey()
	if err != nil {
		t.Fatalf("GenerateRSAKey: %v", err)
	}
	token, _, err := IssueAccessToken(priv, "kid-1", "https://issuer.example", "ddag-api", "client-1", "site.read", time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	claims, err := ParseAccessTokenWithValidation(token, func(kid string) (*rsa.PublicKey, bool) {
		return &priv.PublicKey, kid == "kid-1"
	}, TokenValidation{
		Issuer:    "https://issuer.example",
		Audience:  "ddag-api",
		ClockSkew: time.Second,
	})
	if err != nil {
		t.Fatalf("ParseAccessTokenWithValidation: %v", err)
	}
	if claims.ClientID != "client-1" || claims.Scope != "site.read" {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestAccessTokenRejectsWrongAudience(t *testing.T) {
	priv, err := GenerateRSAKey()
	if err != nil {
		t.Fatalf("GenerateRSAKey: %v", err)
	}
	token, _, err := IssueAccessToken(priv, "kid-1", "https://issuer.example", "ddag-api", "client-1", "site.read", time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	_, err = ParseAccessTokenWithValidation(token, func(kid string) (*rsa.PublicKey, bool) {
		return &priv.PublicKey, kid == "kid-1"
	}, TokenValidation{
		Issuer:    "https://issuer.example",
		Audience:  "other-api",
		ClockSkew: time.Second,
	})
	if err == nil {
		t.Fatal("expected wrong audience to be rejected")
	}
}
