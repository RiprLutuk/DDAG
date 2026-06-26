package adminsvc

import (
	"context"
	"net/http"
	"time"

	"github.com/ddag/ddag/internal/audit"
	"github.com/ddag/ddag/internal/bootstrap"
	"github.com/ddag/ddag/internal/cache"
	"github.com/ddag/ddag/internal/config"
	"github.com/ddag/ddag/internal/db"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/metrics"
	"github.com/ddag/ddag/internal/secret"
	"github.com/ddag/ddag/internal/server"
	"github.com/ddag/ddag/internal/store"
	"github.com/redis/go-redis/v9"
)

type service struct {
	cfg        config.Config
	store      *store.Store
	secrets    secret.Store
	audit      *audit.Recorder
	cache      *cache.Cache
	rdb        *redis.Client
	connectors map[string]string // db_type -> connector base URL
	httpc      *http.Client
	log        *logging.Logger
}

// Run starts the admin-backend and blocks. On boot it migrates the schema and
// seeds core data so the dashboard is immediately usable (idempotent).
func Run() error {
	cfg := config.Load("admin-backend")
	if err := cfg.Validate(); err != nil {
		return err
	}
	log := logging.New("admin-backend", cfg.LogLevel)
	cfg.LogWarnings(log)
	m := metrics.New("admin-backend")
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.Metadata)
	if err != nil {
		return err
	}

	// Auto-migrate + core-seed on boot so the dashboard is usable out of the box
	// (idempotent). Disable with DDAG_AUTO_MIGRATE=false in managed deployments.
	if cfg.Env == "dev" || getBool("DDAG_AUTO_MIGRATE", true) {
		if err := bootstrap.Migrate(ctx, pool, log); err != nil {
			return err
		}
		if err := bootstrap.SeedCore(ctx, pool, log); err != nil {
			return err
		}
	}

	sec, err := secret.NewEnvelopeStore(pool, cfg.Secret.MasterKeyB64)
	if err != nil {
		return err
	}
	st := store.New(pool)

	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})

	svc := &service{
		cfg:        cfg,
		store:      st,
		secrets:    sec,
		audit:      audit.New(st),
		cache:      cache.NewWithClient(rdb),
		rdb:        rdb,
		connectors: cfg.Gateway.ConnectorURLs,
		httpc:      &http.Client{Timeout: 30 * time.Second},
		log:        log,
	}

	return server.Service{
		Name: "admin-backend", Addr: cfg.HTTPAddr, Handler: svc.routes(), Logger: log, Metrics: m,
		Ready:      func() bool { return pool.Ping(ctx) == nil },
		OnShutdown: func(context.Context) { _ = rdb.Close(); pool.Close() },
	}.Run()
}
