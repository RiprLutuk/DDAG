package gateway

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	connectorv1 "github.com/ddag/ddag/api/connector/v1"
	"github.com/ddag/ddag/internal/grpcauth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

type grpcTestConnector struct {
	connectorv1.UnimplementedConnectorServiceServer
}

func (grpcTestConnector) Query(ctx context.Context, req *connectorv1.QueryRequest) (*connectorv1.QueryResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("missing metadata")
	}
	payload, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	if err := grpcauth.Verify(md, req.RequestId, payload, "test-secret-hmac", time.Now(), time.Minute); err != nil {
		return nil, err
	}
	return &connectorv1.QueryResponse{Success: true, RowCount: 1, RowsJson: []byte(`[{"id":1}]`), CircuitState: "closed"}, nil
}

func TestGRPCConnectorQueryPreservesRawRows(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	connectorv1.RegisterConnectorServiceServer(server, grpcTestConnector{})
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()

	conn, err := grpc.NewClient("passthrough:///test", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := newGRPCConnectorClient(
		map[string]connectorv1.ConnectorServiceClient{"postgres": connectorv1.NewConnectorServiceClient(conn)},
		"test-secret-hmac",
	)
	got, apiErr := client.Query(context.Background(), "postgres", ConnectorRequest{
		RequestID:    "req-1",
		ConnectionID: "conn-1",
		Parameters:   map[string]any{"id": 1},
	})
	if apiErr != nil {
		t.Fatalf("Query error: %v", apiErr)
	}
	if string(got.Rows) != `[{"id":1}]` {
		t.Fatalf("rows = %s", got.Rows)
	}
	if got.RowCount != 1 || got.CircuitState != "closed" {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestGRPCConnectorQueryMissingTargetIsUnavailable(t *testing.T) {
	client := newGRPCConnectorClient(nil)
	_, apiErr := client.Query(context.Background(), "oracle", ConnectorRequest{})
	if apiErr == nil {
		t.Fatal("expected unavailable error")
	}
}
