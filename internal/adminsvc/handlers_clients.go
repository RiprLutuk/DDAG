package adminsvc

import (
	"net/http"

	"github.com/ddag/ddag/internal/auth"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

func (s *service) listClients(w http.ResponseWriter, r *http.Request) {
	p := listParams(r)
	clients, total, err := s.store.ListClients(r.Context(), p)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	list(w, r, clients, p, total)
}

func (s *service) getClient(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	c, err := s.store.GetClientByPK(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, c)
}

func (s *service) createClient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID        string   `json:"client_id"`
		ClientName      string   `json:"client_name"`
		Environment     string   `json:"environment"`
		Description     string   `json:"description"`
		AccessTokenTTL  int      `json:"access_token_ttl_seconds"`
		RefreshTokenTTL int      `json:"refresh_token_ttl_seconds"`
		Scopes          []string `json:"scopes"`
		APIs            []string `json:"apis"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.ClientID == "" || req.ClientName == "" {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "client_id and client_name are required")
		return
	}
	secret, err := auth.GenerateSecret(24)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to generate secret")
		return
	}
	hash, err := auth.HashPassword(secret)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to hash secret")
		return
	}
	actor := principalOf(r).UserID
	c := &models.Client{
		ClientID: req.ClientID, ClientName: req.ClientName, ClientSecretHash: hash,
		Environment: defStr(req.Environment, "dev"), Status: "active", Description: req.Description,
		AccessTokenTTLSeconds: defInt(req.AccessTokenTTL, 3600), RefreshTokenTTLSeconds: defInt(req.RefreshTokenTTL, 2592000),
		CreatedBy: &actor,
	}
	id, err := s.store.CreateClient(r.Context(), c)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeConflict, "client_id may already exist")
		return
	}
	if len(req.Scopes) > 0 {
		_ = s.store.SetClientScopes(r.Context(), id, req.Scopes)
	}
	if len(req.APIs) > 0 {
		_ = s.store.SetClientAPIs(r.Context(), id, parseUUIDs(req.APIs))
	}
	s.publishMetadataSync(r.Context(), "client:create")
	s.audit.Write(r.Context(), r, s.actorEvent(r, "create_client", "client", id.String(), map[string]any{"client_id": req.ClientID}))
	created, _ := s.store.GetClientByPK(r.Context(), id)
	// Secret is shown exactly once (PRD §11.4 AC).
	httpx.Created(w, r, map[string]any{"client": created, "client_secret": secret})
}

func (s *service) updateClient(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var req struct {
		ClientName      string `json:"client_name"`
		Environment     string `json:"environment"`
		Status          string `json:"status"`
		Description     string `json:"description"`
		AccessTokenTTL  int    `json:"access_token_ttl_seconds"`
		RefreshTokenTTL int    `json:"refresh_token_ttl_seconds"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.store.UpdateClient(r.Context(), id, req.ClientName, defStr(req.Environment, "dev"),
		defStr(req.Status, "active"), req.Description, defInt(req.AccessTokenTTL, 3600), defInt(req.RefreshTokenTTL, 2592000)); err != nil {
		storeErr(w, r, err)
		return
	}
	s.publishMetadataSync(r.Context(), "client:update")
	s.audit.Write(r.Context(), r, s.actorEvent(r, "update_client", "client", id.String(), nil))
	c, _ := s.store.GetClientByPK(r.Context(), id)
	ok(w, r, c)
}

func (s *service) rotateClientSecret(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	secret, err := auth.GenerateSecret(24)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to generate secret")
		return
	}
	hash, err := auth.HashPassword(secret)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to hash secret")
		return
	}
	if err := s.store.RotateClientSecret(r.Context(), id, hash); err != nil {
		storeErr(w, r, err)
		return
	}
	// Rotating a secret invalidates existing refresh tokens.
	_ = s.store.RevokeClientRefreshTokens(r.Context(), id)
	s.audit.Write(r.Context(), r, s.actorEvent(r, "rotate_secret", "client", id.String(), nil))
	ok(w, r, map[string]any{"client_secret": secret})
}

func (s *service) setClientScopes(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var req struct {
		Scopes []string `json:"scopes"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.store.SetClientScopes(r.Context(), id, req.Scopes); err != nil {
		storeErr(w, r, err)
		return
	}
	s.publishMetadataSync(r.Context(), "client:scopes")
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_scope", "client", id.String(), map[string]any{"scopes": req.Scopes}))
	c, _ := s.store.GetClientByPK(r.Context(), id)
	ok(w, r, c)
}

func (s *service) setClientAPIs(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var req struct {
		APIs []string `json:"apis"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.store.SetClientAPIs(r.Context(), id, parseUUIDs(req.APIs)); err != nil {
		storeErr(w, r, err)
		return
	}
	s.publishMetadataSync(r.Context(), "client:apis")
	s.audit.Write(r.Context(), r, s.actorEvent(r, "update_client", "client", id.String(), map[string]any{"apis": req.APIs}))
	c, _ := s.store.GetClientByPK(r.Context(), id)
	ok(w, r, c)
}

func (s *service) deleteClient(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	if err := s.store.DeleteClient(r.Context(), id); err != nil {
		storeErr(w, r, err)
		return
	}
	s.publishMetadataSync(r.Context(), "client:delete")
	s.audit.Write(r.Context(), r, s.actorEvent(r, "delete_client", "client", id.String(), nil))
	ok(w, r, map[string]bool{"ok": true})
}

func parseUUIDs(in []string) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(in))
	for _, s := range in {
		if id, err := uuid.Parse(s); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func defStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func defInt(n, def int) int {
	if n <= 0 {
		return def
	}
	return n
}
