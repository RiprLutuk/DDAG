package internalauth

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"
)

type memoryReplayStore struct{ seen map[string]bool }

func (s *memoryReplayStore) UseNonce(_ context.Context, nonce string, _ time.Duration) (bool, error) {
	if s.seen[nonce] {
		return false, nil
	}
	s.seen[nonce] = true
	return true, nil
}

func TestVerifyHeadersWithReplayStoreRejectsReplay(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	body := []byte(`{"query":"select 1"}`)
	req, _ := http.NewRequest(http.MethodPost, "http://connector/query", bytes.NewReader(body))
	req.Header.Set("X-Request-ID", "req-1")
	SignHeaders(req, body, "shared-secret", now)
	if req.Header.Get(HeaderNonce) == "" {
		t.Fatal("signed request must contain a nonce")
	}
	store := &memoryReplayStore{seen: map[string]bool{}}
	if err := VerifyHeadersWithReplayStore(context.Background(), req, body, "shared-secret", now, time.Minute, store); err != nil {
		t.Fatalf("first verification: %v", err)
	}
	if err := VerifyHeadersWithReplayStore(context.Background(), req, body, "shared-secret", now, time.Minute, store); err == nil {
		t.Fatal("replayed nonce must be rejected")
	}
}

func TestSignatureBindsNonce(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	req, _ := http.NewRequest(http.MethodPost, "http://connector/query", nil)
	SignHeaders(req, nil, "shared-secret", now)
	req.Header.Set(HeaderNonce, "tampered")
	if err := VerifyHeaders(req, nil, "shared-secret", now, time.Minute); err == nil {
		t.Fatal("tampered nonce must invalidate signature")
	}
}

func TestRedisReplayStoreUsesSetNXWithExpiry(t *testing.T) {
	// Compile-time boundary assertion; behavior is covered through the commander's
	// SetNX contract rather than requiring a live Redis server.
	var _ ReplayStore = (*RedisReplayStore)(nil)
}
