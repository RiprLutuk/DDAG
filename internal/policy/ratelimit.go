// Package policy implements the data-plane access checks: OAuth2 scope, IP
// whitelist, and Redis-backed rate limiting (PRD §11.8-§11.10). It is used
// in-process by the gateway and re-exported by the standalone policy-engine.
package policy

import (
	"context"
	_ "embed"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// multiWindowScript evaluates every rate-limit window for a key in a single
// atomic round-trip. For each window i it INCRs KEYS[i], sets the expiry on
// first use (ARGV pairs are [limit, duration_ms] per window), and appends
// {current, ttl} to the result. It stops at the first window whose count
// exceeds its limit — matching the ordered short-circuit of the caller so a
// rejected request never consumes the larger windows' quota. Running all
// windows in one script (instead of one round-trip each) cuts Redis latency on
// the hot path and makes the windows mutually consistent under concurrency.
// State lives in shared Redis so limits hold across gateway pods (PRD §14.3).
var multiWindowScript = redis.NewScript(`
local out = {}
for i = 1, #KEYS do
  local limit = tonumber(ARGV[(i-1)*2 + 1])
  local dur   = tonumber(ARGV[(i-1)*2 + 2])
  local current = redis.call('INCR', KEYS[i])
  if current == 1 then
    redis.call('PEXPIRE', KEYS[i], dur)
  end
  local ttl = redis.call('PTTL', KEYS[i])
  out[#out+1] = current
  out[#out+1] = ttl
  if current > limit then
    return out
  end
end
return out
`)

// Window is a single rate-limit window definition.
type Window struct {
	Suffix   string        // key suffix, e.g. "m"
	Limit    int           // max requests in the window (0 = unlimited)
	Duration time.Duration // window length
}

// RateLimitFailureDecision maps a Redis/limiter error to the configured fail
// mode. "open" prioritizes availability; "closed" denies the request.
func RateLimitFailureDecision(mode string) (Decision, bool) {
	if strings.EqualFold(strings.TrimSpace(mode), "closed") {
		return Decision{Allowed: false, ExceededScope: "fail_closed"}, true
	}
	return Decision{Allowed: true}, false
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

	keys := make([]string, 0, len(windows))
	argv := make([]any, 0, len(windows)*2)
	active := make([]Window, 0, len(windows))
	for _, w := range windows {
		if w.Limit <= 0 {
			continue
		}
		keys = append(keys, "ddag:rl:"+base+":"+w.Suffix)
		argv = append(argv, w.Limit, w.Duration.Milliseconds())
		active = append(active, w)
	}
	if len(keys) == 0 {
		d.Remaining = 0
		return d, nil
	}

	res, err := multiWindowScript.Run(ctx, rl.rdb, keys, argv...).Slice()
	if err != nil {
		return d, err
	}

	// res holds {current, ttl} per window the script evaluated, in order; it
	// stops after the first exceeded window, so len(res)/2 may be < len(active).
	for i, w := range active {
		if i*2+1 >= len(res) {
			break
		}
		current := toInt(res[i*2])
		ttlMS := toInt(res[i*2+1])
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
