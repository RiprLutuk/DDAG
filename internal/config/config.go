// Package config loads DDAG service configuration from environment variables.
// Every value has a sensible local-dev default so services run out of the box
// against the local PostgreSQL (port 1921) and Redis instances.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the union of all settings any DDAG service may need. Each service
// reads only the sub-sections relevant to it.
type Config struct {
	Service  string // logical service name, used in logs/metrics
	Env      string // dev | staging | prod
	HTTPAddr string // listen address for the public/HTTP server
	LogLevel string // debug | info | warn | error

	Metadata PostgresConfig
	Redis    RedisConfig
	Secret   SecretConfig
	Auth     AuthConfig
	Session  SessionConfig
	Gateway  GatewayConfig
	Circuit  CircuitConfig

	// ConnCacheTTL bounds how long a connector caches a resolved connection
	// (metadata row + decrypted secret) before reloading. Keeps the metadata DB
	// and secret decryption off the data-plane hot path; also the staleness
	// window for picking up connection config changes.
	ConnCacheTTL time.Duration

	DashboardOrigins []string
}

// PostgresConfig describes a pgx connection pool. Used for the metadata DB and,
// with per-connection overrides, by the Postgres connector.
type PostgresConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	Database     string
	SSLMode      string
	MinConns     int32
	MaxConns     int32
	MaxConnLife  time.Duration
	MaxConnIdle  time.Duration
	HealthPeriod time.Duration
	ConnectTO    time.Duration
}

// DSN renders a pgx URL connection string. Using the URL form (rather than
// key/value) handles empty passwords correctly and URL-encodes credentials.
// Password is included only for the pool builder; it must never be logged.
func (p PostgresConfig) DSN() string {
	ssl := p.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	u := url.URL{
		Scheme:   "postgres",
		Host:     fmt.Sprintf("%s:%d", p.Host, p.Port),
		Path:     "/" + p.Database,
		RawQuery: "sslmode=" + ssl,
	}
	if p.Password != "" {
		u.User = url.UserPassword(p.User, p.Password)
	} else {
		u.User = url.User(p.User)
	}
	return u.String()
}

// RedisConfig configures the cache + rate-limit Redis client.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// SecretConfig holds the master key used by the envelope secret store.
type SecretConfig struct {
	// MasterKeyB64 is a base64-encoded 32-byte AES-256 master key.
	MasterKeyB64 string
}

// AuthConfig configures OAuth2 token issuance (auth-service) and the JWKS URL
// the gateway uses to verify access tokens.
type AuthConfig struct {
	Issuer              string
	Audience            string
	PrivateKeyPEM       string        // RS256 private key (PEM); auth-service only
	PrivateKeyPath      string        // optional file path alternative
	KeyID               string        // kid published in JWKS
	AccessTokenTTL      time.Duration // default access-token lifetime
	RefreshTokenTTL     time.Duration // default refresh-token lifetime
	JWKSURL             string        // gateway: where to fetch verification keys
	JWKSRefreshInterval time.Duration
	ClockSkew           time.Duration
}

// SessionConfig configures dashboard (human) login sessions in admin-backend.
type SessionConfig struct {
	Secret         string        // HS256 signing secret for session JWTs
	TTL            time.Duration // session lifetime / idle timeout
	CookieName     string
	CookieSecure   bool
	MaxFailedLogin int           // account lockout threshold
	LockoutWindow  time.Duration // window over which failures are counted
}

// GatewayConfig configures the dynamic API gateway data plane.
type GatewayConfig struct {
	PolicyMode          string            // inprocess | remote
	CacheMode           string            // inprocess | remote
	PolicyURL           string            // when PolicyMode=remote
	CacheURL            string            // when CacheMode=remote
	ConnectorURLs       map[string]string // db_type -> connector base URL
	RouteRefresh        time.Duration     // how often the gateway reloads API definitions
	DefaultLimit        int               // default row limit for list/search
	MaxLimit            int               // hard cap on row limit
	TrustedProxies      []string          // CIDR/single-IP ranges allowed to set X-Forwarded-For
	RateLimitFailMode   string            // open | closed
	InternalAuthSecret  string            // HMAC secret for gateway->connector requests
	BackpressureSize    int               // per API/connector queue capacity
	BackpressureTimeout time.Duration     // max wait before BACKPRESSURE_LIMIT
	RequestLogBuffer    int               // async access-log channel capacity (overflow is dropped + counted)
	RequestLogBatch     int               // max records per batched INSERT
	RequestLogFlush     time.Duration     // max time a record waits before flush
}

// CircuitConfig configures connector-side circuit breakers.
type CircuitConfig struct {
	MaxRequests      int
	Interval         time.Duration
	Timeout          time.Duration
	FailureThreshold int
	FailureRatio     float64
}

