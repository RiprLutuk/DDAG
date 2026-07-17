package store

import (
	"context"
	"fmt"
	"time"

	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

const backupRunColumns = `id, kind, status, destination_id, artifact_path, manifest_path, sha256, bytes, database_name, detail, error, started_at, completed_at`

func (s *Store) CreateBackupRun(ctx context.Context, c models.BackupRunCreate) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.pool.Exec(ctx, `INSERT INTO backup_runs (id, kind, status, database_name, started_at) VALUES ($1, $2, 'running', $3, $4)`, id, c.Kind, c.DatabaseName, time.Now().UTC())
	return id, err
}

func (s *Store) CompleteBackupRun(ctx context.Context, id uuid.UUID, status string, art, man, sha string, sz int64, details []byte, errMsg string) error {
	_, err := s.pool.Exec(ctx, `UPDATE backup_runs SET status=$2, artifact_path=$3, manifest_path=$4, sha256=$5, bytes=$6, detail=COALESCE($7::jsonb,'{}'::jsonb), error=$8, completed_at=$9 WHERE id=$1`, id, status, art, man, sha, sz, details, errMsg, time.Now().UTC())
	return err
}

func (s *Store) ListBackupRuns(ctx context.Context, limit int) ([]models.BackupRun, error) {
	var out []models.BackupRun
	err := s.selectRows(ctx, &out, fmt.Sprintf(`SELECT %s FROM backup_runs ORDER BY started_at DESC LIMIT $1`, backupRunColumns), limit)
	return out, err
}
