package adminsvc

import (
	"net/http"

	"github.com/ddag/ddag/internal/audit"
	"github.com/ddag/ddag/internal/auth"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/models"
)

func (s *service) handleOverview(w http.ResponseWriter, r *http.Request) {
	ov, err := s.store.Overview(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, ov)
}

func (s *service) listUsers(w http.ResponseWriter, r *http.Request) {
	p := listParams(r)
	users, total, err := s.store.ListUsers(r.Context(), p)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	list(w, r, users, p, total)
}

func (s *service) getUser(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	u, err := s.store.GetUserByID(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, u)
}

func (s *service) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string   `json:"name"`
		Email    string   `json:"email"`
		Username string   `json:"username"`
		Password string   `json:"password"`
		Tenant   *string  `json:"tenant"`
		Roles    []string `json:"roles"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name == "" || req.Email == "" || req.Username == "" || req.Password == "" {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "name, email, username and password are required")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to hash password")
		return
	}
	actor := principalOf(r).UserID
	id, err := s.store.CreateUser(r.Context(), &models.User{
		Name: req.Name, Email: req.Email, Username: req.Username, PasswordHash: hash,
		Status: "active", Tenant: req.Tenant, CreatedBy: &actor,
	})
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeConflict, "could not create user (email/username may exist)")
		return
	}
	if len(req.Roles) > 0 {
		_ = s.store.SetUserRoles(r.Context(), id, req.Roles)
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "create_user", "user", id.String(), nil))
	u, _ := s.store.GetUserByID(r.Context(), id)
	httpx.Created(w, r, u)
}

func (s *service) updateUser(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var req struct {
		Name   string  `json:"name"`
		Email  string  `json:"email"`
		Tenant *string `json:"tenant"`
		Status string  `json:"status"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if err := s.store.UpdateUser(r.Context(), id, req.Name, req.Email, req.Tenant, req.Status); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "update_user", "user", id.String(), nil))
	u, _ := s.store.GetUserByID(r.Context(), id)
	ok(w, r, u)
}

func (s *service) setUserRoles(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var req struct {
		Roles []string `json:"roles"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.store.SetUserRoles(r.Context(), id, req.Roles); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_rbac", "user", id.String(), map[string]any{"roles": req.Roles}))
	u, _ := s.store.GetUserByID(r.Context(), id)
	ok(w, r, u)
}

func (s *service) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Password) < 6 {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "password must be at least 6 characters")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to hash password")
		return
	}
	if err := s.store.SetUserPassword(r.Context(), id, hash); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "reset_password", "user", id.String(), nil))
	ok(w, r, map[string]bool{"ok": true})
}

func (s *service) disableUser(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	u, err := s.store.GetUserByID(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	if err := s.store.UpdateUser(r.Context(), id, u.Name, u.Email, u.Tenant, "inactive"); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "disable_user", "user", id.String(), nil))
	ok(w, r, map[string]bool{"ok": true})
}

// actorEvent builds an audit event attributed to the current dashboard user.
func (s *service) actorEvent(r *http.Request, action, resType, resID string, meta any) audit.Event {
	p := principalOf(r)
	e := audit.Event{Action: action, ResourceType: resType, ResourceID: resID, Status: "success", Metadata: meta, ActorType: audit.ActorUser}
	if p != nil {
		e.ActorID = p.UserID.String()
		e.ActorLabel = p.Username
	}
	return e
}
