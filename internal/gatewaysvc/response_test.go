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
		Rows:         []map[string]any{{"id": "site-1"}},
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
