package metrics

import "testing"

func TestV2MetricsAreRegistered(t *testing.T) {
	m := New("test-service")
	m.SingleflightActive.Inc()
	m.SingleflightShared.Inc()
	m.MetadataSync.Inc()
	m.CircuitState.WithLabelValues("conn-1", "postgres").Set(2)
	m.CircuitOpen.WithLabelValues("conn-1", "postgres").Inc()
	m.CircuitHalfOpen.WithLabelValues("conn-1", "postgres").Inc()

	families, err := m.reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	names := map[string]bool{}
	for _, f := range families {
		names[f.GetName()] = true
	}
	for _, want := range []string{
		"ddag_singleflight_active",
		"ddag_singleflight_shared",
		"ddag_metadata_sync_total",
		"ddag_circuit_state",
		"ddag_circuit_open_total",
		"ddag_circuit_half_open_total",
	} {
		if !names[want] {
			t.Fatalf("missing metric %s", want)
		}
	}
}

func TestV3MetricsAreRegistered(t *testing.T) {
	m := New("test-service")
	m.ConnectorRequests.WithLabelValues("conn-1", "postgres").Inc()
	m.DBPoolActive.WithLabelValues("conn-1").Set(1)
	m.DBPoolIdle.WithLabelValues("conn-1").Set(2)
	m.DBPoolWaitCount.WithLabelValues("conn-1").Set(3)
	m.QueueDepth.WithLabelValues("api-1").Set(1)
	m.QueuedRequests.WithLabelValues("api-1").Inc()
	m.QueueTimeout.WithLabelValues("api-1").Inc()
	m.RejectedRequests.WithLabelValues("api-1").Inc()

	families, err := m.reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	names := map[string]bool{}
	for _, f := range families {
		names[f.GetName()] = true
	}
	for _, want := range []string{
		"ddag_connector_requests_total",
		"ddag_db_pool_active",
		"ddag_db_pool_idle",
		"ddag_db_pool_wait_count",
		"ddag_queue_depth",
		"ddag_queued_requests_total",
		"ddag_queue_timeout_total",
		"ddag_rejected_requests_total",
	} {
		if !names[want] {
			t.Fatalf("missing metric %s", want)
		}
	}
}
