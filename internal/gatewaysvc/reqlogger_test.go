package gatewaysvc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/models"
)

type fakeLogStore struct {
	mu       sync.Mutex
	batches  int
	rows     int
	maxBatch int
}

func (f *fakeLogStore) InsertRequestLogBatch(ctx context.Context, rs []*models.APIRequestLog) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batches++
	f.rows += len(rs)
	if len(rs) > f.maxBatch {
		f.maxBatch = len(rs)
	}
	return nil
}

func (f *fakeLogStore) snapshot() (batches, rows, maxBatch int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.batches, f.rows, f.maxBatch
}

func testLogger() *logging.Logger { return logging.New("test", "error") }

func TestRequestLogger_BatchesBySize(t *testing.T) {
	st := &fakeLogStore{}
	// Big flush interval so batching is driven by size, not the ticker.
	l := newRequestLogger(st, testLogger(), nil, 1024, 10, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	l.start(ctx)

	for i := 0; i < 25; i++ {
		l.enqueue(&models.APIRequestLog{RequestID: "r"})
	}
	cancel()
	l.stop() // drains + flushes the remainder

	batches, rows, maxBatch := st.snapshot()
	if rows != 25 {
		t.Fatalf("rows=%d, want 25", rows)
	}
	if maxBatch > 10 {
		t.Fatalf("maxBatch=%d, want <=10 (batchSize)", maxBatch)
	}
	if batches < 3 {
		t.Fatalf("batches=%d, want >=3 (two full + remainder)", batches)
	}
}

func TestRequestLogger_FlushesOnInterval(t *testing.T) {
	st := &fakeLogStore{}
	l := newRequestLogger(st, testLogger(), nil, 1024, 1000, 20*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	l.start(ctx)
	defer func() { cancel(); l.stop() }()

	l.enqueue(&models.APIRequestLog{RequestID: "r"})
	// Wait past a couple of ticker intervals for the time-based flush.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, rows, _ := st.snapshot(); rows == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("record was not flushed on the interval")
}

func TestRequestLogger_DropsWhenFull(t *testing.T) {
	// Buffer of 1 and no flusher started: the channel fills and overflow drops.
	l := newRequestLogger(&fakeLogStore{}, testLogger(), nil, 1, 10, time.Hour)
	for i := 0; i < 50; i++ {
		l.enqueue(&models.APIRequestLog{RequestID: "r"})
	}
	if got := l.dropped.Load(); got == 0 {
		t.Fatal("expected some dropped records when buffer is full")
	}
}
