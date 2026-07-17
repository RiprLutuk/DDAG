-- DDAG metadata schema: identity, RBAC, secrets, signing keys, settings.
-- Target: PostgreSQL 18. gen_random_uuid() is built-in (no extension needed).

-- Shared trigger to keep updated_at fresh.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Envelope-encrypted secrets. Plaintext is never stored (PRD §13.4).
CREATE TABLE secrets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ciphertext  BYTEA NOT NULL,
    nonce       BYTEA NOT NULL,
    wrapped_dek BYTEA NOT NULL,
    dek_nonce   BYTEA NOT NULL,
    key_version INT  NOT NULL DEFAULT 1,
    purpose     TEXT NOT NULL DEFAULT 'generic',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name               TEXT NOT NULL,
    email              TEXT NOT NULL UNIQUE,
    username           TEXT NOT NULL UNIQUE,
    password_hash      TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive')),
    tenant             TEXT,
    failed_login_count INT  NOT NULL DEFAULT 0,
    locked_until       TIMESTAMPTZ,
    last_login_at      TIMESTAMPTZ,
    created_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_users_updated BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    is_system   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_roles_updated BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

-- RS256 signing keys for OAuth2 access tokens. Private key is stored encrypted
-- via the secrets table; rotation = insert a new active key, retire the old one.
CREATE TABLE jwt_signing_keys (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kid                TEXT NOT NULL UNIQUE,
    public_key_pem     TEXT NOT NULL,
    private_secret_ref UUID NOT NULL REFERENCES secrets(id),
    algorithm          TEXT NOT NULL DEFAULT 'RS256',
    status             TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_jwt_keys_updated BEFORE UPDATE ON jwt_signing_keys
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Free-form platform settings backing the dashboard Settings page.
CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      JSONB NOT NULL,
    updated_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
