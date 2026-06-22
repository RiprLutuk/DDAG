// Package rbac defines the permission catalog, the default role→permission
// assignments, and the principal/permission-check middleware used by the admin
// backend. Permissions are always enforced on the backend (PRD §11.3 AC), never
// only in the UI.
package rbac

import (
	"context"
	"net/http"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/google/uuid"
)

// Permission codes (PRD §11.3).
const (
	ViewDashboard     = "view_dashboard"
	ManageUser        = "manage_user"
	ManageRole        = "manage_role"
	ManageClient      = "manage_client"
	ManageScope       = "manage_scope"
	ManageConnection  = "manage_connection"
	ViewDBSecret      = "view_db_secret"
	TestConnection    = "test_connection"
	CreateAPI         = "create_api"
	EditAPI           = "edit_api"
	ApproveAPI        = "approve_api"
	PublishAPI        = "publish_api"
	DisableAPI        = "disable_api"
	PurgeCache        = "purge_cache"
	ViewAudit         = "view_audit"
	ViewMonitoring    = "view_monitoring"
	ManageRateLimit   = "manage_rate_limit"
	ManageIPWhitelist = "manage_ip_whitelist"
)

// Role names (PRD §11.3).
const (
	RoleSuperAdmin    = "SUPER_ADMIN"
	RolePlatformAdmin = "PLATFORM_ADMIN"
	RoleDBA           = "DBA"
	RoleAppAdmin      = "APP_ADMIN"
	RoleDeveloper     = "DEVELOPER"
	RoleViewer        = "VIEWER"
	RoleAuditor       = "AUDITOR"
)

// AllPermissions is the catalog with human descriptions (used by the seeder).
var AllPermissions = []struct{ Code, Description string }{
	{ViewDashboard, "View the admin dashboard"},
	{ManageUser, "Create, update and disable users"},
	{ManageRole, "Manage roles and their permissions"},
	{ManageClient, "Manage OAuth2 clients/applications"},
	{ManageScope, "Manage OAuth2 scopes"},
	{ManageConnection, "Manage source database connections"},
	{ViewDBSecret, "View database secret references"},
	{TestConnection, "Test database connections"},
	{CreateAPI, "Create dynamic APIs"},
	{EditAPI, "Edit dynamic APIs"},
	{ApproveAPI, "Approve/review dynamic APIs"},
	{PublishAPI, "Publish dynamic APIs"},
	{DisableAPI, "Disable dynamic APIs"},
	{PurgeCache, "Purge endpoint cache"},
	{ViewAudit, "View audit logs"},
	{ViewMonitoring, "View monitoring dashboards"},
	{ManageRateLimit, "Manage rate-limit rules"},
	{ManageIPWhitelist, "Manage IP whitelist rules"},
}

// DefaultRolePermissions maps each system role to its permission set.
var DefaultRolePermissions = map[string][]string{
	RoleSuperAdmin: codes(AllPermissions), // everything
	RolePlatformAdmin: {
		ViewDashboard, ManageClient, ManageScope, CreateAPI, EditAPI, ApproveAPI,
		PublishAPI, DisableAPI, PurgeCache, ManageRateLimit, ManageIPWhitelist,
		ViewMonitoring, ViewAudit, TestConnection,
	},
	RoleDBA: {
		ViewDashboard, ManageConnection, ViewDBSecret, TestConnection, CreateAPI,
		EditAPI, ApproveAPI, ViewMonitoring,
	},
	RoleAppAdmin:  {ViewDashboard, ViewMonitoring},
	RoleDeveloper: {ViewDashboard},
	RoleViewer:    {ViewDashboard, ViewMonitoring, ViewAudit},
	RoleAuditor:   {ViewDashboard, ViewAudit, ViewMonitoring},
}

// RoleDescriptions describes each system role.
var RoleDescriptions = map[string]string{
	RoleSuperAdmin:    "Full access to the entire system",
	RolePlatformAdmin: "Operational platform configuration and API management",
	RoleDBA:           "Database connections and query performance",
	RoleAppAdmin:      "Tenant/application administration",
	RoleDeveloper:     "API consumer",
	RoleViewer:        "Read-only access",
	RoleAuditor:       "Audit and compliance read-only access",
}

func codes(in []struct{ Code, Description string }) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		out = append(out, p.Code)
	}
	return out
}

// Principal is the authenticated dashboard user attached to a request context.
type Principal struct {
	UserID      uuid.UUID
	Username    string
	Roles       []string
	Permissions map[string]struct{}
}

// Has reports whether the principal holds a permission.
func (p *Principal) Has(code string) bool {
	if p == nil {
		return false
	}
	_, ok := p.Permissions[code]
	return ok
}

type principalKey struct{}

// WithPrincipal stores a principal on the context.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// FromContext returns the principal stored on the context, if any.
func FromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(*Principal)
	return p, ok && p != nil
}

// NewPrincipal builds a principal from role/permission lists.
func NewPrincipal(userID uuid.UUID, username string, roles, perms []string) *Principal {
	set := make(map[string]struct{}, len(perms))
	for _, c := range perms {
		set[c] = struct{}{}
	}
	return &Principal{UserID: userID, Username: username, Roles: roles, Permissions: set}
}

// RequirePermission returns middleware that rejects requests whose principal
// lacks the given permission with a 403.
func RequirePermission(code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := FromContext(r.Context())
			if !ok {
				httpx.ErrorCode(w, r, httpx.CodeUnauthorized, "Authentication required")
				return
			}
			if !p.Has(code) {
				httpx.ErrorCode(w, r, httpx.CodeForbidden, "You do not have permission: "+code)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
