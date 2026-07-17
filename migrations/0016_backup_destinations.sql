-- Provider-neutral backup destinations. Configuration may contain non-secret
-- values only (endpoint, bucket, prefix, region). Credentials always reference
-- the envelope-encrypted secrets table.
CREATE TABLE backup_destinations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                TEXT NOT NULL UNIQUE,
    provider            TEXT NOT NULL CHECK (provider IN ('local','s3','b2','gdrive','sftp')),
    config              JSONB NOT NULL DEFAULT '{}'::jsonb,
    credential_secret_ref UUID REFERENCES secrets(id) ON DELETE SET NULL,
    status              TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','verified','enabled','failed','disabled')),
    last_verified_at    TIMESTAMPTZ,
    last_error          TEXT NOT NULL DEFAULT '',
    created_by          UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_backup_destinations_updated BEFORE UPDATE ON backup_destinations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_backup_destinations_provider_status ON backup_destinations(provider, status);

-- One zero-cost default makes the feature usable on a fresh OSS deployment.
INSERT INTO backup_destinations (name, provider, config, status)
VALUES ('local-encrypted', 'local', '{"path":"backups"}'::jsonb, 'enabled')
ON CONFLICT (name) DO NOTHING;

-- Destination-specific desired state replaces a single global provider selector.
INSERT INTO settings (key, value, category, value_type, scope, description, default_value, restart_required, updated_at)
VALUES ('backup.destination_id', 'null'::jsonb, 'Backups & Recovery', 'json', 'global', 'Selected verified destination for scheduled backups', 'null'::jsonb, false, now())
ON CONFLICT (key) DO NOTHING;
