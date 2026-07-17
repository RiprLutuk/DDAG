package adminsvc

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *service) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.AuditFilter{
		ListParams:   listParams(r),
		ActorType:    q.Get("actor_type"),
		ActorID:      q.Get("actor_id"),
		Action:       q.Get("action"),
		ResourceType: q.Get("resource_type"),
		ResourceID:   q.Get("resource_id"),
		RequestID:    q.Get("request_id"),
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
	q := r.URL.Query()
	f := store.RequestLogFilter{ListParams: listParams(r), RequestID: q.Get("request_id")}
	if v := q.Get("client_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.ClientID = &id
		}
	}
	if v := q.Get("api_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.APIID = &id
		}
	}
	if v, err := strconv.Atoi(q.Get("status_code")); err == nil {
		f.StatusCode = v
	}
	if v, err := strconv.Atoi(q.Get("min_latency_ms")); err == nil {
		f.MinLatencyMS = v
	}
	if v, err := strconv.Atoi(q.Get("max_latency_ms")); err == nil {
		f.MaxLatencyMS = v
	}
	if v := q.Get("cached"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			f.Cached = &b
		}
	}
	logs, total, err := s.store.ListRequestLogs(r.Context(), f)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	list(w, r, logs, f.ListParams, total)
}

func (s *service) exportAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.AuditFilter{ListParams: store.ListParams{Page: 1, Limit: 10000}, ActorType: q.Get("actor_type"), ActorID: q.Get("actor_id"), Action: q.Get("action"), ResourceType: q.Get("resource_type"), ResourceID: q.Get("resource_id"), RequestID: q.Get("request_id"), Status: q.Get("status")}
	if raw := q.Get("from"); raw != "" {
		v, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpx.ErrorCode(w, r, httpx.CodeBadRequest, "from must be RFC3339")
			return
		}
		f.From = &v
	}
	if raw := q.Get("to"); raw != "" {
		v, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpx.ErrorCode(w, r, httpx.CodeBadRequest, "to must be RFC3339")
			return
		}
		f.To = &v
	}
	rows, _, err := s.store.ListAuditLogs(r.Context(), f)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	if q.Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=ddag-audit-logs.csv")
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"id", "request_id", "actor_type", "actor_id", "action", "resource_type", "resource_id", "status", "created_at"})
		for _, v := range rows {
			_ = cw.Write([]string{v.ID.String(), v.RequestID, v.ActorType, v.ActorID, v.Action, v.ResourceType, v.ResourceID, v.Status, v.CreatedAt.UTC().Format(time.RFC3339)})
		}
		cw.Flush()
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=ddag-audit-logs.json")
	_ = json.NewEncoder(w).Encode(rows)
}

// ---- Settings ----

func (s *service) listSettings(w http.ResponseWriter, r *http.Request) {
	p := listParams(r)
	if err := s.ensureSelfManagementDefaults(r.Context()); err != nil {
		storeErr(w, r, err)
		return
	}
	settings, err := s.store.ListSettings(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	filtered := settings[:0]
	q := strings.ToLower(strings.TrimSpace(p.Search))
	for _, row := range settings {
		if q == "" || strings.Contains(strings.ToLower(row.Key), q) || strings.Contains(strings.ToLower(row.Category), q) || strings.Contains(strings.ToLower(row.Description), q) {
			filtered = append(filtered, row)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		var a, b string
		switch p.SortBy {
		case "key":
			a, b = filtered[i].Key, filtered[j].Key
		case "type", "value_type":
			a, b = filtered[i].ValueType, filtered[j].ValueType
		case "updated_at":
			return p.SortDir != "asc" && filtered[i].UpdatedAt.After(filtered[j].UpdatedAt) || p.SortDir == "asc" && filtered[i].UpdatedAt.Before(filtered[j].UpdatedAt)
		default:
			a, b = filtered[i].Category+filtered[i].Key, filtered[j].Category+filtered[j].Key
		}
		if p.SortDir == "desc" {
			return a > b
		}
		return a < b
	})
	total := int64(len(filtered))
	start := p.Offset()
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + p.Limit
	if end > len(filtered) {
		end = len(filtered)
	}
	list(w, r, filtered[start:end], p, total)
}

func (s *service) putSetting(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var value json.RawMessage
	if !decode(w, r, &value) {
		return
	}
	if existing, err := s.store.GetSetting(r.Context(), key); err == nil {
		if err := validateSettingValue(existing.ValueType, value); err != nil {
			httpx.ErrorCodeWithDetails(w, r, httpx.CodeBadRequest, "invalid value for setting type", map[string]any{
				"fields": map[string]string{key: "provided value does not match expected type " + existing.ValueType},
			})
			return
		}
		if key == "backup.offsite.provider" {
			var provider string
			if err := json.Unmarshal(value, &provider); err == nil {
				if err := validateBackupProvider(provider); err != nil {
					httpx.ErrorCodeWithDetails(w, r, httpx.CodeBadRequest, "invalid backup provider", map[string]any{
						"fields": map[string]string{key: err.Error()},
					})
					return
				}
			}
		}
	}
	actor := principalOf(r).UserID
	if err := s.store.UpsertSetting(r.Context(), key, value, &actor); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "update_setting", "setting", key, nil))
	ok(w, r, map[string]bool{"ok": true})
}
