package gatewaysvc

import (
	"sort"

	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

type metadataSnapshot struct {
	clientsByClientID map[string]models.Client
	cacheRules        map[uuid.UUID]models.CacheRule
	rateRules         []models.RateLimitRule
	ipWhitelists      []models.IPWhitelist
}

func newMetadataSnapshot(clients []models.Client, cacheRules []models.CacheRule, rateRules []models.RateLimitRule, ips []models.IPWhitelist) *metadataSnapshot {
	s := &metadataSnapshot{
		clientsByClientID: map[string]models.Client{},
		cacheRules:        map[uuid.UUID]models.CacheRule{},
		rateRules:         append([]models.RateLimitRule(nil), rateRules...),
		ipWhitelists:      append([]models.IPWhitelist(nil), ips...),
	}
	for _, c := range clients {
		s.clientsByClientID[c.ClientID] = c
	}
	for _, cr := range cacheRules {
		s.cacheRules[cr.APIDefinitionID] = cr
	}
	return s
}

func (s *metadataSnapshot) clientByClientID(clientID string) (models.Client, bool) {
	if s == nil {
		return models.Client{}, false
	}
	c, ok := s.clientsByClientID[clientID]
	return c, ok
}

func (s *metadataSnapshot) clientHasAPIAccess(clientID, apiID uuid.UUID) bool {
	if s == nil {
		return false
	}
	for _, c := range s.clientsByClientID {
		if c.ID != clientID {
			continue
		}
		for _, id := range c.APIs {
			if id == apiID {
				return true
			}
		}
		return false
	}
	return false
}

func (s *metadataSnapshot) cacheRuleFor(apiID uuid.UUID) (models.CacheRule, bool) {
	if s == nil {
		return models.CacheRule{}, false
	}
	cr, ok := s.cacheRules[apiID]
	return cr, ok
}

func (s *metadataSnapshot) rateLimitRulesFor(clientID, apiID uuid.UUID) []models.RateLimitRule {
	if s == nil {
		return nil
	}
	out := make([]models.RateLimitRule, 0, len(s.rateRules))
	for _, r := range s.rateRules {
		if r.ClientID != nil && *r.ClientID != clientID {
			continue
		}
		if r.APIDefinitionID != nil && *r.APIDefinitionID != apiID {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return specificity(out[i]) > specificity(out[j])
	})
	return out
}

func (s *metadataSnapshot) ipWhitelistsFor(clientID, apiID uuid.UUID) []models.IPWhitelist {
	if s == nil {
		return nil
	}
	out := make([]models.IPWhitelist, 0, len(s.ipWhitelists))
	for _, ip := range s.ipWhitelists {
		if ip.Status != "active" {
			continue
		}
		if ip.ScopeLevel == "global" ||
			(ip.ClientID != nil && *ip.ClientID == clientID) ||
			(ip.APIDefinitionID != nil && *ip.APIDefinitionID == apiID) {
			out = append(out, ip)
		}
	}
	return out
}

func specificity(r models.RateLimitRule) int {
	n := 0
	if r.ClientID != nil {
		n++
	}
	if r.APIDefinitionID != nil {
		n++
	}
	return n
}
