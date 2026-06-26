package adminsvc

import (
	"net/http"
	"os"
	"strings"

	"github.com/ddag/ddag/internal/auth"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/rbac"
)

// allowedOrigins lists dashboard origins permitted for credentialed CORS.
func (s *service) allowedOrigins() []string {
	if len(s.cfg.DashboardOrigins) > 0 {
		return s.cfg.DashboardOrigins
	}
	if v := os.Getenv("DDAG_DASHBOARD_ORIGINS"); v != "" {
		return strings.Split(v, ",")
	}
	return []string{"http://localhost:3000", "http://127.0.0.1:3000"}
}

// cors echoes an allowed origin and enables credentialed requests so the Nuxt
// dashboard can call the API with its session cookie.
func (s *service) cors(next http.Handler) http.Handler {
	allowed := s.allowedOrigins()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		for _, a := range allowed {
			if strings.TrimSpace(a) == origin && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Vary", "Origin")
				break
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-CSRF-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *service) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}
		if _, err := r.Cookie(s.cfg.Session.CookieName); err != nil {
			next.ServeHTTP(w, r)
			return
		}
		csrfCookie, err := r.Cookie("ddag_csrf")
		if err != nil || csrfCookie.Value == "" || r.Header.Get("X-CSRF-Token") != csrfCookie.Value {
			httpx.ErrorCode(w, r, httpx.CodeForbidden, "csrf token is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// sessionToken extracts the session JWT from the cookie or Bearer header.
func (s *service) sessionToken(r *http.Request) string {
	if c, err := r.Cookie(s.cfg.Session.CookieName); err == nil && c.Value != "" {
		return c.Value
	}
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return ""
}

// requireSession authenticates the dashboard user, loads their effective
// permissions fresh from the DB (so role changes take effect immediately, per
// PRD §11.2 AC), and stores the principal on the request context.
func (s *service) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := s.sessionToken(r)
		if tok == "" {
			httpx.ErrorCode(w, r, httpx.CodeUnauthorized, "authentication required")
			return
		}
		userID, err := auth.ParseSession(s.cfg.Session.Secret, tok)
		if err != nil {
			httpx.ErrorCode(w, r, httpx.CodeUnauthorized, "invalid or expired session")
			return
		}
		user, err := s.store.GetUserByID(r.Context(), userID)
		if err != nil || user.Status != "active" {
			httpx.ErrorCode(w, r, httpx.CodeUnauthorized, "user not found or inactive")
			return
		}
		perms, err := s.store.UserPermissionCodes(r.Context(), userID)
		if err != nil {
			httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to load permissions")
			return
		}
		principal := rbac.NewPrincipal(user.ID, user.Username, user.Roles, perms)
		next.ServeHTTP(w, r.WithContext(rbac.WithPrincipal(r.Context(), principal)))
	})
}

// principal returns the authenticated principal for the request.
func principalOf(r *http.Request) *rbac.Principal {
	p, _ := rbac.FromContext(r.Context())
	return p
}
