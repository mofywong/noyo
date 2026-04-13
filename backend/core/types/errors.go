package types

import "errors"

// Standard Errors
var (
	ErrNotFound       = errors.New("not found")
	ErrAlreadyExists  = errors.New("already exists")
	ErrInvalidParam   = errors.New("invalid parameter")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrNotImplemented = errors.New("not implemented")
	ErrTimeout        = errors.New("timeout")
	ErrBusy           = errors.New("busy")
	ErrInternal       = errors.New("internal error")
)

// ErrorCode represents a machine-readable error code
type ErrorCode int

const (
	CodeCommon   ErrorCode = 1000
	CodeParams   ErrorCode = 1001
	CodeNotFound ErrorCode = 1004
	CodeAuth     ErrorCode = 1003
	CodeInternal ErrorCode = 5000
)
