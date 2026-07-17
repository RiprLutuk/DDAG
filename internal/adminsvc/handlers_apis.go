package adminsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
	ResponseMapping      json.RawMessage       `json:"response_mapping"`
	RequiredScope        string                `json:"required_scope"`
	DefaultLimit         int                   `json:"default_limit"`
	MaxLimit             int                   `json:"max_limit"`
	IsWrite              bool                  `json:"is_write"`
	Parameters           []models.APIParameter `json:"parameters"`
}

type lifecycleCommentInput struct {
	Comment string `json:"comment"`
}

func validateAPIMethod(method string, isWrite bool) error {
	switch method {
	case "GET", "QUERY":
		if isWrite {
			return fmt.Errorf("%s APIs are always read-only", method)
		}
	case "POST":
		// POST remains compatible with existing read/search endpoints.
	case "PUT", "PATCH", "DELETE":
		if !isWrite {
			return fmt.Errorf("%s requires Enable write operation", method)
		}
	default:
		return fmt.Errorf("method must be GET, QUERY, POST, PUT, PATCH, or DELETE")
	}
	return nil
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
	in.Method = strings.ToUpper(strings.TrimSpace(in.Method))
	if err := validateAPIMethod(in.Method, in.IsWrite); err != nil {
		httpx.ErrorCodeWithDetails(w, r, httpx.CodeValidation, "API method configuration is invalid", map[string]any{
			"fields": map[string]string{"method": "unsupported method or write-operation combination"},
		})
		return
	}
	actor := principalOf(r).UserID
	a := &models.APIDefinition{
		Name: in.Name, Namespace: in.Namespace, Path: in.Path, Method: in.Method,
		Description: in.Description, DatabaseConnectionID: &connID, ConnectorType: conn.DatabaseType,
		QueryTemplate: in.QueryTemplate, ResponseMapping: in.ResponseMapping, Status: "DRAFT", RequiredScope: in.RequiredScope,
		DefaultLimit: defInt(in.DefaultLimit, 100), MaxLimit: defInt(in.MaxLimit, 1000), IsWrite: in.IsWrite, CreatedBy: &actor,
	}
	id, err := s.store.CreateAPIWithParameters(r.Context(), a, in.Parameters)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeConflict, "an API with this method+path may already exist")
		return
	}
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
	in.Method = strings.ToUpper(strings.TrimSpace(in.Method))
	if err := validateAPIMethod(in.Method, in.IsWrite); err != nil {
		httpx.ErrorCodeWithDetails(w, r, httpx.CodeValidation, "API method configuration is invalid", map[string]any{
			"fields": map[string]string{"method": "unsupported method or write-operation combination"},
		})
		return
	}
	a := &models.APIDefinition{
		ID: id, Name: in.Name, Namespace: in.Namespace, Path: in.Path, Method: in.Method,
		Description: in.Description, DatabaseConnectionID: &connID, ConnectorType: conn.DatabaseType,
		QueryTemplate: in.QueryTemplate, ResponseMapping: in.ResponseMapping, RequiredScope: in.RequiredScope,
		DefaultLimit: defInt(in.DefaultLimit, 100), MaxLimit: defInt(in.MaxLimit, 1000), IsWrite: in.IsWrite,
	}
	if err := s.store.UpdateAPIWithParameters(r.Context(), a, in.Parameters); err != nil {
		storeErr(w, r, err)
		return
	}
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

// publishAPI validates the query for safety and only allows APPROVED APIs to
// create a new immutable published revision.
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
	if err := canPublishAPIStatus(a.Status); err != nil {
		httpx.ErrorCodeWithDetails(w, r, httpx.CodeValidation, "API cannot be published from its current lifecycle state", map[string]any{
			"fields": map[string]string{"status": "transition to PUBLISHED is not permitted"},
		})
		return
	}
	if err := gateway.ValidateForPublish(*a, a.Parameters); err != nil {
		httpx.ErrorCodeWithDetails(w, r, httpx.CodeValidation, "API definition is not ready to publish", map[string]any{
			"fields": map[string]string{"api": "definition does not meet publish requirements"},
		})
		return
	}
	actor := principalOf(r).UserID
	rev, err := s.store.PublishAPI(r.Context(), a, actor)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	_ = s.store.AddAPILifecycleEvent(r.Context(), id, a.Status, "PUBLISHED", &actor, a.ApprovedComment, json.RawMessage(`{"revision_created":true}`))
	s.publishMetadataSync(r.Context(), "api:publish")
	s.audit.Write(r.Context(), r, s.actorEvent(r, "publish_api", "api", id.String(), map[string]any{"revision": rev.Revision}))
	updated, _ := s.store.GetAPI(r.Context(), id)
	ok(w, r, updated)
}

