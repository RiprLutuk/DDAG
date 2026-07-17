-- Service control-plane registry. Static environment configuration remains valid;
-- this table adds optional metadata and health observations for managed services.
CREATE TABLE service_registry (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name               TEXT NOT NULL UNIQUE,
    kind               TEXT NOT NULL,
    base_url           TEXT NOT NULL DEFAULT '',
    enabled            BOOLEAN NOT NULL DEFAULT true,
    managed_by         TEXT NOT NULL DEFAULT 'static',
    version            TEXT NOT NULL DEFAULT '',
    commit_sha         TEXT NOT NULL DEFAULT '',
    health_url         TEXT NOT NULL DEFAULT '',
    ready_url          TEXT NOT NULL DEFAULT '',
    metrics_url        TEXT NOT NULL DEFAULT '',
    capabilities       JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_seen_at       TIMESTAMPTZ,
    last_health_status TEXT NOT NULL DEFAULT 'unknown',
    last_error         TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_service_registry_kind ON service_registry(kind);
CREATE INDEX idx_service_registry_status ON service_registry(last_health_status);
CREATE TRIGGER trg_service_registry_updated BEFORE UPDATE ON service_registry
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
