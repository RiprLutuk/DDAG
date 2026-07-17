package adminsvc

import (
	"context"
	"encoding/json"
	"testing"
)

func TestValidateSettingValueByType(t *testing.T) {
	cases := []struct {
		typ string
		raw string
		ok  bool
	}{{"boolean", "true", true}, {"integer", "12", true}, {"integer", "1.2", false}, {"string", `"x"`, true}, {"duration", `"5m"`, true}, {"duration", `"later"`, false}, {"json", `{"x":1}`, true}}
	for _, c := range cases {
		if err := validateSettingValue(c.typ, json.RawMessage(c.raw)); (err == nil) != c.ok {
			t.Errorf("%s %s: %v", c.typ, c.raw, err)
		}
	}
}

func TestSafeJobRunnerRejectsUnknownAndConcurrent(t *testing.T) {
	r := newSafeJobRunner(map[string]jobFunc{"health_sweep": func(context.Context) (json.RawMessage, error) { return json.RawMessage(`{"ok":true}`), nil }})
	if _, err := r.run(context.Background(), "unknown"); err == nil {
		t.Fatal("unknown job accepted")
	}
	r.mu.Lock()
	r.running["health_sweep"] = true
	r.mu.Unlock()
	if _, err := r.run(context.Background(), "health_sweep"); err == nil {
		t.Fatal("concurrent job accepted")
	}
}

func TestRedactBackupValue(t *testing.T) {
	in := map[string]any{"name": "ok", "client_secret": "bad", "nested": map[string]any{"password": "bad", "enabled": true}}
	out := redactBackupValue(in).(map[string]any)
	if out["client_secret"] != "[REDACTED]" || out["name"] != "ok" {
		t.Fatalf("unexpected: %#v", out)
	}
	if out["nested"].(map[string]any)["password"] != "[REDACTED]" {
		t.Fatal("nested secret leaked")
	}
}

func TestBackupTablesAreMetadataOnly(t *testing.T) {
	for _, n := range backupTables {
		if n == "secrets" || n == "oauth_tokens" {
			t.Fatalf("sensitive table included: %s", n)
		}
	}
}
func TestMaintenanceJobsAreAllowlisted(t *testing.T) {
	jobs := defaultMaintenanceJobs()
	if len(jobs) < 3 {
		t.Fatalf("jobs=%d", len(jobs))
	}
	for _, j := range jobs {
		if !j.Safe {
			t.Fatalf("unsafe default job: %s", j.Key)
		}
	}
}

func TestSettingSchemaCategories(t *testing.T) {
	s := defaultSettingSchema()
	seen := map[string]bool{}
	for _, x := range s {
		seen[x.Category] = true
	}
	for _, c := range []string{"Security", "Gateway", "Connectors", "Cache", "Rate limits", "Observability", "Retention", "Backups & Recovery", "Notifications", "Feature flags"} {
		if !seen[c] {
			t.Errorf("missing %s", c)
		}
	}
}

func TestBackupRecoverySettingsAreSafeAndDynamic(t *testing.T) {
	settings := map[string]settingSchemaItem{}
	for _, item := range defaultSettingSchema() {
		settings[item.Key] = item
	}
	for _, key := range []string{
		"backup.enabled", "backup.schedule", "backup.retention_days", "backup.encryption_enabled",
		"backup.offsite.provider", "backup.offsite.enabled", "backup.restore_drill_enabled",
		"backup.restore_drill_schedule", "backup.pitr_enabled", "backup.pgbouncer_mode",
	} {
		item, ok := settings[key]
		if !ok {
			t.Errorf("missing backup/recovery setting %q", key)
			continue
		}
		if item.Category != "Backups & Recovery" {
			t.Errorf("%s category = %q", key, item.Category)
		}
	}
	if got := string(settings["backup.offsite.provider"].Default); got != `"local"` {
		t.Errorf("offsite provider default = %s, want local", got)
	}
	if got := string(settings["backup.offsite.enabled"].Default); got != "false" {
		t.Errorf("offsite must default disabled, got %s", got)
	}
}

func TestNotificationSettingsAreDisabledByDefaultAndNonSecret(t *testing.T) {
	settings := map[string]settingSchemaItem{}
	for _, item := range defaultSettingSchema() {
		settings[item.Key] = item
	}
	for _, key := range []string{"notifications.enabled", "notifications.alertmanager_url"} {
		item, ok := settings[key]
		if !ok {
			t.Errorf("missing notification setting %q", key)
			continue
		}
		if item.Category != "Notifications" {
			t.Errorf("%s category = %q", key, item.Category)
		}
	}
	if got := string(settings["notifications.enabled"].Default); got != "false" {
		t.Errorf("notifications must default disabled, got %s", got)
	}
	if got := string(settings["notifications.alertmanager_url"].Default); got != `""` {
		t.Errorf("alertmanager URL default = %s, want empty", got)
	}
}

func TestBackupProviderValidationRejectsUnknownProvider(t *testing.T) {
	if err := validateBackupProvider("s3"); err != nil {
		t.Fatalf("s3 should be accepted: %v", err)
	}
	if err := validateBackupProvider("free-bypass"); err == nil {
		t.Fatal("unknown backup provider accepted")
	}
}

var _ = context.Background
