// Package gatewaysvc implements the api-gateway data plane: dynamic routing,
// JWT verification via JWKS, in-process policy (scope/IP/rate limit), response
// caching, connector dispatch, and request logging.
package gatewaysvc

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ddag/ddag/internal/auth"
)

// jwksCache fetches and caches the auth-service JWKS so the gateway can verify
// access tokens locally without a network hop per request (PRD §14.3).
type jwksCache struct {
	url  string
	http *http.Client
	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey
}

func newJWKS(url string) *jwksCache {
	return &jwksCache{
		url:  url,
		http: &http.Client{Timeout: 5 * time.Second},
		keys: map[string]*rsa.PublicKey{},
	}
}

func (j *jwksCache) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.url, nil)
	if err != nil {
		return err
	}
	resp, err := j.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks fetch status %d", resp.StatusCode)
	}
	var set auth.JWKS
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return err
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, jwk := range set.Keys {
		if pub, err := auth.JWKToPublic(jwk); err == nil {
			keys[jwk.Kid] = pub
		}
	}
	if len(keys) == 0 {
		return fmt.Errorf("jwks contained no usable keys")
	}
	j.mu.Lock()
	j.keys = keys
	j.mu.Unlock()
	return nil
}

// keyfunc implements auth.Keyfunc.
func (j *jwksCache) keyfunc(kid string) (*rsa.PublicKey, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	k, ok := j.keys[kid]
	return k, ok
}

func (j *jwksCache) startRefresh(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := j.refresh(ctx); err != nil {
				// keep serving with the last good key set
				continue
			}
		}
	}
}
