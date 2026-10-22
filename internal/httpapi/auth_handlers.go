package httpapi

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/novian001/helpdesk-api/internal/apperr"
	"github.com/novian001/helpdesk-api/internal/models"
)

// registerRequest is the body of POST /api/v1/auth/register.
type registerRequest struct {
	Name     string `json:"name" example:"Budi Santoso"`
	Email    string `json:"email" example:"budi@example.com"`
	Password string `json:"password" example:"supersecret123"`
}

// loginRequest is the body of POST /api/v1/auth/login.
type loginRequest struct {
	Email    string `json:"email" example:"budi@example.com"`
	Password string `json:"password" example:"supersecret123"`
}

// refreshRequest is the body of POST /api/v1/auth/refresh.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// register handles POST /api/v1/auth/register.
func (h *Handlers) register(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, apperr.BadRequest("request body is not valid JSON"))
	}
	pair, err := h.auth.Register(c.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusCreated, pair)
}

// login handles POST /api/v1/auth/login.
func (h *Handlers) login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, apperr.BadRequest("request body is not valid JSON"))
	}
	pair, err := h.auth.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, pair)
}

// refresh handles POST /api/v1/auth/refresh.
func (h *Handlers) refresh(c *fiber.Ctx) error {
	var req refreshRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, apperr.BadRequest("request body is not valid JSON"))
	}
	pair, err := h.auth.Refresh(c.Context(), req.RefreshToken)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, pair)
}

// logout handles POST /api/v1/auth/logout.
func (h *Handlers) logout(c *fiber.Ctx) error {
	var req refreshRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, apperr.BadRequest("request body is not valid JSON"))
	}
	if err := h.auth.Logout(c.Context(), req.RefreshToken); err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, fiber.Map{"message": "logged out"})
}

// me handles GET /api/v1/me.
func (h *Handlers) me(c *fiber.Ctx) error {
	actor, err := currentUser(c)
	if err != nil {
		return fail(c, err)
	}
	user, err := h.auth.Me(c.Context(), actor.ID)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, user)
}

// updateUserRoleRequest is the body of PATCH /api/v1/users/:id/role.
type updateUserRoleRequest struct {
	Role string `json:"role" enums:"ADMIN,AGENT,USER" example:"AGENT"`
}

// updateUserRole handles PATCH /api/v1/users/:id/role. Admin only.
func (h *Handlers) updateUserRole(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return fail(c, err)
	}
	var req updateUserRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, apperr.BadRequest("request body is not valid JSON"))
	}
	user, err := h.auth.SetRole(c.Context(), id, models.Role(strings.ToUpper(strings.TrimSpace(req.Role))))
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, user)
}

// listUsers handles GET /api/v1/users. Admin only.
func (h *Handlers) listUsers(c *fiber.Ctx) error {
	users, err := h.auth.ListUsers(c.Context())
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.StatusOK, fiber.Map{"data": users})
}
