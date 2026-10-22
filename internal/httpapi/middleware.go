package httpapi

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/novian001/helpdesk-api/internal/apperr"
	"github.com/novian001/helpdesk-api/internal/auth"
	"github.com/novian001/helpdesk-api/internal/models"
)

const (
	// localsUserID is the context key of the authenticated user id.
	localsUserID = "user_id"
	// localsRole is the context key of the authenticated role.
	localsRole = "role"
	// localsRequestID is the context key of the request id.
	localsRequestID = "request_id"
)

// requestID attaches an id to every request and echoes it in the response.
func requestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Locals(localsRequestID, id)
		c.Set("X-Request-ID", id)
		return c.Next()
	}
}

// requestLogger logs one structured line per request.
func requestLogger(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()
		attrs := []any{
			"request_id", c.Locals(localsRequestID),
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", c.IP(),
		}
		if err != nil {
			log.Error("http request", append(attrs, "error", err.Error())...)
		} else {
			log.Info("http request", attrs...)
		}
		return err
	}
}

// recoverPanic turns a panic into a 500 response instead of killing the
// process. The stack trace is logged, never returned to the client.
func recoverPanic(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered", "request_id", c.Locals(localsRequestID), "panic", r)
				err = fail(c, apperr.Internal(errors.New("panic recovered")))
			}
		}()
		return c.Next()
	}
}

// authRequired rejects requests without a valid access token.
func authRequired(manager *auth.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			return fail(c, apperr.Unauthorized("authorization header is missing"))
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		claims, err := manager.Parse(token, auth.KindAccess)
		if err != nil {
			return fail(c, apperr.Unauthorized("invalid or expired access token"))
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return fail(c, apperr.Unauthorized("invalid access token"))
		}
		c.Locals(localsUserID, userID)
		c.Locals(localsRole, claims.Role)
		return c.Next()
	}
}

// requireRole rejects a request when the caller does not hold one of the
// allowed roles. Authorization is enforced in the backend, never by hiding
// endpoints in the frontend.
func requireRole(roles ...models.Role) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals(localsRole).(models.Role)
		if !ok {
			return fail(c, apperr.Unauthorized("authentication required"))
		}
		for _, allowed := range roles {
			if role == allowed {
				return c.Next()
			}
		}
		return fail(c, apperr.Forbidden("insufficient permissions"))
	}
}

// currentUser rebuilds the acting user from the request context.
func currentUser(c *fiber.Ctx) (models.User, error) {
	userID, ok := c.Locals(localsUserID).(uuid.UUID)
	if !ok {
		return models.User{}, apperr.Unauthorized("authentication required")
	}
	role, _ := c.Locals(localsRole).(models.Role)
	return models.User{ID: userID, Role: role}, nil
}
