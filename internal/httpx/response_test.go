package httpx

import (
	"encoding/json"
	"testing"
)

func TestMetaIncludesCircuitState(t *testing.T) {
	b, err := json.Marshal(Meta{Cached: true, DurationMS: 12, CircuitState: "closed"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["circuit_state"] != "closed" {
		t.Fatalf("circuit_state = %#v", got["circuit_state"])
	}
}
