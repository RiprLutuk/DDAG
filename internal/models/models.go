// Package models holds the metadata entity structs. `db` tags map to columns
// (used by scany), `json` tags define the dashboard/API wire format. Secret
// material is always `json:"-"`.
package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// User is a dashboard (human) account.
type User struct {
	ID               uuid.UUID  `db:"id" json:"id"`
	Name             string     `db:"name" json:"name"`
	Email            string     `db:"email" json:"email"`
	Username         string     `db:"username" json:"username"`
	PasswordHash     string     `db:"password_hash" json:"-"`
	Status           string     `db:"status" json:"status"`
	Tenant           *string    `db:"tenant" json:"tenant,omitempty"`
	FailedLoginCount int        `db:"failed_login_count" json:"-"`
	LockedUntil      *time.Time `db:"locked_until" json:"locked_until,omitempty"`
	LastLoginAt      *time.Time `db:"last_login_at" json:"last_login_at,omitempty"`
	CreatedBy        *uuid.UUID `db:"created_by" json:"created_by,omitempty"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
	Roles            []string   `db:"-" json:"roles,omitempty"`
}

// Role is an RBAC role.
type Role struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	IsSystem    bool      `db:"is_system" json:"is_system"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
	Permissions []string  `db:"-" json:"permissions,omitempty"`
}

