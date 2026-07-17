package connector

import (
	"context"
	"testing"
	"time"
)

func TestAdmissionSnapshotTracksInflightAndWaiting(t *testing.T) {
	a := newAdmissionController(1, time.Second)
	release, ok := a.Acquire(context.Background(), "source-a", 1)
	if !ok {
		t.Fatal("first request was not admitted")
	}
	defer release()

	started := make(chan struct{})
	finish := make(chan struct{})
	go func() {
		close(started)
		r, admitted := a.Acquire(context.Background(), "source-a", 1)
		if admitted {
			r()
		}
		close(finish)
	}()
	<-started
	deadline := time.Now().Add(time.Second)
	for a.Snapshot("source-a").Waiting != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("snapshot = %#v, want one waiter", a.Snapshot("source-a"))
		}
		time.Sleep(time.Millisecond)
	}

	s := a.Snapshot("source-a")
	if s.Inflight != 1 || s.Waiting != 1 {
		t.Fatalf("snapshot = %#v", s)
	}
	release()
	<-finish
}
