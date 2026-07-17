package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// BackupDestination is a provider-neutral storage target for backup uploads.
// Public configuration (endpoint, bucket, prefix, host) lives in Config.
// Credentials are envelope-encrypted in the secrets table and referenced by ID.
type BackupDestination struct {
	ID                  uuid.UUID       `db:"id" json:"id"`
	Name                string          `db:"name" json:"name"`
	Provider            string          `db:"provider" json:"provider"`
	Config              json.RawMessage `db:"config" json:"config"`
	CredentialSecretRef *uuid.UUID      `db:"credential_secret_ref" json:"-"`
	HasCredential       bool            `db:"has_credential" json:"has_credential"`
	Status              string          `db:"status" json:"status"`
	LastVerifiedAt      *time.Time      `db:"last_verified_at" json:"last_verified_at,omitempty"`
	LastError           string          `db:"last_error" json:"last_error"`
	CreatedBy           *uuid.UUID      `db:"created_by" json:"created_by,omitempty"`
	CreatedAt           time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time       `db:"updated_at" json:"updated_at"`
}
