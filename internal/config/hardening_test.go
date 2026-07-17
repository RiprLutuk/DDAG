package config

import (
	"strings"
	"testing"
)

func TestValidateProductionRejectsDefaultSuperadminPassword(t *testing.T) {
	t.Setenv("DDAG_SUPERADMIN_PASSWORD", "Admin#12345")
	cfg := Config{
		Env: "prod",
		Secret: SecretConfig{
			MasterKeyB64: "ZmFrZS1ub24tZGVmYXVsdC0zMi1ieXRlLWtleSE=",
		},
		Session: SessionConfig{
			Secret:       "prod-session-secret",
			CookieSecure: true,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected default superadmin password to fail in production")
	}
	if !strings.Contains(err.Error(), "DDAG_SUPERADMIN_PASSWORD") {
		t.Fatalf("error %q should mention DDAG_SUPERADMIN_PASSWORD", err.Error())
	}
}

func TestProductionWarningsIncludeOriginsAndSSLMode(t *testing.T) {
	cfg := Config{
		Env: "prod",
		Metadata: PostgresConfig{
			SSLMode: "disable",
		},
		DashboardOrigins: []string{"http://localhost:3000", "*"},
	}

	warnings := cfg.Warnings()
	got := strings.Join(warnings, "\n")
	for _, want := range []string{"DDAG_DASHBOARD_ORIGINS", "DDAG_DB_SSLMODE"} {
		if !strings.Contains(got, want) {
			t.Fatalf("warnings %q should mention %s", got, want)
		}
	}
}

func TestValidateProductionRejectsMissingInternalAuthSecret(t *testing.T) {
	t.Setenv("DDAG_SUPERADMIN_PASSWORD", "not-the-default-password")
	cfg := Config{
		Env: "prod",
		Secret: SecretConfig{
			MasterKeyB64: "ZmFrZS1ub24tZGVmYXVsdC0zMi1ieXRlLWtleSE=",
		},
		Session: SessionConfig{
			Secret:       "prod-session-secret",
			CookieSecure: true,
		},
		Gateway: GatewayConfig{
			InternalAuthSecret: "",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected missing internal auth secret to fail in production")
	}
	if !strings.Contains(err.Error(), "DDAG_INTERNAL_AUTH_SECRET") {
		t.Fatalf("error %q should mention DDAG_INTERNAL_AUTH_SECRET", err.Error())
	}
}
