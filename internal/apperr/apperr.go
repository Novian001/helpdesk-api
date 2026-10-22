// Package apperr defines the single error type used across the API. Handlers
// return *Error values; the Fiber error handler turns them into the JSON error
// envelope, so error wording lives in one place.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is an application error with an HTTP status, a stable machine code and
// an optional map of field-level details.
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]string
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the wrapped cause for errors.Is/errors.As.
func (e *Error) Unwrap() error { return e.cause }

// WithDetails attaches field level validation details.
func (e *Error) WithDetails(details map[string]string) *Error {
	e.Details = details
	return e
}

// WithCause attaches an internal cause. The cause is logged but never sent to
// the client.
func (e *Error) WithCause(err error) *Error {
	e.cause = err
	return e
}

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, "BAD_REQUEST", message)
}

func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, "UNAUTHORIZED", message)
}

func Forbidden(message string) *Error {
	return New(http.StatusForbidden, "FORBIDDEN", message)
}

func NotFound(code, message string) *Error {
	return New(http.StatusNotFound, code, message)
}

func Conflict(code, message string) *Error {
	return New(http.StatusConflict, code, message)
}

func Unprocessable(message string) *Error {
	return New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", message)
}

func Internal(cause error) *Error {
	return New(http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error").WithCause(cause)
}

// From converts any error into an *Error. Unknown errors become opaque 500s so
// internal details never leak to clients.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal(err)
}
