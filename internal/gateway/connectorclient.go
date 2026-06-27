package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/internalauth"
)

// ConnectorRequest is the gateway→connector query request (PRD §16.2).
type ConnectorRequest struct {
	RequestID     string         `json:"request_id"`
	ConnectionID  string         `json:"connection_id"`
	QueryTemplate string         `json:"query_template"`
	Parameters    map[string]any `json:"parameters"`
	TimeoutMS     int            `json:"timeout_ms"`
	Limit         int            `json:"limit"`
	Offset        int            `json:"offset"`
}

// ConnectorResponse is the connector→gateway query response (PRD §16.2).
type ConnectorResponse struct {
	Success      bool             `json:"success"`
	DurationMS   int64            `json:"duration_ms"`
	RowCount     int              `json:"row_count"`
	CircuitState string           `json:"circuit_state,omitempty"`
	Rows         []map[string]any `json:"rows"`
	Error        *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ConnectorClient calls a connector service for a given database type.
type ConnectorClient struct {
	urls         map[string]string // db_type -> base url
	http         *http.Client
	internalAuth string
	circuits     *circuitStateCache
}

// NewConnectorClient builds a client from a db_type→URL map.
func NewConnectorClient(urls map[string]string, internalAuth ...string) *ConnectorClient {
	secret := ""
	if len(internalAuth) > 0 {
		secret = internalAuth[0]
	}
	return &ConnectorClient{
		urls:         urls,
		http:         &http.Client{Timeout: 60 * time.Second},
		internalAuth: secret,
		circuits:     newCircuitStateCache(30 * time.Second),
	}
}

// Query sends a query to the connector for dbType and maps transport/connector
// failures to stable DDAG error codes (never leaking raw DB errors).
func (c *ConnectorClient) Query(ctx context.Context, dbType string, req ConnectorRequest) (*ConnectorResponse, *httpx.APIError) {
	base, ok := c.urls[dbType]
	if !ok {
		return nil, httpx.NewError(httpx.CodeConnectorUnavailable, "no connector configured for "+dbType)
	}
	if c.circuits.IsOpen(req.ConnectionID) {
		return nil, httpx.NewError(httpx.CodeCircuitBreakerOpen, "Database connection temporarily unavailable (circuit open)")
	}
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/query", bytes.NewReader(body))
	if err != nil {
		return nil, httpx.NewError(httpx.CodeInternal, "failed to build connector request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(httpx.RequestIDHeader, req.RequestID)
	if c.internalAuth != "" {
		internalauth.SignHeaders(httpReq, body, c.internalAuth, time.Now())
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, httpx.NewError(httpx.CodeConnectorUnavailable, "connector unavailable")
	}
	defer resp.Body.Close()

	var cr ConnectorResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, httpx.NewError(httpx.CodeConnectorUnavailable, "invalid connector response")
	}
	c.circuits.Set(req.ConnectionID, cr.CircuitState)
	if resp.StatusCode == http.StatusRequestTimeout {
		return nil, httpx.NewError(httpx.CodeDBQueryTimeout, "query timed out")
	}
	if resp.StatusCode >= 400 || !cr.Success {
		code := httpx.CodeConnectorError
		msg := "connector error"
		if cr.Error != nil && cr.Error.Code != "" {
			code = cr.Error.Code
			msg = cr.Error.Message
		}
		return nil, httpx.NewError(code, msg)
	}
	return &cr, nil
}
