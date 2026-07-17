package models

import "testing"

func TestOperationGovernance(t *testing.T) {
	for _, op := range []OperationType{OperationCreate, OperationUpdate, OperationDelete, OperationUnknown} {
		if op.IsCacheable() || op.AllowsAutomaticRetry() {
			t.Fatalf("%s must not be cacheable/retryable", op)
		}
	}
	if !OperationRead.IsCacheable() || !OperationRead.AllowsAutomaticRetry() {
		t.Fatal("read operations should be cacheable and retryable")
	}
}
