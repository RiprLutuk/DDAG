package store

import (
	"context"
	"strings"
	"time"

	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

// ---- Audit logs ----

func (s *Store) InsertAuditLog(ctx context.Context, a *models.AuditLog) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_logs (request_id, actor_type, actor_id, actor_label, action, resource_type,
			resource_id, ip_address, user_agent, status, metadata_json)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		a.RequestID, a.ActorType, a.ActorID, a.ActorLabel, a.Action, a.ResourceType, a.ResourceID,
		a.IPAddress, a.UserAgent, a.Status, a.MetadataJSON)
	return err
}

// AuditFilter narrows an audit-log query.
type AuditFilter struct {
	ListParams
	ActorType    string
	Action       string
	ResourceType string
	Status       string
	From         *time.Time
	To           *time.Time
}

func (s *Store) ListAuditLogs(ctx context.Context, f AuditFilter) ([]models.AuditLog, int64, error) {
	f.Normalize()
	where := []string{"1=1"}
	args := []interface{}{}
	add := func(cond string, val interface{}) {
		args = append(args, val)
		where = append(where, strings.Replace(cond, "?", "$"+itoa(len(args)), 1))
	}
	if f.ActorType != "" {
		add("actor_type=?", f.ActorType)
	}
	if f.Action != "" {
		add("action=?", f.Action)
	}
	if f.ResourceType != "" {
		add("resource_type=?", f.ResourceType)
	}
	if f.Status != "" {
		add("status=?", f.Status)
	}
	if f.From != nil {
		add("created_at>=?", *f.From)
	}
	if f.To != nil {
		add("created_at<=?", *f.To)
	}
	clause := strings.Join(where, " AND ")

	var total int64
	if err := s.get(ctx, &total, `SELECT count(*) FROM audit_logs WHERE `+clause, args...); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset())
	var out []models.AuditLog
	err := s.selectRows(ctx, &out, `
		SELECT id, request_id, actor_type, actor_id, actor_label, action, resource_type, resource_id,
			ip_address, user_agent, status, metadata_json, created_at
		FROM audit_logs WHERE `+clause+`
		ORDER BY created_at DESC LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	return out, total, err
}

// ---- API request logs ----

func (s *Store) InsertRequestLog(ctx context.Context, r *models.APIRequestLog) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO api_request_logs (request_id, client_id, api_definition_id, client_label, api_label,
			method, path, status_code, error_code, latency_ms, cached, source_db_duration_ms, ip_address)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		r.RequestID, r.ClientID, r.APIDefinitionID, r.ClientLabel, r.APILabel, r.Method, r.Path,
		r.StatusCode, r.ErrorCode, r.LatencyMS, r.Cached, r.SourceDBDurationMS, r.IPAddress)
	return err
}

func (s *Store) ListRequestLogs(ctx context.Context, p ListParams, clientID *uuid.UUID) ([]models.APIRequestLog, int64, error) {
	p.Normalize()
	var total int64
	if err := s.get(ctx, &total,
		`SELECT count(*) FROM api_request_logs WHERE ($1::uuid IS NULL OR client_id=$1)`, clientID); err != nil {
		return nil, 0, err
	}
	var out []models.APIRequestLog
	err := s.selectRows(ctx, &out, `
		SELECT id, request_id, client_id, api_definition_id, client_label, api_label, method, path,
			status_code, error_code, latency_ms, cached, source_db_duration_ms, ip_address, created_at
		FROM api_request_logs WHERE ($1::uuid IS NULL OR client_id=$1)
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, clientID, p.Limit, p.Offset())
	return out, total, err
}

// itoa is a tiny strconv.Itoa wrapper kept local to avoid an import in callers.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
