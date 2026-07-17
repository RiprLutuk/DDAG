package store

import (
	"context"

	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

// ActiveSigningKeys returns all active JWT signing keys (for JWKS publication).
func (s *Store) ActiveSigningKeys(ctx context.Context) ([]models.SigningKey, error) {
	var out []models.SigningKey
	err := s.selectRows(ctx, &out, `
		SELECT id, kid, public_key_pem, private_secret_ref, algorithm, status, created_at, updated_at
		FROM jwt_signing_keys WHERE status='active' ORDER BY created_at DESC`)
	return out, err
}

// AllSigningKeys returns active + retired keys (active first) so the gateway can
// still verify tokens signed by a key that was just retired.
func (s *Store) AllSigningKeys(ctx context.Context) ([]models.SigningKey, error) {
	var out []models.SigningKey
	err := s.selectRows(ctx, &out, `
		SELECT id, kid, public_key_pem, private_secret_ref, algorithm, status, created_at, updated_at
		FROM jwt_signing_keys ORDER BY (status='active') DESC, created_at DESC`)
	return out, err
}

// CurrentSigningKey returns the newest active signing key (used to sign).
func (s *Store) CurrentSigningKey(ctx context.Context) (*models.SigningKey, error) {
	var k models.SigningKey
	err := s.get(ctx, &k, `
		SELECT id, kid, public_key_pem, private_secret_ref, algorithm, status, created_at, updated_at
		FROM jwt_signing_keys WHERE status='active' ORDER BY created_at DESC LIMIT 1`)
	return &k, err
}

// CreateSigningKey inserts a new signing key referencing an encrypted private key.
func (s *Store) CreateSigningKey(ctx context.Context, kid, publicPEM string, secretRef uuid.UUID, alg string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO jwt_signing_keys (kid, public_key_pem, private_secret_ref, algorithm)
		VALUES ($1,$2,$3,$4) RETURNING id`, kid, publicPEM, secretRef, alg).Scan(&id)
	return id, err
}

// RetireSigningKey marks a key retired (kept for verification, no longer signs).
func (s *Store) RetireSigningKey(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE jwt_signing_keys SET status='retired' WHERE id=$1`, id)
	return err
}
