package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/novian001/helpdesk-api/internal/apperr"
	"github.com/novian001/helpdesk-api/internal/models"
	"github.com/novian001/helpdesk-api/internal/store"
)

// TicketService implements the ticket business rules.
type TicketService struct {
	tickets *store.TicketStore
	users   *store.UserStore
}

// NewTicketService wires the ticket service.
func NewTicketService(tickets *store.TicketStore, users *store.UserStore) *TicketService {
	return &TicketService{tickets: tickets, users: users}
}

// CreateTicketInput is the payload of a ticket creation request.
type CreateTicketInput struct {
	Title       string
	Description string
	Category    string
	Priority    string
}

// Create validates the input and stores a new ticket. All field errors are
// collected so the client sees every problem in one response.
func (s *TicketService) Create(ctx context.Context, actorID uuid.UUID, in CreateTicketInput) (models.Ticket, error) {
	details := map[string]string{}
	if err := validateTitle(in.Title); err != nil {
		details["title"] = fieldDetail(err, "title")
	}
	if err := validateDescription(in.Description); err != nil {
		details["description"] = fieldDetail(err, "description")
	}
	if strings.TrimSpace(in.Category) == "" {
		details["category"] = "must not be empty"
	}
	if !validPriority(in.Priority) {
		details["priority"] = "must be one of LOW, MEDIUM, HIGH, CRITICAL"
	}
	if len(details) > 0 {
		return models.Ticket{}, apperr.Unprocessable("the request contains invalid fields").WithDetails(details)
	}
	ticket, err := s.tickets.Create(ctx, models.Ticket{
		ID:          uuid.New(),
		Title:       strings.TrimSpace(in.Title),
		Description: strings.TrimSpace(in.Description),
		Category:    strings.TrimSpace(in.Category),
		Priority:    in.Priority,
		Status:      models.StatusOpen,
		CreatedBy:   actorID,
	}, actorID)
	if err != nil {
		return models.Ticket{}, apperr.Internal(err)
	}
	return ticket, nil
}

// ListInput holds the list filters and pagination.
type ListInput struct {
	Status   string
	Priority string
	Search   string
	Page     int
	Limit    int
}

// ListResult is a page of tickets with pagination metadata.
type ListResult struct {
	Tickets    []models.Ticket   `json:"data"`
	Pagination models.Pagination `json:"pagination"`
}

// List returns a filtered page of tickets. A USER only ever sees their own
// tickets; the restriction is applied in SQL, not by hiding fields.
func (s *TicketService) List(ctx context.Context, actor models.User, in ListInput) (ListResult, error) {
	if in.Status != "" && !validStatus(in.Status) {
		return ListResult{}, apperr.Unprocessable("status is not valid").WithDetails(map[string]string{
			"status": "must be one of OPEN, IN_PROGRESS, RESOLVED, CLOSED",
		})
	}
	if in.Priority != "" && !validPriority(in.Priority) {
		return ListResult{}, apperr.Unprocessable("priority is not valid").WithDetails(map[string]string{
			"priority": "must be one of LOW, MEDIUM, HIGH, CRITICAL",
		})
	}
	if in.Page < 1 {
		in.Page = 1
	}
	if in.Limit < 1 {
		in.Limit = 20
	}
	if in.Limit > 100 {
		in.Limit = 100
	}

	filter := store.TicketFilter{
		Status:   in.Status,
		Priority: in.Priority,
		Search:   strings.TrimSpace(in.Search),
	}
	if actor.Role == models.RoleUser {
		filter.CreatedBy = &actor.ID
	}

	result, err := s.tickets.List(ctx, filter, in.Limit, (in.Page-1)*in.Limit)
	if err != nil {
		return ListResult{}, apperr.Internal(err)
	}
	totalPages := 0
	if result.Total > 0 {
		totalPages = (result.Total + in.Limit - 1) / in.Limit
	}
	return ListResult{
		Tickets: result.Tickets,
		Pagination: models.Pagination{
			Page:       in.Page,
			Limit:      in.Limit,
			Total:      result.Total,
			TotalPages: totalPages,
		},
	}, nil
}

// Get returns a ticket if the actor is allowed to see it.
func (s *TicketService) Get(ctx context.Context, actor models.User, id uuid.UUID) (models.Ticket, error) {
	ticket, err := s.tickets.Get(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return models.Ticket{}, apperr.NotFound("TICKET_NOT_FOUND", "ticket not found")
	}
	if err != nil {
		return models.Ticket{}, apperr.Internal(err)
	}
	if !canView(actor, ticket) {
		return models.Ticket{}, apperr.Forbidden("you are not allowed to view this ticket")
	}
	return ticket, nil
}

// UpdateInput holds the mutable ticket fields. Nil fields are left unchanged.
type UpdateInput struct {
	Title       *string
	Description *string
	Category    *string
	Priority    *string
	Status      *string
}

