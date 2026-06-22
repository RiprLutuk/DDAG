package adminsvc

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ddag/ddag/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *service) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.AuditFilter{
		ListParams:   listParams(r),
		ActorType:    q.Get("actor_type"),
		Action:       q.Get("action"),
		ResourceType: q.Get("resource_type"),
		Status:       q.Get("status"),
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}
	logs, total, err := s.store.ListAuditLogs(r.Context(), f)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	list(w, r, logs, f.ListParams, total)
}

func (s *service) listRequestLogs(w http.ResponseWriter, r *http.Request) {
	p := listParams(r)
	var clientID *uuid.UUID
	if v := r.URL.Query().Get("client_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			clientID = &id
		}
	}
	logs, total, err := s.store.ListRequestLogs(r.Context(), p, clientID)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	list(w, r, logs, p, total)
}

// ---- Settings ----

func (s *service) listSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.ListSettings(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, settings)
}

func (s *service) putSetting(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var value json.RawMessage
	if !decode(w, r, &value) {
		return
	}
	actor := principalOf(r).UserID
	if err := s.store.UpsertSetting(r.Context(), key, value, &actor); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "update_setting", "setting", key, nil))
	ok(w, r, map[string]bool{"ok": true})
}
