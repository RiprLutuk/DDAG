// Package cacheservice is the standalone cache management pod (PRD §9
// cache-service): it exposes cache-rule listing and purge over HTTP, wrapping
// internal/cache + the metadata store. The admin-backend can proxy to it, or it
// can be operated directly.
package cacheservice

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ddag/ddag/internal/cache"
	"github.com/ddag/ddag/internal/config"
	"github.com/ddag/ddag/internal/db"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/metrics"
	"github.com/ddag/ddag/internal/server"
	"github.com/ddag/ddag/internal/store"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type service struct {
	store *store.Store
	cache *cache.Cache
	log   *logging.Logger
}

// Run starts the cache-service and blocks.
func Run() error {
	cfg := config.Load("cache-service")
	log := logging.New("cache-service", cfg.LogLevel)
	m := metrics.New("cache-service")
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.Metadata)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	svc := &service{store: store.New(pool), cache: cache.NewWithClient(rdb), log: log}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /cache/rules", svc.handleRules)
	mux.HandleFunc("POST /cache/purge", svc.handlePurge)

	return server.Service{
		Name: "cache-service", Addr: cfg.HTTPAddr, Handler: mux, Logger: log, Metrics: m,
		Ready:      func() bool { return pool.Ping(ctx) == nil && rdb.Ping(ctx).Err() == nil },
		OnShutdown: func(context.Context) { _ = rdb.Close(); pool.Close() },
	}.Run()
}

func (s *service) handleRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.ListCacheRules(r.Context())
	if err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list rules"})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func (s *service) handlePurge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIID string `json:"api_id"`
		All   bool   `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
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
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "api_id or all required"})
			return
		}
		count, err = s.cache.PurgeEndpoint(r.Context(), id)
	}
	if err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "purge failed"})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"purged": count})
}
