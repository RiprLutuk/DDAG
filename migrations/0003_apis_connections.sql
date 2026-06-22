-- Source database connections, dynamic API definitions, parameters, and the
-- per-endpoint policy/cache rules.

CREATE TABLE database_connections (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                  TEXT NOT NULL UNIQUE,
    database_type         TEXT NOT NULL CHECK (database_type IN ('postgres','mysql','oracle','sqlserver')),
    host                  TEXT NOT NULL,
    port                  INT  NOT NULL,
    database_name         TEXT NOT NULL DEFAULT '',
    service_name          TEXT NOT NULL DEFAULT '',
    schema_name           TEXT NOT NULL DEFAULT '',
    username              TEXT NOT NULL,
    secret_ref            UUID REFERENCES secrets(id),
    ssl_mode              TEXT NOT NULL DEFAULT 'disable',
    -- Per-connection pool configuration (PRD §27: every connection has a pool).
    min_pool_size         INT  NOT NULL DEFAULT 2,
    max_pool_size         INT  NOT NULL DEFAULT 10,
    connection_timeout_ms INT  NOT NULL DEFAULT 5000,
    query_timeout_ms      INT  NOT NULL DEFAULT 30000,
    max_conn_lifetime_ms  INT  NOT NULL DEFAULT 3600000,
    max_conn_idle_ms      INT  NOT NULL DEFAULT 1800000,
    environment           TEXT NOT NULL DEFAULT 'dev' CHECK (environment IN ('dev','staging','prod')),
    status                TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive')),
    tags                  TEXT[] NOT NULL DEFAULT '{}',
    -- Bumped on every config change so connector pools can be invalidated.
    config_version        INT  NOT NULL DEFAULT 1,
    last_health_status    TEXT NOT NULL DEFAULT 'unknown',
    last_health_at        TIMESTAMPTZ,
    created_by            UUID,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_connections_updated BEFORE UPDATE ON database_connections
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE api_definitions (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                   TEXT NOT NULL,
    namespace              TEXT NOT NULL DEFAULT '',
    path                   TEXT NOT NULL,
    method                 TEXT NOT NULL CHECK (method IN ('GET','POST')),
    description            TEXT NOT NULL DEFAULT '',
    database_connection_id UUID REFERENCES database_connections(id) ON DELETE RESTRICT,
    connector_type         TEXT NOT NULL,
    query_template         TEXT NOT NULL,
    response_mapping       JSONB,
    status                 TEXT NOT NULL DEFAULT 'DRAFT'
                              CHECK (status IN ('DRAFT','REVIEW','PUBLISHED','DISABLED','ARCHIVED')),
    version                INT  NOT NULL DEFAULT 1,
    required_scope         TEXT NOT NULL DEFAULT '',
    default_limit          INT  NOT NULL DEFAULT 100,
    max_limit              INT  NOT NULL DEFAULT 1000,
    is_write               BOOLEAN NOT NULL DEFAULT false,
    created_by             UUID,
    approved_by            UUID,
    published_at           TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- A method+path pair uniquely identifies a route.
    UNIQUE (method, path)
);
CREATE TRIGGER trg_apis_updated BEFORE UPDATE ON api_definitions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX idx_apis_status ON api_definitions(status);

CREATE TABLE api_parameters (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_definition_id UUID NOT NULL REFERENCES api_definitions(id) ON DELETE CASCADE,
    name              TEXT NOT NULL,
    source            TEXT NOT NULL CHECK (source IN ('path','query','body')),
    param_type        TEXT NOT NULL DEFAULT 'string'
                         CHECK (param_type IN ('string','int','number','bool','uuid','date')),
    required          BOOLEAN NOT NULL DEFAULT false,
    default_value     TEXT,
    max_length        INT,
    validation_rule   TEXT,
    position          INT NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (api_definition_id, name)
);

CREATE TABLE client_api_access (
    client_id         UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    api_definition_id UUID NOT NULL REFERENCES api_definitions(id) ON DELETE CASCADE,
    allowed           BOOLEAN NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (client_id, api_definition_id)
);

CREATE TABLE cache_rules (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_definition_id  UUID NOT NULL UNIQUE REFERENCES api_definitions(id) ON DELETE CASCADE,
    enabled            BOOLEAN NOT NULL DEFAULT false,
    ttl_seconds        INT NOT NULL DEFAULT 300,
    cache_key_strategy TEXT NOT NULL DEFAULT 'client_id:path:query_params',
    vary_by_client     BOOLEAN NOT NULL DEFAULT true,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_cache_rules_updated BEFORE UPDATE ON cache_rules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE rate_limit_rules (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id           UUID REFERENCES clients(id) ON DELETE CASCADE,
    api_definition_id   UUID REFERENCES api_definitions(id) ON DELETE CASCADE,
    applies_to          TEXT NOT NULL DEFAULT 'client'
                           CHECK (applies_to IN ('client','api','ip','global')),
    requests_per_second INT NOT NULL DEFAULT 0,
    requests_per_minute INT NOT NULL DEFAULT 0,
    requests_per_hour   INT NOT NULL DEFAULT 0,
    requests_per_day    INT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_rate_rules_updated BEFORE UPDATE ON rate_limit_rules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX idx_rate_rules_client ON rate_limit_rules(client_id);
CREATE INDEX idx_rate_rules_api ON rate_limit_rules(api_definition_id);

CREATE TABLE ip_whitelists (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id         UUID REFERENCES clients(id) ON DELETE CASCADE,
    api_definition_id UUID REFERENCES api_definitions(id) ON DELETE CASCADE,
    ip_cidr           TEXT NOT NULL,
    scope_level       TEXT NOT NULL DEFAULT 'client'
                         CHECK (scope_level IN ('global','client','api')),
    status            TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive')),
    description       TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_ip_whitelist_updated BEFORE UPDATE ON ip_whitelists
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX idx_ip_whitelist_client ON ip_whitelists(client_id);
