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

// CommentService implements the comment rules.
type CommentService struct {
	comments *store.CommentStore
	tickets  *store.TicketStore
}

// NewCommentService wires the comment service.
func NewCommentService(comments *store.CommentStore, tickets *store.TicketStore) *CommentService {
	return &CommentService{comments: comments, tickets: tickets}
}

// Add appends a comment to a ticket the actor may access.
func (s *CommentService) Add(ctx context.Context, actor models.User, ticketID uuid.UUID, body string) (models.Comment, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return models.Comment{}, apperr.Unprocessable("body is required").WithDetails(map[string]string{"body": "must not be empty"})
	}
	if len(trimmed) > 5000 {
		return models.Comment{}, apperr.Unprocessable("body is too long").WithDetails(map[string]string{"body": "must be at most 5000 characters"})
	}
	ticket, err := s.tickets.Get(ctx, ticketID)
	if errors.Is(err, store.ErrNotFound) {
		return models.Comment{}, apperr.NotFound("TICKET_NOT_FOUND", "ticket not found")
	}
	if err != nil {
		return models.Comment{}, apperr.Internal(err)
	}
	if !canView(actor, ticket) {
		return models.Comment{}, apperr.Forbidden("you are not allowed to comment on this ticket")
	}
	comment, err := s.comments.Create(ctx, ticketID, actor.ID, trimmed)
	if err != nil {
		return models.Comment{}, apperr.Internal(err)
	}
	return comment, nil
}

// List returns the comments of a ticket.
func (s *CommentService) List(ctx context.Context, actor models.User, ticketID uuid.UUID) ([]models.Comment, error) {
	ticket, err := s.tickets.Get(ctx, ticketID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, apperr.NotFound("TICKET_NOT_FOUND", "ticket not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if !canView(actor, ticket) {
		return nil, apperr.Forbidden("you are not allowed to view this ticket")
	}
	comments, err := s.comments.List(ctx, ticketID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return comments, nil
}

// Delete removes a comment. The author may delete their own comment, an admin
// may delete any comment.
func (s *CommentService) Delete(ctx context.Context, actor models.User, commentID uuid.UUID) error {
	comment, err := s.comments.Get(ctx, commentID)
	if errors.Is(err, store.ErrNotFound) {
		return apperr.NotFound("COMMENT_NOT_FOUND", "comment not found")
	}
	if err != nil {
		return apperr.Internal(err)
	}
	if actor.Role != models.RoleAdmin && comment.UserID != actor.ID {
		return apperr.Forbidden("you can only delete your own comments")
	}
	if err := s.comments.Delete(ctx, commentID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NotFound("COMMENT_NOT_FOUND", "comment not found")
		}
		return apperr.Internal(err)
	}
	return nil
}
