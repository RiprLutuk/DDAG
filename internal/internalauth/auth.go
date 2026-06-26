// Package internalauth signs service-to-service HTTP requests inside the DDAG
// cluster. It is used for gateway->connector calls so connector pods reject
// unsigned lateral traffic.
package internalauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderTimestamp = "X-DDAG-Internal-Timestamp"
	HeaderSignature = "X-DDAG-Internal-Signature"
)

// SignHeaders attaches timestamp and HMAC signature headers to req.
func SignHeaders(req *http.Request, body []byte, secret string, now time.Time) {
	ts := strconv.FormatInt(now.UTC().Unix(), 10)
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderSignature, signature(req, body, secret, ts))
}

// VerifyHeaders verifies the request signature and timestamp skew.
func VerifyHeaders(req *http.Request, body []byte, secret string, now time.Time, maxSkew time.Duration) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("internal auth secret is required")
	}
	ts := req.Header.Get(HeaderTimestamp)
	got := req.Header.Get(HeaderSignature)
	if ts == "" || got == "" {
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
		hex.EncodeToString(sum[:]),
	}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}
