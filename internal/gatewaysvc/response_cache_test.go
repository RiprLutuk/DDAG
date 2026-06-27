package gatewaysvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWriteCachedSetsCacheHeaders(t *testing.T) {
	s := &service{}
	p := payload{Data: json.RawMessage(`{"ok":true}`)}
	b, _ := json.Marshal(p)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	s.writeCachedWithTTL(rec, req, b, time.Now(), 45*time.Second)

	if got := rec.Header().Get("X-Cache"); got != "HIT" {
		t.Fatalf("X-Cache = %q, want HIT", got)
	}
	if got := rec.Header().Get("X-Cache-TTL"); got != "45" {
		t.Fatalf("X-Cache-TTL = %q, want 45", got)
	}
}

func TestWritePayloadSetsMissHeader(t *testing.T) {
	s := &service{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	s.writePayload(rec, req, payload{Data: json.RawMessage(`[]`)}, false, time.Now(), 0)

	if got := rec.Header().Get("X-Cache"); got != "MISS" {
		t.Fatalf("X-Cache = %q, want MISS", got)
	}
}
