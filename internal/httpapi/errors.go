package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/novian001/helpdesk-api/internal/apperr"
)

// notFoundError returns the error used for unmatched routes.
func notFoundError() *apperr.Error {
	return apperr.NotFound("ROUTE_NOT_FOUND", "route not found")
}

// methodNotAllowedError returns the error used for a wrong HTTP method.
func methodNotAllowedError() *apperr.Error {
	return apperr.New(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
}

// fiberError converts a Fiber error into the application error envelope.
func fiberError(fe *fiber.Error) *apperr.Error {
	switch fe.Code {
	case fiber.StatusBadRequest:
		return apperr.BadRequest(fe.Message)
	case fiber.StatusUnauthorized:
		return apperr.Unauthorized(fe.Message)
	case fiber.StatusForbidden:
		return apperr.Forbidden(fe.Message)
	case fiber.StatusNotFound:
		return notFoundError()
	case fiber.StatusRequestEntityTooLarge:
		return apperr.BadRequest("request body is too large")
	default:
		return apperr.New(fe.Code, "REQUEST_ERROR", fe.Message)
	}
}
