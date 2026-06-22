package policy

import (
	"net"
	"strings"
)

// HasScope reports whether the space-separated tokenScopes grant required. An
// empty required scope means the endpoint requires no specific scope.
func HasScope(tokenScopes, required string) bool {
	if strings.TrimSpace(required) == "" {
		return true
	}
	for _, s := range strings.Fields(tokenScopes) {
		if s == required {
			return true
		}
	}
	return false
}

// IPAllowed reports whether ip matches any entry in the whitelist. An empty
// whitelist means no restriction (allow all). Entries may be single IPs or CIDRs.
func IPAllowed(ip string, cidrs []string) bool {
	if len(cidrs) == 0 {
		return true
	}
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return false
	}
	for _, entry := range cidrs {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err == nil && network.Contains(parsed) {
				return true
			}
			continue
		}
		if single := net.ParseIP(entry); single != nil && single.Equal(parsed) {
			return true
		}
	}
	return false
}
