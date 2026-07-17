package gateway

import (
	"context"
	"encoding/json"
	"time"

	connectorv1 "github.com/ddag/ddag/api/connector/v1"
	"github.com/ddag/ddag/internal/grpcauth"
	"github.com/ddag/ddag/internal/httpx"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type grpcConnectorClient struct {
	clients            map[string]connectorv1.ConnectorServiceClient
	circuits           *circuitStateCache
	internalAuthSecret string
}

func newGRPCConnectorClient(clients map[string]connectorv1.ConnectorServiceClient, internalAuth ...string) *grpcConnectorClient {
	secret := ""
	if len(internalAuth) > 0 {
		secret = internalAuth[0]
	}
	return &grpcConnectorClient{
		clients: clients, circuits: newCircuitStateCache(30 * time.Second), internalAuthSecret: secret,
	}
}

func (c *grpcConnectorClient) Query(ctx context.Context, dbType string, req ConnectorRequest) (*ConnectorResponse, *httpx.APIError) {
	client, ok := c.clients[dbType]
	if !ok {
		return nil, httpx.NewError(httpx.CodeConnectorUnavailable, "no gRPC connector configured for "+dbType)
	}
	if c.circuits.IsOpen(req.ConnectionID) {
		return nil, httpx.NewError(httpx.CodeCircuitBreakerOpen, "Database connection temporarily unavailable (circuit open)")
	}

	paramsBytes, err := json.Marshal(req.Parameters)
	if err != nil {
		return nil, httpx.NewError(httpx.CodeInternal, "failed to marshal query parameters")
	}

	timeout := time.Duration(req.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	grpcReq := &connectorv1.QueryRequest{
		RequestId: req.RequestID, ConnectionId: req.ConnectionID, QueryTemplate: req.QueryTemplate,
		ParametersJson: paramsBytes, TimeoutMs: int32(req.TimeoutMS), Limit: int32(req.Limit), Offset: int32(req.Offset),
	}
	if c.internalAuthSecret != "" {
		payload, err := proto.Marshal(grpcReq)
		if err != nil {
			return nil, httpx.NewError(httpx.CodeInternal, "failed to marshal gRPC request")
		}
		ctx = metadata.NewOutgoingContext(ctx, grpcauth.Outgoing(metadata.MD{}, req.RequestID, payload, c.internalAuthSecret, time.Now()))
	}
	res, err := client.Query(ctx, grpcReq)
	if err != nil {
		return nil, httpx.NewError(httpx.CodeConnectorUnavailable, "gRPC call failed: "+err.Error())
	}

	c.circuits.Set(req.ConnectionID, res.CircuitState)

	if !res.Success {
		code := httpx.CodeConnectorError
		msg := "connector error"
		if res.Error != nil && res.Error.Code != "" {
			code = res.Error.Code
			msg = res.Error.Message
		}
		return nil, httpx.NewError(code, msg)
	}

	return &ConnectorResponse{
		Success:      res.Success,
		DurationMS:   res.DurationMs,
		RowCount:     int(res.RowCount),
		CircuitState: res.CircuitState,
		Rows:         res.RowsJson,
	}, nil
}
