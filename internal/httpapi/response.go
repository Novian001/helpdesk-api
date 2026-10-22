// Package httpapi contains the HTTP layer: routing, middleware and handlers.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/novian001/helpdesk-api/internal/apperr"
)

// errorBody is the JSON error envelope returned by every failing request.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// fail writes the error envelope for an application error.
func fail(c *fiber.Ctx, err error) error {
	appErr := apperr.From(err)
	body := errorBody{Error: errorDetail{
		Code:    appErr.Code,
		Message: appErr.Message,
		Details: appErr.Details,
	}}
	return c.Status(appErr.Status).JSON(body)
}

// ok writes a successful JSON response.
func ok(c *fiber.Ctx, status int, payload any) error {
	return c.Status(status).JSON(payload)
}

// notFound handles unmatched routes with the same envelope.
func notFound(c *fiber.Ctx) error {
	return fail(c, apperr.NotFound("ROUTE_NOT_FOUND", "route not found"))
}

// methodNotAllowed handles a known path with the wrong method.
func methodNotAllowed(c *fiber.Ctx) error {
	return fail(c, apperr.New(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed"))
}

// logger returns the structured logger used by the HTTP layer.
func logger(c *fiber.Ctx) *slog.Logger {
	if l, ok := c.Locals("logger").(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
