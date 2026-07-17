package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateProductionRejectsDefaultSecrets(t *testing.T) {
	cfg := Config{
		Env: "prod",
		Metadata: PostgresConfig{
			SSLMode: "disable",
		},
		Secret: SecretConfig{
			MasterKeyB64: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		},
		Session: SessionConfig{
			Secret:       "dev-insecure-session-secret-change-me",
			CookieSecure: false,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected production config validation to fail")
	}
	for _, want := range []string{"DDAG_MASTER_KEY", "DDAG_SESSION_SECRET", "DDAG_SESSION_COOKIE_SECURE"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("validation error %q should mention %s", err.Error(), want)
		}
	}
}

func TestLoadIncludesV2HardeningDefaults(t *testing.T) {
	t.Setenv("DDAG_TOKEN_AUDIENCE", "ddag-api")
	t.Setenv("DDAG_TOKEN_CLOCK_SKEW", "45s")
	t.Setenv("DDAG_TRUSTED_PROXIES", "10.0.0.0/8, 172.16.0.0/12")
	t.Setenv("DDAG_RATE_LIMIT_FAIL_MODE", "closed")
	t.Setenv("DDAG_INTERNAL_AUTH_SECRET", "internal-secret")

	cfg := Load("api-gateway")

	if cfg.Auth.Audience != "ddag-api" {
		t.Fatalf("Auth.Audience = %q", cfg.Auth.Audience)
	}
	if cfg.Auth.ClockSkew != 45*time.Second {
		t.Fatalf("Auth.ClockSkew = %s", cfg.Auth.ClockSkew)
	}
	if cfg.Gateway.RateLimitFailMode != "closed" {
		t.Fatalf("Gateway.RateLimitFailMode = %q", cfg.Gateway.RateLimitFailMode)
	}
	if cfg.Gateway.InternalAuthSecret != "internal-secret" {
		t.Fatalf("Gateway.InternalAuthSecret = %q", cfg.Gateway.InternalAuthSecret)
	}
	if got := len(cfg.Gateway.TrustedProxies); got != 2 {
		t.Fatalf("TrustedProxies len = %d", got)
	}
}
