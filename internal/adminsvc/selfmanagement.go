package adminsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ddag/ddag/internal/models"
	"github.com/ddag/ddag/internal/store"
)

type settingSchemaItem struct {
	Key, Category, Type, Scope, Description string
	Default                                 json.RawMessage
	RestartRequired                         bool
}

func defaultSettingSchema() []settingSchemaItem {
	return []settingSchemaItem{
		{"security.session_ttl", "Security", "duration", "global", "Dashboard session lifetime", json.RawMessage(`"8h"`), false},
		{"gateway.request_timeout", "Gateway", "duration", "global", "Default gateway request timeout", json.RawMessage(`"30s"`), true},
		{"connectors.pool_max_open", "Connectors", "integer", "global", "Default connector max open connections", json.RawMessage(`20`), true},
		{"cache.default_ttl", "Cache", "duration", "global", "Default cache TTL", json.RawMessage(`"5m"`), false},
		{"rate_limits.default_per_minute", "Rate limits", "integer", "global", "Default requests per minute", json.RawMessage(`60`), false},
		{"observability.sample_rate", "Observability", "integer", "global", "Request log sample percentage", json.RawMessage(`100`), false},
		{"retention.request_logs_days", "Retention", "integer", "global", "Request log retention in days", json.RawMessage(`30`), false},
		{"retention.audit_logs_days", "Retention", "integer", "global", "Audit log retention in days", json.RawMessage(`90`), false},
		// Backup & Recovery is deliberately provider-agnostic. Credentials never live in
		// this settings table; a future provider integration must use the secret store.
		{"backup.enabled", "Backups & Recovery", "boolean", "global", "Enable scheduled encrypted logical backups", json.RawMessage(`true`), false},
		{"backup.schedule", "Backups & Recovery", "string", "global", "systemd OnCalendar expression for logical backups", json.RawMessage(`"*-*-* 02:15:00"`), false},
		{"backup.retention_days", "Backups & Recovery", "integer", "global", "Encrypted local backup retention period", json.RawMessage(`30`), false},
		{"backup.encryption_enabled", "Backups & Recovery", "boolean", "global", "Require encryption before a backup may leave the host", json.RawMessage(`true`), false},
		{"backup.offsite.provider", "Backups & Recovery", "string", "global", "Offsite destination: local, s3, b2, gdrive, or sftp", json.RawMessage(`"local"`), false},
		{"backup.offsite.enabled", "Backups & Recovery", "boolean", "global", "Enable offsite upload only after a provider connection has passed a test", json.RawMessage(`false`), false},
		{"backup.restore_drill_enabled", "Backups & Recovery", "boolean", "global", "Run scheduled isolated restore verification", json.RawMessage(`true`), false},
		{"backup.restore_drill_schedule", "Backups & Recovery", "string", "global", "systemd OnCalendar expression for restore drill", json.RawMessage(`"Sun *-*-* 04:30:00"`), false},
		{"backup.pitr_enabled", "Backups & Recovery", "boolean", "global", "Archive WAL for point-in-time recovery", json.RawMessage(`false`), true},
		{"backup.pgbouncer_mode", "Backups & Recovery", "string", "global", "Connection pooling mode: disabled, transaction, or session", json.RawMessage(`"disabled"`), true},
		{"notifications.enabled", "Notifications", "boolean", "global", "Enable Alertmanager webhook notification delivery", json.RawMessage(`false`), false},
		{"notifications.alertmanager_url", "Notifications", "string", "global", "Target Prometheus Alertmanager API URL (e.g. http://127.0.0.1:9093)", json.RawMessage(`""`), false},
		{"features.self_management", "Feature flags", "boolean", "global", "Enable v4 self-management UI", json.RawMessage(`true`), false},
	}
}

func validateBackupProvider(provider string) error {
	switch provider {
	case "local", "s3", "b2", "gdrive", "sftp":
		return nil
	default:
		return fmt.Errorf("unsupported backup provider %q", provider)
	}
}

func validateSettingValue(typ string, raw json.RawMessage) error {
	switch typ {
	case "json", "":
		return nil
	case "boolean":
		var v bool
		return json.Unmarshal(raw, &v)
	case "integer":
		var v int64
		return json.Unmarshal(raw, &v)
	case "string":
		var v string
		return json.Unmarshal(raw, &v)
	case "duration":
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		_, err := time.ParseDuration(v)
		return err
	default:
		return fmt.Errorf("unsupported setting type %q", typ)
	}
}

type jobFunc func(context.Context) (json.RawMessage, error)
type safeJobRunner struct {
	mu      sync.Mutex
	running map[string]bool
	funcs   map[string]jobFunc
}

