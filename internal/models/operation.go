package models

import "strings"

// OperationType classifies an API's HTTP-method-defined semantic operation.
// It is derived from the HTTP method to prevent drift between the manual
// IsWrite boolean and the actual method contract.
type OperationType string

const (
	OperationRead    OperationType = "read"
	OperationCreate  OperationType = "create"
	OperationUpdate  OperationType = "update"
	OperationDelete  OperationType = "delete"
	OperationUnknown OperationType = "unknown"
)

// IsWriteOperation reports whether the operation mutates data.
func (o OperationType) IsWriteOperation() bool {
	switch o {
	case OperationCreate, OperationUpdate, OperationDelete:
		return true
	}
	return false
}

// IsCacheable reports whether replaying an operation is semantically safe.
func (o OperationType) IsCacheable() bool { return o == OperationRead }

// AllowsAutomaticRetry reports whether an operation may be retried safely.
func (o OperationType) AllowsAutomaticRetry() bool { return o.IsCacheable() }

// OperationFromMethod derives the operation type from the HTTP method.
// POST is classified as "command" (write) because it may create or invoke
// a stored procedure; callers that use POST for read-only search endpoints
// can preserve backward compatibility via the legacy IsWrite=false override.
func OperationFromMethod(method string) OperationType {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "GET", "QUERY":
		return OperationRead
	case "POST":
		return OperationCreate
	case "PUT", "PATCH":
		return OperationUpdate
	case "DELETE":
		return OperationDelete
	default:
		return OperationUnknown
	}
}
