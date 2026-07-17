package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// BackupRun is immutable operational evidence for a backup or restore drill.
type BackupRun struct {
	ID            uuid.UUID       `db:"id" json:"id"`
	Kind          string          `db:"kind" json:"kind"`
	Status        string          `db:"status" json:"status"`
	DestinationID *uuid.UUID      `db:"destination_id" json:"destination_id,omitempty"`
	ArtifactPath  string          `db:"artifact_path" json:"artifact_path,omitempty"`
	ManifestPath  string          `db:"manifest_path" json:"manifest_path,omitempty"`
	SHA256        string          `db:"sha256" json:"sha256,omitempty"`
	Bytes         int64           `db:"bytes" json:"bytes,omitempty"`
	DatabaseName  string          `db:"database_name" json:"database_name"`
	Detail        json.RawMessage `db:"detail" json:"detail,omitempty"`
	Error         string          `db:"error" json:"error,omitempty"`
	StartedAt     time.Time       `db:"started_at" json:"started_at"`
	CompletedAt   *time.Time      `db:"completed_at" json:"completed_at,omitempty"`
}

type BackupRunCreate struct{ Kind, DatabaseName string }