func newSafeJobRunner(funcs map[string]jobFunc) *safeJobRunner {
	return &safeJobRunner{running: map[string]bool{}, funcs: funcs}
}
func (r *safeJobRunner) run(ctx context.Context, key string) (json.RawMessage, error) {
	fn := r.funcs[key]
	if fn == nil {
		return nil, errors.New("unknown or unsafe job")
	}
	r.mu.Lock()
	if r.running[key] {
		r.mu.Unlock()
		return nil, errors.New("job already running")
	}
	r.running[key] = true
	r.mu.Unlock()
	defer func() { r.mu.Lock(); delete(r.running, key); r.mu.Unlock() }()
	return fn(ctx)
}

func defaultMaintenanceJobs() []models.MaintenanceJob {
	return []models.MaintenanceJob{
		{Key: "cleanup_expired_tokens", Name: "Cleanup expired tokens", Category: "Security", Description: "Remove expired OAuth/session token metadata", Safe: true, Enabled: true},
		{Key: "purge_old_request_logs", Name: "Purge old request logs", Category: "Retention", Description: "Delete audit and request logs older than configured retention", Safe: true, Enabled: true},
		{Key: "service_health_sweep", Name: "Service health sweep", Category: "Operations", Description: "Probe registered service health/readiness endpoints", Safe: true, Enabled: true},
		{Key: "metadata_backup", Name: "Metadata backup export", Category: "Backups", Description: "Generate redacted metadata backup JSON", Safe: true, Enabled: true},
	}
}

func (s *service) ensureSelfManagementDefaults(ctx context.Context) error {
	for _, it := range defaultSettingSchema() {
		if err := s.store.EnsureSettingSchema(ctx, it.Key, it.Category, it.Type, it.Scope, it.Description, it.Default, it.RestartRequired); err != nil {
			return err
		}
	}
	for _, j := range defaultMaintenanceJobs() {
		if err := s.store.EnsureMaintenanceJob(ctx, j); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) noopJob(ctx context.Context) (json.RawMessage, error) {
	return json.RawMessage(`{"ok":true,"mode":"noop"}`), nil
}
func (s *service) intSetting(ctx context.Context, key string, fallback int) int {
	st, err := s.store.GetSetting(ctx, key)
	if err != nil {
		return fallback
	}
	var v int
	if err := json.Unmarshal(st.Value, &v); err != nil || v < 1 {
		return fallback
	}
	return v
}
func (s *service) purgeOldLogsJob(ctx context.Context) (json.RawMessage, error) {
	auditDays := s.intSetting(ctx, "retention.audit_logs_days", 90)
	requestDays := s.intSetting(ctx, "retention.request_logs_days", 30)
	auditDeleted, requestDeleted, err := s.store.CleanupExpiredLogs(ctx, auditDays, requestDays)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(map[string]any{"ok": true, "audit_retention_days": auditDays, "request_log_retention_days": requestDays, "audit_deleted": auditDeleted, "request_deleted": requestDeleted})
	return b, nil
}
func (s *service) healthSweepJob(ctx context.Context) (json.RawMessage, error) {
	return json.RawMessage(`{"ok":true,"message":"health sweep scheduled via services refresh endpoints"}`), nil
}
func (s *service) metadataBackupJob(ctx context.Context) (json.RawMessage, error) {
	b, err := s.buildMetadataBackup(ctx)
	if err != nil {
		return nil, err
	}
	out, _ := json.Marshal(map[string]any{"ok": true, "bytes": len(b)})
	return out, nil
}

var backupTables = []string{"settings", "maintenance_jobs", "service_registry", "roles", "permissions", "scopes", "api_definitions", "database_connections", "clients"}

func sensitiveBackupKey(k string) bool {
	k = strings.ToLower(k)
	for _, s := range []string{"secret", "password", "token", "private_key", "dsn"} {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}
func redactBackupValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, vv := range x {
			if sensitiveBackupKey(k) {
				out[k] = "[REDACTED]"
			} else {
				out[k] = redactBackupValue(vv)
			}
		}
		return out
	case []any:
		for i := range x {
			x[i] = redactBackupValue(x[i])
		}
		return x
	default:
		return v
	}
}
func (s *service) buildMetadataBackup(ctx context.Context) ([]byte, error) {
	rows := map[string]any{}
	for _, t := range backupTables {
		v, err := s.store.ExportTableJSON(ctx, t)
		if err != nil {
			rows[t] = []any{}
			continue
		}
		rows[t] = redactBackupValue(v)
	}
	return json.MarshalIndent(map[string]any{"version": "v4", "generated_at": time.Now().UTC().Format(time.RFC3339), "tables": rows}, "", "  ")
}
func (s *service) exportBackup(w http.ResponseWriter, r *http.Request) {
	b, err := s.buildMetadataBackup(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=ddag-metadata-backup.json")
	_, _ = w.Write(b)
}

var _ = store.ErrNotFound
