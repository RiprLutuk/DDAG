package adminsvc

import (
	"net/http"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

// optUUID parses an optional UUID string ("" → nil).
func optUUID(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	if id, err := uuid.Parse(s); err == nil {
		return &id
	}
	return nil
}

// ---- Rate limit rules ----

func (s *service) listRateLimits(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.ListRateLimitRules(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, rules)
}

type rateInput struct {
	ClientID          string `json:"client_id"`
	APIDefinitionID   string `json:"api_definition_id"`
	AppliesTo         string `json:"applies_to"`
	RequestsPerSecond int    `json:"requests_per_second"`
	RequestsPerMinute int    `json:"requests_per_minute"`
	RequestsPerHour   int    `json:"requests_per_hour"`
	RequestsPerDay    int    `json:"requests_per_day"`
}

func (s *service) createRateLimit(w http.ResponseWriter, r *http.Request) {
	var in rateInput
	if !decode(w, r, &in) {
		return
	}
	rule := &models.RateLimitRule{
		ClientID: optUUID(in.ClientID), APIDefinitionID: optUUID(in.APIDefinitionID),
		AppliesTo: defStr(in.AppliesTo, "client"), RequestsPerSecond: in.RequestsPerSecond,
		RequestsPerMinute: in.RequestsPerMinute, RequestsPerHour: in.RequestsPerHour, RequestsPerDay: in.RequestsPerDay,
	}
	id, err := s.store.CreateRateLimitRule(r.Context(), rule)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_rate_limit", "rate_limit_rule", id.String(), nil))
	rule.ID = id
	ok(w, r, rule)
}

func (s *service) updateRateLimit(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var in rateInput
	if !decode(w, r, &in) {
		return
	}
	rule := &models.RateLimitRule{
		ID: id, ClientID: optUUID(in.ClientID), APIDefinitionID: optUUID(in.APIDefinitionID),
		AppliesTo: defStr(in.AppliesTo, "client"), RequestsPerSecond: in.RequestsPerSecond,
		RequestsPerMinute: in.RequestsPerMinute, RequestsPerHour: in.RequestsPerHour, RequestsPerDay: in.RequestsPerDay,
	}
	if err := s.store.UpdateRateLimitRule(r.Context(), rule); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_rate_limit", "rate_limit_rule", id.String(), nil))
	ok(w, r, rule)
}

func (s *service) deleteRateLimit(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	if err := s.store.DeleteRateLimitRule(r.Context(), id); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_rate_limit", "rate_limit_rule", id.String(), map[string]any{"deleted": true}))
	ok(w, r, map[string]bool{"ok": true})
}

// ---- IP whitelists ----

func (s *service) listIPWhitelists(w http.ResponseWriter, r *http.Request) {
	entries, err := s.store.ListIPWhitelists(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, entries)
}

type ipInput struct {
	ClientID        string `json:"client_id"`
	APIDefinitionID string `json:"api_definition_id"`
	IPCIDR          string `json:"ip_cidr"`
	ScopeLevel      string `json:"scope_level"`
	Status          string `json:"status"`
	Description     string `json:"description"`
}

func (s *service) createIPWhitelist(w http.ResponseWriter, r *http.Request) {
	var in ipInput
	if !decode(w, r, &in) {
		return
	}
	if in.IPCIDR == "" {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "ip_cidr is required")
		return
	}
	entry := &models.IPWhitelist{
		ClientID: optUUID(in.ClientID), APIDefinitionID: optUUID(in.APIDefinitionID), IPCIDR: in.IPCIDR,
		ScopeLevel: defStr(in.ScopeLevel, "client"), Status: defStr(in.Status, "active"), Description: in.Description,
	}
	id, err := s.store.CreateIPWhitelist(r.Context(), entry)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_ip_whitelist", "ip_whitelist", id.String(), map[string]any{"cidr": in.IPCIDR}))
	entry.ID = id
	ok(w, r, entry)
}

func (s *service) updateIPWhitelist(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var in ipInput
	if !decode(w, r, &in) {
		return
	}
	entry := &models.IPWhitelist{
		ID: id, ClientID: optUUID(in.ClientID), APIDefinitionID: optUUID(in.APIDefinitionID), IPCIDR: in.IPCIDR,
		ScopeLevel: defStr(in.ScopeLevel, "client"), Status: defStr(in.Status, "active"), Description: in.Description,
	}
	if err := s.store.UpdateIPWhitelist(r.Context(), entry); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_ip_whitelist", "ip_whitelist", id.String(), nil))
	ok(w, r, entry)
}

func (s *service) deleteIPWhitelist(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	if err := s.store.DeleteIPWhitelist(r.Context(), id); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_ip_whitelist", "ip_whitelist", id.String(), map[string]any{"deleted": true}))
	ok(w, r, map[string]bool{"ok": true})
}

// ---- Cache management ----

func (s *service) listCacheRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.ListCacheRules(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, rules)
}

// purgeCache purges the cache for an API (by api_id) or all DDAG cache entries.
func (s *service) purgeCache(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIID string `json:"api_id"`
		All   bool   `json:"all"`
	}
	if !decode(w, r, &req) {
		return
	}
	var (
		count int
		err   error
	)
	if req.All {
		count, err = s.cache.PurgeAll(r.Context())
	} else {
		id, perr := uuid.Parse(req.APIID)
		if perr != nil {
			httpx.ErrorCode(w, r, httpx.CodeValidation, "api_id or all is required")
			return
		}
		count, err = s.cache.PurgeEndpoint(r.Context(), id)
	}
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeInternal, "purge failed")
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "purge_cache", "cache", req.APIID, map[string]any{"keys": count, "all": req.All}))
	ok(w, r, map[string]any{"purged": count})
}
