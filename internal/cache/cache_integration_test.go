//go:build integration

package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestRedisCacheIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	addr, explicit := os.LookupEnv("DDAG_INTEGRATION_REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		if explicit {
			t.Fatalf("redis integration target unavailable at %s: %v", addr, err)
		}
		t.Skipf("redis integration target unavailable at %s: %v", addr, err)
	}

	c := NewWithClient(rdb)
	apiID := uuid.New()
	key := Key(apiID, "client-a", true, map[string]any{"q": "brim", "page": 1})
	defer c.PurgeEndpoint(context.Background(), apiID)

	if err := c.Set(ctx, key, []byte(`{"success":true}`), time.Minute); err != nil {
		t.Fatalf("set cache: %v", err)
	}
	b, ok, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("get cache: %v", err)
	}
	if !ok || string(b) != `{"success":true}` {
		t.Fatalf("cached value = %q present=%v", b, ok)
	}

	n, err := c.PurgeEndpoint(ctx, apiID)
	if err != nil {
		t.Fatalf("purge endpoint: %v", err)
	}
	if n != 1 {
		t.Fatalf("purged keys = %d, want 1", n)
	}
	_, ok, err = c.Get(ctx, key)
	if err != nil {
		t.Fatalf("get after purge: %v", err)
	}
	if ok {
		t.Fatal("expected cache key to be purged")
	}
}
