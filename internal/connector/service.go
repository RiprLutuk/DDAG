// Package connector is the shared implementation of a DB-type connector
// service. Each cmd/connector-<type> binary calls Run(<type>) so they share one
// codebase but deploy as separate pods/images (PRD §9, §19.1). The connector
// owns the pools, resolves secrets securely, and never accepts raw SQL from the
// public internet (PRD §11.12).
package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ddag/ddag/internal/config"
	"github.com/ddag/ddag/internal/connectorpool"
	"github.com/ddag/ddag/internal/connectors"
	"github.com/ddag/ddag/internal/db"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/metrics"
	"github.com/ddag/ddag/internal/models"
	"github.com/ddag/ddag/internal/secret"
	"github.com/ddag/ddag/internal/server"
	"github.com/ddag/ddag/internal/store"
	"github.com/google/uuid"
)

type service struct {
	dbType   string
	store    *store.Store
	secrets  secret.Store
	registry *connectorpool.Registry
	metrics  *metrics.Metrics
	log      *logging.Logger
}

// Run starts the connector service for the given database type and blocks.
func Run(dbType string) error {
	serviceName := "connector-" + dbType
	cfg := config.Load(serviceName)
	log := logging.New(serviceName, cfg.LogLevel)
	m := metrics.New(serviceName)
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.Metadata)
	if err != nil {
		return err
	}
	sec, err := secret.NewEnvelopeStore(pool, cfg.Secret.MasterKeyB64)
	if err != nil {
		return err
	}
	reg := connectorpool.New(m)

	svc := &service{
		dbType: dbType, store: store.New(pool), secrets: sec, registry: reg, metrics: m, log: log,
	}

	statsCtx, cancelStats := context.WithCancel(ctx)
	go reg.StartStatsLoop(statsCtx, 15*time.Second)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /query", svc.handleQuery)
	mux.HandleFunc("POST /test", svc.handleTest)

	return server.Service{
		Name: serviceName, Addr: cfg.HTTPAddr, Handler: mux, Logger: log, Metrics: m,
		Ready: func() bool { return pool.Ping(ctx) == nil },
		OnShutdown: func(context.Context) {
			cancelStats()
			reg.CloseAll()
			pool.Close()
		},
	}.Run()
}

