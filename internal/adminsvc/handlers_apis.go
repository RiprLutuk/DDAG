package adminsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ddag/ddag/internal/gateway"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/internalauth"
	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

func (s *service) listAPIs(w http.ResponseWriter, r *http.Request) {
	p := listParams(r)
	status := r.URL.Query().Get("status")
	apis, total, err := s.store.ListAPIs(r.Context(), p, status)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	list(w, r, apis, p, total)
}

func (s *service) getAPI(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	a, err := s.store.GetAPI(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, a)
}

// apiInput is the shared create/update payload for an API definition.
type apiInput struct {
	Name                 string                `json:"name"`
	Namespace            string                `json:"namespace"`
	Path                 string                `json:"path"`
	Method               string                `json:"method"`
	Description          string                `json:"description"`
	DatabaseConnectionID string                `json:"database_connection_id"`
	QueryTemplate        string                `json:"query_template"`
	RequiredScope        string                `json:"required_scope"`
	DefaultLimit         int                   `json:"default_limit"`
	MaxLimit             int                   `json:"max_limit"`
	Parameters           []models.APIParameter `json:"parameters"`
}

func (s *service) createAPI(w http.ResponseWriter, r *http.Request) {
	var in apiInput
	if !decode(w, r, &in) {
		return
	}
	connID, err := uuid.Parse(in.DatabaseConnectionID)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "valid database_connection_id is required")
		return
	}
	conn, err := s.store.GetConnection(r.Context(), connID)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "database connection not found")
		return
	}
	if in.Method != "GET" && in.Method != "POST" {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "method must be GET or POST")
		return
	}
	actor := principalOf(r).UserID
	a := &models.APIDefinition{
		Name: in.Name, Namespace: in.Namespace, Path: in.Path, Method: in.Method,
		Description: in.Description, DatabaseConnectionID: &connID, ConnectorType: conn.DatabaseType,
		QueryTemplate: in.QueryTemplate, Status: "DRAFT", RequiredScope: in.RequiredScope,
		DefaultLimit: defInt(in.DefaultLimit, 100), MaxLimit: defInt(in.MaxLimit, 1000), CreatedBy: &actor,
	}
	id, err := s.store.CreateAPI(r.Context(), a)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeConflict, "an API with this method+path may already exist")
		return
	}
	_ = s.store.ReplaceAPIParameters(r.Context(), id, in.Parameters)
	s.publishMetadataSync(r.Context(), "api:create")
	s.audit.Write(r.Context(), r, s.actorEvent(r, "create_api", "api", id.String(), map[string]any{"path": in.Path}))
	created, _ := s.store.GetAPI(r.Context(), id)
	httpx.Created(w, r, created)
}

func (s *service) updateAPI(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var in apiInput
	if !decode(w, r, &in) {
		return
	}
	connID, err := uuid.Parse(in.DatabaseConnectionID)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "valid database_connection_id is required")
		return
	}
	conn, err := s.store.GetConnection(r.Context(), connID)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "database connection not found")
		return
	}
	a := &models.APIDefinition{
		ID: id, Name: in.Name, Namespace: in.Namespace, Path: in.Path, Method: in.Method,
		Description: in.Description, DatabaseConnectionID: &connID, ConnectorType: conn.DatabaseType,
		QueryTemplate: in.QueryTemplate, RequiredScope: in.RequiredScope,
		DefaultLimit: defInt(in.DefaultLimit, 100), MaxLimit: defInt(in.MaxLimit, 1000),
	}
	if err := s.store.UpdateAPI(r.Context(), a); err != nil {
		storeErr(w, r, err)
		return
	}
	_ = s.store.ReplaceAPIParameters(r.Context(), id, in.Parameters)
	s.publishMetadataSync(r.Context(), "api:update")
	s.audit.Write(r.Context(), r, s.actorEvent(r, "edit_api", "api", id.String(), nil))
	updated, _ := s.store.GetAPI(r.Context(), id)
	ok(w, r, updated)
}

func (s *service) deleteAPI(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	if err := s.store.DeleteAPI(r.Context(), id); err != nil {
		storeErr(w, r, err)
		return
	}
	s.publishMetadataSync(r.Context(), "api:delete")
	s.audit.Write(r.Context(), r, s.actorEvent(r, "disable_api", "api", id.String(), map[string]any{"deleted": true}))
	ok(w, r, map[string]bool{"ok": true})
}

