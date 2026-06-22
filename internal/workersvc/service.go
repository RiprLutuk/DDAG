// Package workersvc is the background worker: periodic maintenance (expired
// refresh-token cleanup), with hooks for cache warming and async audit flushing
// (PRD §9 worker-service). It exposes only health/metrics.
package workersvc

import (
	"context"
	"net/http"
	"time"

	"github.com/ddag/ddag/internal/config"
	"github.com/ddag/ddag/internal/db"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/metrics"
	"github.com/ddag/ddag/internal/server"
	"github.com/ddag/ddag/internal/store"
)

// Run starts the worker and blocks.
func Run() error {
	cfg := config.Load("worker")
	log := logging.New("worker", cfg.LogLevel)
	m := metrics.New("worker")
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.Metadata)
	if err != nil {
		return err
	}
	st := store.New(pool)

	loopCtx, cancel := context.WithCancel(ctx)
	go maintenanceLoop(loopCtx, st, log)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })

	return server.Service{
		Name: "worker", Addr: cfg.HTTPAddr, Handler: mux, Logger: log, Metrics: m,
		Ready:      func() bool { return pool.Ping(ctx) == nil },
		OnShutdown: func(context.Context) { cancel(); pool.Close() },
	}.Run()
}

// maintenanceLoop runs periodic housekeeping tasks.
func maintenanceLoop(ctx context.Context, st *store.Store, log *logging.Logger) {
	t := time.NewTicker(1 * time.Hour)
	defer t.Stop()
	run := func() {
		// Purge refresh tokens that expired more than 7 days ago.
		c, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if n, err := st.DeleteExpiredRefreshTokens(c, 7*24*time.Hour); err != nil {
			log.Warn("refresh_token_cleanup_failed", "error", err.Error())
		} else if n > 0 {
			log.Info("refresh_tokens_purged", "count", n)
		}
	}
	run() // run once on start
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}
