package store

import (
	"context"

	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

const backupDestinationColumns = `id,name,provider,config,credential_secret_ref,
    (credential_secret_ref IS NOT NULL) AS has_credential,status,last_verified_at,last_error,created_by,created_at,updated_at`

func (s *Store) ListBackupDestinations(ctx context.Context) ([]models.BackupDestination, error) {
	var out []models.BackupDestination
	err := s.selectRows(ctx, &out, `SELECT `+backupDestinationColumns+` FROM backup_destinations ORDER BY name`)
	if out == nil {
		out = []models.BackupDestination{}
	}
	return out, err
}

func (s *Store) GetBackupDestination(ctx context.Context, id uuid.UUID) (*models.BackupDestination, error) {
	var out models.BackupDestination
	err := s.get(ctx, &out, `SELECT `+backupDestinationColumns+` FROM backup_destinations WHERE id=$1`, id)
	return &out, err
}

func (s *Store) CreateBackupDestination(ctx context.Context, d *models.BackupDestination) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `INSERT INTO backup_destinations (name,provider,config,credential_secret_ref,status,created_by)
        VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, d.Name, d.Provider, d.Config, d.CredentialSecretRef, d.Status, d.CreatedBy).Scan(&id)
	return id, err
}

func (s *Store) UpdateBackupDestination(ctx context.Context, d *models.BackupDestination) error {
	_, err := s.pool.Exec(ctx, `UPDATE backup_destinations SET name=$2,provider=$3,config=$4,status=$5,last_error='' WHERE id=$1`, d.ID, d.Name, d.Provider, d.Config, d.Status)
	return err
}
func (s *Store) SetBackupDestinationCredential(ctx context.Context, id, ref uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE backup_destinations SET credential_secret_ref=$2,status='draft',last_error='' WHERE id=$1`, id, ref)
	return err
}
func (s *Store) SetBackupDestinationVerification(ctx context.Context, id uuid.UUID, status, lastError string) error {
	_, err := s.pool.Exec(ctx, `UPDATE backup_destinations SET status=$2,last_error=$3,last_verified_at=now() WHERE id=$1`, id, status, lastError)
	return err
}
func (s *Store) DeleteBackupDestination(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM backup_destinations WHERE id=$1 AND name <> 'local-encrypted'`, id)
	return err
}