// handleQuery executes a query against a saved connection (PRD §16.2). The
// gateway is the only caller; raw SQL is never accepted from clients — the
// query template comes from a published, validated API definition.
func (s *service) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RequestID     string         `json:"request_id"`
		ConnectionID  string         `json:"connection_id"`
		QueryTemplate string         `json:"query_template"`
		Parameters    map[string]any `json:"parameters"`
		TimeoutMS     int            `json:"timeout_ms"`
		Limit         int            `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErr(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	connID, err := uuid.Parse(req.ConnectionID)
	if err != nil {
		s.writeErr(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid connection id")
		return
	}
	conn, err := s.store.GetConnection(r.Context(), connID)
	if err != nil {
		s.writeErr(w, http.StatusNotFound, httpx.CodeNotFound, "connection not found")
		return
	}
	if conn.DatabaseType != s.dbType {
		s.writeErr(w, http.StatusBadRequest, httpx.CodeBadRequest, "connection is not a "+s.dbType+" database")
		return
	}
	if conn.Status != "active" {
		s.writeErr(w, http.StatusServiceUnavailable, httpx.CodeSourceDBUnavailable, "connection is disabled")
		return
	}

	cfg, err := s.poolConfig(r.Context(), conn)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, httpx.CodeInternal, "failed to resolve connection secret")
		return
	}
	c, err := s.registry.Acquire(r.Context(), cfg, conn.ConfigVersion)
	if err != nil {
		s.metrics.ConnectorErr.WithLabelValues(conn.ID.String(), s.dbType).Inc()
		s.log.Error("pool_acquire_failed", "connection", conn.Name, "error", err.Error())
		s.writeErr(w, http.StatusServiceUnavailable, httpx.CodeSourceDBUnavailable, "source database unavailable")
		return
	}

	res, err := c.Query(r.Context(), connectors.QueryRequest{
		RequestID: req.RequestID, QueryTemplate: req.QueryTemplate,
		Parameters: req.Parameters, TimeoutMS: req.TimeoutMS, Limit: req.Limit,
	})
	if err != nil {
		s.metrics.ConnectorErr.WithLabelValues(conn.ID.String(), s.dbType).Inc()
		// Sanitize: do not leak raw driver errors to the caller (PRD §13.5).
		s.log.Warn("query_failed", "connection", conn.Name, "error", err.Error())
		code, status := classifyQueryErr(err)
		s.writeErr(w, status, code, "query failed")
		return
	}
	s.metrics.QueryDuration.WithLabelValues(conn.ID.String(), s.dbType).Observe(float64(res.DurationMS) / 1000.0)
	httpx.WriteJSON(w, http.StatusOK, res)
}

// handleTest tests connectivity using the provided (unsaved) parameters so the
// dashboard can "Test connection" before saving (PRD §11.6 AC).
func (s *service) handleTest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host             string `json:"host"`
		Port             int    `json:"port"`
		Database         string `json:"database_name"`
		ServiceName      string `json:"service_name"`
		Schema           string `json:"schema_name"`
		Username         string `json:"username"`
		Password         string `json:"password"`
		SSLMode          string `json:"ssl_mode"`
		ConnectTimeoutMS int    `json:"connection_timeout_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErr(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	cfg := connectors.PoolConfig{
		ConnectionID: "test-" + uuid.NewString(), DatabaseType: s.dbType,
		Host: req.Host, Port: req.Port, Database: req.Database, ServiceName: req.ServiceName,
		Schema: req.Schema, Username: req.Username, Password: req.Password, SSLMode: req.SSLMode,
		MinPool: 1, MaxPool: 2,
		ConnectTimeout: msDur(req.ConnectTimeoutMS, 5000),
	}
	start := time.Now()
	if err := connectors.TestConnectivity(r.Context(), cfg); err != nil {
		s.log.Warn("test_connection_failed", "host", req.Host, "error", err.Error())
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"success": false, "message": sanitizeConnErr(err),
		})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true, "message": "connection successful", "duration_ms": time.Since(start).Milliseconds(),
	})
}

func (s *service) poolConfig(ctx context.Context, conn *models.DatabaseConnection) (connectors.PoolConfig, error) {
	password := ""
	if conn.SecretRef != nil {
		b, err := s.secrets.Get(ctx, *conn.SecretRef)
		if err != nil {
			return connectors.PoolConfig{}, err
		}
		password = string(b)
	}
	return connectors.PoolConfig{
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
	}, nil
}

func (s *service) writeErr(w http.ResponseWriter, status int, code, msg string) {
	httpx.WriteJSON(w, status, map[string]any{
		"success": false,
		"error":   map[string]string{"code": code, "message": msg},
	})
}

func msDur(ms, def int) time.Duration {
	if ms <= 0 {
		ms = def
	}
	return time.Duration(ms) * time.Millisecond
}

func classifyQueryErr(err error) (code string, status int) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "context deadline exceeded"), strings.Contains(msg, "timeout"):
		return httpx.CodeQueryTimeout, http.StatusRequestTimeout
	case strings.Contains(msg, "missing value for parameter"):
		return httpx.CodeBadRequest, http.StatusBadRequest
	default:
		return httpx.CodeConnectorError, http.StatusBadGateway
	}
}

// sanitizeConnErr returns a client-safe connection error message.
func sanitizeConnErr(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "connection refused"):
		return "connection refused — check host/port"
	case strings.Contains(msg, "password authentication failed"), strings.Contains(msg, "authentication failed"):
		return "authentication failed — check username/password"
	case strings.Contains(msg, "does not exist"):
		return "database or schema does not exist"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"):
		return "connection timed out"
	default:
		return "connection failed"
	}
}
