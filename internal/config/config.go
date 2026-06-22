// Package config loads DDAG service configuration from environment variables.
// Every value has a sensible local-dev default so services run out of the box
// against the local PostgreSQL (port 1921) and Redis instances.
package config

import (
	"fmt"
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
	PrivateKeyPEM       string        // RS256 private key (PEM); auth-service only
	PrivateKeyPath      string        // optional file path alternative
	KeyID               string        // kid published in JWKS
	AccessTokenTTL      time.Duration // default access-token lifetime
	RefreshTokenTTL     time.Duration // default refresh-token lifetime
	JWKSURL             string        // gateway: where to fetch verification keys
	JWKSRefreshInterval time.Duration
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
	PolicyMode    string            // inprocess | remote
	CacheMode     string            // inprocess | remote
	PolicyURL     string            // when PolicyMode=remote
	CacheURL      string            // when CacheMode=remote
	ConnectorURLs map[string]string // db_type -> connector base URL
	RouteRefresh  time.Duration     // how often the gateway reloads API definitions
	DefaultLimit  int               // default row limit for list/search
	MaxLimit      int               // hard cap on row limit
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
			PrivateKeyPEM:       getEnv("DDAG_JWT_PRIVATE_KEY", ""),
			PrivateKeyPath:      getEnv("DDAG_JWT_PRIVATE_KEY_PATH", "configs/dev-jwt-private.pem"),
			KeyID:               getEnv("DDAG_JWT_KID", "ddag-dev-1"),
			AccessTokenTTL:      getEnvDuration("DDAG_ACCESS_TOKEN_TTL", time.Hour),
			RefreshTokenTTL:     getEnvDuration("DDAG_REFRESH_TOKEN_TTL", 720*time.Hour),
			JWKSURL:             getEnv("DDAG_JWKS_URL", "http://localhost:8081/.well-known/jwks.json"),
			JWKSRefreshInterval: getEnvDuration("DDAG_JWKS_REFRESH", 5*time.Minute),
		},
		Session: SessionConfig{
			Secret:         getEnv("DDAG_SESSION_SECRET", "dev-insecure-session-secret-change-me"),
			TTL:            getEnvDuration("DDAG_SESSION_TTL", 8*time.Hour),
			CookieName:     getEnv("DDAG_SESSION_COOKIE", "ddag_session"),
			CookieSecure:   getEnvBool("DDAG_SESSION_COOKIE_SECURE", false),
			MaxFailedLogin: getEnvInt("DDAG_MAX_FAILED_LOGIN", 5),
			LockoutWindow:  getEnvDuration("DDAG_LOCKOUT_WINDOW", 15*time.Minute),
		},
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
			RouteRefresh: getEnvDuration("DDAG_ROUTE_REFRESH", 15*time.Second),
			DefaultLimit: getEnvInt("DDAG_DEFAULT_LIMIT", 100),
			MaxLimit:     getEnvInt("DDAG_MAX_LIMIT", 1000),
		},
	}
	return c
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
	}
	return def
}
