package internalauth

import (
	"bytes"
	"net/http"
	"testing"
	"time"
)

func TestSignAndVerifyHeaders(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	body := []byte(`{"query":"select 1"}`)
	req, err := http.NewRequest(http.MethodPost, "http://connector/query", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("X-Request-ID", "req-1")

	SignHeaders(req, body, "shared-secret", now)

	verifyReq, err := http.NewRequest(http.MethodPost, "http://connector/query", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	verifyReq.Header = req.Header.Clone()
	if err := VerifyHeaders(verifyReq, body, "shared-secret", now.Add(10*time.Second), time.Minute); err != nil {
		t.Fatalf("VerifyHeaders: %v", err)
	}
}

func TestVerifyHeadersRejectsTamperedBody(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	body := []byte(`{"query":"select 1"}`)
	req, err := http.NewRequest(http.MethodPost, "http://connector/query", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("X-Request-ID", "req-1")
	SignHeaders(req, body, "shared-secret", now)

	if err := VerifyHeaders(req, []byte(`{"query":"select 2"}`), "shared-secret", now, time.Minute); err == nil {
		t.Fatal("expected tampered body to be rejected")
	}
}
