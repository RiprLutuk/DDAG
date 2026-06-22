package gatewaysvc

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ddag/ddag/internal/audit"
	"github.com/ddag/ddag/internal/auth"
	"github.com/ddag/ddag/internal/cache"
	"github.com/ddag/ddag/internal/config"
	"github.com/ddag/ddag/internal/db"
	"github.com/ddag/ddag/internal/gateway"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/metrics"
	"github.com/ddag/ddag/internal/models"
	"github.com/ddag/ddag/internal/policy"
	"github.com/ddag/ddag/internal/server"
	"github.com/ddag/ddag/internal/store"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type service struct {
	cfg       config.Config
	store     *store.Store
	router    *gateway.Router
	jwks      *jwksCache
	cache     *cache.Cache
	limiter   *policy.RateLimiter
	connector *gateway.ConnectorClient
	metrics   *metrics.Metrics
	audit     *audit.Recorder
	log       *logging.Logger

	crMu       sync.RWMutex
	cacheRules map[uuid.UUID]models.CacheRule
}

// Run starts the api-gateway and blocks.
func Run() error {
	cfg := config.Load("api-gateway")
	log := logging.New("api-gateway", cfg.LogLevel)
	m := metrics.New("api-gateway")
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.Metadata)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})

	svc := &service{
		cfg:        cfg,
		store:      store.New(pool),
		router:     gateway.NewRouter(),
		jwks:       newJWKS(cfg.Auth.JWKSURL),
		cache:      cache.NewWithClient(rdb),
		limiter:    policy.NewRateLimiter(rdb),
		connector:  gateway.NewConnectorClient(cfg.Gateway.ConnectorURLs),
		metrics:    m,
		audit:      audit.New(store.New(pool)),
		log:        log,
		cacheRules: map[uuid.UUID]models.CacheRule{},
	}

	// Initial JWKS load with a short retry (auth-service may start concurrently).
	for i := 0; i < 5; i++ {
		if err := svc.jwks.refresh(ctx); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err := svc.loadRoutes(ctx); err != nil {
		log.Warn("initial_route_load_failed", "error", err.Error())
	}
	log.Info("routes_loaded", "count", svc.router.Count())

	refreshCtx, cancelRefresh := context.WithCancel(ctx)
	go svc.jwks.startRefresh(refreshCtx, cfg.Auth.JWKSRefreshInterval)
	go svc.routeRefreshLoop(refreshCtx, cfg.Gateway.RouteRefresh)

	return server.Service{
		Name: "api-gateway", Addr: cfg.HTTPAddr, Handler: http.HandlerFunc(svc.serve), Logger: log, Metrics: m,
		Ready: func() bool { return pool.Ping(ctx) == nil && svc.jwks.hasKeys() },
		OnShutdown: func(context.Context) {
			cancelRefresh()
			_ = rdb.Close()
			pool.Close()
		},
	}.Run()
}

func (j *jwksCache) hasKeys() bool {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return len(j.keys) > 0
}

func (s *service) loadRoutes(ctx context.Context) error {
	apis, err := s.store.ListPublishedAPIs(ctx)
	if err != nil {
		return err
	}
	s.router.Build(apis)
	rules := map[uuid.UUID]models.CacheRule{}
	for _, a := range apis {
		if cr, err := s.store.GetCacheRule(ctx, a.ID); err == nil {
			rules[a.ID] = *cr
		}
	}
	s.crMu.Lock()
	s.cacheRules = rules
	s.crMu.Unlock()
	return nil
}

func (s *service) routeRefreshLoop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.loadRoutes(ctx); err != nil {
				s.log.Warn("route_refresh_failed", "error", err.Error())
			}
		}
	}
}

func (s *service) cacheRuleFor(apiID uuid.UUID) (models.CacheRule, bool) {
	s.crMu.RLock()
	defer s.crMu.RUnlock()
	cr, ok := s.cacheRules[apiID]
	return cr, ok
}

