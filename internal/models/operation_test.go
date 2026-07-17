package models

import "testing"

func TestOperationFromMethod(t *testing.T) {
	tests := []struct {
		method string
		want   OperationType
	}{
		{"GET", OperationRead},
		{"get", OperationRead},
		{"QUERY", OperationRead},
		{"POST", OperationCreate},
		{"PUT", OperationUpdate},
		{"PATCH", OperationUpdate},
		{"DELETE", OperationDelete},
		{"", OperationUnknown},
		{"BOGUS", OperationUnknown},
	}
	for _, tt := range tests {
		got := OperationFromMethod(tt.method)
		if got != tt.want {
			t.Errorf("OperationFromMethod(%q) = %q, want %q", tt.method, got, tt.want)
		}
	}
}

func TestIsWriteOperation(t *testing.T) {
	if OperationRead.IsWriteOperation() {
		t.Fatal("read should not be write")
	}
	if !OperationCreate.IsWriteOperation() {
		t.Fatal("create should be write")
	}
	if !OperationUpdate.IsWriteOperation() {
		t.Fatal("update should be write")
	}
	if !OperationDelete.IsWriteOperation() {
		t.Fatal("delete should be write")
	}
	if !OperationCreate.IsWriteOperation() {
		t.Fatal("create should be write")
	}
	if OperationUnknown.IsWriteOperation() {
		t.Fatal("unknown should not be write")
	}
}
