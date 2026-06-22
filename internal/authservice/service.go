package authservice

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ddag/ddag/internal/audit"
	"github.com/ddag/ddag/internal/auth"
	"github.com/ddag/ddag/internal/config"
	"github.com/ddag/ddag/internal/db"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/metrics"
	"github.com/ddag/ddag/internal/models"
	"github.com/ddag/ddag/internal/secret"
	"github.com/ddag/ddag/internal/server"
	"github.com/ddag/ddag/internal/store"
)

type service struct {
	cfg     config.Config
	store   *store.Store
	keys    *keyManager
	metrics *metrics.Metrics
	audit   *audit.Recorder
	log     *logging.Logger
}

// Run starts the auth-service and blocks.
func Run() error {
	cfg := config.Load("auth-service")
	log := logging.New("auth-service", cfg.LogLevel)
	m := metrics.New("auth-service")
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.Metadata)
	if err != nil {
		return err
	}
	sec, err := secret.NewEnvelopeStore(pool, cfg.Secret.MasterKeyB64)
	if err != nil {
		return err
	}
	st := store.New(pool)
	km, err := loadKeyManager(ctx, st, sec)
	if err != nil {
		return err
	}
	signKID, _ := km.signer()
	log.Info("signing_key_loaded", "kid", signKID, "jwks_keys", len(km.jwks().Keys))

	svc := &service{cfg: cfg, store: st, keys: km, metrics: m, audit: audit.New(st), log: log}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/token", svc.handleToken)
	mux.HandleFunc("POST /oauth/refresh", svc.handleRefresh)
	mux.HandleFunc("POST /oauth/revoke", svc.handleRevoke)
	mux.HandleFunc("POST /oauth/introspect", svc.handleIntrospect)
	mux.HandleFunc("GET /.well-known/jwks.json", svc.handleJWKS)

	return server.Service{
		Name: "auth-service", Addr: cfg.HTTPAddr, Handler: mux, Logger: log, Metrics: m,
		Ready:      func() bool { return pool.Ping(ctx) == nil },
		OnShutdown: func(context.Context) { pool.Close() },
	}.Run()
}

// tokenRequest captures token/refresh/revoke fields from JSON or form bodies.
type tokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
	Token        string `json:"token"`
}

func parseTokenRequest(r *http.Request) tokenRequest {
	var req tokenRequest
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&req)
		return req
	}
	_ = r.ParseForm()
	req.GrantType = r.PostForm.Get("grant_type")
	req.ClientID = r.PostForm.Get("client_id")
	req.ClientSecret = r.PostForm.Get("client_secret")
	req.Scope = r.PostForm.Get("scope")
	req.RefreshToken = r.PostForm.Get("refresh_token")
	req.Token = r.PostForm.Get("token")
	// Support HTTP Basic auth for client credentials.
	if id, secretVal, ok := r.BasicAuth(); ok {
		req.ClientID, req.ClientSecret = id, secretVal
	}
	return req
}

func (s *service) handleToken(w http.ResponseWriter, r *http.Request) {
	req := parseTokenRequest(r)
	switch req.GrantType {
	case "client_credentials":
		s.clientCredentials(w, r, req)
	case "refresh_token":
		s.refresh(w, r, req.RefreshToken)
	default:
		s.oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be client_credentials or refresh_token")
	}
}

func (s *service) handleRefresh(w http.ResponseWriter, r *http.Request) {
	req := parseTokenRequest(r)
	s.refresh(w, r, req.RefreshToken)
}

func (s *service) clientCredentials(w http.ResponseWriter, r *http.Request, req tokenRequest) {
	client, err := s.store.GetClientByClientID(r.Context(), req.ClientID)
	if err != nil || client.Status != "active" {
		s.metrics.TokenFailed.Inc()
		s.audit.Write(r.Context(), r, audit.Event{
			ActorType: audit.ActorClient, ActorID: req.ClientID, Action: "token_issue",
			ResourceType: "token", Status: "failure", Metadata: map[string]string{"reason": "invalid_client"},
		})
		s.oauthError(w, http.StatusUnauthorized, "invalid_client", "client not found or inactive")
		return
	}
	if !auth.CheckPassword(client.ClientSecretHash, req.ClientSecret) {
		s.metrics.TokenFailed.Inc()
		s.audit.Write(r.Context(), r, audit.Event{
			ActorType: audit.ActorClient, ActorID: client.ClientID, ActorLabel: client.ClientName,
			Action: "token_issue", ResourceType: "token", Status: "failure",
			Metadata: map[string]string{"reason": "invalid_secret"},
		})
		s.oauthError(w, http.StatusUnauthorized, "invalid_client", "invalid client secret")
		return
	}

	granted, ok := resolveScope(req.Scope, client.Scopes)
	if !ok {
		s.metrics.TokenFailed.Inc()
		s.oauthError(w, http.StatusBadRequest, "invalid_scope", "requested scope exceeds client grants")
		return
	}
	s.issueTokens(w, r, client, granted)
}

