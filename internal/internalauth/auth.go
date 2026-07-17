// Package internalauth signs service-to-service HTTP requests inside the DDAG
// cluster. It is used for gateway->connector calls so connector pods reject
// unsigned lateral traffic.
package internalauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	HeaderTimestamp = "X-DDAG-Internal-Timestamp"
	HeaderSignature = "X-DDAG-Internal-Signature"
	HeaderNonce     = "X-DDAG-Internal-Nonce"
)

// ReplayStore atomically consumes a nonce for the supplied TTL. false means it
// has already been consumed. Implementations must be shared across replicas.
type ReplayStore interface {
	UseNonce(ctx context.Context, nonce string, ttl time.Duration) (bool, error)
}

type redisSetNXer interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
}

// RedisReplayStore is the connector-boundary replay store. SetNX maps to Redis
// SET key value NX EX/ PX, making nonce consumption atomic across replicas.
type RedisReplayStore struct {
	client redisSetNXer
	prefix string
}

func NewRedisReplayStore(client redisSetNXer) *RedisReplayStore {
	return &RedisReplayStore{client: client, prefix: "ddag:internal-auth:nonce:"}
}

func (s *RedisReplayStore) UseNonce(ctx context.Context, nonce string, ttl time.Duration) (bool, error) {
	if s == nil || s.client == nil {
		return false, errors.New("replay store is required")
	}
	if ttl <= 0 {
		return false, errors.New("replay nonce ttl must be positive")
	}
	return s.client.SetNX(ctx, s.prefix+nonce, "1", ttl).Result()
}

// SignHeaders attaches timestamp and HMAC signature headers to req.
func SignHeaders(req *http.Request, body []byte, secret string, now time.Time) {
	ts := strconv.FormatInt(now.UTC().Unix(), 10)
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		panic(fmt.Sprintf("generate internal auth nonce: %v", err))
	}
	req.Header.Set(HeaderNonce, hex.EncodeToString(nonceBytes))
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderSignature, signature(req, body, secret, ts))
}

// VerifyHeaders verifies the request signature and timestamp skew.
func VerifyHeaders(req *http.Request, body []byte, secret string, now time.Time, maxSkew time.Duration) error {
	return verifySignature(req, body, secret, now, maxSkew)
}

// VerifyHeadersWithReplayStore verifies authenticity, then atomically consumes
// the signed nonce. Replay-store failure is fail-closed.
func VerifyHeadersWithReplayStore(ctx context.Context, req *http.Request, body []byte, secret string, now time.Time, maxSkew time.Duration, store ReplayStore) error {
	if err := verifySignature(req, body, secret, now, maxSkew); err != nil {
		return err
	}
	if store == nil {
		return errors.New("internal auth replay store is required")
	}
	ok, err := store.UseNonce(ctx, req.Header.Get(HeaderNonce), maxSkew*2)
	if err != nil {
		return fmt.Errorf("consume internal auth nonce: %w", err)
	}
	if !ok {
		return errors.New("internal auth nonce replayed")
	}
	return nil
}

func verifySignature(req *http.Request, body []byte, secret string, now time.Time, maxSkew time.Duration) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("internal auth secret is required")
	}
	ts := req.Header.Get(HeaderTimestamp)
	got := req.Header.Get(HeaderSignature)
	if ts == "" || got == "" || req.Header.Get(HeaderNonce) == "" {
		return errors.New("missing internal auth headers")
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return errors.New("invalid internal auth timestamp")
	}
	signedAt := time.Unix(sec, 0).UTC()
	if maxSkew > 0 {
		if signedAt.Before(now.Add(-maxSkew)) || signedAt.After(now.Add(maxSkew)) {
			return errors.New("internal auth timestamp outside allowed skew")
		}
	}
	want := signature(req, body, secret, ts)
	if !hmac.Equal([]byte(got), []byte(want)) {
		return errors.New("invalid internal auth signature")
	}
	return nil
}

func signature(req *http.Request, body []byte, secret, ts string) string {
	sum := sha256.Sum256(body)
	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	canonical := strings.Join([]string{
		req.Method,
		path,
		req.Header.Get("X-Request-ID"),
		ts,
		req.Header.Get(HeaderNonce),
		hex.EncodeToString(sum[:]),
	}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}
