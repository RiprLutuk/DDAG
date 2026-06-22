-- OAuth2 clients, scopes and refresh tokens.

CREATE TABLE scopes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_code  TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE clients (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id                 TEXT NOT NULL UNIQUE,
    client_name               TEXT NOT NULL,
    client_secret_hash        TEXT NOT NULL,
    owner_user_id             UUID REFERENCES users(id) ON DELETE SET NULL,
    environment               TEXT NOT NULL DEFAULT 'dev' CHECK (environment IN ('dev','staging','prod')),
    status                    TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive')),
    access_token_ttl_seconds  INT  NOT NULL DEFAULT 3600,
    refresh_token_ttl_seconds INT  NOT NULL DEFAULT 2592000,
    description               TEXT NOT NULL DEFAULT '',
    created_by                UUID,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_clients_updated BEFORE UPDATE ON clients
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE client_scopes (
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    scope_id  UUID NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    PRIMARY KEY (client_id, scope_id)
);

-- Opaque, revocable refresh tokens. Only the sha256 hash is stored.
CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash TEXT NOT NULL UNIQUE,
    client_id  UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    scope      TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    revoked    BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_tokens_client ON refresh_tokens(client_id);
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens(expires_at);
