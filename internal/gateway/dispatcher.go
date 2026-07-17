package gateway

import (
	"context"

	connectorv1 "github.com/ddag/ddag/api/connector/v1"
	"github.com/ddag/ddag/internal/httpx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ConnectorDispatcher routes queries to connector backends. It is abstract
// to allow HTTP-to-gRPC transition (dual-transport) and offline mock tests.
type ConnectorDispatcher interface {
	Query(ctx context.Context, dbType string, req ConnectorRequest) (*ConnectorResponse, *httpx.APIError)
	Close()
}

// DualTransportClient wraps both HTTP and gRPC protocols, preferring gRPC
// if its target address is configured as a gRPC endpoint (detected by grpc:// schema).
type DualTransportClient struct {
	httpClient *ConnectorClient
	grpcClient *grpcConnectorClient
	grpcConns  []*grpc.ClientConn
}

func NewDualTransportClient(httpUrls map[string]string, grpcUrls map[string]string, internalAuth ...string) (*DualTransportClient, error) {
	c := &DualTransportClient{
		httpClient: NewConnectorClient(httpUrls, internalAuth...),
	}
	if len(grpcUrls) == 0 {
		return c, nil
	}

	clients := make(map[string]connectorv1.ConnectorServiceClient)
	for dbType, addr := range grpcUrls {
		if addr == "" {
			continue // empty config deliberately preserves the HTTP fallback
		}
		conn, err := grpc.Dial(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*64)),
		)
		if err != nil {
			c.Close()
			return nil, err
		}
		c.grpcConns = append(c.grpcConns, conn)
		clients[dbType] = connectorv1.NewConnectorServiceClient(conn)
	}
	c.grpcClient = newGRPCConnectorClient(clients, internalAuth...)
	return c, nil
}

func (c *DualTransportClient) Query(ctx context.Context, dbType string, req ConnectorRequest) (*ConnectorResponse, *httpx.APIError) {
	if c.grpcClient != nil {
		if _, ok := c.grpcClient.clients[dbType]; ok {
			response, apiErr := c.grpcClient.Query(ctx, dbType, req)
			// Only a transport-level gRPC failure falls back to HTTP. Connector
			// responses (including admission or circuit protection) must be
			// returned unchanged so HTTP cannot bypass source-DB safeguards.
			if apiErr == nil || apiErr.Code != httpx.CodeConnectorUnavailable {
				return response, apiErr
			}
		}
	}
	return c.httpClient.Query(ctx, dbType, req)
}

func (c *DualTransportClient) Close() {
	for _, conn := range c.grpcConns {
		_ = conn.Close()
	}
	c.grpcConns = nil
}
