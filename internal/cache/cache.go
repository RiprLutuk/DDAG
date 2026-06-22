// Package cache implements the Redis-backed response cache (PRD §11.11) with
// per-endpoint keys, TTLs, client-varying, and prefix purge.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/ddag/ddag/internal/config"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const keyPrefix = "ddag:cache:"

// Cache is the response cache.
type Cache struct {
	rdb *redis.Client
}

// New builds a Cache from Redis config.
func New(c config.RedisConfig) *Cache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.Addr,
		Password: c.Password,
		DB:       c.DB,
	})
	return &Cache{rdb: rdb}
}

// NewWithClient wraps an existing redis client (shared with the rate limiter).
func NewWithClient(rdb *redis.Client) *Cache { return &Cache{rdb: rdb} }

// Client exposes the underlying redis client.
func (c *Cache) Client() *redis.Client { return c.rdb }

// Ping checks Redis connectivity.
func (c *Cache) Ping(ctx context.Context) error { return c.rdb.Ping(ctx).Err() }

// Key builds a deterministic cache key for an endpoint invocation. The endpoint
// id namespaces the key (enabling per-endpoint purge); the sorted resolved
// parameters (and optionally the client id) form the variant hash.
func Key(apiID uuid.UUID, clientID string, varyByClient bool, params map[string]any) string {
	payload := map[string]any{"params": sortedCopy(params)}
	if varyByClient {
		payload["client"] = clientID
	}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return keyPrefix + apiID.String() + ":" + hex.EncodeToString(sum[:])
}

func sortedCopy(in map[string]any) map[string]any {
	// JSON marshalling already sorts map keys; we just defensively copy.
	out := make(map[string]any, len(in))
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out[k] = in[k]
	}
	return out
}

// Get returns the cached bytes for a key and whether it was present.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

// Set stores bytes under a key with a TTL.
func (c *Cache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

// PurgeEndpoint deletes all cache entries for an API (by id prefix). Returns the
// number of keys removed.
func (c *Cache) PurgeEndpoint(ctx context.Context, apiID uuid.UUID) (int, error) {
	return c.purgePrefix(ctx, keyPrefix+apiID.String()+":")
}

// PurgeAll deletes every DDAG cache entry.
func (c *Cache) PurgeAll(ctx context.Context) (int, error) {
	return c.purgePrefix(ctx, keyPrefix)
}

// PurgeKey deletes a single cache key.
func (c *Cache) PurgeKey(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

func (c *Cache) purgePrefix(ctx context.Context, prefix string) (int, error) {
	var (
		cursor uint64
		count  int
	)
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, prefix+"*", 256).Result()
		if err != nil {
			return count, err
		}
		if len(keys) > 0 {
			if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
				return count, err
			}
			count += len(keys)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return count, nil
}
