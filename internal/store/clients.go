package store

import (
	"context"
	"time"

	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

// ---- Scopes ----

func (s *Store) ListScopes(ctx context.Context) ([]models.Scope, error) {
	var out []models.Scope
	err := s.selectRows(ctx, &out,
		`SELECT id, scope_code, description, created_at, updated_at FROM scopes ORDER BY scope_code`)
	return out, err
}

func (s *Store) CreateScope(ctx context.Context, code, description string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO scopes (scope_code, description) VALUES ($1,$2)
		 ON CONFLICT (scope_code) DO UPDATE SET description=EXCLUDED.description RETURNING id`,
		code, description).Scan(&id)
	return id, err
}

func (s *Store) DeleteScope(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM scopes WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Clients ----

const clientCols = `id, client_id, client_name, client_secret_hash, owner_user_id, environment,
	status, access_token_ttl_seconds, refresh_token_ttl_seconds, description, created_by, created_at, updated_at`

func (s *Store) ListClients(ctx context.Context, p ListParams) ([]models.Client, int64, error) {
	p.Normalize()
	var total int64
	if err := s.get(ctx, &total,
		`SELECT count(*) FROM clients WHERE ($1='' OR client_name ILIKE '%'||$1||'%' OR client_id ILIKE '%'||$1||'%')`,
		p.Search); err != nil {
		return nil, 0, err
	}
	var clients []models.Client
	if err := s.selectRows(ctx, &clients,
		`SELECT `+clientCols+` FROM clients
		 WHERE ($1='' OR client_name ILIKE '%'||$1||'%' OR client_id ILIKE '%'||$1||'%')
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, p.Search, p.Limit, p.Offset()); err != nil {
		return nil, 0, err
	}
	for i := range clients {
		if err := s.hydrateClient(ctx, &clients[i]); err != nil {
			return nil, 0, err
		}
	}
	return clients, total, nil
}

func (s *Store) GetClientByPK(ctx context.Context, id uuid.UUID) (*models.Client, error) {
	var c models.Client
	if err := s.get(ctx, &c, `SELECT `+clientCols+` FROM clients WHERE id=$1`, id); err != nil {
		return nil, err
	}
	if err := s.hydrateClient(ctx, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// GetClientByClientID looks up a client by its public client_id (for OAuth2).
func (s *Store) GetClientByClientID(ctx context.Context, clientID string) (*models.Client, error) {
	var c models.Client
	if err := s.get(ctx, &c, `SELECT `+clientCols+` FROM clients WHERE client_id=$1`, clientID); err != nil {
		return nil, err
	}
	if err := s.hydrateClient(ctx, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) hydrateClient(ctx context.Context, c *models.Client) error {
	var scopes []string
	if err := s.selectRows(ctx, &scopes,
		`SELECT sc.scope_code FROM scopes sc JOIN client_scopes cs ON cs.scope_id=sc.id
		 WHERE cs.client_id=$1 ORDER BY sc.scope_code`, c.ID); err != nil {
		return err
	}
	if scopes == nil {
		scopes = []string{}
	}
	c.Scopes = scopes
	var apis []uuid.UUID
	if err := s.selectRows(ctx, &apis,
		`SELECT api_definition_id FROM client_api_access WHERE client_id=$1 AND allowed=true`, c.ID); err != nil {
		return err
	}
	if apis == nil {
		apis = []uuid.UUID{}
	}
	c.APIs = apis
	return nil
}

func (s *Store) CreateClient(ctx context.Context, c *models.Client) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO clients (client_id, client_name, client_secret_hash, owner_user_id, environment,
			status, access_token_ttl_seconds, refresh_token_ttl_seconds, description, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		c.ClientID, c.ClientName, c.ClientSecretHash, c.OwnerUserID, c.Environment, c.Status,
		c.AccessTokenTTLSeconds, c.RefreshTokenTTLSeconds, c.Description, c.CreatedBy).Scan(&id)
	return id, err
}

func (s *Store) UpdateClient(ctx context.Context, id uuid.UUID, name, environment, status, description string,
	accessTTL, refreshTTL int) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE clients SET client_name=$2, environment=$3, status=$4, description=$5,
			access_token_ttl_seconds=$6, refresh_token_ttl_seconds=$7 WHERE id=$1`,
		id, name, environment, status, description, accessTTL, refreshTTL)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RotateClientSecret stores a new client secret hash.
func (s *Store) RotateClientSecret(ctx context.Context, id uuid.UUID, hash string) error {
	_, err := s.pool.Exec(ctx, `UPDATE clients SET client_secret_hash=$2 WHERE id=$1`, id, hash)
	return err
}

func (s *Store) SetClientScopes(ctx context.Context, clientID uuid.UUID, codes []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM client_scopes WHERE client_id=$1`, clientID); err != nil {
		return err
	}
	for _, code := range codes {
		if _, err := tx.Exec(ctx,
			`INSERT INTO client_scopes (client_id, scope_id) SELECT $1, id FROM scopes WHERE scope_code=$2
			 ON CONFLICT DO NOTHING`, clientID, code); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) SetClientAPIs(ctx context.Context, clientID uuid.UUID, apiIDs []uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM client_api_access WHERE client_id=$1`, clientID); err != nil {
		return err
	}
	for _, apiID := range apiIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO client_api_access (client_id, api_definition_id, allowed) VALUES ($1,$2,true)
			 ON CONFLICT (client_id, api_definition_id) DO UPDATE SET allowed=true`, clientID, apiID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ClientHasAPIAccess reports whether a client is granted a given API.
func (s *Store) ClientHasAPIAccess(ctx context.Context, clientID, apiID uuid.UUID) (bool, error) {
	var ok bool
	err := s.get(ctx, &ok,
		`SELECT EXISTS(SELECT 1 FROM client_api_access WHERE client_id=$1 AND api_definition_id=$2 AND allowed=true)`,
		clientID, apiID)
	return ok, err
}

func (s *Store) DeleteClient(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM clients WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Refresh tokens ----

func (s *Store) CreateRefreshToken(ctx context.Context, hash string, clientID uuid.UUID, scope string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (token_hash, client_id, scope, expires_at) VALUES ($1,$2,$3,$4)`,
		hash, clientID, scope, expiresAt)
	return err
}

func (s *Store) GetRefreshToken(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var rt models.RefreshToken
	err := s.get(ctx, &rt,
		`SELECT id, token_hash, client_id, scope, expires_at, revoked, created_at FROM refresh_tokens WHERE token_hash=$1`,
		hash)
	return &rt, err
}

func (s *Store) RevokeRefreshToken(ctx context.Context, hash string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked=true WHERE token_hash=$1`, hash)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// RevokeClientRefreshTokens revokes all refresh tokens for a client.
func (s *Store) RevokeClientRefreshTokens(ctx context.Context, clientID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked=true WHERE client_id=$1`, clientID)
	return err
}

// DeleteExpiredRefreshTokens purges refresh tokens that expired before the
// retention cutoff. Returns the number removed. Used by the worker.
func (s *Store) DeleteExpiredRefreshTokens(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE expires_at < now() - $1::interval`, olderThan.String())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
