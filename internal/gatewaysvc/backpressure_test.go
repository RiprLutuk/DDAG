package gatewaysvc

import (
	"context"
	"testing"
	"time"
)

func TestBackpressureQueueRejectsWhenTimeoutExpires(t *testing.T) {
	q := newBackpressureManager(1, 10*time.Millisecond)
	release, ok := q.Acquire(context.Background(), "api-a")
	if !ok {
		t.Fatal("first acquire rejected")
	}
	defer release()

	_, ok = q.Acquire(context.Background(), "api-a")
	if ok {
		t.Fatal("second acquire should be rejected after timeout")
	}
	if q.Depth("api-a") != 1 {
		t.Fatalf("depth = %d, want 1", q.Depth("api-a"))
	}
}

func TestBackpressureQueueIsPerKey(t *testing.T) {
	q := newBackpressureManager(1, time.Second)
	releaseA, ok := q.Acquire(context.Background(), "api-a")
	if !ok {
		t.Fatal("first acquire rejected")
	}
	defer releaseA()

	releaseB, ok := q.Acquire(context.Background(), "api-b")
	if !ok {
		t.Fatal("different key should acquire independently")
	}
	releaseB()
}
