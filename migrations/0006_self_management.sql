-- v4 M3 self-management metadata.
ALTER TABLE settings
    ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'Feature flags',
    ADD COLUMN IF NOT EXISTS value_type TEXT NOT NULL DEFAULT 'json',
    ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'global',
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS default_value JSONB NOT NULL DEFAULT 'null'::jsonb,
    ADD COLUMN IF NOT EXISTS restart_required BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_settings_category ON settings(category);

CREATE TABLE maintenance_jobs (
    key TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'Maintenance',
    description TEXT NOT NULL DEFAULT '',
    safe BOOLEAN NOT NULL DEFAULT true,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_run_at TIMESTAMPTZ,
    last_status TEXT NOT NULL DEFAULT 'never',
    last_duration_ms INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    last_result JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_maintenance_jobs_updated BEFORE UPDATE ON maintenance_jobs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
