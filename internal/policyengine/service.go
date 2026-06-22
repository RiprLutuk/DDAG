// Package policyengine is the standalone policy decision service exposing the
// PRD §16.1 contract (POST /policy/check). It wraps the same internal/policy +
// store logic the gateway uses in-process, so the gateway can run with
// DDAG_POLICY_MODE=remote and offload decisions to dedicated pods.
package policyengine

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ddag/ddag/internal/config"
	"github.com/ddag/ddag/internal/db"
	"github.com/ddag/ddag/internal/gateway"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/metrics"
	"github.com/ddag/ddag/internal/policy"
	"github.com/ddag/ddag/internal/server"
	"github.com/ddag/ddag/internal/store"
	"github.com/redis/go-redis/v9"
)

type service struct {
	store   *store.Store
	router  *gateway.Router
	limiter *policy.RateLimiter
	log     *logging.Logger
}

// Run starts the policy-engine and blocks.
func Run() error {
	cfg := config.Load("policy-engine")
	log := logging.New("policy-engine", cfg.LogLevel)
	m := metrics.New("policy-engine")
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.Metadata)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	svc := &service{store: store.New(pool), router: gateway.NewRouter(), limiter: policy.NewRateLimiter(rdb), log: log}
	_ = svc.reload(ctx)

	refreshCtx, cancel := context.WithCancel(ctx)
	go svc.refreshLoop(refreshCtx, cfg.Gateway.RouteRefresh)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /policy/check", svc.handleCheck)

	return server.Service{
		Name: "policy-engine", Addr: cfg.HTTPAddr, Handler: mux, Logger: log, Metrics: m,
		Ready:      func() bool { return pool.Ping(ctx) == nil },
		OnShutdown: func(context.Context) { cancel(); _ = rdb.Close(); pool.Close() },
	}.Run()
}

func (s *service) reload(ctx context.Context) error {
	apis, err := s.store.ListPublishedAPIs(ctx)
	if err != nil {
		return err
	}
	s.router.Build(apis)
	return nil
}

func (s *service) refreshLoop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = s.reload(ctx)
		}
	}
}

// CheckRequest is the gateway→policy-engine request (PRD §16.1).
type CheckRequest struct {
	RequestID string `json:"request_id"`
	ClientID  string `json:"client_id"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Scope     string `json:"scope"`
	IPAddress string `json:"ip_address"`
}

// CheckResponse is the decision (PRD §16.1).
type CheckResponse struct {
	Allowed   bool    `json:"allowed"`
	Reason    *string `json:"reason"`
	RateLimit struct {
		Remaining    int `json:"remaining"`
		ResetSeconds int `json:"reset_seconds"`
	} `json:"rate_limit"`
}

func (s *service) handleCheck(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	deny := func(reason string) {
		httpx.WriteJSON(w, http.StatusOK, CheckResponse{Allowed: false, Reason: &reason})
	}

	route, _, ok := s.router.Match(req.Method, req.Path)
	if !ok {
		deny("api_not_found")
		return
	}
	api := route.API
	client, err := s.store.GetClientByClientID(r.Context(), req.ClientID)
	if err != nil || client.Status != "active" {
		deny("client_inactive")
		return
	}
	if !policy.HasScope(req.Scope, api.RequiredScope) {
		deny("scope_denied")
		return
	}
	if allowed, _ := s.store.ClientHasAPIAccess(r.Context(), client.ID, api.ID); !allowed {
		deny("access_denied")
		return
	}
	if entries, _ := s.store.IPWhitelistsFor(r.Context(), client.ID, api.ID); len(entries) > 0 {
		cidrs := make([]string, 0, len(entries))
		for _, e := range entries {
			cidrs = append(cidrs, e.IPCIDR)
		}
		if !policy.IPAllowed(req.IPAddress, cidrs) {
			deny("ip_blocked")
			return
		}
	}

	resp := CheckResponse{Allowed: true}
	resp.RateLimit.Remaining = -1
	if rules, _ := s.store.RateLimitRulesFor(r.Context(), client.ID, api.ID); len(rules) > 0 {
		rule := rules[0]
		windows := policy.WindowsFromRule(rule.RequestsPerSecond, rule.RequestsPerMinute, rule.RequestsPerHour, rule.RequestsPerDay)
		dec, err := s.limiter.Allow(r.Context(), client.ClientID+":"+api.ID.String(), windows)
		if err == nil {
			resp.RateLimit.Remaining = dec.Remaining
			resp.RateLimit.ResetSeconds = dec.ResetSeconds
			if !dec.Allowed {
				deny("rate_limited")
				return
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}
