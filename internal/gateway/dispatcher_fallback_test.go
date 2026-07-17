package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDualTransportFallsBackToHTTPWhenGRPCTransportFails(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"row_count":1,"rows":[{"transport":"http"}]}`))
	}))
	defer httpServer.Close()

	client, err := NewDualTransportClient(
		map[string]string{"postgres": httpServer.URL},
		map[string]string{"postgres": "127.0.0.1:1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	got, apiErr := client.Query(context.Background(), "postgres", ConnectorRequest{ConnectionID: "conn-fallback"})
	if apiErr != nil {
		t.Fatalf("expected HTTP fallback, got %v", apiErr)
	}
	if string(got.Rows) != `[{"transport":"http"}]` {
		t.Fatalf("fallback rows = %s", got.Rows)
	}
}
