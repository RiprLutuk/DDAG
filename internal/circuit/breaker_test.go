package circuit

import (
	"testing"
	"time"
)

func TestBreakerTripsOpenAndRecoversThroughHalfOpen(t *testing.T) {
	now := time.Unix(1000, 0)
	b := New(Settings{
		MaxRequests:      1,
		Timeout:          time.Minute,
		FailureThreshold: 2,
	})

	if !b.Allow(now) {
		t.Fatal("closed breaker should allow requests")
	}
	b.Report(false, now)
	b.Report(false, now.Add(time.Second))

	if got := b.State(); got != StateOpen {
		t.Fatalf("state = %s, want open", got)
	}
	if b.Allow(now.Add(10 * time.Second)) {
		t.Fatal("open breaker should reject before timeout")
	}
	if !b.Allow(now.Add(time.Minute + time.Second)) {
		t.Fatal("breaker should allow one half-open probe after timeout")
	}
	if got := b.State(); got != StateHalfOpen {
		t.Fatalf("state = %s, want half-open", got)
	}
	b.Report(true, now.Add(time.Minute+2*time.Second))
	if got := b.State(); got != StateClosed {
		t.Fatalf("state = %s, want closed", got)
	}
}

func TestBreakerHalfOpenFailureReopens(t *testing.T) {
	now := time.Unix(1000, 0)
	b := New(Settings{
		MaxRequests:      1,
		Timeout:          time.Minute,
		FailureThreshold: 1,
	})

	b.Report(false, now)
	if !b.Allow(now.Add(time.Minute + time.Second)) {
		t.Fatal("expected half-open probe")
	}
	b.Report(false, now.Add(time.Minute+2*time.Second))
	if got := b.State(); got != StateOpen {
		t.Fatalf("state = %s, want open", got)
	}
}