// serve is the dynamic data-plane handler executing the full pipeline:
// route → JWT → scope → access → IP → rate limit → cache → connector.
func (s *service) serve(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	route, pathParams, ok := s.router.Match(r.Method, r.URL.Path)
	if !ok {
		metrics.SetRouteLabel(r.Context(), "unmatched")
		httpx.ErrorCode(w, r, httpx.CodeNotFound, "Endpoint not found or not published")
		return
	}
	api := route.API
	metrics.SetRouteLabel(r.Context(), api.Method+" "+api.Path)

	rlog := &reqLog{start: start, requestID: httpx.RequestID(r.Context()), method: r.Method, path: r.URL.Path, ip: httpx.ClientIP(r), apiID: api.ID, apiLabel: api.Method + " " + api.Path}
	defer func() { s.writeRequestLog(rlog) }()

	// 1. Authentication: verify bearer JWT.
	claims, apiErr := s.authenticate(r)
	if apiErr != nil {
		s.metrics.Unauthorized.Inc()
		rlog.status, rlog.errCode = http.StatusUnauthorized, apiErr.Code
		s.recordSecurity(r, "unauthorized_request", "", api.ID.String())
		httpx.Error(w, r, apiErr)
		return
	}
	rlog.clientLabel = claims.ClientID

	// 2. Scope.
	if !policy.HasScope(claims.Scope, api.RequiredScope) {
		s.metrics.Forbidden.Inc()
		rlog.status, rlog.errCode = http.StatusForbidden, httpx.CodeForbidden
		s.recordSecurity(r, "forbidden_request", claims.ClientID, api.ID.String())
		httpx.ErrorCode(w, r, httpx.CodeForbidden, "Token scope does not grant access to this API")
		return
	}

	// 3. Resolve client + API access grant.
	client, err := s.store.GetClientByClientID(r.Context(), claims.ClientID)
	if err != nil || client.Status != "active" {
		s.metrics.Forbidden.Inc()
		rlog.status, rlog.errCode = http.StatusForbidden, httpx.CodeForbidden
		httpx.ErrorCode(w, r, httpx.CodeForbidden, "Client is not active")
		return
	}
	rlog.clientID = &client.ID
	allowed, _ := s.store.ClientHasAPIAccess(r.Context(), client.ID, api.ID)
	if !allowed {
		s.metrics.Forbidden.Inc()
		rlog.status, rlog.errCode = http.StatusForbidden, httpx.CodeForbidden
		s.recordSecurity(r, "forbidden_request", claims.ClientID, api.ID.String())
		httpx.ErrorCode(w, r, httpx.CodeForbidden, "Client is not granted access to this API")
		return
	}

	// 4. IP whitelist.
	if blocked := s.checkIP(r, client.ID, api.ID); blocked {
		s.metrics.IPBlocked.WithLabelValues(client.ClientID).Inc()
		rlog.status, rlog.errCode = http.StatusForbidden, httpx.CodeForbidden
		s.recordSecurity(r, "ip_blocked", claims.ClientID, api.ID.String())
		httpx.ErrorCode(w, r, httpx.CodeForbidden, "Source IP is not allowed")
		return
	}

	// 5. Rate limit.
	if dec, limited := s.checkRateLimit(r, client, api); limited {
		s.metrics.RateLimited.WithLabelValues(client.ClientID, api.Path).Inc()
		w.Header().Set("Retry-After", strconv.Itoa(dec.ResetSeconds))
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(dec.Limit))
		w.Header().Set("X-RateLimit-Remaining", "0")
		rlog.status, rlog.errCode = http.StatusTooManyRequests, httpx.CodeRateLimited
		httpx.ErrorCode(w, r, httpx.CodeRateLimited, "Rate limit exceeded, retry later")
		return
	}

	// 6. Resolve + validate parameters.
	body := s.readBody(r)
	params, apiErr := gateway.ResolveParams(api.Parameters, pathParams, r.URL.Query(), body)
	if apiErr != nil {
		rlog.status, rlog.errCode = apiErr.HTTPStatus(), apiErr.Code
		httpx.Error(w, r, apiErr)
		return
	}

	effLimit := s.effectiveLimit(r, api)
	isList := api.MaxLimit != 1

	// 7. Cache lookup.
	cr, hasCR := s.cacheRuleFor(api.ID)
	cacheEnabled := hasCR && cr.Enabled
	var cacheKey string
	if cacheEnabled {
		cacheKey = cache.Key(api.ID, client.ClientID, cr.VaryByClient, params)
		if b, found, _ := s.cache.Get(r.Context(), cacheKey); found {
			s.metrics.CacheHits.WithLabelValues(api.Path).Inc()
			rlog.cached, rlog.status = true, http.StatusOK
			s.writeCached(w, r, b, start)
			return
		}
		s.metrics.CacheMisses.WithLabelValues(api.Path).Inc()
	}

	// 8. Connector dispatch.
	if api.DatabaseConnectionID == nil {
		rlog.status, rlog.errCode = http.StatusInternalServerError, httpx.CodeInternal
		httpx.ErrorCode(w, r, httpx.CodeInternal, "API has no database connection configured")
		return
	}
	connResp, apiErr := s.connector.Query(r.Context(), api.ConnectorType, gateway.ConnectorRequest{
		RequestID:     httpx.RequestID(r.Context()),
		ConnectionID:  api.DatabaseConnectionID.String(),
		QueryTemplate: api.QueryTemplate,
		Parameters:    params,
		TimeoutMS:     0,
		Limit:         effLimit,
	})
	if apiErr != nil {
		rlog.status, rlog.errCode = apiErr.HTTPStatus(), apiErr.Code
		httpx.Error(w, r, apiErr)
		return
	}
	rlog.sourceMS = int(connResp.DurationMS)
	rlog.status = http.StatusOK

	// 9. Build payload, cache, respond.
	payload := buildPayload(connResp, isList, effLimit)
	if cacheEnabled {
		if b, err := json.Marshal(payload); err == nil {
			_ = s.cache.Set(r.Context(), cacheKey, b, time.Duration(cr.TTLSeconds)*time.Second)
		}
	}
	s.writePayload(w, r, payload, false, start, connResp.DurationMS)
}

