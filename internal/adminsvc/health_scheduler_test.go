package adminsvc

import (
	"errors"
	"testing"
)

func TestConnectionHealthStatus(t *testing.T) {
	tests := []struct {
		name string
		res  map[string]any
		err  error
		want string
	}{
		{name: "healthy connector response", res: map[string]any{"success": true}, want: "healthy"},
		{name: "database rejects test", res: map[string]any{"success": false}, want: "unhealthy"},
		{name: "connector unreachable", err: errors.New("dial tcp: connection refused"), want: "unreachable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := connectionHealthStatus(tt.res, tt.err); got != tt.want {
				t.Fatalf("connectionHealthStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
