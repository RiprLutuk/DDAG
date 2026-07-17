package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/ddag/ddag/internal/models"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
)

// ---- Database connections ----

const connCols = `id, name, database_type, host, port, database_name, service_name, schema_name,
	username, secret_ref, ssl_mode, min_pool_size, max_pool_size, connection_timeout_ms,
	query_timeout_ms, max_conn_lifetime_ms, max_conn_idle_ms, environment, status, tags,
	config_version, last_health_status, last_health_at, created_by, created_at, updated_at`

func (s *Store) ListConnections(ctx context.Context, p ListParams) ([]models.DatabaseConnection, int64, error) {
	p.Normalize()
	var total int64
	if err := s.get(ctx, &total,
		`SELECT count(*) FROM database_connections WHERE ($1='' OR name ILIKE '%'||$1||'%')`, p.Search); err != nil {
		return nil, 0, err
	}
	var conns []models.DatabaseConnection
	err := s.selectRows(ctx, &conns,
		`SELECT `+connCols+` FROM database_connections
		 WHERE ($1='' OR name ILIKE '%'||$1||'%')
		 ORDER BY `+p.OrderBy(map[string]string{"name": "name", "type": "database_type", "status": "status", "health": "last_health_status", "created_at": "created_at"}, "created_at")+` LIMIT $2 OFFSET $3`, p.Search, p.Limit, p.Offset())
	return conns, total, err
}

func (s *Store) ListAllConnections(ctx context.Context) ([]models.DatabaseConnection, error) {
	var conns []models.DatabaseConnection
	err := s.selectRows(ctx, &conns, `SELECT `+connCols+` FROM database_connections ORDER BY name`)
	return conns, err
}

func (s *Store) GetConnection(ctx context.Context, id uuid.UUID) (*models.DatabaseConnection, error) {
	var c models.DatabaseConnection
	err := s.get(ctx, &c, `SELECT `+connCols+` FROM database_connections WHERE id=$1`, id)
	return &c, err
}

func (s *Store) CreateConnection(ctx context.Context, c *models.DatabaseConnection) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO database_connections (name, database_type, host, port, database_name, service_name,
			schema_name, username, secret_ref, ssl_mode, min_pool_size, max_pool_size, connection_timeout_ms,
			query_timeout_ms, max_conn_lifetime_ms, max_conn_idle_ms, environment, status, tags, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20) RETURNING id`,
		c.Name, c.DatabaseType, c.Host, c.Port, c.DatabaseName, c.ServiceName, c.SchemaName, c.Username,
		c.SecretRef, c.SSLMode, c.MinPoolSize, c.MaxPoolSize, c.ConnectionTimeoutMS, c.QueryTimeoutMS,
		c.MaxConnLifetimeMS, c.MaxConnIdleMS, c.Environment, c.Status, c.Tags, c.CreatedBy).Scan(&id)
	return id, err
}

// UpdateConnection updates connection fields and bumps config_version so any
// connector pools keyed on this connection get rebuilt.
func (s *Store) UpdateConnection(ctx context.Context, c *models.DatabaseConnection) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE database_connections SET name=$2, database_type=$3, host=$4, port=$5, database_name=$6,
			service_name=$7, schema_name=$8, username=$9, ssl_mode=$10, min_pool_size=$11, max_pool_size=$12,
			connection_timeout_ms=$13, query_timeout_ms=$14, max_conn_lifetime_ms=$15, max_conn_idle_ms=$16,
			environment=$17, status=$18, tags=$19, config_version=config_version+1
		WHERE id=$1`,
		c.ID, c.Name, c.DatabaseType, c.Host, c.Port, c.DatabaseName, c.ServiceName, c.SchemaName,
		c.Username, c.SSLMode, c.MinPoolSize, c.MaxPoolSize, c.ConnectionTimeoutMS, c.QueryTimeoutMS,
		c.MaxConnLifetimeMS, c.MaxConnIdleMS, c.Environment, c.Status, c.Tags)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetConnectionSecret points a connection at an (encrypted) secret and bumps version.
func (s *Store) SetConnectionSecret(ctx context.Context, id, secretRef uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE database_connections SET secret_ref=$2, config_version=config_version+1 WHERE id=$1`, id, secretRef)
	return err
}

func (s *Store) SetConnectionHealth(ctx context.Context, id uuid.UUID, status string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE database_connections SET last_health_status=$2, last_health_at=now() WHERE id=$1`, id, status)
	return err
}

