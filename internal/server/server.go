// Package server wires the shared middleware chain, health/metrics endpoints,
// and graceful shutdown used by every DDAG HTTP service.
package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/metrics"
)

// Service describes one runnable HTTP service.
type Service struct {
	Name       string
	Addr       string
	Handler    http.Handler // application routes
	Logger     *logging.Logger
	Metrics    *metrics.Metrics
	Ready      metrics.ReadyFunc // optional readiness probe
	OnShutdown func(context.Context)
}

// Run starts the service and blocks until a termination signal, then shuts down
// gracefully. /healthz, /readyz and /metrics are served outside the app
// middleware so probes and scrapes stay cheap and uninstrumented.
func (s Service) Run() error {
	root := http.NewServeMux()
	s.Metrics.MountHealth(root, s.Ready)

	chain := httpx.RequestIDMiddleware(
		s.Metrics.HTTPMiddleware(
			httpx.LoggingMiddleware(s.Logger)(
				httpx.RecoverMiddleware(s.Handler))))
	root.Handle("/", chain)

	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           root,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.Logger.Info("listening", "addr", s.Addr)
		metrics.SetReady(true)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		s.Logger.Info("shutting down", "signal", sig.String())
	}

	metrics.SetReady(false)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if s.OnShutdown != nil {
		s.OnShutdown(ctx)
	}
	return srv.Shutdown(ctx)
}
