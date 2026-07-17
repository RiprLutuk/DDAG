package adminsvc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCanTransitionAPIStatusOnlyAllowsDocumentedWorkflow(t *testing.T) {
	valid := [][2]string{
		{"DRAFT", "REVIEW"},
		{"REVIEW", "APPROVED"},
		{"APPROVED", "PUBLISHED"},
		{"PUBLISHED", "DEPRECATED"},
		{"PUBLISHED", "ARCHIVED"},
		{"PUBLISHED", "DISABLED"},
	}
	for _, transition := range valid {
		if err := canTransitionAPIStatus(transition[0], transition[1]); err != nil {
			t.Fatalf("expected %s -> %s to be allowed: %v", transition[0], transition[1], err)
		}
	}
	for _, transition := range [][2]string{
		{"DRAFT", "APPROVED"},
		{"ARCHIVED", "REVIEW"},
		{"DISABLED", "PUBLISHED"},
		{"PUBLISHED", "REVIEW"},
	} {
		if err := canTransitionAPIStatus(transition[0], transition[1]); err == nil {
			t.Fatalf("expected %s -> %s to be rejected", transition[0], transition[1])
		}
	}
}

func TestCanPublishAPIStatusRequiresApproved(t *testing.T) {
	if err := canPublishAPIStatus("DRAFT"); err == nil {
		t.Fatal("DRAFT should not be publishable")
	}
	if err := canPublishAPIStatus("REVIEW"); err == nil {
		t.Fatal("REVIEW should not be publishable")
	}
	if err := canPublishAPIStatus("APPROVED"); err != nil {
		t.Fatalf("APPROVED should be publishable: %v", err)
	}
}

func TestDiffJSONReportsAddedRemovedAndChangedFields(t *testing.T) {
	before := json.RawMessage(`{"name":"orders","path":"/orders","old":"x","nested":{"limit":100}}`)
	after := json.RawMessage(`{"name":"orders-v2","path":"/orders","new":"y","nested":{"limit":200}}`)
	diff, err := diffJSON(before, after)
	if err != nil {
		t.Fatalf("diffJSON: %v", err)
	}
	if len(diff.Changed) == 0 || len(diff.Added) == 0 || len(diff.Removed) == 0 {
		t.Fatalf("expected changed/added/removed diff, got %#v", diff)
	}
	if diff.Changed["name"].Before != "orders" || diff.Changed["name"].After != "orders-v2" {
		t.Fatalf("name change missing: %#v", diff.Changed["name"])
	}
}

func TestPromotionDryRunRejectsDuplicateRoutes(t *testing.T) {
	bundle := promotionBundle{
		APIs: []promotionAPI{{Name: "a", Method: "GET", Path: "/orders"}, {Name: "b", Method: "GET", Path: "/orders"}},
	}
	result := validatePromotionBundle(bundle)
	if result.Valid {
		t.Fatalf("duplicate method+path should be invalid: %#v", result)
	}
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "duplicate route") {
		t.Fatalf("duplicate route error missing: %s", joined)
	}
}