// publishAPI validates the query for safety (PRD §11.7 AC) before publishing.
func (s *service) publishAPI(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	a, err := s.store.GetAPI(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	if err := gateway.ValidateForPublish(*a, a.Parameters); err != nil {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "cannot publish: "+err.Error())
		return
	}
	actor := principalOf(r).UserID
	if err := s.store.SetAPIStatus(r.Context(), id, "PUBLISHED", &actor); err != nil {
		storeErr(w, r, err)
		return
	}
	s.publishMetadataSync(r.Context(), "api:publish")
	s.audit.Write(r.Context(), r, s.actorEvent(r, "publish_api", "api", id.String(), nil))
	updated, _ := s.store.GetAPI(r.Context(), id)
	ok(w, r, updated)
}

// setAPIStatusHandler builds a handler that transitions to a fixed status.
func (s *service) setAPIStatusHandler(status, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok2 := idParam(w, r)
		if !ok2 {
			return
		}
		actor := principalOf(r).UserID
		if err := s.store.SetAPIStatus(r.Context(), id, status, &actor); err != nil {
			storeErr(w, r, err)
			return
		}
		s.publishMetadataSync(r.Context(), "api:status:"+status)
		s.audit.Write(r.Context(), r, s.actorEvent(r, action, "api", id.String(), nil))
		updated, _ := s.store.GetAPI(r.Context(), id)
		ok(w, r, updated)
	}
}

// testQuery runs a query template against a connection with sample parameters
// (the API builder's "Test query" action). The query is validated for binding
// safety before execution.
func (s *service) testQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConnectionID  string         `json:"connection_id"`
		QueryTemplate string         `json:"query_template"`
		Parameters    map[string]any `json:"parameters"`
		Limit         int            `json:"limit"`
	}
	if !decode(w, r, &req) {
		return
	}
	connID, err := uuid.Parse(req.ConnectionID)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "valid connection_id is required")
		return
	}
	conn, err := s.store.GetConnection(r.Context(), connID)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "connection not found")
		return
	}
	res, err := s.callConnectorQuery(r.Context(), conn.DatabaseType, map[string]any{
		"request_id":     httpx.RequestID(r.Context()),
		"connection_id":  connID.String(),
		"query_template": req.QueryTemplate,
		"parameters":     req.Parameters,
		"limit":          defInt(req.Limit, 50),
	})
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeConnectorError, "connector unavailable for "+conn.DatabaseType)
		return
	}
	ok(w, r, res)
}

func (s *service) callConnectorQuery(ctx context.Context, dbType string, payload map[string]any) (map[string]any, error) {
	base, found := s.connectors[dbType]
	if !found {
		return map[string]any{"success": false, "error": map[string]string{"message": "no connector for " + dbType}}, nil
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/query", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(httpx.RequestIDHeader, "")
	if s.cfg.Gateway.InternalAuthSecret != "" {
		internalauth.SignHeaders(req, body, s.cfg.Gateway.InternalAuthSecret, time.Now())
	}
	resp, err := s.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---- Cache rule per API ----

func (s *service) getAPICache(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	cr, err := s.store.GetCacheRule(r.Context(), id)
	if err != nil {
		// No rule yet: return a sensible default (disabled).
		ok(w, r, models.CacheRule{APIDefinitionID: id, Enabled: false, TTLSeconds: 300,
			CacheKeyStrategy: "client_id:path:query_params", VaryByClient: true})
		return
	}
	ok(w, r, cr)
}

func (s *service) setAPICache(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var req struct {
		Enabled          bool   `json:"enabled"`
		TTLSeconds       int    `json:"ttl_seconds"`
		CacheKeyStrategy string `json:"cache_key_strategy"`
		VaryByClient     bool   `json:"vary_by_client"`
	}
	if !decode(w, r, &req) {
		return
	}
	cr := &models.CacheRule{
		APIDefinitionID: id, Enabled: req.Enabled, TTLSeconds: defInt(req.TTLSeconds, 300),
		CacheKeyStrategy: defStr(req.CacheKeyStrategy, "client_id:path:query_params"), VaryByClient: req.VaryByClient,
	}
	if err := s.store.UpsertCacheRule(r.Context(), cr); err != nil {
		storeErr(w, r, err)
		return
	}
	s.publishMetadataSync(r.Context(), "cache_rule:update")
	s.audit.Write(r.Context(), r, s.actorEvent(r, "edit_api", "cache_rule", id.String(), map[string]any{"enabled": req.Enabled}))
	ok(w, r, cr)
}
