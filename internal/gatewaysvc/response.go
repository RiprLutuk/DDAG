package gatewaysvc

import (
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
// endpoints) or a list with pagination. The connector's rows array is passed
// through as raw JSON — no per-row unmarshal/remarshal — and counts come from
// RowCount.
func buildPayload(resp *gateway.ConnectorResponse, isList bool, page, effLimit, offset int) payload {
	if !isList {
		first, ok := firstRow(resp.Rows)
		if !ok {
			return payload{Data: json.RawMessage("null"), CircuitState: resp.CircuitState}
		}
		return payload{Data: first, CircuitState: resp.CircuitState}
	}
	data := resp.Rows
	if len(data) == 0 || string(data) == "null" {
		data = json.RawMessage("[]")
	}
	return payload{
		Data:         data,
		CircuitState: resp.CircuitState,
		Pagination:   &httpx.Pagination{Page: page, Limit: effLimit, Offset: offset, Total: int64(resp.RowCount), HasNext: resp.RowCount == effLimit},
	}
}

// firstRow returns the first element of a JSON rows array without unmarshaling
// each row's fields. Single-row endpoints carry at most one row, so this stays
// cheap.
func firstRow(rows json.RawMessage) (json.RawMessage, bool) {
	if len(rows) == 0 {
		return nil, false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(rows, &arr); err != nil || len(arr) == 0 {
		return nil, false
	}
	return arr[0], true
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

func (s *service) writeCached(w http.ResponseWriter, r *http.Request, b []byte, start time.Time) bool {
	return s.writeCachedWithTTL(w, r, b, start, 0)
}

func (s *service) writeCachedWithTTL(w http.ResponseWriter, r *http.Request, b []byte, start time.Time, ttl time.Duration) bool {
	var p payload
	if err := json.Unmarshal(b, &p); err != nil {
		return false
	}
	w.Header().Set("X-Cache", "HIT")
	if ttl > 0 {
		w.Header().Set("X-Cache-TTL", strconv.FormatInt(int64(ttl.Seconds()), 10))
	}
	s.writePayload(w, r, p, true, start, 0)
	return true
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
	operation   string
}

// writeRequestLog hands the data-plane request record to the async batched
// logger so it never adds latency to the response path and never spawns a
// goroutine + INSERT per request.
func (s *service) writeRequestLog(rl *reqLog) {
	if rl.status == 0 {
		rl.status = http.StatusOK
	}
	s.reqLogger.enqueue(&models.APIRequestLog{
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
		Operation:          rl.operation,
		SourceDBDurationMS: rl.sourceMS,
		IPAddress:          rl.ip,
	})
}