// Update applies changes to a ticket with role based rules.
func (s *TicketService) Update(ctx context.Context, actor models.User, id uuid.UUID, in UpdateInput) (models.Ticket, error) {
	ticket, err := s.tickets.Get(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return models.Ticket{}, apperr.NotFound("TICKET_NOT_FOUND", "ticket not found")
	}
	if err != nil {
		return models.Ticket{}, apperr.Internal(err)
	}

	// A USER may only close or reopen their own ticket; everything else is
	// staff work.
	if actor.Role == models.RoleUser {
		if !canView(actor, ticket) {
			return models.Ticket{}, apperr.Forbidden("you are not allowed to update this ticket")
		}
		if in.Title != nil || in.Description != nil || in.Category != nil || in.Priority != nil {
			return models.Ticket{}, apperr.Forbidden("users cannot change title, description, category or priority")
		}
		if in.Status != nil && *in.Status != models.StatusClosed && *in.Status != models.StatusOpen {
			return models.Ticket{}, apperr.Forbidden("users can only close or reopen a ticket")
		}
	}

	if in.Title != nil {
		if err := validateTitle(*in.Title); err != nil {
			return models.Ticket{}, err
		}
	}
	if in.Description != nil {
		if err := validateDescription(*in.Description); err != nil {
			return models.Ticket{}, err
		}
	}
	if in.Category != nil && strings.TrimSpace(*in.Category) == "" {
		return models.Ticket{}, apperr.Unprocessable("category must not be empty")
	}
	if in.Priority != nil && !validPriority(*in.Priority) {
		return models.Ticket{}, apperr.Unprocessable("priority is not valid").WithDetails(map[string]string{
			"priority": "must be one of LOW, MEDIUM, HIGH, CRITICAL",
		})
	}
	if in.Status != nil && !validStatus(*in.Status) {
		return models.Ticket{}, apperr.Unprocessable("status is not valid").WithDetails(map[string]string{
			"status": "must be one of OPEN, IN_PROGRESS, RESOLVED, CLOSED",
		})
	}

	changes := store.UpdateChanges{
		Title:       in.Title,
		Description: in.Description,
		Category:    in.Category,
		Priority:    in.Priority,
		Status:      in.Status,
	}
	updated, err := s.tickets.Update(ctx, id, changes, actor.ID)
	if err != nil {
		return models.Ticket{}, apperr.Internal(err)
	}
	return updated, nil
}

// Delete removes a ticket. Only ADMIN may delete.
func (s *TicketService) Delete(ctx context.Context, actor models.User, id uuid.UUID) error {
	if actor.Role != models.RoleAdmin {
		return apperr.Forbidden("only admins can delete tickets")
	}
	if err := s.tickets.Delete(ctx, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NotFound("TICKET_NOT_FOUND", "ticket not found")
		}
		return apperr.Internal(err)
	}
	return nil
}

// Assign assigns a ticket to an agent. ADMIN and AGENT may assign, USER may
// not.
func (s *TicketService) Assign(ctx context.Context, actor models.User, id uuid.UUID, assigneeID uuid.UUID) (models.Ticket, error) {
	if actor.Role == models.RoleUser {
		return models.Ticket{}, apperr.Forbidden("only staff can assign tickets")
	}
	if _, err := s.tickets.Get(ctx, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return models.Ticket{}, apperr.NotFound("TICKET_NOT_FOUND", "ticket not found")
		}
		return models.Ticket{}, apperr.Internal(err)
	}
	assignee, err := s.users.GetByID(ctx, assigneeID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return models.Ticket{}, apperr.NotFound("USER_NOT_FOUND", "assignee not found")
		}
		return models.Ticket{}, apperr.Internal(err)
	}
	if assignee.Role != models.RoleAgent && assignee.Role != models.RoleAdmin {
		return models.Ticket{}, apperr.Unprocessable("tickets can only be assigned to AGENT or ADMIN")
	}
	updated, err := s.tickets.Update(ctx, id, store.UpdateChanges{AssignedTo: &assigneeID}, actor.ID)
	if err != nil {
		return models.Ticket{}, apperr.Internal(err)
	}
	return updated, nil
}

// History returns the audit trail of a ticket.
func (s *TicketService) History(ctx context.Context, actor models.User, id uuid.UUID) ([]models.HistoryEntry, error) {
	ticket, err := s.tickets.Get(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, apperr.NotFound("TICKET_NOT_FOUND", "ticket not found")
		}
		return nil, apperr.Internal(err)
	}
	if !canView(actor, ticket) {
		return nil, apperr.Forbidden("you are not allowed to view this ticket")
	}
	entries, err := s.tickets.History(ctx, id)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return entries, nil
}

// canView reports whether the actor may read the ticket.
func canView(actor models.User, ticket models.Ticket) bool {
	switch actor.Role {
	case models.RoleAdmin, models.RoleAgent:
		return true
	default:
		return ticket.CreatedBy == actor.ID
	}
}

// validateTitle checks the ticket title.
func validateTitle(title string) error {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return apperr.Unprocessable("title is required").WithDetails(map[string]string{"title": "must not be empty"})
	}
	if len(trimmed) > 200 {
		return apperr.Unprocessable("title is too long").WithDetails(map[string]string{"title": "must be at most 200 characters"})
	}
	return nil
}

// validateDescription checks the ticket description.
func validateDescription(description string) error {
	trimmed := strings.TrimSpace(description)
	if trimmed == "" {
		return apperr.Unprocessable("description is required").WithDetails(map[string]string{"description": "must not be empty"})
	}
	if len(trimmed) > 5000 {
		return apperr.Unprocessable("description is too long").WithDetails(map[string]string{"description": "must be at most 5000 characters"})
	}
	return nil
}

// validStatus reports whether s is a known ticket status.
func validStatus(s string) bool {
	switch s {
	case models.StatusOpen, models.StatusInProgress, models.StatusResolved, models.StatusClosed:
		return true
	}
	return false
}

// validPriority reports whether p is a known priority.
func validPriority(p string) bool {
	switch p {
	case models.PriorityLow, models.PriorityMedium, models.PriorityHigh, models.PriorityCritical:
		return true
	}
	return false
}
