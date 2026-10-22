package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/novian001/helpdesk-api/internal/models"
)

// CommentStore reads and writes ticket comments.
type CommentStore struct {
	pool *pgxpool.Pool
}

// NewCommentStore builds a CommentStore.
func NewCommentStore(pool *pgxpool.Pool) *CommentStore { return &CommentStore{pool: pool} }

const commentColumns = `id, ticket_id, user_id, body, created_at, updated_at`

// Create inserts a comment.
func (s *CommentStore) Create(ctx context.Context, ticketID, userID uuid.UUID, body string) (models.Comment, error) {
	const q = `INSERT INTO comments (id, ticket_id, user_id, body) VALUES ($1, $2, $3, $4) RETURNING ` + commentColumns
	var c models.Comment
	err := s.pool.QueryRow(ctx, q, uuid.New(), ticketID, userID, body).Scan(
		&c.ID, &c.TicketID, &c.UserID, &c.Body, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return models.Comment{}, fmt.Errorf("insert comment: %w", err)
	}
	return c, nil
}

// List returns the comments of a ticket, oldest first.
func (s *CommentStore) List(ctx context.Context, ticketID uuid.UUID) ([]models.Comment, error) {
	const q = `SELECT ` + commentColumns + ` FROM comments WHERE ticket_id = $1 ORDER BY created_at, id`
	rows, err := s.pool.Query(ctx, q, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()
	comments := []models.Comment{}
	for rows.Next() {
		var c models.Comment
		if err := rows.Scan(&c.ID, &c.TicketID, &c.UserID, &c.Body, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// Get loads one comment.
func (s *CommentStore) Get(ctx context.Context, id uuid.UUID) (models.Comment, error) {
	const q = `SELECT ` + commentColumns + ` FROM comments WHERE id = $1`
	var c models.Comment
	err := s.pool.QueryRow(ctx, q, id).Scan(&c.ID, &c.TicketID, &c.UserID, &c.Body, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Comment{}, ErrNotFound
	}
	if err != nil {
		return models.Comment{}, fmt.Errorf("get comment: %w", err)
	}
	return c, nil
}

// Delete removes a comment. It reports ErrNotFound when nothing was deleted so
// the caller can answer 404 instead of a silent success.
func (s *CommentStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM comments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
