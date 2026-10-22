package httpapi

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/novian001/helpdesk-api/internal/apperr"
	"github.com/novian001/helpdesk-api/internal/service"
)

// createTicketRequest is the body of POST /api/v1/tickets.
type createTicketRequest struct {
	Title       string `json:"title" example:"Database timeout on checkout"`
	Description string `json:"description" example:"Checkout request times out after 30s."`
	Category    string `json:"category" example:"database"`
	Priority    string `json:"priority" enums:"LOW,MEDIUM,HIGH,CRITICAL" example:"HIGH"`
}

// updateTicketRequest is the body of PATCH /api/v1/tickets/:id. Every field is
// optional; only the provided fields are changed.
type updateTicketRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
	Priority    *string `json:"priority" enums:"LOW,MEDIUM,HIGH,CRITICAL"`
	Status      *string `json:"status" enums:"OPEN,IN_PROGRESS,RESOLVED,CLOSED"`
}

// assignRequest is the body of POST /api/v1/tickets/:id/assign.
type assignRequest struct {
	UserID string `json:"user_id" example:"3f7d0d5c-1d24-4b5e-9f1a-9a3c9b6d1e11"`
}

// commentRequest is the body of POST /api/v1/tickets/:id/comments.
type commentRequest struct {
	Body string `json:"body" example:"We reproduced the timeout in staging."`
}

// createTicket handles POST /api/v1/tickets.
func (h *Handlers) createTicket(c *fiber.Ctx) error {
	actor, err := currentUser(c)
	if err != nil {
		return fail(c, err)
	}
	var req createTicketRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, apperr.BadRequest("request body is not valid JSON"))
	}
	ticket, err := h.tickets.Create(c.Context(), actor.ID, service.CreateTicketInput{
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
		Priority:    req.Priority,
	})
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusCreated, ticket)
}

// listTickets handles GET /api/v1/tickets.
func (h *Handlers) listTickets(c *fiber.Ctx) error {
	actor, err := currentUser(c)
	if err != nil {
		return fail(c, err)
	}
	in := service.ListInput{
		Status:   strings.ToUpper(strings.TrimSpace(c.Query("status"))),
		Priority: strings.ToUpper(strings.TrimSpace(c.Query("priority"))),
		Search:   strings.TrimSpace(c.Query("search")),
	}
	if in.Page, err = queryInt(c, "page", 1); err != nil {
		return fail(c, err)
	}
	if in.Limit, err = queryInt(c, "limit", 20); err != nil {
		return fail(c, err)
	}
	if in.Page < 1 {
		return fail(c, apperr.Unprocessable("page must be at least 1").WithDetails(map[string]string{"page": "must be >= 1"}))
	}
	if in.Limit < 1 || in.Limit > 100 {
		return fail(c, apperr.Unprocessable("limit must be between 1 and 100").WithDetails(map[string]string{"limit": "must be between 1 and 100"}))
	}
	result, err := h.tickets.List(c.Context(), actor, in)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, result)
}

// getTicket handles GET /api/v1/tickets/:id.
func (h *Handlers) getTicket(c *fiber.Ctx) error {
	actor, err := currentUser(c)
	if err != nil {
		return fail(c, err)
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return fail(c, err)
	}
	ticket, err := h.tickets.Get(c.Context(), actor, id)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, ticket)
}

// updateTicket handles PATCH /api/v1/tickets/:id.
func (h *Handlers) updateTicket(c *fiber.Ctx) error {
	actor, err := currentUser(c)
	if err != nil {
		return fail(c, err)
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return fail(c, err)
	}
	var req updateTicketRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, apperr.BadRequest("request body is not valid JSON"))
	}
	ticket, err := h.tickets.Update(c.Context(), actor, id, service.UpdateInput{
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
		Priority:    req.Priority,
		Status:      req.Status,
	})
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, ticket)
}

// deleteTicket handles DELETE /api/v1/tickets/:id.
func (h *Handlers) deleteTicket(c *fiber.Ctx) error {
	actor, err := currentUser(c)
	if err != nil {
		return fail(c, err)
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return fail(c, err)
	}
	if err := h.tickets.Delete(c.Context(), actor, id); err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, fiber.Map{"message": "ticket deleted"})
}

// assignTicket handles POST /api/v1/tickets/:id/assign.
func (h *Handlers) assignTicket(c *fiber.Ctx) error {
	actor, err := currentUser(c)
	if err != nil {
		return fail(c, err)
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return fail(c, err)
	}
	var req assignRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, apperr.BadRequest("request body is not valid JSON"))
	}
	assignee, err := uuid.Parse(strings.TrimSpace(req.UserID))
	if err != nil {
		return fail(c, apperr.Unprocessable("user_id is not a valid UUID").WithDetails(map[string]string{"user_id": "must be a UUID"}))
	}
	ticket, err := h.tickets.Assign(c.Context(), actor, id, assignee)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, ticket)
}

// ticketHistory handles GET /api/v1/tickets/:id/history.
func (h *Handlers) ticketHistory(c *fiber.Ctx) error {
	actor, err := currentUser(c)
	if err != nil {
		return fail(c, err)
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return fail(c, err)
	}
	entries, err := h.tickets.History(c.Context(), actor, id)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, fiber.Map{"data": entries})
}

// addComment handles POST /api/v1/tickets/:id/comments.
func (h *Handlers) addComment(c *fiber.Ctx) error {
	actor, err := currentUser(c)
	if err != nil {
		return fail(c, err)
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return fail(c, err)
	}
	var req commentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, apperr.BadRequest("request body is not valid JSON"))
	}
	comment, err := h.comments.Add(c.Context(), actor, id, req.Body)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusCreated, comment)
}

// listComments handles GET /api/v1/tickets/:id/comments.
func (h *Handlers) listComments(c *fiber.Ctx) error {
	actor, err := currentUser(c)
	if err != nil {
		return fail(c, err)
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return fail(c, err)
	}
	comments, err := h.comments.List(c.Context(), actor, id)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, fiber.Map{"data": comments})
}

// deleteComment handles DELETE /api/v1/comments/:id.
func (h *Handlers) deleteComment(c *fiber.Ctx) error {
	actor, err := currentUser(c)
	if err != nil {
		return fail(c, err)
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return fail(c, err)
	}
	if err := h.comments.Delete(c.Context(), actor, id); err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, fiber.Map{"message": "comment deleted"})
}

// queryInt reads an integer query parameter with a default.
func queryInt(c *fiber.Ctx, key string, fallback int) (int, error) {
	raw := c.Query(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, apperr.Unprocessable(key + " must be an integer").WithDetails(map[string]string{key: "must be an integer"})
	}
	return v, nil
}

// pathUUID parses a UUID path parameter.
func pathUUID(c *fiber.Ctx, key string) (uuid.UUID, error) {
	raw := c.Params(key)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperr.Unprocessable(key + " is not a valid UUID").WithDetails(map[string]string{key: "must be a UUID"})
	}
	return id, nil
}
