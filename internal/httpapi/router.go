package httpapi

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/novian001/helpdesk-api/internal/auth"
	"github.com/novian001/helpdesk-api/internal/models"
	"github.com/novian001/helpdesk-api/internal/service"
)

// Handlers bundles the services used by the HTTP handlers.
type Handlers struct {
	auth     *service.AuthService
	tickets  *service.TicketService
	comments *service.CommentService
}

// NewHandlers builds the handler set.
func NewHandlers(authSvc *service.AuthService, tickets *service.TicketService, comments *service.CommentService) *Handlers {
	return &Handlers{auth: authSvc, tickets: tickets, comments: comments}
}

// NewApp builds the Fiber application with all routes and middleware.
func NewApp(h *Handlers, manager *auth.Manager, log *slog.Logger) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "helpdesk-api",
		ErrorHandler: errorHandler(log),
	})

	app.Use(requestID())
	app.Use(requestLogger(log))
	app.Use(recoverPanic(log))

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	app.Get("/swagger", swaggerRedirect)
	app.Get("/swagger/*", serveSwagger)

	api := app.Group("/api/v1")

	// Public auth routes.
	api.Post("/auth/register", h.register)
	api.Post("/auth/login", h.login)
	api.Post("/auth/refresh", h.refresh)
	api.Post("/auth/logout", h.logout)

	// Authenticated routes.
	protected := api.Group("", authRequired(manager))
	protected.Get("/me", h.me)
	protected.Get("/users", requireRole(models.RoleAdmin), h.listUsers)
	protected.Patch("/users/:id/role", requireRole(models.RoleAdmin), h.updateUserRole)

	protected.Post("/tickets", h.createTicket)
	protected.Get("/tickets", h.listTickets)
	protected.Get("/tickets/:id", h.getTicket)
	protected.Patch("/tickets/:id", h.updateTicket)
	protected.Delete("/tickets/:id", requireRole(models.RoleAdmin), h.deleteTicket)
	protected.Post("/tickets/:id/assign", requireRole(models.RoleAdmin, models.RoleAgent), h.assignTicket)
	protected.Get("/tickets/:id/history", h.ticketHistory)
	protected.Post("/tickets/:id/comments", h.addComment)
	protected.Get("/tickets/:id/comments", h.listComments)
	protected.Delete("/comments/:id", h.deleteComment)

	app.Use(notFound)
	return app
}

// errorHandler renders every error, including errors returned by Fiber itself,
// with the same JSON envelope.
func errorHandler(log *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		if fe, ok := err.(*fiber.Error); ok {
			if fe.Code == fiber.StatusNotFound {
				return fail(c, notFoundError())
			}
			if fe.Code == fiber.StatusMethodNotAllowed {
				return fail(c, methodNotAllowedError())
			}
			return fail(c, fiberError(fe))
		}
		return fail(c, err)
	}
}
