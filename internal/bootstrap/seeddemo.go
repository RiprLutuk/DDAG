package bootstrap

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/ddag/ddag/internal/auth"
	"github.com/ddag/ddag/internal/config"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/secret"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DemoClientSecret is the fixed dev secret for the seeded demo client so the
// consumer verification flow is reproducible.
func DemoClientSecret() string {
	if s := os.Getenv("DDAG_DEMO_CLIENT_SECRET"); s != "" {
		return s
	}
	return "demo-secret-brim-001"
}

// SeedDemo provisions a real source database (ddag_demo) with sample data, then
// registers a demo connection, two published APIs, and a demo client so the
// dashboard and gateway return live (non-dummy) data immediately. Idempotent.
func SeedDemo(ctx context.Context, pool *pgxpool.Pool, sec secret.Store, cfg config.Config, log *logging.Logger) error {
	if err := provisionDemoSourceDB(ctx, cfg.Metadata, log); err != nil {
		return fmt.Errorf("provision demo source db: %w", err)
	}

	// Demo connection password = empty (local trust). Store via secret store so
	// the secure-resolution path is exercised end to end.
	secretRef, err := sec.Put(ctx, []byte(cfg.Metadata.Password), "db_password")
	if err != nil {
		return err
	}

	var connID uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO database_connections
			(name, database_type, host, port, database_name, schema_name, username, secret_ref,
			 ssl_mode, min_pool_size, max_pool_size, connection_timeout_ms, query_timeout_ms,
			 environment, status, tags)
		VALUES ('demo-postgres','postgres',$1,$2,'ddag_demo','public',$3,$4,'disable',2,10,5000,30000,'dev','active',ARRAY['demo'])
		ON CONFLICT (name) DO UPDATE SET host=EXCLUDED.host, port=EXCLUDED.port, secret_ref=EXCLUDED.secret_ref,
			config_version=database_connections.config_version+1
		RETURNING id`,
		cfg.Metadata.Host, cfg.Metadata.Port, cfg.Metadata.User, secretRef).Scan(&connID)
	if err != nil {
		return err
	}

	// API 1: GET /api/v1/brim/sites/{site_id}
	sitesID, err := upsertAPI(ctx, pool, demoAPI{
		Name: "Get BRIM Site", Namespace: "brim", Path: "/api/v1/brim/sites/{site_id}", Method: "GET",
		Description: "Fetch a single BRIM site by id", ConnID: connID, RequiredScope: "brim.site.read",
		DefaultLimit: 1, MaxLimit: 1,
		// Pagination is applied by the connector. Do not include LIMIT here or the
		// connector would append a second LIMIT and produce invalid PostgreSQL.
		Query: "SELECT site_id, customer_name, status, created_at\nFROM customer_site\nWHERE site_id = :site_id",
	})
	if err != nil {
		return err
	}
	if err := upsertParam(ctx, pool, sitesID, "site_id", "path", "string", true, 50, 0); err != nil {
		return err
	}
	if err := upsertCacheRule(ctx, pool, sitesID, true, 300, true); err != nil {
		return err
	}

	// API 2: POST /api/v1/brim/workorders/search
	woID, err := upsertAPI(ctx, pool, demoAPI{
		Name: "Search BRIM Work Orders", Namespace: "brim", Path: "/api/v1/brim/workorders/search", Method: "POST",
		Description: "Search work orders, optionally by status", ConnID: connID, RequiredScope: "brim.wo.read",
		DefaultLimit: 100, MaxLimit: 500,
		Query: "SELECT wo_id, site_id, status, priority, created_at\nFROM work_orders\nWHERE (:status = '' OR status = :status)\nORDER BY created_at DESC",
	})
	if err != nil {
		return err
	}
	if err := upsertParamDefault(ctx, pool, woID, "status", "body", "string", false, 30, 0, ""); err != nil {
		return err
	}

	// Demo client app-brim.
	if err := seedDemoClient(ctx, pool, []uuid.UUID{sitesID, woID}, log); err != nil {
		return err
	}

	log.Info("demo_seed_complete", "connection", "demo-postgres", "apis", 2, "client", "app-brim")
	return nil
}

// provisionDemoSourceDB creates the ddag_demo database (if missing) and fills it
// with sample tables/rows by connecting to the same PostgreSQL server.
func provisionDemoSourceDB(ctx context.Context, m config.PostgresConfig, log *logging.Logger) error {
	adminConn, err := pgx.Connect(ctx, pgURL(m, "postgres"))
	if err != nil {
		return err
	}
	defer adminConn.Close(ctx)

	var exists bool
	if err := adminConn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname='ddag_demo')`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if _, err := adminConn.Exec(ctx, `CREATE DATABASE ddag_demo`); err != nil &&
			!strings.Contains(err.Error(), "already exists") {
			return err
		}
		log.Info("created_demo_database", "name", "ddag_demo")
	}

	demoConn, err := pgx.Connect(ctx, pgURL(m, "ddag_demo"))
	if err != nil {
		return err
	}
	defer demoConn.Close(ctx)

	if _, err := demoConn.Exec(ctx, demoSchemaSQL); err != nil {
		return err
	}
	return nil
}

