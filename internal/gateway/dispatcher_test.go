package gateway

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	connectorv1 "github.com/ddag/ddag/api/connector/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

func TestDualTransportDispatchesToGrpcWhenAvailable(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	connectorv1.RegisterConnectorServiceServer(server, grpcTestConnector{})
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()

	// Setup dual transport client with grpc target
	client, err := NewDualTransportClient(
		map[string]string{"postgres": "http://localhost:8090"},
		map[string]string{"postgres": "bufconn"},
		"test-secret-hmac",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Inject the mock test bufconn dialer
	for _, conn := range client.grpcConns {
		_ = conn.Close() // Close default connection
	}
	client.grpcConns = nil

	conn, err := grpc.NewClient("passthrough:///test", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	client.grpcConns = append(client.grpcConns, conn)
	client.grpcClient.clients["postgres"] = connectorv1.NewConnectorServiceClient(conn)

	got, apiErr := client.Query(context.Background(), "postgres", ConnectorRequest{
		RequestID:    "req-1",
		ConnectionID: "conn-2",
	})
	if apiErr != nil {
		t.Fatalf("dual query failed: %v", apiErr)
	}
	if got.RowCount != 1 || string(got.Rows) != `[{"id":1}]` {
		t.Fatalf("unexpected dispatch result: %#v", got)
	}
}

func TestDualTransportFallsBackToHttpWhenGrpcTargetAbsent(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"row_count":1,"rows":[{"id":2}]}`))
	}))
	defer httpServer.Close()

	client, err := NewDualTransportClient(
		map[string]string{"mysql": httpServer.URL},
		map[string]string{}, // no grpc config for mysql
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	got, apiErr := client.Query(context.Background(), "mysql", ConnectorRequest{ConnectionID: "conn-3"})
	if apiErr != nil {
		t.Fatalf("HTTP fallback failed: %v", apiErr)
	}
	if string(got.Rows) != `[{"id":2}]` {
		t.Fatalf("HTTP fallback rows = %s", got.Rows)
	}
}
