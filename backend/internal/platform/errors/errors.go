// Package errors defines the typed error hierarchy for the job scheduler.
//
// Design: Each error carries a machine-readable Code (used in HTTP responses),
// a human-readable Message, and an optional underlying cause. Transport layers
// (handlers) translate these into appropriate HTTP status codes. Business logic
// never imports net/http — it simply returns domain errors.
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Code identifies the class of error. Used by handlers to map to HTTP status.
type Code string

const (
	CodeNotFound           Code = "NOT_FOUND"
	CodeAlreadyExists      Code = "ALREADY_EXISTS"
	CodeInvalidInput       Code = "INVALID_INPUT"
	CodeUnauthorized       Code = "UNAUTHORIZED"
	CodeForbidden          Code = "FORBIDDEN"
	CodeConflict           Code = "CONFLICT"
	CodeInternal           Code = "INTERNAL"
	CodeTimeout            Code = "TIMEOUT"
	CodeRateLimited        Code = "RATE_LIMITED"
	CodeUnprocessable      Code = "UNPROCESSABLE"
	CodeServiceUnavailable Code = "SERVICE_UNAVAILABLE"
)

// AppError is the standard application error type.
type AppError struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
	cause   error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap allows errors.Is / errors.As traversal.
func (e *AppError) Unwrap() error { return e.cause }

// HTTPStatus maps an AppError Code to an HTTP status code.
func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeAlreadyExists, CodeConflict:
		return http.StatusConflict
	case CodeInvalidInput, CodeUnprocessable:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeTimeout:
		return http.StatusGatewayTimeout
	case CodeServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// ─── Constructor Functions ─────────────────────────────────────────────────────

func New(code Code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func Wrap(code Code, message string, cause error) *AppError {
	return &AppError{Code: code, Message: message, cause: cause}
}

func WithDetails(code Code, message string, details any) *AppError {
	return &AppError{Code: code, Message: message, Details: details}
}

// ─── Convenience Constructors ──────────────────────────────────────────────────

func NotFound(resource string) *AppError {
	return New(CodeNotFound, fmt.Sprintf("%s not found", resource))
}

func AlreadyExists(resource string) *AppError {
	return New(CodeAlreadyExists, fmt.Sprintf("%s already exists", resource))
}

func InvalidInput(message string) *AppError {
	return New(CodeInvalidInput, message)
}

func InvalidInputWithDetails(message string, details any) *AppError {
	return WithDetails(CodeInvalidInput, message, details)
}

func Unauthorized(message string) *AppError {
	return New(CodeUnauthorized, message)
}

func Forbidden(message string) *AppError {
	return New(CodeForbidden, message)
}

func Conflict(message string) *AppError {
	return New(CodeConflict, message)
}

func Internal(message string, cause error) *AppError {
	return Wrap(CodeInternal, message, cause)
}

func Timeout(message string) *AppError {
	return New(CodeTimeout, message)
}

func RateLimited() *AppError {
	return New(CodeRateLimited, "rate limit exceeded")
}

// ─── Inspection Helpers ────────────────────────────────────────────────────────

// IsNotFound returns true if the error is a NOT_FOUND AppError.
func IsNotFound(err error) bool {
	var ae *AppError
	return errors.As(err, &ae) && ae.Code == CodeNotFound
}

// IsConflict returns true if the error is a CONFLICT / ALREADY_EXISTS AppError.
func IsConflict(err error) bool {
	var ae *AppError
	return errors.As(err, &ae) && (ae.Code == CodeConflict || ae.Code == CodeAlreadyExists)
}

// IsUnauthorized returns true if the error is an UNAUTHORIZED AppError.
func IsUnauthorized(err error) bool {
	var ae *AppError
	return errors.As(err, &ae) && ae.Code == CodeUnauthorized
}

// As unwraps to *AppError if possible.
func As(err error) (*AppError, bool) {
	var ae *AppError
	ok := errors.As(err, &ae)
	return ae, ok
}
