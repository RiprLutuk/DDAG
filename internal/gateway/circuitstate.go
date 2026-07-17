package gateway

import (
	"sync"
	"time"
)

type circuitStateCache struct {
	mu   sync.RWMutex
	ttl  time.Duration
	data map[string]cachedCircuitState
}

type cachedCircuitState struct {
	state     string
	expiresAt time.Time
}

func newCircuitStateCache(ttl time.Duration) *circuitStateCache {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &circuitStateCache{ttl: ttl, data: map[string]cachedCircuitState{}}
}

func (c *circuitStateCache) Set(connectionID, state string) {
	if c == nil || connectionID == "" || state == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if state == "closed" {
		delete(c.data, connectionID)
		return
	}
	c.data[connectionID] = cachedCircuitState{state: state, expiresAt: time.Now().Add(c.ttl)}
}

func (c *circuitStateCache) IsOpen(connectionID string) bool {
	if c == nil || connectionID == "" {
		return false
	}
	c.mu.RLock()
	entry, ok := c.data[connectionID]
	c.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.data, connectionID)
		c.mu.Unlock()
		return false
	}
	return entry.state == "open"
}
