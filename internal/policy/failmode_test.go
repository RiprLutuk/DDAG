package policy

import "testing"

func TestRateLimitFailureDecisionDefaultsOpen(t *testing.T) {
	dec, limited := RateLimitFailureDecision("")
	if limited {
		t.Fatal("default fail mode should not limit")
	}
	if !dec.Allowed {
		t.Fatalf("default fail mode should allow, got %+v", dec)
	}
}

func TestRateLimitFailureDecisionClosed(t *testing.T) {
	dec, limited := RateLimitFailureDecision("closed")
	if !limited {
		t.Fatal("closed fail mode should limit")
	}
	if dec.Allowed {
		t.Fatalf("closed fail mode should deny, got %+v", dec)
	}
}