// Load builds a Config for the named service from the environment.
func Load(service string) Config {
	c := Config{
		Service:  service,
		Env:      getEnv("DDAG_ENV", "dev"),
		HTTPAddr: getEnv("DDAG_HTTP_ADDR", defaultAddr(service)),
		LogLevel: getEnv("DDAG_LOG_LEVEL", "info"),
		Metadata: PostgresConfig{
			Host:         getEnv("DDAG_DB_HOST", "localhost"),
			Port:         getEnvInt("DDAG_DB_PORT", 1921),
			User:         getEnv("DDAG_DB_USER", "lutuk"),
			Password:     getEnv("DDAG_DB_PASSWORD", ""),
			Database:     getEnv("DDAG_DB_NAME", "ddag"),
			SSLMode:      getEnv("DDAG_DB_SSLMODE", "disable"),
			MinConns:     int32(getEnvInt("DDAG_DB_MIN_CONNS", 2)),
			MaxConns:     int32(getEnvInt("DDAG_DB_MAX_CONNS", 10)),
			MaxConnLife:  getEnvDuration("DDAG_DB_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdle:  getEnvDuration("DDAG_DB_MAX_CONN_IDLE", 30*time.Minute),
			HealthPeriod: getEnvDuration("DDAG_DB_HEALTH_PERIOD", time.Minute),
			ConnectTO:    getEnvDuration("DDAG_DB_CONNECT_TIMEOUT", 5*time.Second),
		},
		Redis: RedisConfig{
			Addr:     getEnv("DDAG_REDIS_ADDR", "localhost:6379"),
			Password: getEnv("DDAG_REDIS_PASSWORD", ""),
			DB:       getEnvInt("DDAG_REDIS_DB", 0),
		},
		Secret: SecretConfig{
			// Dev default: deterministic 32-byte key (base64 of 32 'A' bytes-ish).
			// Production MUST override DDAG_MASTER_KEY.
			MasterKeyB64: getEnv("DDAG_MASTER_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="),
		},
		Auth: AuthConfig{
			Issuer:              getEnv("DDAG_TOKEN_ISSUER", "https://ddag.local"),
			Audience:            getEnv("DDAG_TOKEN_AUDIENCE", "ddag-api"),
			PrivateKeyPEM:       getEnv("DDAG_JWT_PRIVATE_KEY", ""),
			PrivateKeyPath:      getEnv("DDAG_JWT_PRIVATE_KEY_PATH", "configs/dev-jwt-private.pem"),
			KeyID:               getEnv("DDAG_JWT_KID", "ddag-dev-1"),
			AccessTokenTTL:      getEnvDuration("DDAG_ACCESS_TOKEN_TTL", time.Hour),
			RefreshTokenTTL:     getEnvDuration("DDAG_REFRESH_TOKEN_TTL", 720*time.Hour),
			JWKSURL:             getEnv("DDAG_JWKS_URL", "http://localhost:8081/.well-known/jwks.json"),
			JWKSRefreshInterval: getEnvDuration("DDAG_JWKS_REFRESH", 5*time.Minute),
			ClockSkew:           getEnvDuration("DDAG_TOKEN_CLOCK_SKEW", 30*time.Second),
		},
		Session: SessionConfig{
			Secret:         getEnv("DDAG_SESSION_SECRET", "dev-insecure-session-secret-change-me"),
			TTL:            getEnvDuration("DDAG_SESSION_TTL", 8*time.Hour),
			CookieName:     getEnv("DDAG_SESSION_COOKIE", "ddag_session"),
			CookieSecure:   getEnvBool("DDAG_SESSION_COOKIE_SECURE", false),
			MaxFailedLogin: getEnvInt("DDAG_MAX_FAILED_LOGIN", 5),
			LockoutWindow:  getEnvDuration("DDAG_LOCKOUT_WINDOW", 15*time.Minute),
		},
		DashboardOrigins: splitCSV(getEnv("DDAG_DASHBOARD_ORIGINS", "")),
		Gateway: GatewayConfig{
			PolicyMode: getEnv("DDAG_POLICY_MODE", "inprocess"),
			CacheMode:  getEnv("DDAG_CACHE_MODE", "inprocess"),
			PolicyURL:  getEnv("DDAG_POLICY_URL", "http://localhost:8083"),
			CacheURL:   getEnv("DDAG_CACHE_URL", "http://localhost:8084"),
			ConnectorURLs: map[string]string{
				"postgres":  getEnv("DDAG_CONNECTOR_POSTGRES_URL", "http://localhost:8090"),
				"mysql":     getEnv("DDAG_CONNECTOR_MYSQL_URL", "http://localhost:8091"),
				"oracle":    getEnv("DDAG_CONNECTOR_ORACLE_URL", "http://localhost:8092"),
				"sqlserver": getEnv("DDAG_CONNECTOR_SQLSERVER_URL", "http://localhost:8093"),
			},
			RouteRefresh:        getEnvDuration("DDAG_ROUTE_REFRESH", 15*time.Second),
			DefaultLimit:        getEnvInt("DDAG_DEFAULT_LIMIT", 100),
			MaxLimit:            getEnvInt("DDAG_MAX_LIMIT", 1000),
			TrustedProxies:      splitCSV(getEnv("DDAG_TRUSTED_PROXIES", "")),
			RateLimitFailMode:   strings.ToLower(getEnv("DDAG_RATE_LIMIT_FAIL_MODE", "open")),
			InternalAuthSecret:  getEnv("DDAG_INTERNAL_AUTH_SECRET", ""),
			BackpressureSize:    getEnvInt("DDAG_BACKPRESSURE_QUEUE_SIZE", 32),
			BackpressureTimeout: getEnvDuration("DDAG_BACKPRESSURE_TIMEOUT", 500*time.Millisecond),
			RequestLogBuffer:    getEnvInt("DDAG_REQUEST_LOG_BUFFER", 4096),
			RequestLogBatch:     getEnvInt("DDAG_REQUEST_LOG_BATCH", 200),
			RequestLogFlush:     getEnvDuration("DDAG_REQUEST_LOG_FLUSH", time.Second),
		},
		Circuit: CircuitConfig{
			MaxRequests:      getEnvInt("DDAG_CB_MAX_REQUESTS", 1),
			Interval:         getEnvDuration("DDAG_CB_INTERVAL", 60*time.Second),
			Timeout:          getEnvDuration("DDAG_CB_TIMEOUT", 30*time.Second),
			FailureThreshold: getEnvInt("DDAG_CB_FAILURE_THRESHOLD", 5),
			FailureRatio:     getEnvFloat("DDAG_CB_FAILURE_RATIO", 0.6),
		},
		ConnCacheTTL: getEnvDuration("DDAG_CONN_CACHE_TTL", 15*time.Second),
	}
	return c
}

// Validate enforces startup safety rules. In production it refuses known-dev
// secrets and insecure cookies so a service cannot accidentally boot with local
// defaults.
func (c Config) Validate() error {
	if strings.ToLower(c.Env) != "prod" {
		return nil
	}
	var problems []string
	if c.Secret.MasterKeyB64 == "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" {
		problems = append(problems, "DDAG_MASTER_KEY must not use the development default")
	}
	if c.Session.Secret == "dev-insecure-session-secret-change-me" {
		problems = append(problems, "DDAG_SESSION_SECRET must not use the development default")
	}
	if !c.Session.CookieSecure {
		problems = append(problems, "DDAG_SESSION_COOKIE_SECURE must be true in production")
	}
	if p := getEnv("DDAG_SUPERADMIN_PASSWORD", "Admin#12345"); p == "" || p == "Admin#12345" {
		problems = append(problems, "DDAG_SUPERADMIN_PASSWORD must not use the development default")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// Warnings returns non-fatal production hardening findings that operators should
// fix before exposing DDAG.
func (c Config) Warnings() []string {
	if strings.ToLower(c.Env) != "prod" {
		return nil
	}
	var warnings []string
	for _, origin := range c.DashboardOrigins {
		origin = strings.TrimSpace(strings.ToLower(origin))
		if origin == "*" || strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
			warnings = append(warnings, "DDAG_DASHBOARD_ORIGINS should not include wildcard or localhost origins in production")
			break
		}
	}
	if strings.EqualFold(c.Metadata.SSLMode, "disable") {
		warnings = append(warnings, "DDAG_DB_SSLMODE should not be disable in production")
	}
	return warnings
}

func (c Config) LogWarnings(log *slog.Logger) {
	if log == nil {
		return
	}
	for _, warning := range c.Warnings() {
		log.Warn("config_warning", "warning", warning)
	}
}

// defaultAddr assigns a stable local port per service so all of them can run
// side by side during development.
func defaultAddr(service string) string {
	ports := map[string]string{
		"admin-backend":       ":8080",
		"auth-service":        ":8081",
		"api-gateway":         ":8082",
		"policy-engine":       ":8083",
		"cache-service":       ":8084",
		"worker":              ":8085",
		"connector-postgres":  ":8090",
		"connector-mysql":     ":8091",
		"connector-oracle":    ":8092",
		"connector-sqlserver": ":8093",
	}
	if p, ok := ports[service]; ok {
		return p
	}
	return ":8080"
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d
		}
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return def
}

func getEnvFloat(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return n
		}
	}
	return def
}

func splitCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
