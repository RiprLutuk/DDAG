package gateway

import (
	"testing"
	"time"
)

func TestCircuitStateCacheRejectsOpenConnection(t *testing.T) {
	cache := newCircuitStateCache(30 * time.Second)
	cache.Set("conn-1", "open")

	if !cache.IsOpen("conn-1") {
		t.Fatal("open circuit should be cached")
	}

	cache.Set("conn-1", "closed")
	if cache.IsOpen("conn-1") {
		t.Fatal("closed circuit should not reject")
	}
}
