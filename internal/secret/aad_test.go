package secret

import (
	"testing"

	"github.com/google/uuid"
)

func TestSecretAADBindsIdentity(t *testing.T) {
	id := uuid.New()
	got := string(secretAAD(id, "connector", 2))
	want := id.String() + "|connector|2"
	if got != want {
		t.Fatalf("AAD = %q, want %q", got, want)
	}
	if string(secretAAD(id, "other", 2)) == got {
		t.Fatal("purpose must be bound into AAD")
	}
}

func TestEnvelopeV2RoundTripAndTamperFails(t *testing.T) {
	s := testStore(t)
	id := uuid.New()
	ct, nonce, wrapped, dekNonce, err := s.sealV2(id, "connector", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.openV2(id, "connector", aadKeyVersion, ct, nonce, wrapped, dekNonce)
	if err != nil || string(got) != "secret" {
		t.Fatalf("round trip: %q, %v", got, err)
	}
	if _, err := s.openV2(id, "other", aadKeyVersion, ct, nonce, wrapped, dekNonce); err == nil {
		t.Fatal("tampered purpose must fail authentication")
	}
}

func TestLegacyEnvelopeStillOpens(t *testing.T) {
	s := testStore(t)
	ct, nonce, wrapped, dekNonce, err := s.seal([]byte("legacy"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.open(ct, nonce, wrapped, dekNonce)
	if err != nil || string(got) != "legacy" {
		t.Fatalf("legacy round trip: %q, %v", got, err)
	}
}

func testStore(t *testing.T) *EnvelopeStore {
	t.Helper()
	s, err := NewEnvelopeStore(nil, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

var _ = uuid.Nil