const demoSchemaSQL = `
CREATE TABLE IF NOT EXISTS customer_site (
    site_id       TEXT PRIMARY KEY,
    customer_name TEXT NOT NULL,
    status        TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS work_orders (
    wo_id      TEXT PRIMARY KEY,
    site_id    TEXT NOT NULL,
    status     TEXT NOT NULL,
    priority   TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO customer_site (site_id, customer_name, status) VALUES
    ('ABC123','PT Maju Jaya','ACTIVE'),
    ('ABC124','CV Sentosa','ACTIVE'),
    ('ABC125','PT Nusantara','INACTIVE')
ON CONFLICT (site_id) DO NOTHING;
INSERT INTO work_orders (wo_id, site_id, status, priority) VALUES
    ('WO-1001','ABC123','OPEN','HIGH'),
    ('WO-1002','ABC123','CLOSED','LOW'),
    ('WO-1003','ABC124','OPEN','MEDIUM')
ON CONFLICT (wo_id) DO NOTHING;
`

type demoAPI struct {
	Name, Namespace, Path, Method, Description, RequiredScope, Query string
	ConnID                                                           uuid.UUID
	DefaultLimit, MaxLimit                                           int
}

func upsertAPI(ctx context.Context, pool *pgxpool.Pool, a demoAPI) (uuid.UUID, error) {
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO api_definitions
			(name, namespace, path, method, description, database_connection_id, connector_type,
			 query_template, status, required_scope, default_limit, max_limit, published_at)
		VALUES ($1,$2,$3,$4,$5,$6,'postgres',$7,'PUBLISHED',$8,$9,$10,now())
		ON CONFLICT (method, path) DO UPDATE SET
			name=EXCLUDED.name, description=EXCLUDED.description,
			database_connection_id=EXCLUDED.database_connection_id, query_template=EXCLUDED.query_template,
			status='PUBLISHED', required_scope=EXCLUDED.required_scope, default_limit=EXCLUDED.default_limit,
			max_limit=EXCLUDED.max_limit, published_at=now()
		RETURNING id`,
		a.Name, a.Namespace, a.Path, a.Method, a.Description, a.ConnID, a.Query, a.RequiredScope,
		a.DefaultLimit, a.MaxLimit).Scan(&id)
	return id, err
}

func upsertParam(ctx context.Context, pool *pgxpool.Pool, apiID uuid.UUID, name, source, ptype string, required bool, maxLen, pos int) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO api_parameters (api_definition_id, name, source, param_type, required, max_length, position)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (api_definition_id, name) DO UPDATE SET source=EXCLUDED.source,
			param_type=EXCLUDED.param_type, required=EXCLUDED.required, max_length=EXCLUDED.max_length`,
		apiID, name, source, ptype, required, maxLen, pos)
	return err
}

