package gatewaysvc

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ddag/ddag/internal/gateway"
)

func TestPayloadWritesCircuitStateInMeta(t *testing.T) {
	p := buildPayload(&gateway.ConnectorResponse{
		Success:      true,
		DurationMS:   3,
		CircuitState: "closed",
		RowCount:     1,
		Rows:         json.RawMessage(`[{"id":"site-1"}]`),
	}, false, 1, 1, 0)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	s := &service{}
	s.writePayload(rec, req, p, false, time.Now(), 3)

	var got struct {
		Meta struct {
			CircuitState string `json:"circuit_state"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Meta.CircuitState != "closed" {
		t.Fatalf("circuit_state = %q", got.Meta.CircuitState)
	}
}

func TestBuildPayloadListPassesRowsThrough(t *testing.T) {
	raw := json.RawMessage(`[{"id":"a"},{"id":"b"}]`)
	p := buildPayload(&gateway.ConnectorResponse{RowCount: 2, Rows: raw}, true, 1, 2, 0)
	if string(p.Data) != string(raw) {
		t.Fatalf("data = %s, want passthrough %s", p.Data, raw)
	}
	if p.Pagination == nil || p.Pagination.Total != 2 || !p.Pagination.HasNext {
		t.Fatalf("pagination = %+v (want Total=2, HasNext=true from RowCount)", p.Pagination)
	}
}

func TestBuildPayloadEmptyListIsArray(t *testing.T) {
	p := buildPayload(&gateway.ConnectorResponse{RowCount: 0, Rows: nil}, true, 1, 10, 0)
	if string(p.Data) != "[]" {
		t.Fatalf("data = %s, want []", p.Data)
	}
	if p.Pagination.HasNext {
		t.Fatal("HasNext should be false for an empty page")
	}
}

func TestBuildPayloadSingleRowEmptyIsNull(t *testing.T) {
	p := buildPayload(&gateway.ConnectorResponse{RowCount: 0, Rows: json.RawMessage(`[]`)}, false, 1, 1, 0)
	if string(p.Data) != "null" {
		t.Fatalf("data = %s, want null", p.Data)
	}
}
