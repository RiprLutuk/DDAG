-- Append-only audit log and high-volume request log.

CREATE TABLE audit_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id    TEXT NOT NULL DEFAULT '',
    actor_type    TEXT NOT NULL DEFAULT 'system' CHECK (actor_type IN ('user','client','system')),
    actor_id      TEXT NOT NULL DEFAULT '',
    actor_label   TEXT NOT NULL DEFAULT '',
    action        TEXT NOT NULL,
    resource_type TEXT NOT NULL DEFAULT '',
    resource_id   TEXT NOT NULL DEFAULT '',
    ip_address    TEXT NOT NULL DEFAULT '',
    user_agent    TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'success' CHECK (status IN ('success','failure')),
    metadata_json JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_created ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_action ON audit_logs(action);
CREATE INDEX idx_audit_actor ON audit_logs(actor_type, actor_id);
CREATE INDEX idx_audit_resource ON audit_logs(resource_type, resource_id);

-- audit_logs is append-only: revoke UPDATE/DELETE at the app layer (the app
-- connects as a role without these grants in production). A trigger blocks
-- mutation defensively (PRD §11.14 AC: audit log cannot be edited).
CREATE OR REPLACE FUNCTION block_audit_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_audit_no_update BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION block_audit_mutation();

CREATE TABLE api_request_logs (
    id                    BIGSERIAL PRIMARY KEY,
    request_id            TEXT NOT NULL DEFAULT '',
    client_id             UUID,
    api_definition_id     UUID,
    client_label          TEXT NOT NULL DEFAULT '',
    api_label             TEXT NOT NULL DEFAULT '',
    method                TEXT NOT NULL DEFAULT '',
    path                  TEXT NOT NULL DEFAULT '',
    status_code           INT  NOT NULL DEFAULT 0,
    error_code            TEXT NOT NULL DEFAULT '',
    latency_ms            INT  NOT NULL DEFAULT 0,
    cached                BOOLEAN NOT NULL DEFAULT false,
    source_db_duration_ms INT  NOT NULL DEFAULT 0,
    ip_address            TEXT NOT NULL DEFAULT '',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_reqlog_created ON api_request_logs(created_at DESC);
CREATE INDEX idx_reqlog_client ON api_request_logs(client_id);
CREATE INDEX idx_reqlog_api ON api_request_logs(api_definition_id);
CREATE INDEX idx_reqlog_status ON api_request_logs(status_code);