func upsertParamDefault(ctx context.Context, pool *pgxpool.Pool, apiID uuid.UUID, name, source, ptype string, required bool, maxLen, pos int, def string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO api_parameters (api_definition_id, name, source, param_type, required, max_length, position, default_value)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (api_definition_id, name) DO UPDATE SET source=EXCLUDED.source,
			param_type=EXCLUDED.param_type, required=EXCLUDED.required, max_length=EXCLUDED.max_length,
			default_value=EXCLUDED.default_value`,
		apiID, name, source, ptype, required, maxLen, pos, def)
	return err
}

func upsertCacheRule(ctx context.Context, pool *pgxpool.Pool, apiID uuid.UUID, enabled bool, ttl int, varyByClient bool) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO cache_rules (api_definition_id, enabled, ttl_seconds, vary_by_client)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (api_definition_id) DO UPDATE SET enabled=EXCLUDED.enabled,
			ttl_seconds=EXCLUDED.ttl_seconds, vary_by_client=EXCLUDED.vary_by_client`,
		apiID, enabled, ttl, varyByClient)
	return err
}

func seedDemoClient(ctx context.Context, pool *pgxpool.Pool, apiIDs []uuid.UUID, log *logging.Logger) error {
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clients WHERE client_id='app-brim')`).Scan(&exists); err != nil {
		return err
	}
	var clientID uuid.UUID
	if !exists {
		hash, err := auth.HashPassword(DemoClientSecret())
		if err != nil {
			return err
		}
		if err := pool.QueryRow(ctx, `
			INSERT INTO clients (client_id, client_name, client_secret_hash, environment, status, description)
			VALUES ('app-brim','BRIM Application',$1,'dev','active','Seeded demo client')
			RETURNING id`, hash).Scan(&clientID); err != nil {
			return err
		}
		log.Info("seeded_demo_client", "client_id", "app-brim", "client_secret", DemoClientSecret())
	} else {
		if err := pool.QueryRow(ctx, `SELECT id FROM clients WHERE client_id='app-brim'`).Scan(&clientID); err != nil {
			return err
		}
	}

	// Scopes.
	for _, code := range []string{"brim.site.read", "brim.wo.read"} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO client_scopes (client_id, scope_id) SELECT $1, id FROM scopes WHERE scope_code=$2
			 ON CONFLICT DO NOTHING`, clientID, code); err != nil {
			return err
		}
	}
	// API access.
	for _, apiID := range apiIDs {
		if _, err := pool.Exec(ctx,
			`INSERT INTO client_api_access (client_id, api_definition_id, allowed) VALUES ($1,$2,true)
			 ON CONFLICT (client_id, api_definition_id) DO UPDATE SET allowed=true`, clientID, apiID); err != nil {
			return err
		}
	}
	// A generous rate limit rule for the demo client.
	var hasRule bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM rate_limit_rules WHERE client_id=$1 AND api_definition_id IS NULL)`, clientID).Scan(&hasRule); err != nil {
		return err
	}
	if !hasRule {
		if _, err := pool.Exec(ctx, `
			INSERT INTO rate_limit_rules (client_id, applies_to, requests_per_minute, requests_per_day)
			VALUES ($1,'client',300,100000)`, clientID); err != nil {
			return err
		}
	}
	return nil
}

// pgURL builds a pgx URL DSN for a given database on the configured server,
// handling empty passwords (local trust auth) correctly.
func pgURL(m config.PostgresConfig, dbname string) string {
	ssl := m.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	u := url.URL{
		Scheme:   "postgres",
		Host:     fmt.Sprintf("%s:%d", m.Host, m.Port),
		Path:     "/" + dbname,
		RawQuery: "sslmode=" + ssl,
	}
	if m.Password != "" {
		u.User = url.UserPassword(m.User, m.Password)
	} else {
		u.User = url.User(m.User)
	}
	return u.String()
}
