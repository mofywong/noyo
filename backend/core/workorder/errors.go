package workorder

import "fmt"

const (
	CodeVersionConflict     = "VERSION_CONFLICT"
	CodeIdempotencyConflict = "IDEMPOTENCY_CONFLICT"
	CodeWorkOrderNotFound   = "WORK_ORDER_NOT_FOUND"
	CodeInvalidTransition   = "INVALID_TRANSITION"
	CodePermissionDenied    = "PERMISSION_DENIED"
	CodeValidationFailed    = "VALIDATION_FAILED"
)

// Error is the stable error contract exposed by CommandPort.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewError(code, message string) *Error { return &Error{Code: code, Message: message} }