func (s *Store) DeleteConnection(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM database_connections WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- API definitions ----

const apiCols = `id, name, namespace, path, method, description, database_connection_id, connector_type,
	query_template, response_mapping, status, version, required_scope, default_limit, max_limit, is_write,
	created_by, approved_by, approved_comment, published_at, deprecated_at, created_at, updated_at`

func (s *Store) ListAPIs(ctx context.Context, p ListParams, status string) ([]models.APIDefinition, int64, error) {
	p.Normalize()
	var total int64
	if err := s.get(ctx, &total, `
		SELECT count(*) FROM api_definitions
		WHERE ($1='' OR name ILIKE '%'||$1||'%' OR path ILIKE '%'||$1||'%')
		  AND ($2='' OR status=$2)`, p.Search, status); err != nil {
		return nil, 0, err
	}
	var apis []models.APIDefinition
	if err := s.selectRows(ctx, &apis, `
		SELECT `+apiCols+` FROM api_definitions
		WHERE ($1='' OR name ILIKE '%'||$1||'%' OR path ILIKE '%'||$1||'%')
		  AND ($2='' OR status=$2)
		ORDER BY `+p.OrderBy(map[string]string{"name": "name", "route": "path", "connection_name": "database_connection_id", "scope": "required_scope", "status": "status", "created_at": "created_at"}, "created_at")+` LIMIT $3 OFFSET $4`, p.Search, status, p.Limit, p.Offset()); err != nil {
		return nil, 0, err
	}
	// attach connection name for display
	for i := range apis {
		if apis[i].DatabaseConnectionID != nil {
			_ = s.get(ctx, &apis[i].ConnectionName,
				`SELECT name FROM database_connections WHERE id=$1`, *apis[i].DatabaseConnectionID)
		}
	}
	return apis, total, nil
}

func (s *Store) GetAPI(ctx context.Context, id uuid.UUID) (*models.APIDefinition, error) {
	var a models.APIDefinition
	if err := s.get(ctx, &a, `SELECT `+apiCols+` FROM api_definitions WHERE id=$1`, id); err != nil {
		return nil, err
	}
	params, err := s.ListAPIParameters(ctx, id)
	if err != nil {
		return nil, err
	}
	a.Parameters = params
	if a.DatabaseConnectionID != nil {
		_ = s.get(ctx, &a.ConnectionName, `SELECT name FROM database_connections WHERE id=$1`, *a.DatabaseConnectionID)
	}
	return &a, nil
}

// listPublishedRevisionSnapshotsSQL only loads revisions whose current API
// definition is live. A historical revision must never keep a disabled,
// deprecated, or archived API routable in the gateway.
const listPublishedRevisionSnapshotsSQL = `
	SELECT DISTINCT ON (r.api_definition_id) r.snapshot
	FROM api_revisions r
	JOIN api_definitions a ON a.id = r.api_definition_id
	WHERE a.status = 'PUBLISHED'
	ORDER BY r.api_definition_id, r.revision DESC`

// ListPublishedAPIs returns every currently published API from immutable
// revision snapshots. This lets operators edit a new draft without changing the
// live gateway catalog until publish creates the next revision.
func (s *Store) ListPublishedAPIs(ctx context.Context) ([]models.APIDefinition, error) {
	var snapshots []json.RawMessage
	if err := s.selectRows(ctx, &snapshots, listPublishedRevisionSnapshotsSQL); err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		var apis []models.APIDefinition
		if err := s.selectRows(ctx, &apis,
			`SELECT `+apiCols+` FROM api_definitions WHERE status='PUBLISHED' ORDER BY path`); err != nil {
			return nil, err
		}
		for i := range apis {
			params, err := s.ListAPIParameters(ctx, apis[i].ID)
			if err != nil {
				return nil, err
			}
			apis[i].Parameters = params
		}
		return apis, nil
	}
	apis := make([]models.APIDefinition, 0, len(snapshots))
	for _, raw := range snapshots {
		var api models.APIDefinition
		if err := json.Unmarshal(raw, &api); err != nil {
			return nil, err
		}
		apis = append(apis, api)
	}
	return apis, nil
}

