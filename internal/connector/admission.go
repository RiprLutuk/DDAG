package connector

import (
	"context"
	"sync"
	"time"
)

// admissionController bounds work sent to one physical source database. Unlike a
// process-wide queue it is keyed by connection ID, so a saturated SQL Server
// source cannot consume the capacity assigned to another source.
type admissionController struct {
	mu           sync.Mutex
	queueSize    int
	queueTimeout time.Duration
	states       map[string]*admissionState
}

type admissionState struct {
	inflight int
	waiting  int
	notify   chan struct{}
}

func newAdmissionController(queueSize int, queueTimeout time.Duration) *admissionController {
	if queueSize < 0 {
		queueSize = 0
	}
	return &admissionController{
		queueSize: queueSize, queueTimeout: queueTimeout,
		states: make(map[string]*admissionState),
	}
}

func (a *admissionController) state(key string) *admissionState {
	s := a.states[key]
	if s == nil {
		s = &admissionState{notify: make(chan struct{})}
		a.states[key] = s
	}
	return s
}

// Acquire reserves one concurrent query slot. The limit is supplied on every
// call from the saved connection pool configuration, so config changes take
// effect for new requests without restarting the connector.
func (a *admissionController) Acquire(ctx context.Context, key string, limit int) (func(), bool) {
	if a == nil {
		return func() {}, true
	}
	if limit < 1 {
		limit = 1
	}
	var timer <-chan time.Time
	if a.queueTimeout > 0 {
		t := time.NewTimer(a.queueTimeout)
		defer t.Stop()
		timer = t.C
	}
	queued := false
	for {
		a.mu.Lock()
		s := a.state(key)
		if s.inflight < limit {
			s.inflight++
			if queued {
				s.waiting--
			}
			a.mu.Unlock()
			return a.release(key), true
		}
		if !queued {
			if a.queueSize == 0 || s.waiting >= a.queueSize {
				a.mu.Unlock()
				return nil, false
			}
			s.waiting++
			queued = true
		}
		notify := s.notify
		a.mu.Unlock()

		select {
		case <-ctx.Done():
			a.dequeue(key)
			return nil, false
		case <-timer:
			a.dequeue(key)
			return nil, false
		case <-notify:
		}
	}
}

func (a *admissionController) dequeue(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s := a.states[key]; s != nil && s.waiting > 0 {
		s.waiting--
	}
}

func (a *admissionController) release(key string) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			a.mu.Lock()
			defer a.mu.Unlock()
			s := a.states[key]
			if s == nil || s.inflight == 0 {
				return
			}
			s.inflight--
			close(s.notify)
			s.notify = make(chan struct{})
		})
	}
}

func (a *admissionController) Depth(key string) int {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if s := a.states[key]; s != nil {
		return s.inflight + s.waiting
	}
	return 0
}

type AdmissionSnapshot struct {
	Inflight int
	Waiting  int
}

func (a *admissionController) Snapshot(key string) AdmissionSnapshot {
	if a == nil {
		return AdmissionSnapshot{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if s := a.states[key]; s != nil {
		return AdmissionSnapshot{Inflight: s.inflight, Waiting: s.waiting}
	}
	return AdmissionSnapshot{}
}
