package httpx

import "net/http"

// Error codes from PRD §18. These string codes appear in the error envelope's
// "code" field and are stable contract values clients may switch on.
const (
	CodeBadRequest          = "BAD_REQUEST"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeForbidden           = "FORBIDDEN"
	CodeNotFound            = "API_NOT_FOUND"
	CodeQueryTimeout        = "QUERY_TIMEOUT"
	CodeConflict            = "CONFLICT"
	CodeRateLimited         = "RATE_LIMITED"
	CodeInternal            = "INTERNAL_ERROR"
	CodeConnectorError      = "CONNECTOR_ERROR"
	CodeSourceDBUnavailable = "SOURCE_DB_UNAVAILABLE"
	CodeValidation          = "VALIDATION_ERROR"
)

// statusByCode maps an error code to its HTTP status (PRD §18).
var statusByCode = map[string]int{
	CodeBadRequest:          http.StatusBadRequest,
	CodeValidation:          http.StatusBadRequest,
	CodeUnauthorized:        http.StatusUnauthorized,
	CodeForbidden:           http.StatusForbidden,
	CodeNotFound:            http.StatusNotFound,
	CodeQueryTimeout:        http.StatusRequestTimeout,
	CodeConflict:            http.StatusConflict,
	CodeRateLimited:         http.StatusTooManyRequests,
	CodeInternal:            http.StatusInternalServerError,
	CodeConnectorError:      http.StatusBadGateway,
	CodeSourceDBUnavailable: http.StatusServiceUnavailable,
}

// APIError is an error carrying a stable DDAG error code and a client-safe
// message. It never wraps raw DB/driver errors into the message.
type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

// NewError builds an APIError.
func NewError(code, message string) *APIError {
	return &APIError{Code: code, Message: message}
}

// HTTPStatus returns the HTTP status for the error's code.
func (e *APIError) HTTPStatus() int {
	if s, ok := statusByCode[e.Code]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// Error writes a standard error envelope. If err is an *APIError its code and
// message are used; otherwise a generic INTERNAL_ERROR is emitted so internal
// details never leak to clients (PRD §13.5).
func Error(w http.ResponseWriter, r *http.Request, err error) {
	apiErr, ok := err.(*APIError)
	if !ok {
		apiErr = NewError(CodeInternal, "An internal error occurred")
	}
	WriteJSON(w, apiErr.HTTPStatus(), ErrorEnvelope{
		Success:   false,
		RequestID: RequestID(r.Context()),
		Error:     ErrorBody{Code: apiErr.Code, Message: apiErr.Message},
	})
}

// ErrorCode writes an error envelope from a code + message directly.
func ErrorCode(w http.ResponseWriter, r *http.Request, code, message string) {
	Error(w, r, NewError(code, message))
}
