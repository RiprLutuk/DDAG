package connector

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	connectorv1 "github.com/ddag/ddag/api/connector/v1"
	"github.com/ddag/ddag/internal/connectors"
	"github.com/ddag/ddag/internal/grpcauth"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type grpcServer struct {
	connectorv1.UnimplementedConnectorServiceServer
	svc *service
}

func newGRPCServer(svc *service) *grpcServer {
	return &grpcServer{svc: svc}
}

func (s *grpcServer) Query(ctx context.Context, req *connectorv1.QueryRequest) (*connectorv1.QueryResponse, error) {
	// Verify an HMAC over the complete Protobuf request before touching pools.
	if s.svc.internalAuthSecret != "" {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, errors.New("missing gRPC metadata")
		}
		payload, err := proto.Marshal(req)
		if err != nil {
			return nil, errors.New("failed to marshal gRPC request for authentication")
		}
		if err := grpcauth.Verify(md, req.RequestId, payload, s.svc.internalAuthSecret, time.Now(), time.Minute); err != nil {
			return nil, errors.New("invalid internal service signature")
		}
	}

	connID, err := uuid.Parse(req.ConnectionId)
	if err != nil {
		return s.errorResponse(httpx.CodeBadRequest, "invalid connection id", ""), nil
	}

	rc, err := s.svc.conns.Resolve(ctx, connID)
	if err != nil {
		if errors.Is(err, errConnNotFound) {
			return s.errorResponse(httpx.CodeNotFound, "connection not found", ""), nil
		}
		breaker := s.svc.breakerFor(connID.String())
		prevState := breaker.State()
		breaker.Report(false, time.Now())
		s.svc.observeCircuit(connID.String(), breaker, prevState)
		s.svc.metrics.ConnectorErr.WithLabelValues(connID.String(), s.svc.dbType).Inc()
		return s.errorResponse(httpx.CodeInternal, "failed to resolve connection secret", string(breaker.State())), nil
	}

	if rc.dbType != s.svc.dbType {
		return s.errorResponse(httpx.CodeBadRequest, "connection type mismatch", ""), nil
	}
	if rc.status != "active" {
		return s.errorResponse(httpx.CodeConnectorUnavailable, "connection is disabled", ""), nil
	}

	s.svc.metrics.ConnectorRequests.WithLabelValues(rc.idStr, s.svc.dbType).Inc()
	breaker := s.svc.breakerFor(rc.idStr)
	prevState := breaker.State()
	if !breaker.Allow(time.Now()) {
		s.svc.observeCircuit(rc.idStr, breaker, prevState)
		return s.errorResponse(httpx.CodeCircuitBreakerOpen, "Database connection temporarily unavailable (circuit open)", string(breaker.State())), nil
	}
	s.svc.observeCircuit(rc.idStr, breaker, prevState)

	// Enforce DB Target Admission/Backpressure bounds.
	release, admitted := s.svc.admission.Acquire(ctx, rc.idStr, rc.cfg.MaxPool)
	s.svc.observeAdmission(rc.idStr)
	if !admitted {
		s.svc.metrics.AdmissionRejected.WithLabelValues(rc.idStr, s.svc.dbType).Inc()
		s.svc.metrics.ConnectorErr.WithLabelValues(rc.idStr, s.svc.dbType).Inc()
		return s.errorResponse(httpx.CodeBackpressureLimit, "Database is busy. Please retry later.", string(breaker.State())), nil
	}
	defer func() { release(); s.svc.observeAdmission(rc.idStr) }()

	c, err := s.svc.registry.Acquire(ctx, rc.cfg, rc.version)
	if err != nil {
		prevState := breaker.State()
		breaker.Report(false, time.Now())
		s.svc.observeCircuit(rc.idStr, breaker, prevState)
		s.svc.metrics.ConnectorErr.WithLabelValues(rc.idStr, s.svc.dbType).Inc()
		code, _ := classifyPoolErr(err)
		return s.errorResponse(code, safePoolMessage(code), string(breaker.State())), nil
	}

	var params map[string]any
	if len(req.ParametersJson) > 0 {
		if err := json.Unmarshal(req.ParametersJson, &params); err != nil {
			return s.errorResponse(httpx.CodeBadRequest, "invalid parameters json", string(breaker.State())), nil
		}
	}

	res, err := c.Query(ctx, connectors.QueryRequest{
		RequestID: req.RequestId, QueryTemplate: req.QueryTemplate,
		Parameters: params, TimeoutMS: int(req.TimeoutMs), Limit: int(req.Limit), Offset: int(req.Offset),
	})
	if err != nil {
		prevState := breaker.State()
		breaker.Report(false, time.Now())
		s.svc.observeCircuit(rc.idStr, breaker, prevState)
		s.svc.metrics.ConnectorErr.WithLabelValues(rc.idStr, s.svc.dbType).Inc()
		code, _ := classifyQueryErr(err)
		return s.errorResponse(code, "query failed", string(breaker.State())), nil
	}

	prevState = breaker.State()
	breaker.Report(true, time.Now())
	s.svc.observeCircuit(rc.idStr, breaker, prevState)
	s.svc.metrics.QueryDuration.WithLabelValues(rc.idStr, s.svc.dbType).Observe(float64(res.DurationMS) / 1000.0)

	rowsJSON, err := json.Marshal(res.Rows)
	if err != nil {
		return s.errorResponse(httpx.CodeInternal, "failed to encode query rows", string(breaker.State())), nil
	}
	return &connectorv1.QueryResponse{
		Success:      true,
		DurationMs:   res.DurationMS,
		RowCount:     int32(res.RowCount),
		CircuitState: string(breaker.State()),
		RowsJson:     rowsJSON,
	}, nil
}

func (s *grpcServer) errorResponse(code, msg, state string) *connectorv1.QueryResponse {
	return &connectorv1.QueryResponse{
		Success:      false,
		CircuitState: state,
		Error: &connectorv1.ErrorDetail{
			Code:    code,
			Message: msg,
		},
	}
}
