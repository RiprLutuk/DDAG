package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPWithTrustedProxiesHonorsForwardedFor(t *testing.T) {
	proxies, err := ParseTrustedProxies("10.0.0.0/8")
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.1.2.3:5678"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.1.2.3")

	if got := ClientIPWithTrustedProxies(r, proxies); got != "203.0.113.9" {
		t.Fatalf("client ip = %q", got)
	}
}

func TestClientIPWithTrustedProxiesIgnoresUntrustedForwardedFor(t *testing.T) {
	proxies, err := ParseTrustedProxies("10.0.0.0/8")
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "198.51.100.10:5678"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	r.Header.Set("X-Real-IP", "203.0.113.10")

	if got := ClientIPWithTrustedProxies(r, proxies); got != "198.51.100.10" {
		t.Fatalf("client ip = %q", got)
	}
}
