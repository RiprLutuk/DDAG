package adminsvc

import (
	"net/http"
	"time"

	"github.com/ddag/ddag/internal/audit"
	"github.com/ddag/ddag/internal/auth"
	"github.com/ddag/ddag/internal/httpx"
)

// handleLogin authenticates a dashboard user, enforcing account lockout after
// repeated failures (PRD §11.1), and issues an httpOnly session cookie.
func (s *service) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	user, err := s.store.GetUserByLogin(r.Context(), req.Login)
	if err != nil {
		s.audit.Write(r.Context(), r, audit.Event{
			ActorType: audit.ActorUser, ActorID: req.Login, Action: "login",
			ResourceType: "session", Status: "failure", Metadata: map[string]string{"reason": "unknown_user"},
		})
		httpx.ErrorCode(w, r, httpx.CodeUnauthorized, "invalid credentials")
		return
	}
	if user.Status != "active" {
		httpx.ErrorCode(w, r, httpx.CodeForbidden, "account is inactive")
		return
	}
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		s.audit.Write(r.Context(), r, audit.Event{
			ActorType: audit.ActorUser, ActorID: user.ID.String(), ActorLabel: user.Username,
			Action: "login", ResourceType: "session", Status: "failure",
			Metadata: map[string]string{"reason": "locked"},
		})
		httpx.ErrorCode(w, r, httpx.CodeForbidden, "account is temporarily locked")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		_ = s.store.RecordLoginFailure(r.Context(), user.ID, s.cfg.Session.MaxFailedLogin, s.cfg.Session.LockoutWindow)
		s.audit.Write(r.Context(), r, audit.Event{
			ActorType: audit.ActorUser, ActorID: user.ID.String(), ActorLabel: user.Username,
			Action: "login", ResourceType: "session", Status: "failure",
			Metadata: map[string]string{"reason": "bad_password"},
		})
		httpx.ErrorCode(w, r, httpx.CodeUnauthorized, "invalid credentials")
		return
	}

	_ = s.store.RecordLoginSuccess(r.Context(), user.ID)
	token, exp, err := auth.IssueSession(s.cfg.Session.Secret, user.ID, s.cfg.Session.TTL)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to create session")
		return
	}
	csrfToken, err := auth.GenerateSecret(24)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to create csrf token")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.Session.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.Session.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "ddag_csrf",
		Value:    csrfToken,
		Path:     "/",
		HttpOnly: false,
		Secure:   s.cfg.Session.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
	})
	s.audit.Write(r.Context(), r, audit.Event{
		ActorType: audit.ActorUser, ActorID: user.ID.String(), ActorLabel: user.Username,
		Action: "login", ResourceType: "session", Status: "success",
	})

	perms, _ := s.store.UserPermissionCodes(r.Context(), user.ID)
	ok(w, r, map[string]interface{}{
		"token":       token, // also returned for non-cookie clients
		"user":        user,
		"permissions": perms,
	})
}

// handleLogout clears the session cookie.
func (s *service) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: s.cfg.Session.CookieName, Value: "", Path: "/", HttpOnly: true,
		Secure: s.cfg.Session.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name: "ddag_csrf", Value: "", Path: "/", HttpOnly: false,
		Secure: s.cfg.Session.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	if p := principalOf(r); p != nil {
		s.audit.Write(r.Context(), r, audit.Event{
			ActorType: audit.ActorUser, ActorID: p.UserID.String(), ActorLabel: p.Username,
			Action: "logout", ResourceType: "session", Status: "success",
		})
	}
	ok(w, r, map[string]bool{"ok": true})
}

// handleMe returns the current user and effective permissions.
func (s *service) handleMe(w http.ResponseWriter, r *http.Request) {
	p := principalOf(r)
	user, err := s.store.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	perms := make([]string, 0, len(p.Permissions))
	for c := range p.Permissions {
		perms = append(perms, c)
	}
	ok(w, r, map[string]interface{}{"user": user, "permissions": perms, "roles": p.Roles})
}
