package gatewaysvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ddag/ddag/internal/gateway"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/metrics"
	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

type mockBlockedDispatcher struct{}

func (mockBlockedDispatcher) Query(context.Context, string, gateway.ConnectorRequest) (*gateway.ConnectorResponse, *httpx.APIError) {
	return nil, httpx.NewError(httpx.CodeBackpressureLimit, "Database is busy. Please retry later.")
}
func (mockBlockedDispatcher) Close() {}

func TestResolvePayloadPropagatesConnectorAdmissionBackpressure(t *testing.T) {
	connectionID := uuid.New()
	svc := &service{
		connector:    mockBlockedDispatcher{},
		metrics:      metrics.New("api-gateway"),
		backpressure: newBackpressureManager(1, time.Second),
		flights:      newFlightGroup(),
	}
	api := models.APIDefinition{
		ID:                   uuid.New(),
		Path:                 "/test-endpoint",
		ConnectorType:        "postgres",
		DatabaseConnectionID: &connectionID,
	}
	req := httptest.NewRequest(http.MethodGet, "/test-endpoint", nil)

	_, apiErr := svc.resolvePayload(req, api, "SELECT 1", nil, 10, 0, 1, true, false, "", models.CacheRule{})
	if apiErr == nil {
		t.Fatal("expected connector admission backpressure error")
	}
	if apiErr.HTTPStatus() != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", apiErr.HTTPStatus())
	}
	if apiErr.Code != httpx.CodeBackpressureLimit {
		t.Fatalf("code = %q, want backpressure_limit_reached", apiErr.Code)
	}
}
