package gatewaysvc

import (
	"testing"

	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

func TestMetadataSnapshotServesClientPolicyFromMemory(t *testing.T) {
	clientID := uuid.New()
	apiID := uuid.New()
	globalRuleID := uuid.New()
	specificRuleID := uuid.New()
	snap := newMetadataSnapshot(
		[]models.Client{{
			ID:       clientID,
			ClientID: "app-1",
			Status:   "active",
			APIs:     []uuid.UUID{apiID},
		}},
		[]models.CacheRule{{APIDefinitionID: apiID, Enabled: true, TTLSeconds: 30}},
		[]models.RateLimitRule{
			{ID: globalRuleID, RequestsPerMinute: 100},
			{ID: specificRuleID, ClientID: &clientID, APIDefinitionID: &apiID, RequestsPerMinute: 10},
		},
		[]models.IPWhitelist{
			{ClientID: &clientID, IPCIDR: "203.0.113.10", Status: "active"},
			{IPCIDR: "198.51.100.0/24", ScopeLevel: "global", Status: "active"},
			{IPCIDR: "192.0.2.1", Status: "inactive"},
		},
	)

	client, ok := snap.clientByClientID("app-1")
	if !ok || client.ID != clientID {
		t.Fatalf("client lookup failed: %+v ok=%v", client, ok)
	}
	if !snap.clientHasAPIAccess(clientID, apiID) {
		t.Fatal("expected API grant from snapshot")
	}
	if cr, ok := snap.cacheRuleFor(apiID); !ok || !cr.Enabled {
		t.Fatalf("cache rule = %+v ok=%v", cr, ok)
	}
	rules := snap.rateLimitRulesFor(clientID, apiID)
	if len(rules) != 2 || rules[0].ID != specificRuleID || rules[1].ID != globalRuleID {
		t.Fatalf("rate rule order = %+v", rules)
	}
	ips := snap.ipWhitelistsFor(clientID, apiID)
	if len(ips) != 2 {
		t.Fatalf("ip whitelist = %+v", ips)
	}
}