// Permission is a single permission code.
type Permission struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Code        string    `db:"code" json:"code"`
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// Scope is an OAuth2 scope.
type Scope struct {
	ID          uuid.UUID `db:"id" json:"id"`
	ScopeCode   string    `db:"scope_code" json:"scope_code"`
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// Client is an OAuth2 client/application.
type Client struct {
	ID                     uuid.UUID   `db:"id" json:"id"`
	ClientID               string      `db:"client_id" json:"client_id"`
	ClientName             string      `db:"client_name" json:"client_name"`
	ClientSecretHash       string      `db:"client_secret_hash" json:"-"`
	OwnerUserID            *uuid.UUID  `db:"owner_user_id" json:"owner_user_id,omitempty"`
	Environment            string      `db:"environment" json:"environment"`
	Status                 string      `db:"status" json:"status"`
	AccessTokenTTLSeconds  int         `db:"access_token_ttl_seconds" json:"access_token_ttl_seconds"`
	RefreshTokenTTLSeconds int         `db:"refresh_token_ttl_seconds" json:"refresh_token_ttl_seconds"`
	Description            string      `db:"description" json:"description"`
	CreatedBy              *uuid.UUID  `db:"created_by" json:"created_by,omitempty"`
	CreatedAt              time.Time   `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time   `db:"updated_at" json:"updated_at"`
	Scopes                 []string    `db:"-" json:"scopes,omitempty"`
	APIs                   []uuid.UUID `db:"-" json:"apis,omitempty"`
}

// RefreshToken is a stored, revocable refresh token (hash only).
type RefreshToken struct {
	ID        uuid.UUID `db:"id" json:"id"`
	TokenHash string    `db:"token_hash" json:"-"`
	ClientID  uuid.UUID `db:"client_id" json:"client_id"`
	Scope     string    `db:"scope" json:"scope"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	Revoked   bool      `db:"revoked" json:"revoked"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// DatabaseConnection describes a pooled connection to a source database.
type DatabaseConnection struct {
	ID                  uuid.UUID  `db:"id" json:"id"`
	Name                string     `db:"name" json:"name"`
	DatabaseType        string     `db:"database_type" json:"database_type"`
	Host                string     `db:"host" json:"host"`
	Port                int        `db:"port" json:"port"`
	DatabaseName        string     `db:"database_name" json:"database_name"`
	ServiceName         string     `db:"service_name" json:"service_name"`
	SchemaName          string     `db:"schema_name" json:"schema_name"`
	Username            string     `db:"username" json:"username"`
	SecretRef           *uuid.UUID `db:"secret_ref" json:"-"`
	SSLMode             string     `db:"ssl_mode" json:"ssl_mode"`
	MinPoolSize         int        `db:"min_pool_size" json:"min_pool_size"`
	MaxPoolSize         int        `db:"max_pool_size" json:"max_pool_size"`
	ConnectionTimeoutMS int        `db:"connection_timeout_ms" json:"connection_timeout_ms"`
	QueryTimeoutMS      int        `db:"query_timeout_ms" json:"query_timeout_ms"`
	MaxConnLifetimeMS   int        `db:"max_conn_lifetime_ms" json:"max_conn_lifetime_ms"`
	MaxConnIdleMS       int        `db:"max_conn_idle_ms" json:"max_conn_idle_ms"`
	Environment         string     `db:"environment" json:"environment"`
	Status              string     `db:"status" json:"status"`
	Tags                []string   `db:"tags" json:"tags"`
	ConfigVersion       int        `db:"config_version" json:"config_version"`
	LastHealthStatus    string     `db:"last_health_status" json:"last_health_status"`
	LastHealthAt        *time.Time `db:"last_health_at" json:"last_health_at,omitempty"`
	CreatedBy           *uuid.UUID `db:"created_by" json:"created_by,omitempty"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at" json:"updated_at"`
}

// APIDefinition is a dynamic API endpoint.
type APIDefinition struct {
	ID                   uuid.UUID       `db:"id" json:"id"`
	Name                 string          `db:"name" json:"name"`
	Namespace            string          `db:"namespace" json:"namespace"`
	Path                 string          `db:"path" json:"path"`
	Method               string          `db:"method" json:"method"`
	Description          string          `db:"description" json:"description"`
	DatabaseConnectionID *uuid.UUID      `db:"database_connection_id" json:"database_connection_id,omitempty"`
	ConnectorType        string          `db:"connector_type" json:"connector_type"`
	QueryTemplate        string          `db:"query_template" json:"query_template"`
	ResponseMapping      json.RawMessage `db:"response_mapping" json:"response_mapping,omitempty"`
	Status               string          `db:"status" json:"status"`
	Version              int             `db:"version" json:"version"`
	RequiredScope        string          `db:"required_scope" json:"required_scope"`
	DefaultLimit         int             `db:"default_limit" json:"default_limit"`
	MaxLimit             int             `db:"max_limit" json:"max_limit"`
	IsWrite              bool            `db:"is_write" json:"is_write"`
	CreatedBy            *uuid.UUID      `db:"created_by" json:"created_by,omitempty"`
	ApprovedBy           *uuid.UUID      `db:"approved_by" json:"approved_by,omitempty"`
	ApprovedComment      string          `db:"approved_comment" json:"approved_comment,omitempty"`
	PublishedAt          *time.Time      `db:"published_at" json:"published_at,omitempty"`
	DeprecatedAt         *time.Time      `db:"deprecated_at" json:"deprecated_at,omitempty"`
	CreatedAt            time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time       `db:"updated_at" json:"updated_at"`
	Parameters           []APIParameter  `db:"-" json:"parameters,omitempty"`
	ConnectionName       string          `db:"-" json:"connection_name,omitempty"`
}

// OperationType derives semantic operation from Method without adding a metadata
// schema column, preserving compatibility with existing rows and migrations.
func (a APIDefinition) OperationType() OperationType { return OperationFromMethod(a.Method) }

// APIRevision is an immutable published snapshot for API lifecycle history.
type APIRevision struct {
	ID               uuid.UUID       `db:"id" json:"id"`
	APIDefinitionID  uuid.UUID       `db:"api_definition_id" json:"api_definition_id"`
	Revision         int             `db:"revision" json:"revision"`
	Snapshot         json.RawMessage `db:"snapshot" json:"snapshot"`
	SnapshotChecksum string          `db:"snapshot_checksum" json:"snapshot_checksum"`
	ApprovedBy       *uuid.UUID      `db:"approved_by" json:"approved_by,omitempty"`
	ApprovedComment  string          `db:"approved_comment" json:"approved_comment,omitempty"`
	PublishedBy      *uuid.UUID      `db:"published_by" json:"published_by,omitempty"`
	PublishedAt      time.Time       `db:"published_at" json:"published_at"`
	CreatedAt        time.Time       `db:"created_at" json:"created_at"`
}

// APIParameter is a typed, validated input for an API.
type APIParameter struct {
	ID              uuid.UUID `db:"id" json:"id"`
	APIDefinitionID uuid.UUID `db:"api_definition_id" json:"api_definition_id"`
	Name            string    `db:"name" json:"name"`
	Source          string    `db:"source" json:"source"`
	ParamType       string    `db:"param_type" json:"param_type"`
	Required        bool      `db:"required" json:"required"`
	DefaultValue    *string   `db:"default_value" json:"default_value,omitempty"`
	MaxLength       *int      `db:"max_length" json:"max_length,omitempty"`
	ValidationRule  *string   `db:"validation_rule" json:"validation_rule,omitempty"`
	Position        int       `db:"position" json:"position"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

// CacheRule is the per-endpoint cache configuration.
type CacheRule struct {
	ID               uuid.UUID `db:"id" json:"id"`
	APIDefinitionID  uuid.UUID `db:"api_definition_id" json:"api_definition_id"`
	APIName          string    `db:"api_name" json:"api_name,omitempty"`
	APIMethod        string    `db:"api_method" json:"api_method,omitempty"`
	APIPath          string    `db:"api_path" json:"api_path,omitempty"`
	Enabled          bool      `db:"enabled" json:"enabled"`
	TTLSeconds       int       `db:"ttl_seconds" json:"ttl_seconds"`
	CacheKeyStrategy string    `db:"cache_key_strategy" json:"cache_key_strategy"`
	VaryByClient     bool      `db:"vary_by_client" json:"vary_by_client"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

// RateLimitRule is a rate-limit policy.
type RateLimitRule struct {
	ID                uuid.UUID  `db:"id" json:"id"`
	ClientID          *uuid.UUID `db:"client_id" json:"client_id,omitempty"`
	APIDefinitionID   *uuid.UUID `db:"api_definition_id" json:"api_definition_id,omitempty"`
	AppliesTo         string     `db:"applies_to" json:"applies_to"`
	RequestsPerSecond int        `db:"requests_per_second" json:"requests_per_second"`
	RequestsPerMinute int        `db:"requests_per_minute" json:"requests_per_minute"`
	RequestsPerHour   int        `db:"requests_per_hour" json:"requests_per_hour"`
	RequestsPerDay    int        `db:"requests_per_day" json:"requests_per_day"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

// IPWhitelist is an allowed IP/CIDR entry.
type IPWhitelist struct {
	ID              uuid.UUID  `db:"id" json:"id"`
	ClientID        *uuid.UUID `db:"client_id" json:"client_id,omitempty"`
	APIDefinitionID *uuid.UUID `db:"api_definition_id" json:"api_definition_id,omitempty"`
	IPCIDR          string     `db:"ip_cidr" json:"ip_cidr"`
	ScopeLevel      string     `db:"scope_level" json:"scope_level"`
	Status          string     `db:"status" json:"status"`
	Description     string     `db:"description" json:"description"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
}

// AuditLog is a single audit event (append-only).
type AuditLog struct {
	ID           uuid.UUID       `db:"id" json:"id"`
	RequestID    string          `db:"request_id" json:"request_id"`
	ActorType    string          `db:"actor_type" json:"actor_type"`
	ActorID      string          `db:"actor_id" json:"actor_id"`
	ActorLabel   string          `db:"actor_label" json:"actor_label"`
	Action       string          `db:"action" json:"action"`
	ResourceType string          `db:"resource_type" json:"resource_type"`
	ResourceID   string          `db:"resource_id" json:"resource_id"`
	IPAddress    string          `db:"ip_address" json:"ip_address"`
	UserAgent    string          `db:"user_agent" json:"user_agent"`
	Status       string          `db:"status" json:"status"`
	MetadataJSON json.RawMessage `db:"metadata_json" json:"metadata_json,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
}

// APIRequestLog is a single data-plane request record.
type APIRequestLog struct {
	ID                 int64      `db:"id" json:"id"`
	RequestID          string     `db:"request_id" json:"request_id"`
	ClientID           *uuid.UUID `db:"client_id" json:"client_id,omitempty"`
	APIDefinitionID    *uuid.UUID `db:"api_definition_id" json:"api_definition_id,omitempty"`
	ClientLabel        string     `db:"client_label" json:"client_label"`
	APILabel           string     `db:"api_label" json:"api_label"`
	Method             string     `db:"method" json:"method"`
	Path               string     `db:"path" json:"path"`
	StatusCode         int        `db:"status_code" json:"status_code"`
	ErrorCode          string     `db:"error_code" json:"error_code"`
	LatencyMS          int        `db:"latency_ms" json:"latency_ms"`
	Cached             bool       `db:"cached" json:"cached"`
	Operation          string     `db:"operation" json:"operation"`
	SourceDBDurationMS int        `db:"source_db_duration_ms" json:"source_db_duration_ms"`
	IPAddress          string     `db:"ip_address" json:"ip_address"`
	CreatedAt          time.Time  `db:"created_at" json:"created_at"`
}

// SigningKey is an RS256 JWT signing key (public part + encrypted private ref).
type SigningKey struct {
	ID               uuid.UUID `db:"id" json:"id"`
	KID              string    `db:"kid" json:"kid"`
	PublicKeyPEM     string    `db:"public_key_pem" json:"-"`
	PrivateSecretRef uuid.UUID `db:"private_secret_ref" json:"-"`
	Algorithm        string    `db:"algorithm" json:"algorithm"`
	Status           string    `db:"status" json:"status"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

// Setting is a key/value platform setting.
type Setting struct {
	Key             string          `db:"key" json:"key"`
	Value           json.RawMessage `db:"value" json:"value"`
	Category        string          `db:"category" json:"category"`
	ValueType       string          `db:"value_type" json:"type"`
	Scope           string          `db:"scope" json:"scope"`
	Description     string          `db:"description" json:"description"`
	DefaultValue    json.RawMessage `db:"default_value" json:"default"`
	RestartRequired bool            `db:"restart_required" json:"restart_required"`
	UpdatedBy       *uuid.UUID      `db:"updated_by" json:"updated_by,omitempty"`
	UpdatedAt       time.Time       `db:"updated_at" json:"updated_at"`
}

// MaintenanceJob is a registered allowlisted operational task.
type MaintenanceJob struct {
	Key            string          `db:"key" json:"key"`
	Name           string          `db:"name" json:"name"`
	Category       string          `db:"category" json:"category"`
	Description    string          `db:"description" json:"description"`
	Safe           bool            `db:"safe" json:"safe"`
	Enabled        bool            `db:"enabled" json:"enabled"`
	LastRunAt      *time.Time      `db:"last_run_at" json:"last_run_at,omitempty"`
	LastStatus     string          `db:"last_status" json:"last_status"`
	LastDurationMS int             `db:"last_duration_ms" json:"last_duration_ms"`
	LastError      string          `db:"last_error" json:"last_error"`
	LastResult     json.RawMessage `db:"last_result" json:"last_result"`
	UpdatedAt      time.Time       `db:"updated_at" json:"updated_at"`
}

// Service is a control-plane registry entry for DDAG services.
type Service struct {
	ID               uuid.UUID       `db:"id" json:"id"`
	Name             string          `db:"name" json:"name"`
	Kind             string          `db:"kind" json:"kind"`
	BaseURL          string          `db:"base_url" json:"base_url"`
	Enabled          bool            `db:"enabled" json:"enabled"`
	ManagedBy        string          `db:"managed_by" json:"managed_by"`
	Version          string          `db:"version" json:"version"`
	CommitSHA        string          `db:"commit_sha" json:"commit_sha"`
	HealthURL        string          `db:"health_url" json:"health_url"`
	ReadyURL         string          `db:"ready_url" json:"ready_url"`
	MetricsURL       string          `db:"metrics_url" json:"metrics_url"`
	Capabilities     json.RawMessage `db:"capabilities" json:"capabilities"`
	LastSeenAt       *time.Time      `db:"last_seen_at" json:"last_seen_at,omitempty"`
	LastHealthStatus string          `db:"last_health_status" json:"last_health_status"`
	LastError        string          `db:"last_error" json:"last_error"`
	CreatedAt        time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at" json:"updated_at"`
}