func (s *service) authenticate(r *http.Request) (*auth.AccessClaims, *httpx.APIError) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil, httpx.NewError(httpx.CodeUnauthorized, "Missing or malformed Authorization header")
	}
	token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	claims, err := auth.ParseAccessToken(token, s.jwks.keyfunc)
	if err != nil {
		return nil, httpx.NewError(httpx.CodeUnauthorized, "Invalid or expired token")
	}
	return claims, nil
}

func (s *service) checkIP(r *http.Request, clientID, apiID uuid.UUID) bool {
	entries, err := s.store.IPWhitelistsFor(r.Context(), clientID, apiID)
	if err != nil || len(entries) == 0 {
		return false // no restriction
	}
	cidrs := make([]string, 0, len(entries))
	for _, e := range entries {
		cidrs = append(cidrs, e.IPCIDR)
	}
	return !policy.IPAllowed(httpx.ClientIP(r), cidrs)
}

func (s *service) checkRateLimit(r *http.Request, client *models.Client, api models.APIDefinition) (policy.Decision, bool) {
	rules, err := s.store.RateLimitRulesFor(r.Context(), client.ID, api.ID)
	if err != nil || len(rules) == 0 {
		return policy.Decision{Allowed: true}, false
	}
	rule := rules[0] // most specific (ordered by specificity in the query)
	windows := policy.WindowsFromRule(rule.RequestsPerSecond, rule.RequestsPerMinute, rule.RequestsPerHour, rule.RequestsPerDay)
	base := client.ClientID + ":" + api.ID.String()
	dec, err := s.limiter.Allow(r.Context(), base, windows)
	if err != nil {
		return policy.Decision{Allowed: true}, false // fail open on limiter error
	}
	return dec, !dec.Allowed
}

func (s *service) effectiveLimit(r *http.Request, api models.APIDefinition) int {
	limit := api.DefaultLimit
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}
	if api.MaxLimit > 0 && limit > api.MaxLimit {
		limit = api.MaxLimit
	}
	if limit <= 0 {
		limit = 100
	}
	return limit
}

func (s *service) readBody(r *http.Request) map[string]any {
	if r.Method != http.MethodPost {
		return nil
	}
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		return nil
	}
	var body map[string]any
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(&body); err != nil {
		return nil
	}
	return body
}

func (s *service) recordSecurity(r *http.Request, action, clientID, apiID string) {
	s.audit.Write(r.Context(), r, audit.Event{
		ActorType: audit.ActorClient, ActorID: clientID, Action: action,
		ResourceType: "api", ResourceID: apiID, Status: "failure",
	})
}
