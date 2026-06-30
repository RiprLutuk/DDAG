package connector

import (
	"errors"
	"net/http"
	"testing"

	"github.com/ddag/ddag/internal/httpx"
)

func TestClassifyQueryErr_PoolAcquireIsExhausted(t *testing.T) {
	// The acquire-timeout error also contains "context deadline exceeded"; the
	// "pool acquire" case must win so it maps to pool-exhausted, not query-timeout.
	err := errors.New("pool acquire: context deadline exceeded")
	code, status := classifyQueryErr(err)
	if code != httpx.CodeDBPoolExhausted || status != http.StatusServiceUnavailable {
		t.Fatalf("got (%s, %d), want (%s, 503)", code, status, httpx.CodeDBPoolExhausted)
	}
}

func TestClassifyQueryErr_QueryTimeoutStillTimeout(t *testing.T) {
	code, status := classifyQueryErr(errors.New("context deadline exceeded"))
	if code != httpx.CodeDBQueryTimeout || status != http.StatusRequestTimeout {
		t.Fatalf("got (%s, %d), want (%s, 408)", code, status, httpx.CodeDBQueryTimeout)
	}
}
