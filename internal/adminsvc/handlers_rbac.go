package adminsvc

import (
	"net/http"

	"github.com/ddag/ddag/internal/httpx"
)

// ---- Roles & permissions ----

func (s *service) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := s.store.ListRoles(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, roles)
}

func (s *service) listPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := s.store.ListPermissions(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, perms)
}

func (s *service) createRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name == "" {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "name is required")
		return
	}
	id, err := s.store.CreateRole(r.Context(), req.Name, req.Description)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeConflict, "role may already exist")
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_rbac", "role", id.String(), map[string]any{"created": req.Name}))
	ok(w, r, map[string]any{"id": id, "name": req.Name})
}

func (s *service) updateRole(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var req struct {
		Description string `json:"description"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.store.UpdateRoleDescription(r.Context(), id, req.Description); err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, map[string]bool{"ok": true})
}

func (s *service) deleteRole(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	if err := s.store.DeleteRole(r.Context(), id); err != nil {
		if err.Error() == "not found" {
			httpx.ErrorCode(w, r, httpx.CodeConflict, "cannot delete a system role or unknown role")
			return
		}
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_rbac", "role", id.String(), map[string]any{"deleted": true}))
	ok(w, r, map[string]bool{"ok": true})
}

func (s *service) setRolePermissions(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var req struct {
		Permissions []string `json:"permissions"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.store.SetRolePermissions(r.Context(), id, req.Permissions); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_rbac", "role", id.String(), map[string]any{"permissions": req.Permissions}))
	ok(w, r, map[string]bool{"ok": true})
}

// ---- Scopes ----

func (s *service) listScopes(w http.ResponseWriter, r *http.Request) {
	scopes, err := s.store.ListScopes(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, scopes)
}

func (s *service) createScope(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ScopeCode   string `json:"scope_code"`
		Description string `json:"description"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.ScopeCode == "" {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "scope_code is required")
		return
	}
	id, err := s.store.CreateScope(r.Context(), req.ScopeCode, req.Description)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_scope", "scope", id.String(), map[string]any{"scope": req.ScopeCode}))
	ok(w, r, map[string]any{"id": id, "scope_code": req.ScopeCode})
}

func (s *service) deleteScope(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	if err := s.store.DeleteScope(r.Context(), id); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_scope", "scope", id.String(), map[string]any{"deleted": true}))
	ok(w, r, map[string]bool{"ok": true})
}