func (s *service) approveAPI(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var in lifecycleCommentInput
	_ = decode(w, r, &in)
	current, err := s.store.GetAPI(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	if err := canTransitionAPIStatus(current.Status, "APPROVED"); err != nil {
		httpx.ErrorCodeWithDetails(w, r, httpx.CodeValidation, "lifecycle state transition is not allowed", map[string]any{
			"fields": map[string]string{"status": "transition from " + current.Status + " to APPROVED is not permitted"},
		})
		return
	}
	actor := principalOf(r).UserID
	if err := s.store.ApproveAPI(r.Context(), id, actor, in.Comment); err != nil {
		storeErr(w, r, err)
		return
	}
	_ = s.store.AddAPILifecycleEvent(r.Context(), id, current.Status, "APPROVED", &actor, in.Comment, nil)
	s.audit.Write(r.Context(), r, s.actorEvent(r, "approve_api", "api", id.String(), map[string]any{"comment": in.Comment}))
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
		current, err := s.store.GetAPI(r.Context(), id)
		if err != nil {
			storeErr(w, r, err)
			return
		}
		if err := canTransitionAPIStatus(current.Status, status); err != nil {
			httpx.ErrorCodeWithDetails(w, r, httpx.CodeValidation, "lifecycle state transition is not allowed", map[string]any{
				"fields": map[string]string{"status": "transition from " + current.Status + " to " + status + " is not permitted"},
			})
			return
		}
		actor := principalOf(r).UserID
		if err := s.store.SetAPIStatus(r.Context(), id, status, &actor); err != nil {
			storeErr(w, r, err)
			return
		}
		_ = s.store.AddAPILifecycleEvent(r.Context(), id, current.Status, status, &actor, "", nil)
		s.publishMetadataSync(r.Context(), "api:status:"+status)
		s.audit.Write(r.Context(), r, s.actorEvent(r, action, "api", id.String(), nil))
		updated, _ := s.store.GetAPI(r.Context(), id)
		ok(w, r, updated)
	}
}

func (s *service) listAPIRevisions(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	revs, err := s.store.ListAPIRevisions(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, revs)
}

func (s *service) getAPIDiff(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	apiDef, err := s.store.GetAPI(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	current, err := json.Marshal(apiDef)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	rev, err := s.store.GetLatestAPIRevision(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	diff, err := diffJSON(rev.Snapshot, current)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, map[string]any{"published_revision": rev.Revision, "diff": diff})
}

func (s *service) exportPromotionBundle(w http.ResponseWriter, r *http.Request) {
	apis, err := s.store.ListPublishedAPIs(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	bundle := promotionBundle{Version: "v4"}
	bundle.APIs = make([]promotionAPI, 0, len(apis))
	for _, a := range apis {
		bundle.APIs = append(bundle.APIs, promotionAPI{Name: a.Name, Method: a.Method, Path: a.Path})
	}
	ok(w, r, bundle)
}

func (s *service) importPromotionBundleDryRun(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if !decode(w, r, &raw) {
		return
	}
	bundle, err := decodePromotionBundle(raw)
	if err != nil {
		httpx.ErrorCodeWithDetails(w, r, httpx.CodeValidation, "invalid promotion bundle format", map[string]any{
			"fields": map[string]string{"bundle": "bundle payload is malformed or invalid json"},
		})
		return
	}
	ok(w, r, validatePromotionBundle(bundle))
}

// testQuery runs a query template against a connection with sample parameters
// (the API builder's "Test query" action). The query is validated for binding
// safety before execution.
func (s *service) testQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConnectionID    string            `json:"connection_id"`
		QueryTemplate   string            `json:"query_template"`
		ResponseMapping json.RawMessage   `json:"response_mapping"`
		Parameters      map[string]any    `json:"parameters"`
		Query           map[string]string `json:"query"`
		Limit           int               `json:"limit"`
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
	q := url.Values{}
	for k, v := range req.Query {
		q.Set(k, v)
	}
	built, apiErr := gateway.BuildDynamicQuery(models.APIDefinition{
		QueryTemplate:   req.QueryTemplate,
		ResponseMapping: req.ResponseMapping,
		DefaultLimit:    defInt(req.Limit, 50),
		MaxLimit:        defInt(req.Limit, 50),
	}, q, req.Parameters)
	if apiErr != nil {
		httpx.Error(w, r, apiErr)
		return
	}
	if err := gateway.ValidateForExecution(models.APIDefinition{
		QueryTemplate: built.SQL,
		DefaultLimit:  defInt(req.Limit, 50),
		MaxLimit:      defInt(req.Limit, 50),
	}); err != nil {
		httpx.ErrorCodeWithDetails(w, r, httpx.CodeValidation, "test query validation failed", map[string]any{
			"fields": map[string]string{"query_template": "query is not permitted"},
		})
		return
	}
	res, err := s.callConnectorQuery(r.Context(), conn.DatabaseType, map[string]any{
		"request_id":     httpx.RequestID(r.Context()),
		"connection_id":  connID.String(),
		"query_template": built.SQL,
		"parameters":     built.Params,
		"limit":          defInt(req.Limit, 50),
	})
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeConnectorError, "connector unavailable for "+conn.DatabaseType)
		return
	}
	ok(w, r, res)
}

