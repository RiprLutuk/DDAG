package store

import (
	"context"

	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

// ---- Cache rules ----

func (s *Store) GetCacheRule(ctx context.Context, apiID uuid.UUID) (*models.CacheRule, error) {
	var c models.CacheRule
	err := s.get(ctx, &c, `
		SELECT id, api_definition_id, enabled, ttl_seconds, cache_key_strategy, vary_by_client,
			created_at, updated_at FROM cache_rules WHERE api_definition_id=$1`, apiID)
	return &c, err
}

// UpsertCacheRule creates or replaces the cache rule for an API.
func (s *Store) UpsertCacheRule(ctx context.Context, c *models.CacheRule) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO cache_rules (api_definition_id, enabled, ttl_seconds, cache_key_strategy, vary_by_client)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (api_definition_id) DO UPDATE SET
			enabled=EXCLUDED.enabled, ttl_seconds=EXCLUDED.ttl_seconds,
			cache_key_strategy=EXCLUDED.cache_key_strategy, vary_by_client=EXCLUDED.vary_by_client`,
		c.APIDefinitionID, c.Enabled, c.TTLSeconds, c.CacheKeyStrategy, c.VaryByClient)
	return err
}

func (s *Store) ListCacheRules(ctx context.Context) ([]models.CacheRule, error) {
	var out []models.CacheRule
	err := s.selectRows(ctx, &out, `
		SELECT id, api_definition_id, enabled, ttl_seconds, cache_key_strategy, vary_by_client,
			created_at, updated_at FROM cache_rules ORDER BY created_at DESC`)
	return out, err
}

// ---- Rate limit rules ----

const rateCols = `id, client_id, api_definition_id, applies_to, requests_per_second,
	requests_per_minute, requests_per_hour, requests_per_day, created_at, updated_at`

func (s *Store) ListRateLimitRules(ctx context.Context) ([]models.RateLimitRule, error) {
	var out []models.RateLimitRule
	err := s.selectRows(ctx, &out, `SELECT `+rateCols+` FROM rate_limit_rules ORDER BY created_at DESC`)
	return out, err
}

// RateLimitRulesFor returns rules that apply to a given client/API combination.
func (s *Store) RateLimitRulesFor(ctx context.Context, clientID, apiID uuid.UUID) ([]models.RateLimitRule, error) {
	var out []models.RateLimitRule
	err := s.selectRows(ctx, &out, `
		SELECT `+rateCols+` FROM rate_limit_rules
		WHERE (client_id=$1 OR client_id IS NULL)
		  AND (api_definition_id=$2 OR api_definition_id IS NULL)
		ORDER BY (client_id IS NOT NULL) DESC, (api_definition_id IS NOT NULL) DESC`, clientID, apiID)
	return out, err
}

func (s *Store) CreateRateLimitRule(ctx context.Context, r *models.RateLimitRule) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO rate_limit_rules (client_id, api_definition_id, applies_to, requests_per_second,
			requests_per_minute, requests_per_hour, requests_per_day)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		r.ClientID, r.APIDefinitionID, r.AppliesTo, r.RequestsPerSecond, r.RequestsPerMinute,
		r.RequestsPerHour, r.RequestsPerDay).Scan(&id)
	return id, err
}

func (s *Store) UpdateRateLimitRule(ctx context.Context, r *models.RateLimitRule) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE rate_limit_rules SET client_id=$2, api_definition_id=$3, applies_to=$4,
			requests_per_second=$5, requests_per_minute=$6, requests_per_hour=$7, requests_per_day=$8
		WHERE id=$1`,
		r.ID, r.ClientID, r.APIDefinitionID, r.AppliesTo, r.RequestsPerSecond, r.RequestsPerMinute,
		r.RequestsPerHour, r.RequestsPerDay)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteRateLimitRule(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM rate_limit_rules WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- IP whitelists ----

const ipCols = `id, client_id, api_definition_id, ip_cidr, scope_level, status, description, created_at, updated_at`

func (s *Store) ListIPWhitelists(ctx context.Context) ([]models.IPWhitelist, error) {
	var out []models.IPWhitelist
	err := s.selectRows(ctx, &out, `SELECT `+ipCols+` FROM ip_whitelists ORDER BY created_at DESC`)
	return out, err
}

// IPWhitelistsFor returns active whitelist entries that apply to a client/API
// (or are global). An empty result means "no restriction" (allow all).
func (s *Store) IPWhitelistsFor(ctx context.Context, clientID, apiID uuid.UUID) ([]models.IPWhitelist, error) {
	var out []models.IPWhitelist
	err := s.selectRows(ctx, &out, `
		SELECT `+ipCols+` FROM ip_whitelists
		WHERE status='active'
		  AND (scope_level='global' OR client_id=$1 OR api_definition_id=$2)`, clientID, apiID)
	return out, err
}

func (s *Store) CreateIPWhitelist(ctx context.Context, w *models.IPWhitelist) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO ip_whitelists (client_id, api_definition_id, ip_cidr, scope_level, status, description)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		w.ClientID, w.APIDefinitionID, w.IPCIDR, w.ScopeLevel, w.Status, w.Description).Scan(&id)
	return id, err
}

func (s *Store) UpdateIPWhitelist(ctx context.Context, w *models.IPWhitelist) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE ip_whitelists SET client_id=$2, api_definition_id=$3, ip_cidr=$4, scope_level=$5,
			status=$6, description=$7 WHERE id=$1`,
		w.ID, w.ClientID, w.APIDefinitionID, w.IPCIDR, w.ScopeLevel, w.Status, w.Description)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteIPWhitelist(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM ip_whitelists WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
