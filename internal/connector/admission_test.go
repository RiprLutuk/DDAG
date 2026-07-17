package connector

import (
	"context"
	"testing"
	"time"
)

func TestAdmissionRejectsImmediatelyWhenConnectionCapacityIsFull(t *testing.T) {
	a := newAdmissionController(0, time.Second)
	release, ok := a.Acquire(context.Background(), "sqlserver-a", 1)
	if !ok {
		t.Fatal("first acquire rejected")
	}
	defer release()

	_, ok = a.Acquire(context.Background(), "sqlserver-a", 1)
	if ok {
		t.Fatal("acquire must reject when inflight capacity is full and queue is disabled")
	}
}

func TestAdmissionIsIsolatedPerConnection(t *testing.T) {
	a := newAdmissionController(0, time.Second)
	releaseA, ok := a.Acquire(context.Background(), "oracle-a", 1)
	if !ok {
		t.Fatal("first connection rejected")
	}
	defer releaseA()

	releaseB, ok := a.Acquire(context.Background(), "oracle-b", 1)
	if !ok {
		t.Fatal("different connection must not be blocked")
	}
	defer releaseB()
}

func TestAdmissionQueueTimesOutWithoutLeakingCapacity(t *testing.T) {
	a := newAdmissionController(1, 10*time.Millisecond)
	release, ok := a.Acquire(context.Background(), "postgres-a", 1)
	if !ok {
		t.Fatal("first acquire rejected")
	}
	defer release()

	_, ok = a.Acquire(context.Background(), "postgres-a", 1)
	if ok {
		t.Fatal("queued acquire must expire")
	}
	if got := a.Depth("postgres-a"); got != 1 {
		t.Fatalf("depth after timed out acquire = %d, want 1", got)
	}
}

func TestAdmissionUsesUpdatedLimitForNewRequests(t *testing.T) {
	a := newAdmissionController(0, time.Second)
	release, ok := a.Acquire(context.Background(), "mysql-a", 1)
	if !ok {
		t.Fatal("first acquire rejected")
	}
	defer release()

	releaseTwo, ok := a.Acquire(context.Background(), "mysql-a", 2)
	if !ok {
		t.Fatal("updated higher limit must permit second inflight request")
	}
	defer releaseTwo()
}
