// Package grpcauth authenticates internal gRPC calls with a replay-bounded HMAC.
package grpcauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"
)

const (
	HeaderTimestamp = "x-ddag-internal-timestamp"
	HeaderSignature = "x-ddag-internal-signature"
	HeaderRequestID = "x-request-id"
)

func Outgoing(ctx metadata.MD, requestID string, payload []byte, secret string, now time.Time) metadata.MD {
	ts := strconv.FormatInt(now.UTC().Unix(), 10)
	out := ctx.Copy()
	out.Set(HeaderTimestamp, ts)
	out.Set(HeaderSignature, signature(requestID, payload, secret, ts))
	out.Set(HeaderRequestID, requestID)
	return out
}

func Verify(md metadata.MD, requestID string, payload []byte, secret string, now time.Time, maxSkew time.Duration) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("internal auth secret is required")
	}
	ts := first(md.Get(HeaderTimestamp))
	got := first(md.Get(HeaderSignature))
	if ts == "" || got == "" {
		return errors.New("missing internal auth metadata")
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return errors.New("invalid internal auth timestamp")
	}
	signedAt := time.Unix(sec, 0).UTC()
	if signedAt.Before(now.Add(-maxSkew)) || signedAt.After(now.Add(maxSkew)) {
		return errors.New("internal auth timestamp outside allowed skew")
	}
	want := signature(requestID, payload, secret, ts)
	if !hmac.Equal([]byte(got), []byte(want)) {
		return errors.New("invalid internal auth signature")
	}
	return nil
}

func signature(requestID string, payload []byte, secret, ts string) string {
	sum := sha256.Sum256(payload)
	canonical := strings.Join([]string{"POST", "/ddag.connector.v1.ConnectorService/Query", requestID, ts, hex.EncodeToString(sum[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}
func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}
