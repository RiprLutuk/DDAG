-- Immutable evidence for logical backup and isolated restore-drill executions.
CREATE TABLE backup_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind            TEXT NOT NULL CHECK (kind IN ('logical_backup','restore_drill')),
    status          TEXT NOT NULL CHECK (status IN ('running','succeeded','failed')),
    destination_id  UUID REFERENCES backup_destinations(id) ON DELETE SET NULL,
    artifact_path   TEXT NOT NULL DEFAULT '',
    manifest_path   TEXT NOT NULL DEFAULT '',
    sha256          TEXT NOT NULL DEFAULT '',
    bytes           BIGINT NOT NULL DEFAULT 0,
    database_name   TEXT NOT NULL DEFAULT '',
    detail          JSONB NOT NULL DEFAULT '{}'::jsonb,
    error           TEXT NOT NULL DEFAULT '',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX idx_backup_runs_kind_started ON backup_runs(kind, started_at DESC);
CREATE INDEX idx_backup_runs_status_started ON backup_runs(status, started_at DESC);

INSERT INTO settings (key, value, category, value_type, scope, description, default_value, restart_required, updated_at)
VALUES
 ('backup.runner_enabled', 'false'::jsonb, 'Backups & Recovery', 'boolean', 'global', 'Enable the systemd logical backup timer only after a successful restore drill', 'false'::jsonb, false, now()),
 ('backup.retention_days', '30'::jsonb, 'Backups & Recovery', 'integer', 'global', 'Keep encrypted logical backup artifacts for this many days', '30'::jsonb, false, now())
ON CONFLICT (key) DO NOTHING;
