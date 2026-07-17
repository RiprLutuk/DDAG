package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

const serviceCols = `id, name, kind, base_url, enabled, managed_by, version, commit_sha,
	health_url, ready_url, metrics_url, capabilities, last_seen_at, last_health_status,
	last_error, created_at, updated_at`

func listServicesSQL(p ListParams) string {
	p.Normalize()
	return `SELECT ` + serviceCols + ` FROM service_registry WHERE ($1='' OR name ILIKE '%'||$1||'%' OR kind ILIKE '%'||$1||'%') ORDER BY ` + p.OrderBy(map[string]string{"name": "name", "kind": "kind", "status": "last_health_status", "updated_at": "updated_at", "created_at": "created_at"}, "updated_at") + ` LIMIT $2 OFFSET $3`
}

func (s *Store) ListServices(ctx context.Context, p ListParams) ([]models.Service, int64, error) {
	p.Normalize()
	var total int64
	if err := s.get(ctx, &total, `SELECT count(*) FROM service_registry WHERE ($1='' OR name ILIKE '%'||$1||'%' OR kind ILIKE '%'||$1||'%')`, p.Search); err != nil {
		return nil, 0, err
	}
	var rows []models.Service
	if err := s.selectRows(ctx, &rows, listServicesSQL(p), p.Search, p.Limit, p.Offset()); err != nil {
		return nil, 0, err
	}
	if rows == nil {
		rows = []models.Service{}
	}
	return rows, total, nil
}

func (s *Store) GetService(ctx context.Context, id uuid.UUID) (*models.Service, error) {
	var row models.Service
	if err := s.get(ctx, &row, `SELECT `+serviceCols+` FROM service_registry WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) UpsertService(ctx context.Context, row *models.Service) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `INSERT INTO service_registry (name,kind,base_url,enabled,managed_by,version,commit_sha,health_url,ready_url,metrics_url,capabilities) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT (name) DO UPDATE SET kind=EXCLUDED.kind,base_url=EXCLUDED.base_url,enabled=EXCLUDED.enabled,managed_by=EXCLUDED.managed_by,version=EXCLUDED.version,commit_sha=EXCLUDED.commit_sha,health_url=EXCLUDED.health_url,ready_url=EXCLUDED.ready_url,metrics_url=EXCLUDED.metrics_url,capabilities=EXCLUDED.capabilities RETURNING id`, row.Name, row.Kind, row.BaseURL, row.Enabled, row.ManagedBy, row.Version, row.CommitSHA, row.HealthURL, row.ReadyURL, row.MetricsURL, row.Capabilities).Scan(&id)
	return id, err
}

func (s *Store) UpdateService(ctx context.Context, row *models.Service) error {
	tag, err := s.pool.Exec(ctx, `UPDATE service_registry SET name=$2,kind=$3,base_url=$4,enabled=$5,managed_by=$6,version=$7,commit_sha=$8,health_url=$9,ready_url=$10,metrics_url=$11,capabilities=$12 WHERE id=$1`, row.ID, row.Name, row.Kind, row.BaseURL, row.Enabled, row.ManagedBy, row.Version, row.CommitSHA, row.HealthURL, row.ReadyURL, row.MetricsURL, row.Capabilities)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetServiceHealth(ctx context.Context, id uuid.UUID, status, lastError string, seenAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `UPDATE service_registry SET last_health_status=$2,last_error=$3,last_seen_at=$4 WHERE id=$1`, id, status, lastError, seenAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetServiceHealthMetadata(ctx context.Context, id uuid.UUID, status, lastError string, seenAt time.Time, version, commitSHA string, capabilities json.RawMessage) error {
	query := `UPDATE service_registry SET last_health_status=$2,last_error=$3,last_seen_at=$4`
	args := []any{id, status, lastError, seenAt}
	n := 5
	if strings.TrimSpace(version) != "" {
		query += fmt.Sprintf(`,version=$%d`, n)
		args = append(args, strings.TrimSpace(version))
		n++
	}
	if strings.TrimSpace(commitSHA) != "" {
		query += fmt.Sprintf(`,commit_sha=$%d`, n)
		args = append(args, strings.TrimSpace(commitSHA))
		n++
	}
	if len(capabilities) > 0 && string(capabilities) != "null" {
		query += fmt.Sprintf(`,capabilities=$%d`, n)
		args = append(args, capabilities)
	}
	query += ` WHERE id=$1`
	tag, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func StaticConfiguredServices(urls map[string]string) []models.Service {
	out := make([]models.Service, 0, len(urls))
	for kind, base := range urls {
		name := "connector-" + kind
		base = strings.TrimRight(base, "/")
		out = append(out, models.Service{Name: name, Kind: "connector", BaseURL: base, Enabled: true, ManagedBy: "static", HealthURL: base + "/healthz", ReadyURL: base + "/readyz", MetricsURL: base + "/metrics", Capabilities: json.RawMessage(fmt.Sprintf(`{"connector":%q}`, kind))})
	}
	return out
}
