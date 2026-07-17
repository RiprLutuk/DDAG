//go:build integration

package policy

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisRateLimiterIntegration(t *testing.T) {
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

	limiter := NewRateLimiter(rdb)
	base := "integration:" + time.Now().Format("20060102150405.000000000")
	windows := []Window{{Suffix: "s", Limit: 2, Duration: time.Second}}

	for i := 0; i < 2; i++ {
		decision, err := limiter.Allow(ctx, base, windows)
		if err != nil {
			t.Fatalf("allow %d: %v", i+1, err)
		}
		if !decision.Allowed {
			t.Fatalf("request %d denied: %+v", i+1, decision)
		}
	}

	decision, err := limiter.Allow(ctx, base, windows)
	if err != nil {
		t.Fatalf("third allow: %v", err)
	}
	if decision.Allowed || decision.ExceededScope != "s" || decision.Limit != 2 {
		t.Fatalf("expected second-window limit exceeded, got %+v", decision)
	}
}
