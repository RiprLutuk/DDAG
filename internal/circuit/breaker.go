// Package circuit provides a small per-connection circuit breaker used by
// connector services to fail fast when a source database is unhealthy.
package circuit

import (
	"sync"
	"time"
)

type State string

const (
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half_open"
)

type Settings struct {
	MaxRequests      int
	Interval         time.Duration
	Timeout          time.Duration
	FailureThreshold int
	FailureRatio     float64
}

type Breaker struct {
	mu sync.Mutex

	settings Settings
	state    State

	openedAt      time.Time
	intervalStart time.Time
	requests      int
	failures      int
	halfOpenInUse int
}

func New(settings Settings) *Breaker {
	if settings.MaxRequests <= 0 {
		settings.MaxRequests = 1
	}
	if settings.Timeout <= 0 {
		settings.Timeout = 30 * time.Second
	}
	if settings.Interval <= 0 {
		settings.Interval = time.Minute
	}
	if settings.FailureThreshold <= 0 {
		settings.FailureThreshold = 5
	}
	return &Breaker{settings: settings, state: StateClosed}
}

func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

func (b *Breaker) Allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rotateWindow(now)
	switch b.state {
	case StateClosed:
		b.requests++
		return true
	case StateOpen:
		if now.Sub(b.openedAt) >= b.settings.Timeout {
			b.state = StateHalfOpen
			b.halfOpenInUse = 0
		} else {
			return false
		}
	}
	if b.halfOpenInUse >= b.settings.MaxRequests {
		return false
	}
	b.halfOpenInUse++
	return true
}

func (b *Breaker) Report(success bool, now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rotateWindow(now)

	switch b.state {
	case StateHalfOpen:
		if b.halfOpenInUse > 0 {
			b.halfOpenInUse--
		}
		if success {
			b.close(now)
			return
		}
		b.open(now)
	case StateClosed:
		if !success {
			b.failures++
		}
		if b.shouldOpen() {
			b.open(now)
		}
	}
}

func (b *Breaker) rotateWindow(now time.Time) {
	if b.intervalStart.IsZero() {
		b.intervalStart = now
		return
	}
	if now.Sub(b.intervalStart) >= b.settings.Interval {
		b.intervalStart = now
		b.requests = 0
		b.failures = 0
	}
}

func (b *Breaker) shouldOpen() bool {
	if b.failures >= b.settings.FailureThreshold {
		return true
	}
	if b.settings.FailureRatio > 0 && b.requests > 0 {
		return float64(b.failures)/float64(b.requests) >= b.settings.FailureRatio
	}
	return false
}

func (b *Breaker) open(now time.Time) {
	b.state = StateOpen
	b.openedAt = now
	b.halfOpenInUse = 0
}

func (b *Breaker) close(now time.Time) {
	b.state = StateClosed
	b.intervalStart = now
	b.requests = 0
	b.failures = 0
	b.halfOpenInUse = 0
}
