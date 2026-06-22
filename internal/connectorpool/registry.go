// Package connectorpool maintains exactly one live connection pool per source
// database, keyed by connection id. Pools are built lazily on first use and
// rebuilt when the connection's config_version changes, so admin edits take
// effect without a restart. This is the heart of the "pool per DB, configured
// per DB" requirement (PRD §27).
package connectorpool

import (
	"context"
	"sync"
	"time"

	"github.com/ddag/ddag/internal/connectors"
	"github.com/ddag/ddag/internal/metrics"
)

type entry struct {
	conn    connectors.Connector
	version int
}

// Registry owns the live connectors.
type Registry struct {
	mu      sync.Mutex
	entries map[string]*entry
	metrics *metrics.Metrics
}

// New creates an empty registry. metrics may be nil.
func New(m *metrics.Metrics) *Registry {
	return &Registry{entries: make(map[string]*entry), metrics: m}
}

// Acquire returns a live connector for cfg, (re)building it if absent or if the
// stored config_version differs from version.
func (r *Registry) Acquire(ctx context.Context, cfg connectors.PoolConfig, version int) (connectors.Connector, error) {
	r.mu.Lock()
	if e, ok := r.entries[cfg.ConnectionID]; ok && e.version == version {
		r.mu.Unlock()
		return e.conn, nil
	}
	// Capture any stale entry to close after building the new one.
	stale := r.entries[cfg.ConnectionID]
	r.mu.Unlock()

	conn, err := connectors.BuildFor(ctx, cfg)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	// Another goroutine may have built it concurrently; prefer the existing one.
	if e, ok := r.entries[cfg.ConnectionID]; ok && e.version == version {
		r.mu.Unlock()
		conn.Close()
		return e.conn, nil
	}
	r.entries[cfg.ConnectionID] = &entry{conn: conn, version: version}
	r.mu.Unlock()

	if stale != nil {
		stale.conn.Close()
	}
	return conn, nil
}

// Invalidate drops and closes the pool for a connection id.
func (r *Registry) Invalidate(connID string) {
	r.mu.Lock()
	e := r.entries[connID]
	delete(r.entries, connID)
	r.mu.Unlock()
	if e != nil {
		e.conn.Close()
	}
}

// PublishStats updates pool gauges for all live pools.
func (r *Registry) PublishStats() {
	if r.metrics == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, e := range r.entries {
		s := e.conn.Stats()
		r.metrics.PoolInUse.WithLabelValues(id).Set(float64(s.InUse))
		r.metrics.PoolIdle.WithLabelValues(id).Set(float64(s.Idle))
		r.metrics.PoolMax.WithLabelValues(id).Set(float64(s.Max))
	}
}

// StartStatsLoop publishes pool stats on an interval until ctx is cancelled.
func (r *Registry) StartStatsLoop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.PublishStats()
		}
	}
}

// CloseAll closes every live pool.
func (r *Registry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.entries {
		e.conn.Close()
	}
	r.entries = make(map[string]*entry)
}
