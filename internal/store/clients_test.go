package store

import (
	"strings"
	"testing"
)

func TestRefreshTokenConsumeSQLIsSingleAtomicUpdate(t *testing.T) {
	sql := strings.ToLower(consumeRefreshTokenSQL())

	for _, want := range []string{
		"update refresh_tokens",
		"revoked=false",
		"expires_at > now()",
		"returning",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("consume SQL %q should contain %q", sql, want)
		}
	}
	if strings.Contains(sql, "select ") {
		t.Fatalf("consume SQL %q should not use a separate select", sql)
	}
}
