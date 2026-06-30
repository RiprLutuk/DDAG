package connector

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

// fakeStore counts GetConnection calls and can block to exercise singleflight.
type fakeStore struct {
	calls atomic.Int64
	conn  *models.DatabaseConnection
	err   error
	gate  chan struct{} // if non-nil, GetConnection blocks until closed
}

func (f *fakeStore) GetConnection(ctx context.Context, id uuid.UUID) (*models.DatabaseConnection, error) {
	f.calls.Add(1)
	if f.gate != nil {
		<-f.gate
	}
	if f.err != nil {
		return nil, f.err
	}
	c := *f.conn
	return &c, nil
}

// fakeSecret counts Get calls; the other Store methods are unused here.
type fakeSecret struct{ calls atomic.Int64 }

func (f *fakeSecret) Get(ctx context.Context, id uuid.UUID) ([]byte, error) {
	f.calls.Add(1)
	return []byte("pw"), nil
}
func (f *fakeSecret) Put(context.Context, []byte, string) (uuid.UUID, error) { return uuid.Nil, nil }
func (f *fakeSecret) Update(context.Context, uuid.UUID, []byte) error        { return nil }
func (f *fakeSecret) Delete(context.Context, uuid.UUID) error                { return nil }

func testConn() *models.DatabaseConnection {
	ref := uuid.New()
	return &models.DatabaseConnection{
		ID: uuid.New(), Name: "demo", DatabaseType: "postgres", Host: "h", Port: 5432,
		DatabaseName: "db", Username: "u", SecretRef: &ref, Status: "active",
		MinPoolSize: 2, MaxPoolSize: 10, ConfigVersion: 3,
	}
}

func TestConnCache_CachesWithinTTL(t *testing.T) {
	st := &fakeStore{conn: testConn()}
	sec := &fakeSecret{}
	c := newConnCache(st, sec, time.Minute, nil, "postgres")

	for i := 0; i < 5; i++ {
		rc, err := c.Resolve(context.Background(), st.conn.ID)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if rc.cfg.Password != "pw" || rc.version != 3 || rc.dbType != "postgres" {
			t.Fatalf("unexpected resolution: %+v", rc.cfg)
		}
	}
	if got := st.calls.Load(); got != 1 {
		t.Fatalf("GetConnection called %d times, want 1 (cache hit)", got)
	}
	if got := sec.calls.Load(); got != 1 {
		t.Fatalf("secret.Get called %d times, want 1 (cache hit)", got)
	}
}

func TestConnCache_ReloadsAfterTTL(t *testing.T) {
	st := &fakeStore{conn: testConn()}
	c := newConnCache(st, &fakeSecret{}, time.Minute, nil, "postgres")

	now := time.Unix(1_700_000_000, 0)
	c.now = func() time.Time { return now }

	if _, err := c.Resolve(context.Background(), st.conn.ID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute) // past TTL
	if _, err := c.Resolve(context.Background(), st.conn.ID); err != nil {
		t.Fatal(err)
	}
	if got := st.calls.Load(); got != 2 {
		t.Fatalf("GetConnection called %d times, want 2 (reload after TTL)", got)
	}
}

func TestConnCache_NotFoundNotCached(t *testing.T) {
	st := &fakeStore{err: errors.New("no rows")}
	c := newConnCache(st, &fakeSecret{}, time.Minute, nil, "postgres")

	id := uuid.New()
	for i := 0; i < 3; i++ {
		_, err := c.Resolve(context.Background(), id)
		if !errors.Is(err, errConnNotFound) {
			t.Fatalf("want errConnNotFound, got %v", err)
		}
	}
	if got := st.calls.Load(); got != 3 {
		t.Fatalf("GetConnection called %d times, want 3 (errors not cached)", got)
	}
}

func TestConnCache_SingleflightCollapsesConcurrentMisses(t *testing.T) {
	st := &fakeStore{conn: testConn(), gate: make(chan struct{})}
	c := newConnCache(st, &fakeSecret{}, time.Minute, nil, "postgres")

	const n = 25
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, _ = c.Resolve(context.Background(), st.conn.ID)
		}()
	}
	// Let all goroutines reach the in-flight GetConnection before releasing.
	time.Sleep(50 * time.Millisecond)
	close(st.gate)
	wg.Wait()

	if got := st.calls.Load(); got != 1 {
		t.Fatalf("GetConnection called %d times, want 1 (singleflight collapse)", got)
	}
}
