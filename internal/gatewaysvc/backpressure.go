package gatewaysvc

import (
	"context"
	"sync"
	"time"
)

type backpressureManager struct {
	mu       sync.Mutex
	queues   map[string]chan struct{}
	size     int
	timeout  time.Duration
	disabled bool
}

func newBackpressureManager(size int, timeout time.Duration) *backpressureManager {
	if size <= 0 {
		return &backpressureManager{disabled: true}
	}
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	return &backpressureManager{queues: map[string]chan struct{}{}, size: size, timeout: timeout}
}

func (m *backpressureManager) Acquire(ctx context.Context, key string) (func(), bool) {
	if m == nil || m.disabled {
		return func() {}, true
	}
	q := m.queue(key)
	timer := time.NewTimer(m.timeout)
	defer timer.Stop()
	select {
	case q <- struct{}{}:
		return func() { <-q }, true
	case <-timer.C:
		return nil, false
	case <-ctx.Done():
		return nil, false
	}
}

func (m *backpressureManager) Depth(key string) int {
	if m == nil || m.disabled {
		return 0
	}
	return len(m.queue(key))
}

func (m *backpressureManager) queue(key string) chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if q, ok := m.queues[key]; ok {
		return q
	}
	q := make(chan struct{}, m.size)
	m.queues[key] = q
	return q
}
