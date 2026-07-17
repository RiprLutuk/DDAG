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
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	RequestID    string
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
	if f.ActorID != "" {
		add("actor_id=?", f.ActorID)
	}
	if f.Action != "" {
		add("action=?", f.Action)
	}
	if f.ResourceType != "" {
		add("resource_type=?", f.ResourceType)
	}
	if f.ResourceID != "" {
		add("resource_id=?", f.ResourceID)
	}
	if f.RequestID != "" {
		add("request_id=?", f.RequestID)
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
			method, path, status_code, error_code, latency_ms, cached, operation, source_db_duration_ms, ip_address)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		r.RequestID, r.ClientID, r.APIDefinitionID, r.ClientLabel, r.APILabel, r.Method, r.Path,
		r.StatusCode, r.ErrorCode, r.LatencyMS, r.Cached, r.Operation, r.SourceDBDurationMS, r.IPAddress)
	return err
}

// InsertRequestLogBatch inserts many request logs in a single multi-row INSERT,
// so the async logger amortizes one round-trip + one statement across a whole
// batch instead of one INSERT (and one pooled connection) per request.
func (s *Store) InsertRequestLogBatch(ctx context.Context, rs []*models.APIRequestLog) error {
	if len(rs) == 0 {
		return nil
	}
	const cols = 14
	var b strings.Builder
	b.WriteString(`INSERT INTO api_request_logs (request_id, client_id, api_definition_id, client_label,
		api_label, method, path, status_code, error_code, latency_ms, cached, operation, source_db_duration_ms,
		ip_address) VALUES `)
	args := make([]any, 0, len(rs)*cols)
	for i, r := range rs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('(')
		for j := 0; j < cols; j++ {
			if j > 0 {
				b.WriteByte(',')
			}
			b.WriteByte('$')
			b.WriteString(itoa(i*cols + j + 1))
		}
		b.WriteByte(')')
		args = append(args, r.RequestID, r.ClientID, r.APIDefinitionID, r.ClientLabel, r.APILabel,
			r.Method, r.Path, r.StatusCode, r.ErrorCode, r.LatencyMS, r.Cached, r.Operation, r.SourceDBDurationMS, r.IPAddress)
	}
	_, err := s.pool.Exec(ctx, b.String(), args...)
	return err
}

type RequestLogFilter struct {
	ListParams
	ClientID     *uuid.UUID
	APIID        *uuid.UUID
	StatusCode   int
	MinLatencyMS int
	MaxLatencyMS int
	Cached       *bool
	RequestID    string
}

func (s *Store) ListRequestLogs(ctx context.Context, f RequestLogFilter) ([]models.APIRequestLog, int64, error) {
	f.Normalize()
	where := []string{"1=1"}
	args := []any{}
	add := func(cond string, value any) {
		args = append(args, value)
		where = append(where, strings.Replace(cond, "?", "$"+itoa(len(args)), 1))
	}
	if f.ClientID != nil {
		add("client_id=?", *f.ClientID)
	}
	if f.APIID != nil {
		add("api_definition_id=?", *f.APIID)
	}
	if f.StatusCode > 0 {
		add("status_code=?", f.StatusCode)
	}
	if f.MinLatencyMS > 0 {
		add("latency_ms>=?", f.MinLatencyMS)
	}
	if f.MaxLatencyMS > 0 {
		add("latency_ms<=?", f.MaxLatencyMS)
	}
	if f.Cached != nil {
		add("cached=?", *f.Cached)
	}
	if f.RequestID != "" {
		add("request_id=?", f.RequestID)
	}
	if f.Search != "" {
		n := len(args) + 1
		args = append(args, f.Search, f.Search, f.Search)
		where = append(where, "(path ILIKE '%'||$"+itoa(n)+"||'%' OR api_label ILIKE '%'||$"+itoa(n+1)+"||'%' OR client_label ILIKE '%'||$"+itoa(n+2)+"||'%')")
	}
	clause := strings.Join(where, " AND ")
	var total int64
	if err := s.get(ctx, &total, `SELECT count(*) FROM api_request_logs WHERE `+clause, args...); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset())
	var out []models.APIRequestLog
	err := s.selectRows(ctx, &out, `SELECT id, request_id, client_id, api_definition_id, client_label, api_label, method, path, status_code, error_code, latency_ms, cached, source_db_duration_ms, ip_address, created_at FROM api_request_logs WHERE `+clause+` ORDER BY `+f.OrderBy(map[string]string{"created_at": "created_at", "latency_ms": "latency_ms", "status_code": "status_code", "path": "path", "method": "method"}, "created_at")+` LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	return out, total, err
}

func (s *Store) CleanupExpiredLogs(ctx context.Context, auditDays, requestDays int) (int64, int64, error) {
	if auditDays < 1 || requestDays < 1 {
		return 0, 0, ErrNotFound
	}
	auditTag, err := s.pool.Exec(ctx, `DELETE FROM audit_logs WHERE created_at < now() - ($1 * interval '1 day')`, auditDays)
	if err != nil {
		return 0, 0, err
	}
	requestTag, err := s.pool.Exec(ctx, `DELETE FROM api_request_logs WHERE created_at < now() - ($1 * interval '1 day')`, requestDays)
	if err != nil {
		return 0, 0, err
	}
	return auditTag.RowsAffected(), requestTag.RowsAffected(), nil
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