func (s *service) previewQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		API        apiInput          `json:"api"`
		Query      map[string]string `json:"query"`
		Parameters map[string]any    `json:"parameters"`
	}
	if !decode(w, r, &req) {
		return
	}
	q := url.Values{}
	for k, v := range req.Query {
		q.Set(k, v)
	}
	a := models.APIDefinition{
		Name: req.API.Name, Method: req.API.Method, Path: req.API.Path,
		QueryTemplate: req.API.QueryTemplate, ResponseMapping: req.API.ResponseMapping,
		DefaultLimit: defInt(req.API.DefaultLimit, 100), MaxLimit: defInt(req.API.MaxLimit, 1000),
	}
	built, apiErr := gateway.BuildDynamicQuery(a, q, req.Parameters)
	if apiErr != nil {
		httpx.Error(w, r, apiErr)
		return
	}
	_, builderEnabled, _ := gateway.QueryBuilderFromAPI(a)
	ok(w, r, map[string]any{
		"sql":             built.SQL,
		"parameters":      built.Params,
		"builder_enabled": builderEnabled,
	})
}

func (s *service) explainQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConnectionID string            `json:"connection_id"`
		API          apiInput          `json:"api"`
		Query        map[string]string `json:"query"`
		Parameters   map[string]any    `json:"parameters"`
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
	if conn.DatabaseType != "postgres" && conn.DatabaseType != "mysql" {
		httpx.ErrorCode(w, r, httpx.CodeQueryValidationFailed, "explain is supported for PostgreSQL and MySQL in v3")
		return
	}
	q := url.Values{}
	for k, v := range req.Query {
		q.Set(k, v)
	}
	a := models.APIDefinition{
		Name: req.API.Name, Method: req.API.Method, Path: req.API.Path,
		QueryTemplate: req.API.QueryTemplate, ResponseMapping: req.API.ResponseMapping,
		DefaultLimit: defInt(req.API.DefaultLimit, 100), MaxLimit: defInt(req.API.MaxLimit, 1000),
	}
	built, apiErr := gateway.BuildDynamicQuery(a, q, req.Parameters)
	if apiErr != nil {
		httpx.Error(w, r, apiErr)
		return
	}
	if err := gateway.ValidateForExecution(models.APIDefinition{
		QueryTemplate: built.SQL,
		DefaultLimit:  defInt(req.API.DefaultLimit, 100),
		MaxLimit:      defInt(req.API.MaxLimit, 1000),
	}); err != nil {
		httpx.ErrorCodeWithDetails(w, r, httpx.CodeValidation, "explain query validation failed", map[string]any{
			"fields": map[string]string{"api.query_template": "query is not permitted"},
		})
		return
	}
	res, err := s.callConnectorQuery(r.Context(), conn.DatabaseType, map[string]any{
		"request_id":     httpx.RequestID(r.Context()),
		"connection_id":  connID.String(),
		"query_template": "EXPLAIN " + built.SQL,
		"parameters":     built.Params,
		"limit":          50,
	})
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeConnectorUnavailable, "connector unavailable for "+conn.DatabaseType)
		return
	}
	ok(w, r, res)
}

func (s *service) callConnectorQuery(ctx context.Context, dbType string, payload map[string]any) (map[string]any, error) {
	base, found := s.connectors[dbType]
	if !found {
		return nil, fmt.Errorf("connector not configured for database type %q", dbType)
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/query", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(httpx.RequestIDHeader, httpx.RequestID(ctx))
	if s.cfg.Gateway.InternalAuthSecret != "" {
		internalauth.SignHeaders(req, body, s.cfg.Gateway.InternalAuthSecret, time.Now())
	}
	resp, err := s.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("connector returned HTTP %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if success, ok := out["success"].(bool); ok && !success {
		return nil, fmt.Errorf("connector returned an error response")
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