func (s *Store) CreateAPIWithParameters(ctx context.Context, a *models.APIDefinition, params []models.APIParameter) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO api_definitions (name, namespace, path, method, description, database_connection_id,
			connector_type, query_template, response_mapping, status, required_scope, default_limit, max_limit,
			is_write, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id`,
		a.Name, a.Namespace, a.Path, a.Method, a.Description, a.DatabaseConnectionID, a.ConnectorType,
		a.QueryTemplate, a.ResponseMapping, a.Status, a.RequiredScope, a.DefaultLimit, a.MaxLimit,
		a.IsWrite, a.CreatedBy).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}

	for i, p := range params {
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_parameters (api_definition_id, name, source, param_type, required,
				default_value, max_length, validation_rule, position)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			id, p.Name, p.Source, p.ParamType, p.Required, p.DefaultValue, p.MaxLength, p.ValidationRule, i); err != nil {
			return uuid.Nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *Store) UpdateAPIWithParameters(ctx context.Context, a *models.APIDefinition, params []models.APIParameter) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE api_definitions SET name=$2, namespace=$3, path=$4, method=$5, description=$6,
			database_connection_id=$7, connector_type=$8, query_template=$9, response_mapping=$10,
			required_scope=$11, default_limit=$12, max_limit=$13, is_write=$14,
			status = CASE WHEN status='PUBLISHED' THEN 'DRAFT' ELSE status END,
			version = CASE WHEN status='PUBLISHED' THEN version+1 ELSE version END
		WHERE id=$1`,
		a.ID, a.Name, a.Namespace, a.Path, a.Method, a.Description, a.DatabaseConnectionID, a.ConnectorType,
		a.QueryTemplate, a.ResponseMapping, a.RequiredScope, a.DefaultLimit, a.MaxLimit, a.IsWrite)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM api_parameters WHERE api_definition_id=$1`, a.ID); err != nil {
		return err
	}
	for i, p := range params {
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_parameters (api_definition_id, name, source, param_type, required,
				default_value, max_length, validation_rule, position)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			a.ID, p.Name, p.Source, p.ParamType, p.Required, p.DefaultValue, p.MaxLength, p.ValidationRule, i); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) CreateAPI(ctx context.Context, a *models.APIDefinition) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO api_definitions (name, namespace, path, method, description, database_connection_id,
			connector_type, query_template, response_mapping, status, required_scope, default_limit, max_limit,
			is_write, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id`,
		a.Name, a.Namespace, a.Path, a.Method, a.Description, a.DatabaseConnectionID, a.ConnectorType,
		a.QueryTemplate, a.ResponseMapping, a.Status, a.RequiredScope, a.DefaultLimit, a.MaxLimit,
		a.IsWrite, a.CreatedBy).Scan(&id)
	return id, err
}

func (s *Store) UpdateAPI(ctx context.Context, a *models.APIDefinition) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE api_definitions SET name=$2, namespace=$3, path=$4, method=$5, description=$6,
			database_connection_id=$7, connector_type=$8, query_template=$9, response_mapping=$10,
			required_scope=$11, default_limit=$12, max_limit=$13, is_write=$14,
			status = CASE WHEN status='PUBLISHED' THEN 'DRAFT' ELSE status END,
			version = CASE WHEN status='PUBLISHED' THEN version+1 ELSE version END
		WHERE id=$1`,
		a.ID, a.Name, a.Namespace, a.Path, a.Method, a.Description, a.DatabaseConnectionID, a.ConnectorType,
		a.QueryTemplate, a.ResponseMapping, a.RequiredScope, a.DefaultLimit, a.MaxLimit, a.IsWrite)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAPIStatus transitions an API's lifecycle status.
func (s *Store) SetAPIStatus(ctx context.Context, id uuid.UUID, status string, actor *uuid.UUID) error {
	_ = actor
	tag, err := s.pool.Exec(ctx, `
		UPDATE api_definitions SET status=$2,
			published_at = CASE WHEN $2='PUBLISHED' THEN now() ELSE published_at END,
			deprecated_at = CASE WHEN $2='DEPRECATED' THEN now() ELSE deprecated_at END
		WHERE id=$1`, id, status, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ApproveAPI(ctx context.Context, id uuid.UUID, actor uuid.UUID, comment string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE api_definitions
		SET status='APPROVED', approved_by=$2, approved_comment=$3
		WHERE id=$1`, id, actor, comment)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PublishAPI creates an immutable revision and changes the API state in one
