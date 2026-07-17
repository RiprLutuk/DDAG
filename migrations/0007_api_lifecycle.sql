-- v4 M4 API lifecycle: approvals, immutable revisions, and promotion metadata.
ALTER TABLE api_definitions DROP CONSTRAINT IF EXISTS api_definitions_status_check;
ALTER TABLE api_definitions
    ADD CONSTRAINT api_definitions_status_check
    CHECK (status IN ('DRAFT','REVIEW','APPROVED','PUBLISHED','DEPRECATED','DISABLED','ARCHIVED'));

ALTER TABLE api_definitions
    ADD COLUMN IF NOT EXISTS approved_comment TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS deprecated_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS api_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_definition_id UUID NOT NULL REFERENCES api_definitions(id) ON DELETE CASCADE,
    revision INT NOT NULL,
    snapshot JSONB NOT NULL,
    snapshot_checksum TEXT NOT NULL,
    approved_by UUID,
    approved_comment TEXT NOT NULL DEFAULT '',
    published_by UUID,
    published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (api_definition_id, revision)
);
CREATE INDEX IF NOT EXISTS idx_api_revisions_api ON api_revisions(api_definition_id, revision DESC);
CREATE INDEX IF NOT EXISTS idx_api_revisions_published_at ON api_revisions(published_at DESC);

CREATE TABLE IF NOT EXISTS api_lifecycle_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_definition_id UUID NOT NULL REFERENCES api_definitions(id) ON DELETE CASCADE,
    from_status TEXT NOT NULL DEFAULT '',
    to_status TEXT NOT NULL,
    actor UUID,
    comment TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_api_lifecycle_events_api ON api_lifecycle_events(api_definition_id, created_at DESC);
