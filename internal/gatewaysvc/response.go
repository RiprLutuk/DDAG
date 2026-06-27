package gatewaysvc

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/ddag/ddag/internal/gateway"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

// payload is the cacheable response body: the data plus optional pagination.
type payload struct {
	Data         json.RawMessage   `json:"data"`
	Pagination   *httpx.Pagination `json:"pagination,omitempty"`
	CircuitState string            `json:"circuit_state,omitempty"`
}

// buildPayload shapes the connector rows into a single object (for single-row
// endpoints) or a list with pagination.
func buildPayload(resp *gateway.ConnectorResponse, isList bool, page, effLimit, offset int) payload {
	if !isList {
		if len(resp.Rows) == 0 {
			return payload{Data: json.RawMessage("null"), CircuitState: resp.CircuitState}
		}
		b, _ := json.Marshal(resp.Rows[0])
		return payload{Data: b, CircuitState: resp.CircuitState}
	}
	rows := resp.Rows
	if rows == nil {
		rows = []map[string]any{}
	}
	b, _ := json.Marshal(rows)
	return payload{
		Data:         b,
		CircuitState: resp.CircuitState,
		Pagination:   &httpx.Pagination{Page: page, Limit: effLimit, Offset: offset, Total: int64(len(rows)), HasNext: len(rows) == effLimit},
	}
}

type connectorPayload struct {
	Payload  payload
	SourceMS int64
}

func (s *service) writePayload(w http.ResponseWriter, r *http.Request, p payload, cached bool, start time.Time, sourceMS int64) {
	if cached {
		w.Header().Set("X-Cache", "HIT")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	meta := &httpx.Meta{Cached: cached, DurationMS: time.Since(start).Milliseconds(), CircuitState: p.CircuitState}
	if p.Pagination != nil {
		httpx.WriteJSON(w, http.StatusOK, httpx.ListEnvelope{
			Success: true, RequestID: httpx.RequestID(r.Context()),
			Data: p.Data, Pagination: p.Pagination, Meta: meta,
		})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.SuccessEnvelope{
		Success: true, RequestID: httpx.RequestID(r.Context()),
		Data: p.Data, Meta: meta,
	})
}

func (s *service) writeCached(w http.ResponseWriter, r *http.Request, b []byte, start time.Time) {
	s.writeCachedWithTTL(w, r, b, start, 0)
}

func (s *service) writeCachedWithTTL(w http.ResponseWriter, r *http.Request, b []byte, start time.Time, ttl time.Duration) {
	w.Header().Set("X-Cache", "HIT")
	if ttl > 0 {
		w.Header().Set("X-Cache-TTL", strconv.FormatInt(int64(ttl.Seconds()), 10))
	}
	var p payload
	if err := json.Unmarshal(b, &p); err != nil {
		// Corrupt cache entry: fall back to an empty success rather than failing.
		s.writePayload(w, r, payload{Data: json.RawMessage("null")}, true, start, 0)
		return
	}
	s.writePayload(w, r, p, true, start, 0)
}

// reqLog accumulates per-request log fields written after the response.
type reqLog struct {
	start       time.Time
	requestID   string
	method      string
	path        string
	ip          string
	apiID       uuid.UUID
	apiLabel    string
	clientID    *uuid.UUID
	clientLabel string
	status      int
	errCode     string
	cached      bool
	sourceMS    int
}

// writeRequestLog persists the data-plane request record asynchronously so it
// never adds latency to the response path.
func (s *service) writeRequestLog(rl *reqLog) {
	if rl.status == 0 {
		rl.status = http.StatusOK
	}
	rec := &models.APIRequestLog{
		RequestID:          rl.requestID,
		ClientID:           rl.clientID,
		APIDefinitionID:    &rl.apiID,
		ClientLabel:        rl.clientLabel,
		APILabel:           rl.apiLabel,
		Method:             rl.method,
		Path:               rl.path,
		StatusCode:         rl.status,
		ErrorCode:          rl.errCode,
		LatencyMS:          int(time.Since(rl.start).Milliseconds()),
		Cached:             rl.cached,
		SourceDBDurationMS: rl.sourceMS,
		IPAddress:          rl.ip,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.store.InsertRequestLog(ctx, rec); err != nil {
			s.log.Warn("request_log_failed", "error", err.Error())
		}
	}()
}