// database transaction. A failed status transition therefore cannot leave a
// revision that the gateway may later discover.
func (s *Store) PublishAPI(ctx context.Context, api *models.APIDefinition, publishedBy uuid.UUID) (*models.APIRevision, error) {
	raw, err := json.Marshal(api)
	if err != nil {
		return nil, err
	}
	checksum := sha256.Sum256(raw)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var rev models.APIRevision
	err = pgxscan.Get(ctx, tx, &rev, `
		WITH next_revision AS (
			SELECT COALESCE(MAX(revision), 0) + 1 AS revision
			FROM api_revisions WHERE api_definition_id=$1
		)
		INSERT INTO api_revisions (
			api_definition_id, revision, snapshot, snapshot_checksum,
			approved_by, approved_comment, published_by, published_at
		)
		SELECT $1, next_revision.revision, $2, $3, $4, $5, $6, now()
		FROM next_revision
		RETURNING id, api_definition_id, revision, snapshot, snapshot_checksum,
			approved_by, approved_comment, published_by, published_at, created_at`,
		api.ID, raw, hex.EncodeToString(checksum[:]), api.ApprovedBy, api.ApprovedComment, publishedBy)
	if err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE api_definitions SET status='PUBLISHED', published_at=now()
		WHERE id=$1 AND status='APPROVED'`, api.ID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &rev, nil
}

// CreateAPIRevision remains available to callers that explicitly need to save a
// revision without publishing. Normal lifecycle publishing must use PublishAPI.
func (s *Store) CreateAPIRevision(ctx context.Context, apiID uuid.UUID, publishedBy uuid.UUID) (*models.APIRevision, error) {
	api, err := s.GetAPI(ctx, apiID)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(api)
	if err != nil {
		return nil, err
	}
	checksum := sha256.Sum256(raw)
	var rev models.APIRevision
	err = s.get(ctx, &rev, `
		WITH next_revision AS (
			SELECT COALESCE(MAX(revision), 0) + 1 AS revision
			FROM api_revisions WHERE api_definition_id=$1
		)
		INSERT INTO api_revisions (
			api_definition_id, revision, snapshot, snapshot_checksum,
			approved_by, approved_comment, published_by, published_at
		)
		SELECT $1, next_revision.revision, $2, $3, $4, $5, $6, now()
		FROM next_revision
		RETURNING id, api_definition_id, revision, snapshot, snapshot_checksum,
			approved_by, approved_comment, published_by, published_at, created_at`,
		apiID, raw, hex.EncodeToString(checksum[:]), api.ApprovedBy, api.ApprovedComment, publishedBy)
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

func (s *Store) ListAPIRevisions(ctx context.Context, apiID uuid.UUID) ([]models.APIRevision, error) {
	var rows []models.APIRevision
	if err := s.selectRows(ctx, &rows, `
		SELECT id, api_definition_id, revision, snapshot, snapshot_checksum,
			approved_by, approved_comment, published_by, published_at, created_at
		FROM api_revisions
		WHERE api_definition_id=$1
		ORDER BY revision DESC`, apiID); err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []models.APIRevision{}
	}
	return rows, nil
}

func (s *Store) GetLatestAPIRevision(ctx context.Context, apiID uuid.UUID) (*models.APIRevision, error) {
	var rev models.APIRevision
	if err := s.get(ctx, &rev, `
		SELECT id, api_definition_id, revision, snapshot, snapshot_checksum,
			approved_by, approved_comment, published_by, published_at, created_at
		FROM api_revisions WHERE api_definition_id=$1 ORDER BY revision DESC LIMIT 1`, apiID); err != nil {
		return nil, err
	}
	return &rev, nil
}

func (s *Store) AddAPILifecycleEvent(ctx context.Context, apiID uuid.UUID, fromStatus, toStatus string, actor *uuid.UUID, comment string, metadata json.RawMessage) error {
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO api_lifecycle_events (api_definition_id, from_status, to_status, actor, comment, metadata)
		VALUES ($1,$2,$3,$4,$5,$6)`, apiID, fromStatus, toStatus, actor, comment, metadata)
	return err
}

func (s *Store) DeleteAPI(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM api_definitions WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- API parameters ----

func (s *Store) ListAPIParameters(ctx context.Context, apiID uuid.UUID) ([]models.APIParameter, error) {
	var params []models.APIParameter
	err := s.selectRows(ctx, &params, `
		SELECT id, api_definition_id, name, source, param_type, required, default_value, max_length,
			validation_rule, position, created_at, updated_at
		FROM api_parameters WHERE api_definition_id=$1 ORDER BY position, name`, apiID)
	if params == nil {
		params = []models.APIParameter{}
	}
	return params, err
}

// ReplaceAPIParameters atomically replaces an API's parameter set.
func (s *Store) ReplaceAPIParameters(ctx context.Context, apiID uuid.UUID, params []models.APIParameter) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM api_parameters WHERE api_definition_id=$1`, apiID); err != nil {
		return err
	}
	for i, p := range params {
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_parameters (api_definition_id, name, source, param_type, required,
				default_value, max_length, validation_rule, position)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			apiID, p.Name, p.Source, p.ParamType, p.Required, p.DefaultValue, p.MaxLength, p.ValidationRule, i); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
