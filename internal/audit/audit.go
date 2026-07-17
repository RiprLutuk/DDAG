// Package audit records audit-log events (PRD §11.14). Failures to write an
// audit entry are logged but never fail the originating request.
package audit

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/models"
	"github.com/ddag/ddag/internal/store"
)

// Actor types.
const (
	ActorUser   = "user"
	ActorClient = "client"
	ActorSystem = "system"
)

// Event is a single auditable action.
type Event struct {
	ActorType    string
	ActorID      string
	ActorLabel   string
	Action       string
	ResourceType string
	ResourceID   string
	Status       string // "success" | "failure"
	Metadata     any
}

// Recorder writes audit events.
type Recorder struct {
	store *store.Store
}

// New builds a Recorder.
func New(s *store.Store) *Recorder { return &Recorder{store: s} }

// Write persists an event, enriching it with request_id, IP and user-agent from
// the request. Safe to call from any handler.
func (r *Recorder) Write(ctx context.Context, req *http.Request, e Event) {
	if e.Status == "" {
		e.Status = "success"
	}
	var meta json.RawMessage
	if e.Metadata != nil {
		if b, err := json.Marshal(e.Metadata); err == nil {
			meta = b
		}
	}
	log := &models.AuditLog{
		RequestID:    httpx.RequestID(ctx),
		ActorType:    e.ActorType,
		ActorID:      e.ActorID,
		ActorLabel:   e.ActorLabel,
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		Status:       e.Status,
		MetadataJSON: meta,
	}
	if req != nil {
		log.IPAddress = httpx.ClientIP(req)
		log.UserAgent = req.UserAgent()
	}
	if err := r.store.InsertAuditLog(ctx, log); err != nil {
		logging.FromContext(ctx).Error("audit_write_failed", "action", e.Action, "error", err.Error())
	}
}