func (s *service) refresh(w http.ResponseWriter, r *http.Request, refreshToken string) {
	if refreshToken == "" {
		s.oauthError(w, http.StatusBadRequest, "invalid_request", "refresh_token is required")
		return
	}
	rt, err := s.store.GetRefreshToken(r.Context(), auth.HashToken(refreshToken))
	if err != nil || rt.Revoked || time.Now().After(rt.ExpiresAt) {
		s.metrics.TokenFailed.Inc()
		s.oauthError(w, http.StatusBadRequest, "invalid_grant", "refresh token is invalid, expired or revoked")
		return
	}
	client, err := s.store.GetClientByPK(r.Context(), rt.ClientID)
	if err != nil || client.Status != "active" {
		s.oauthError(w, http.StatusBadRequest, "invalid_grant", "client is inactive")
		return
	}
	// Rotate: revoke the used refresh token, issue a fresh pair.
	_, _ = s.store.RevokeRefreshToken(r.Context(), auth.HashToken(refreshToken))
	s.issueTokens(w, r, client, rt.Scope)
}

func (s *service) issueTokens(w http.ResponseWriter, r *http.Request, client *models.Client, scope string) {
	kid, priv := s.keys.signer()
	accessTTL := time.Duration(client.AccessTokenTTLSeconds) * time.Second
	if accessTTL <= 0 {
		accessTTL = s.cfg.Auth.AccessTokenTTL
	}
	accessToken, _, err := auth.IssueAccessToken(priv, kid, s.cfg.Auth.Issuer, client.ClientID, scope, accessTTL)
	if err != nil {
		s.oauthError(w, http.StatusInternalServerError, "server_error", "failed to issue token")
		return
	}
	refreshTTL := time.Duration(client.RefreshTokenTTLSeconds) * time.Second
	if refreshTTL <= 0 {
		refreshTTL = s.cfg.Auth.RefreshTokenTTL
	}
	refreshToken, err := auth.GenerateSecret(32)
	if err != nil {
		s.oauthError(w, http.StatusInternalServerError, "server_error", "failed to issue refresh token")
		return
	}
	if err := s.store.CreateRefreshToken(r.Context(), auth.HashToken(refreshToken), client.ID, scope, time.Now().Add(refreshTTL)); err != nil {
		s.oauthError(w, http.StatusInternalServerError, "server_error", "failed to persist refresh token")
		return
	}

	s.metrics.TokenIssued.Inc()
	s.audit.Write(r.Context(), r, audit.Event{
		ActorType: audit.ActorClient, ActorID: client.ClientID, ActorLabel: client.ClientName,
		Action: "token_issue", ResourceType: "token", Status: "success",
		Metadata: map[string]string{"scope": scope},
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int(accessTTL.Seconds()),
		"scope":         scope,
	})
}

func (s *service) handleRevoke(w http.ResponseWriter, r *http.Request) {
	req := parseTokenRequest(r)
	token := req.Token
	if token == "" {
		token = req.RefreshToken
	}
	if token == "" {
		s.oauthError(w, http.StatusBadRequest, "invalid_request", "token is required")
		return
	}
	revoked, _ := s.store.RevokeRefreshToken(r.Context(), auth.HashToken(token))
	if revoked {
		s.metrics.TokenRevoked.Inc()
		s.audit.Write(r.Context(), r, audit.Event{
			ActorType: audit.ActorClient, Action: "token_revoke", ResourceType: "token", Status: "success",
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"revoked": revoked})
}

func (s *service) handleIntrospect(w http.ResponseWriter, r *http.Request) {
	req := parseTokenRequest(r)
	if req.Token == "" {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"active": false})
		return
	}
	claims, err := auth.ParseAccessToken(req.Token, s.keys.keyfunc)
	if err != nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"active": false})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"active":     true,
		"client_id":  claims.ClientID,
		"scope":      claims.Scope,
		"token_type": "Bearer",
		"exp":        claims.ExpiresAt.Unix(),
		"iat":        claims.IssuedAt.Unix(),
		"iss":        claims.Issuer,
		"sub":        claims.Subject,
	})
}

func (s *service) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, s.keys.jwks())
}

func (s *service) oauthError(w http.ResponseWriter, status int, code, desc string) {
	httpx.WriteJSON(w, status, map[string]string{"error": code, "error_description": desc})
}

// resolveScope returns the granted scope. With no request, all client scopes are
// granted; otherwise every requested scope must be among the client's grants.
func resolveScope(requested string, allowed []string) (string, bool) {
	allowedSet := map[string]bool{}
	for _, a := range allowed {
		allowedSet[a] = true
	}
	if strings.TrimSpace(requested) == "" {
		return strings.Join(allowed, " "), true
	}
	for _, s := range strings.Fields(requested) {
		if !allowedSet[s] {
			return "", false
		}
	}
	return strings.Join(strings.Fields(requested), " "), true
}
