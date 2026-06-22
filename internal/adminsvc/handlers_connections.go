package adminsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/models"
)

func (s *service) listConnections(w http.ResponseWriter, r *http.Request) {
	p := listParams(r)
	conns, total, err := s.store.ListConnections(r.Context(), p)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	list(w, r, conns, p, total)
}

func (s *service) getConnection(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	c, err := s.store.GetConnection(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, c)
}

// connInput is the shared create/update payload for a connection.
type connInput struct {
	Name                string   `json:"name"`
	DatabaseType        string   `json:"database_type"`
	Host                string   `json:"host"`
	Port                int      `json:"port"`
	DatabaseName        string   `json:"database_name"`
	ServiceName         string   `json:"service_name"`
	SchemaName          string   `json:"schema_name"`
	Username            string   `json:"username"`
	Password            string   `json:"password"`
	SSLMode             string   `json:"ssl_mode"`
	MinPoolSize         int      `json:"min_pool_size"`
	MaxPoolSize         int      `json:"max_pool_size"`
	ConnectionTimeoutMS int      `json:"connection_timeout_ms"`
	QueryTimeoutMS      int      `json:"query_timeout_ms"`
	MaxConnLifetimeMS   int      `json:"max_conn_lifetime_ms"`
	MaxConnIdleMS       int      `json:"max_conn_idle_ms"`
	Environment         string   `json:"environment"`
	Status              string   `json:"status"`
	Tags                []string `json:"tags"`
}

func (s *service) createConnection(w http.ResponseWriter, r *http.Request) {
	var in connInput
	if !decode(w, r, &in) {
		return
	}
	if in.Name == "" || in.DatabaseType == "" || in.Host == "" {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "name, database_type and host are required")
		return
	}
	// Encrypt the password at rest (PRD §13.4). Always stored via the secret store.
	ref, err := s.secrets.Put(r.Context(), []byte(in.Password), "db_password")
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to store secret")
		return
	}
	actor := principalOf(r).UserID
	c := connFromInput(in)
	c.SecretRef = &ref
	c.CreatedBy = &actor
	id, err := s.store.CreateConnection(r.Context(), c)
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeConflict, "connection name may already exist")
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_db_connection", "connection", id.String(), map[string]any{"name": in.Name}))
	created, _ := s.store.GetConnection(r.Context(), id)
	httpx.Created(w, r, created)
}

func (s *service) updateConnection(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	var in connInput
	if !decode(w, r, &in) {
		return
	}
	existing, err := s.store.GetConnection(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	c := connFromInput(in)
	c.ID = id
	// Rotate the secret only when a new password is supplied.
	if in.Password != "" {
		if existing.SecretRef != nil {
			_ = s.secrets.Update(r.Context(), *existing.SecretRef, []byte(in.Password))
		} else {
			ref, _ := s.secrets.Put(r.Context(), []byte(in.Password), "db_password")
			_ = s.store.SetConnectionSecret(r.Context(), id, ref)
		}
	}
	if err := s.store.UpdateConnection(r.Context(), c); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_db_connection", "connection", id.String(), nil))
	updated, _ := s.store.GetConnection(r.Context(), id)
	ok(w, r, updated)
}

func (s *service) deleteConnection(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	if err := s.store.DeleteConnection(r.Context(), id); err != nil {
		httpx.ErrorCode(w, r, httpx.CodeConflict, "connection may be in use by an API")
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "change_db_connection", "connection", id.String(), map[string]any{"deleted": true}))
	ok(w, r, map[string]bool{"ok": true})
}

// testConnection tests an unsaved connection by forwarding the provided params
// to the appropriate connector's /test endpoint (PRD §11.6 AC).
func (s *service) testConnection(w http.ResponseWriter, r *http.Request) {
	var in connInput
	if !decode(w, r, &in) {
		return
	}
	res, err := s.callConnectorTest(r.Context(), in.DatabaseType, map[string]any{
		"host": in.Host, "port": in.Port, "database_name": in.DatabaseName,
		"service_name": in.ServiceName, "schema_name": in.SchemaName,
		"username": in.Username, "password": in.Password, "ssl_mode": in.SSLMode,
		"connection_timeout_ms": in.ConnectionTimeoutMS,
	})
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeConnectorError, "connector unavailable for "+in.DatabaseType)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "test_connection", "connection", "", map[string]any{"host": in.Host, "type": in.DatabaseType}))
	ok(w, r, res)
}

// testSavedConnection tests a stored connection (decrypting its secret) and
// records the resulting health status.
func (s *service) testSavedConnection(w http.ResponseWriter, r *http.Request) {
	id, ok2 := idParam(w, r)
	if !ok2 {
		return
	}
	c, err := s.store.GetConnection(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	password := ""
	if c.SecretRef != nil {
		if b, err := s.secrets.Get(r.Context(), *c.SecretRef); err == nil {
			password = string(b)
		}
	}
	res, err := s.callConnectorTest(r.Context(), c.DatabaseType, map[string]any{
		"host": c.Host, "port": c.Port, "database_name": c.DatabaseName,
		"service_name": c.ServiceName, "schema_name": c.SchemaName,
		"username": c.Username, "password": password, "ssl_mode": c.SSLMode,
		"connection_timeout_ms": c.ConnectionTimeoutMS,
	})
	if err != nil {
		_ = s.store.SetConnectionHealth(r.Context(), id, "unreachable")
		httpx.ErrorCode(w, r, httpx.CodeConnectorError, "connector unavailable for "+c.DatabaseType)
		return
	}
	status := "unhealthy"
	if ok, _ := res["success"].(bool); ok {
		status = "healthy"
	}
	_ = s.store.SetConnectionHealth(r.Context(), id, status)
	s.audit.Write(r.Context(), r, s.actorEvent(r, "test_connection", "connection", id.String(), map[string]any{"status": status}))
	ok(w, r, res)
}

// callConnectorTest posts a /test request to the connector for the db type.
func (s *service) callConnectorTest(ctx context.Context, dbType string, payload map[string]any) (map[string]any, error) {
	base, found := s.connectors[dbType]
	if !found {
		return map[string]any{"success": false, "message": "no connector for " + dbType}, nil
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/test", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func connFromInput(in connInput) *models.DatabaseConnection {
	return &models.DatabaseConnection{
		Name: in.Name, DatabaseType: in.DatabaseType, Host: in.Host, Port: in.Port,
		DatabaseName: in.DatabaseName, ServiceName: in.ServiceName, SchemaName: in.SchemaName,
		Username: in.Username, SSLMode: defStr(in.SSLMode, "disable"),
		MinPoolSize: defInt(in.MinPoolSize, 2), MaxPoolSize: defInt(in.MaxPoolSize, 10),
		ConnectionTimeoutMS: defInt(in.ConnectionTimeoutMS, 5000), QueryTimeoutMS: defInt(in.QueryTimeoutMS, 30000),
		MaxConnLifetimeMS: defInt(in.MaxConnLifetimeMS, 3600000), MaxConnIdleMS: defInt(in.MaxConnIdleMS, 1800000),
		Environment: defStr(in.Environment, "dev"), Status: defStr(in.Status, "active"),
		Tags: in.Tags,
	}
}
