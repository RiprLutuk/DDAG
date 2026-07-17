package connectorpool

import (
	"context"
	"testing"

	"github.com/ddag/ddag/internal/connectors"
)

type fakeConnector struct{ stats connectors.PoolStats }

func (f fakeConnector) Query(context.Context, connectors.QueryRequest) (*connectors.QueryResult, error) {
	return nil, nil
}
func (f fakeConnector) HealthCheck(context.Context) error { return nil }
func (f fakeConnector) Stats() connectors.PoolStats       { return f.stats }
func (f fakeConnector) Close()                            {}

func TestRegistrySnapshotReturnsPoolStats(t *testing.T) {
	r := New(nil)
	r.entries["conn-1"] = &entry{version: 1, conn: fakeConnector{stats: connectors.PoolStats{
		InUse: 2, Idle: 3, Total: 5, Max: 10, WaitCount: 4, WaitDurationMS: 25, TimeoutCount: 1,
	}}}

	got := r.Snapshot()
	if len(got) != 1 {
		t.Fatalf("snapshot length = %d", len(got))
	}
	row := got[0]
	if row.ConnectionID != "conn-1" || row.InUse != 2 || row.Idle != 3 || row.Total != 5 ||
		row.Max != 10 || row.WaitCount != 4 || row.WaitDurationMS != 25 || row.TimeoutCount != 1 {
		t.Fatalf("snapshot row = %+v", row)
	}
}
