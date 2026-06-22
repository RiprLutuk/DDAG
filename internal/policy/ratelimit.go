// Package policy implements the data-plane access checks: OAuth2 scope, IP
// whitelist, and Redis-backed rate limiting (PRD §11.8-§11.10). It is used
// in-process by the gateway and re-exported by the standalone policy-engine.
package policy

import (
	"context"
	_ "embed"
	"time"

	"github.com/redis/go-redis/v9"
)

// fixedWindowScript atomically increments a counter and sets its expiry on first
// use, returning the new count. Fixed-window counters across multiple windows
// (second/minute/hour/day) approximate a multi-rate limiter and stay consistent
// across gateway pods because the state lives in shared Redis (PRD §14.3).
var fixedWindowScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
if current == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
local ttl = redis.call('PTTL', KEYS[1])
return {current, ttl}
`)

// Window is a single rate-limit window definition.
type Window struct {
	Suffix   string        // key suffix, e.g. "m"
	Limit    int           // max requests in the window (0 = unlimited)
	Duration time.Duration // window length
}

// RateLimiter enforces fixed-window limits in Redis.
type RateLimiter struct {
	rdb *redis.Client
}

// NewRateLimiter builds a limiter over a redis client.
func NewRateLimiter(rdb *redis.Client) *RateLimiter { return &RateLimiter{rdb: rdb} }

// Decision is the outcome of a rate-limit check.
type Decision struct {
	Allowed       bool
	Limit         int
	Remaining     int
	ResetSeconds  int
	ExceededScope string // which window was exceeded (e.g. "minute")
}

// Allow checks every configured window for a key base. The first exceeded window
// denies the request. Remaining/reset reflect the most constraining window.
func (rl *RateLimiter) Allow(ctx context.Context, base string, windows []Window) (Decision, error) {
	d := Decision{Allowed: true, Remaining: -1}
	for _, w := range windows {
		if w.Limit <= 0 {
			continue
		}
		key := "ddag:rl:" + base + ":" + w.Suffix
		res, err := fixedWindowScript.Run(ctx, rl.rdb, []string{key}, w.Duration.Milliseconds()).Slice()
		if err != nil {
			return d, err
		}
		current := toInt(res[0])
		ttlMS := toInt(res[1])
		remaining := w.Limit - current
		if remaining < 0 {
			remaining = 0
		}
		reset := int((time.Duration(ttlMS) * time.Millisecond).Seconds())
		if d.Remaining < 0 || remaining < d.Remaining {
			d.Remaining = remaining
			d.Limit = w.Limit
			d.ResetSeconds = reset
		}
		if current > w.Limit {
			d.Allowed = false
			d.ExceededScope = w.Suffix
			d.Remaining = 0
			d.Limit = w.Limit
			d.ResetSeconds = reset
			return d, nil
		}
	}
	if d.Remaining < 0 {
		d.Remaining = 0
	}
	return d, nil
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

// WindowsFromRule converts per-second/minute/hour/day limits into Window slices.
func WindowsFromRule(perSec, perMin, perHour, perDay int) []Window {
	return []Window{
		{Suffix: "s", Limit: perSec, Duration: time.Second},
		{Suffix: "m", Limit: perMin, Duration: time.Minute},
		{Suffix: "h", Limit: perHour, Duration: time.Hour},
		{Suffix: "d", Limit: perDay, Duration: 24 * time.Hour},
	}
}
