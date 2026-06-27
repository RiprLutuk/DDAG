package adminsvc

import (
	"net/http"

	"github.com/ddag/ddag/internal/rbac"
	"github.com/go-chi/chi/v5"
)

// routes builds the admin-backend HTTP router. Public auth endpoints sit outside
// the session guard; everything else requires a session and the relevant
// permission, enforced on the backend (PRD §11.3 AC).
func (s *service) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(s.cors)

	// Public.
	r.Post("/auth/login", s.handleLogin)

	// Authenticated.
	r.Group(func(pr chi.Router) {
		pr.Use(s.requireSession)
		pr.Use(s.csrf)

		pr.Post("/auth/logout", s.handleLogout)
		pr.Get("/auth/me", s.handleMe)

		perm := func(code string, h http.HandlerFunc) http.Handler {
			return rbac.RequirePermission(code)(h)
		}

		// Overview.
		pr.Method("GET", "/api/overview", perm(rbac.ViewDashboard, s.handleOverview))

		// Users.
		pr.Method("GET", "/api/users", perm(rbac.ManageUser, s.listUsers))
		pr.Method("POST", "/api/users", perm(rbac.ManageUser, s.createUser))
		pr.Method("GET", "/api/users/{id}", perm(rbac.ManageUser, s.getUser))
		pr.Method("PUT", "/api/users/{id}", perm(rbac.ManageUser, s.updateUser))
		pr.Method("POST", "/api/users/{id}/roles", perm(rbac.ManageUser, s.setUserRoles))
		pr.Method("POST", "/api/users/{id}/password", perm(rbac.ManageUser, s.resetUserPassword))
		pr.Method("POST", "/api/users/{id}/disable", perm(rbac.ManageUser, s.disableUser))

		// Roles & permissions.
		pr.Method("GET", "/api/roles", perm(rbac.ViewDashboard, s.listRoles))
		pr.Method("POST", "/api/roles", perm(rbac.ManageRole, s.createRole))
		pr.Method("PUT", "/api/roles/{id}", perm(rbac.ManageRole, s.updateRole))
		pr.Method("DELETE", "/api/roles/{id}", perm(rbac.ManageRole, s.deleteRole))
		pr.Method("POST", "/api/roles/{id}/permissions", perm(rbac.ManageRole, s.setRolePermissions))
		pr.Method("GET", "/api/permissions", perm(rbac.ViewDashboard, s.listPermissions))

		// Scopes.
		pr.Method("GET", "/api/scopes", perm(rbac.ViewDashboard, s.listScopes))
		pr.Method("POST", "/api/scopes", perm(rbac.ManageScope, s.createScope))
		pr.Method("DELETE", "/api/scopes/{id}", perm(rbac.ManageScope, s.deleteScope))

		// Clients.
		pr.Method("GET", "/api/clients", perm(rbac.ManageClient, s.listClients))
		pr.Method("POST", "/api/clients", perm(rbac.ManageClient, s.createClient))
		pr.Method("GET", "/api/clients/{id}", perm(rbac.ManageClient, s.getClient))
		pr.Method("PUT", "/api/clients/{id}", perm(rbac.ManageClient, s.updateClient))
		pr.Method("DELETE", "/api/clients/{id}", perm(rbac.ManageClient, s.deleteClient))
		pr.Method("POST", "/api/clients/{id}/rotate-secret", perm(rbac.ManageClient, s.rotateClientSecret))
		pr.Method("POST", "/api/clients/{id}/scopes", perm(rbac.ManageClient, s.setClientScopes))
		pr.Method("POST", "/api/clients/{id}/apis", perm(rbac.ManageClient, s.setClientAPIs))

		// Database connections.
		pr.Method("GET", "/api/connections", perm(rbac.ManageConnection, s.listConnections))
		pr.Method("POST", "/api/connections", perm(rbac.ManageConnection, s.createConnection))
		pr.Method("GET", "/api/connections/{id}", perm(rbac.ManageConnection, s.getConnection))
		pr.Method("PUT", "/api/connections/{id}", perm(rbac.ManageConnection, s.updateConnection))
		pr.Method("DELETE", "/api/connections/{id}", perm(rbac.ManageConnection, s.deleteConnection))
		pr.Method("POST", "/api/connections/test", perm(rbac.TestConnection, s.testConnection))
		pr.Method("POST", "/api/connections/{id}/test", perm(rbac.TestConnection, s.testSavedConnection))

		// Dynamic APIs.
		pr.Method("GET", "/api/apis", perm(rbac.ViewDashboard, s.listAPIs))
		pr.Method("POST", "/api/apis", perm(rbac.CreateAPI, s.createAPI))
		pr.Method("POST", "/api/apis/test", perm(rbac.EditAPI, s.testQuery))
		pr.Method("POST", "/api/apis/preview", perm(rbac.EditAPI, s.previewQuery))
		pr.Method("POST", "/api/apis/explain", perm(rbac.EditAPI, s.explainQuery))
		pr.Method("GET", "/api/apis/{id}", perm(rbac.ViewDashboard, s.getAPI))
		pr.Method("PUT", "/api/apis/{id}", perm(rbac.EditAPI, s.updateAPI))
		pr.Method("DELETE", "/api/apis/{id}", perm(rbac.DisableAPI, s.deleteAPI))
		pr.Method("POST", "/api/apis/{id}/publish", perm(rbac.PublishAPI, s.publishAPI))
		pr.Method("POST", "/api/apis/{id}/review", perm(rbac.ApproveAPI, s.setAPIStatusHandler("REVIEW", "approve_api")))
		pr.Method("POST", "/api/apis/{id}/disable", perm(rbac.DisableAPI, s.setAPIStatusHandler("DISABLED", "disable_api")))
		pr.Method("POST", "/api/apis/{id}/archive", perm(rbac.DisableAPI, s.setAPIStatusHandler("ARCHIVED", "disable_api")))
		pr.Method("GET", "/api/apis/{id}/cache", perm(rbac.ViewDashboard, s.getAPICache))
		pr.Method("PUT", "/api/apis/{id}/cache", perm(rbac.EditAPI, s.setAPICache))

		// Cache management.
		pr.Method("GET", "/api/cache/rules", perm(rbac.ViewMonitoring, s.listCacheRules))
		pr.Method("POST", "/api/cache/purge", perm(rbac.PurgeCache, s.purgeCache))

		// Rate limits.
		pr.Method("GET", "/api/rate-limits", perm(rbac.ManageRateLimit, s.listRateLimits))
		pr.Method("POST", "/api/rate-limits", perm(rbac.ManageRateLimit, s.createRateLimit))
		pr.Method("PUT", "/api/rate-limits/{id}", perm(rbac.ManageRateLimit, s.updateRateLimit))
		pr.Method("DELETE", "/api/rate-limits/{id}", perm(rbac.ManageRateLimit, s.deleteRateLimit))

		// IP whitelists.
		pr.Method("GET", "/api/ip-whitelists", perm(rbac.ManageIPWhitelist, s.listIPWhitelists))
		pr.Method("POST", "/api/ip-whitelists", perm(rbac.ManageIPWhitelist, s.createIPWhitelist))
		pr.Method("PUT", "/api/ip-whitelists/{id}", perm(rbac.ManageIPWhitelist, s.updateIPWhitelist))
		pr.Method("DELETE", "/api/ip-whitelists/{id}", perm(rbac.ManageIPWhitelist, s.deleteIPWhitelist))

		// Logs & audit.
		pr.Method("GET", "/api/circuit-breakers", perm(rbac.ViewCircuitState, s.listCircuitBreakers))
		pr.Method("GET", "/api/pool-stats", perm(rbac.ViewMonitoring, s.listPoolStats))
		pr.Method("GET", "/api/request-logs", perm(rbac.ViewMonitoring, s.listRequestLogs))
		pr.Method("GET", "/api/audit-logs", perm(rbac.ViewAudit, s.listAuditLogs))

		// Settings.
		pr.Method("GET", "/api/settings", perm(rbac.ViewDashboard, s.listSettings))
		pr.Method("PUT", "/api/settings/{key}", perm(rbac.ManageRole, s.putSetting))
	})

	return r
}
