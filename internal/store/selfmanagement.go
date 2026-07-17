package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ddag/ddag/internal/models"
)

func (s *Store) EnsureSettingSchema(ctx context.Context, key, category, typ, scope, description string, def json.RawMessage, restart bool) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO settings (key,value,category,value_type,scope,description,default_value,restart_required,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now()) ON CONFLICT (key) DO UPDATE SET category=EXCLUDED.category,value_type=EXCLUDED.value_type,scope=EXCLUDED.scope,description=EXCLUDED.description,default_value=EXCLUDED.default_value,restart_required=EXCLUDED.restart_required`, key, def, category, typ, scope, description, def, restart)
	return err
}
func (s *Store) EnsureMaintenanceJob(ctx context.Context, j models.MaintenanceJob) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO maintenance_jobs (key,name,category,description,safe,enabled) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (key) DO UPDATE SET name=EXCLUDED.name,category=EXCLUDED.category,description=EXCLUDED.description,safe=EXCLUDED.safe`, j.Key, j.Name, j.Category, j.Description, j.Safe, j.Enabled)
	return err
}
func (s *Store) ListMaintenanceJobs(ctx context.Context) ([]models.MaintenanceJob, error) {
	var rows []models.MaintenanceJob
	err := s.selectRows(ctx, &rows, `SELECT key,name,category,description,safe,enabled,last_run_at,last_status,last_duration_ms,last_error,last_result,updated_at FROM maintenance_jobs ORDER BY category,name`)
	if rows == nil {
		rows = []models.MaintenanceJob{}
	}
	return rows, err
}
func (s *Store) RecordMaintenanceJobRun(ctx context.Context, key, status string, duration int, lastErr string, result json.RawMessage) error {
	tag, err := s.pool.Exec(ctx, `UPDATE maintenance_jobs SET last_run_at=now(),last_status=$2,last_duration_ms=$3,last_error=$4,last_result=$5 WHERE key=$1 AND safe=true AND enabled=true`, key, status, duration, lastErr, result)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) ExportTableJSON(ctx context.Context, table string) ([]any, error) {
	allowed := map[string]bool{"settings": true, "maintenance_jobs": true, "service_registry": true, "roles": true, "permissions": true, "scopes": true, "api_definitions": true, "database_connections": true, "clients": true}
	if !allowed[table] {
		return nil, fmt.Errorf("table not allowlisted")
	}
	rows, err := s.pool.Query(ctx, `SELECT row_to_json(t) FROM (SELECT * FROM `+table+`) t`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
