package gatewaysvc

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIP_LoopbackDefaultWhenNoTrustedProxies(t *testing.T) {
	// When TrustedProxies is not configured, the gateway must still trust
	// loopback (127.0.0.1) so that a same-host reverse proxy (Caddy)
	// forwarding X-Forwarded-For results in the real client IP, not 127.0.0.1.
	svc := &service{trustedProxies: nil}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.RemoteAddr = "127.0.0.1:12345"

	got := svc.clientIP(req)
	if got != "203.0.113.9" {
		t.Fatalf("clientIP with empty TrustedProxies and loopback peer = %q, want 203.0.113.9", got)
	}
}

func TestClientIP_NonLoopbackPeerNotTrustedByDefault(t *testing.T) {
	// A non-loopback direct peer must not have its X-Forwarded-For honored
	// when no explicit trusted proxy list is configured.
	svc := &service{trustedProxies: nil}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.RemoteAddr = "10.0.0.5:12345"

	got := svc.clientIP(req)
	if got == "203.0.113.9" {
		t.Fatalf("clientIP trusted a non-loopback peer without explicit config: got %q", got)
	}
}

func TestClientIP_ExplicitTrustedProxy(t *testing.T) {
	// When an explicit trusted proxy range is configured, it should be honored.
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	svc := &service{trustedProxies: []netip.Prefix{prefix}}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.42")
	req.RemoteAddr = "10.0.0.5:12345"

	got := svc.clientIP(req)
	if got != "198.51.100.42" {
		t.Fatalf("clientIP with explicit trusted proxy = %q, want 198.51.100.42", got)
	}
}
