package connector

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ddag/ddag/internal/connectors"
	"github.com/ddag/ddag/internal/metrics"
	"github.com/ddag/ddag/internal/models"
	"github.com/ddag/ddag/internal/secret"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

// errConnNotFound marks a failed metadata lookup so handleQuery can map it to a
// 404 without charging the circuit breaker — a missing connection is a client/
// config error, not a source-DB failure.
var errConnNotFound = errors.New("connection not found")

// connectionStore is the slice of *store.Store that connCache needs. Declaring
// it as an interface keeps the cache unit-testable without a live metadata DB.
type connectionStore interface {
	GetConnection(ctx context.Context, id uuid.UUID) (*models.DatabaseConnection, error)
}

// resolvedConn is the cached resolution of a database connection: the pool
// config (including the decrypted password) plus the fields handleQuery needs to
// validate and label a request. All of it is stable for a given config_version.
type resolvedConn struct {
	cfg     connectors.PoolConfig
	version int
	dbType  string
	status  string
	name    string
	idStr   string
}

// connCache keeps connection resolution (metadata row + decrypted secret) off
// the data-plane hot path. Each resolution is cached per connection id for a
// short TTL, so a warm request makes zero metadata round-trips and zero secret
// decryptions. The live source pool is still keyed on config_version by the
// registry, so a config change takes effect within one TTL window — the same
// staleness bound the gateway already accepts for route refresh.
type connCache struct {
	store   connectionStore
	secrets secret.Store
	metrics *metrics.Metrics
	dbType  string
	ttl     time.Duration
	now     func() time.Time

	sf singleflight.Group
	mu sync.RWMutex
	m  map[string]cacheEntry
}

type cacheEntry struct {
	rc        *resolvedConn
	expiresAt time.Time
}

func newConnCache(st connectionStore, secrets secret.Store, ttl time.Duration, m *metrics.Metrics, dbType string) *connCache {
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	return &connCache{
		store: st, secrets: secrets, metrics: m, dbType: dbType, ttl: ttl,
		now: time.Now, m: map[string]cacheEntry{},
	}
}

// Resolve returns the cached resolution for connID, loading it on a miss.
// Concurrent misses for the same id are collapsed so a cold cache cannot
// stampede the metadata pool. Errors are never cached.
func (c *connCache) Resolve(ctx context.Context, connID uuid.UUID) (*resolvedConn, error) {
	key := connID.String()

	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if ok && c.now().Before(e.expiresAt) {
		c.hit()
		return e.rc, nil
	}
	c.miss()

	v, err, _ := c.sf.Do(key, func() (any, error) {
		rc, err := c.load(ctx, connID)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.m[key] = cacheEntry{rc: rc, expiresAt: c.now().Add(c.ttl)}
		c.mu.Unlock()
		return rc, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*resolvedConn), nil
}

func (c *connCache) load(ctx context.Context, connID uuid.UUID) (*resolvedConn, error) {
	conn, err := c.store.GetConnection(ctx, connID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errConnNotFound, err)
	}
	password := ""
	if conn.SecretRef != nil {
		b, err := c.secrets.Get(ctx, *conn.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("resolve secret: %w", err)
		}
		password = string(b)
	}
	return &resolvedConn{
		cfg: connectors.PoolConfig{
			ConnectionID:    conn.ID.String(),
			DatabaseType:    conn.DatabaseType,
			Host:            conn.Host,
			Port:            conn.Port,
			Database:        conn.DatabaseName,
			ServiceName:     conn.ServiceName,
			Schema:          conn.SchemaName,
			Username:        conn.Username,
			Password:        password,
			SSLMode:         conn.SSLMode,
			MinPool:         conn.MinPoolSize,
			MaxPool:         conn.MaxPoolSize,
			ConnectTimeout:  msDur(conn.ConnectionTimeoutMS, 5000),
			QueryTimeout:    msDur(conn.QueryTimeoutMS, 30000),
			MaxConnLifetime: msDur(conn.MaxConnLifetimeMS, 3600000),
			MaxConnIdle:     msDur(conn.MaxConnIdleMS, 1800000),
		},
		version: conn.ConfigVersion,
		dbType:  conn.DatabaseType,
		status:  conn.Status,
		name:    conn.Name,
		idStr:   conn.ID.String(),
	}, nil
}

func (c *connCache) hit() {
	if c.metrics != nil {
		c.metrics.ConnCacheHits.WithLabelValues(c.dbType).Inc()
	}
}

func (c *connCache) miss() {
	if c.metrics != nil {
		c.metrics.ConnCacheMisses.WithLabelValues(c.dbType).Inc()
	}
}
