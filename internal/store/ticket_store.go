package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/novian001/helpdesk-api/internal/models"
)

// TicketFilter holds the optional list filters.
type TicketFilter struct {
	Status   string
	Priority string
	Search   string
	// CreatedBy limits the result to tickets created by one user. It is what
	// enforces "a USER only sees their own tickets" in SQL, not in the UI.
	CreatedBy *uuid.UUID
	// AssignedTo limits the result to tickets assigned to one agent.
	AssignedTo *uuid.UUID
}

// TicketListResult carries a page of tickets plus the total row count.
type TicketListResult struct {
	Tickets []models.Ticket
	Total   int
}

// TicketStore reads and writes tickets and their history.
type TicketStore struct {
	pool *pgxpool.Pool
}

// NewTicketStore builds a TicketStore.
func NewTicketStore(pool *pgxpool.Pool) *TicketStore { return &TicketStore{pool: pool} }

const ticketColumns = `id, title, description, category, priority, status, created_by, assigned_to, created_at, updated_at`

// Create inserts a ticket and its CREATED history entry in one transaction.
func (s *TicketStore) Create(ctx context.Context, t models.Ticket, actorID uuid.UUID) (models.Ticket, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Ticket{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	const q = `INSERT INTO tickets (id, title, description, category, priority, status, created_by, assigned_to)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING ` + ticketColumns
	err = tx.QueryRow(ctx, q, t.ID, t.Title, t.Description, t.Category, t.Priority, t.Status, t.CreatedBy, t.AssignedTo).
		Scan(&t.ID, &t.Title, &t.Description, &t.Category, &t.Priority, &t.Status, &t.CreatedBy, &t.AssignedTo, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return models.Ticket{}, fmt.Errorf("insert ticket: %w", err)
	}
	if err := insertHistory(ctx, tx, t.ID, &actorID, "TICKET_CREATED", nil, ptr(t.Title)); err != nil {
		return models.Ticket{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Ticket{}, fmt.Errorf("commit: %w", err)
	}
	return t, nil
}

// Get loads one ticket.
func (s *TicketStore) Get(ctx context.Context, id uuid.UUID) (models.Ticket, error) {
	const q = `SELECT ` + ticketColumns + ` FROM tickets WHERE id = $1`
	var t models.Ticket
	err := s.pool.QueryRow(ctx, q, id).Scan(&t.ID, &t.Title, &t.Description, &t.Category, &t.Priority,
		&t.Status, &t.CreatedBy, &t.AssignedTo, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Ticket{}, ErrNotFound
	}
	if err != nil {
		return models.Ticket{}, fmt.Errorf("get ticket: %w", err)
	}
	return t, nil
}

// List returns a filtered, paginated page of tickets. The count query uses the
// same WHERE clause as the data query so pagination metadata is consistent.
func (s *TicketStore) List(ctx context.Context, f TicketFilter, limit, offset int) (TicketListResult, error) {
	where := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.Priority != "" {
		add("priority = $%d", f.Priority)
	}
	if f.CreatedBy != nil {
		add("created_by = $%d", *f.CreatedBy)
	}
	if f.AssignedTo != nil {
		add("assigned_to = $%d", *f.AssignedTo)
	}
	if f.Search != "" {
		// The GIN index in migration 002 backs this expression.
		args = append(args, f.Search)
		where = append(where, fmt.Sprintf("to_tsvector('simple', title || ' ' || description) @@ plainto_tsquery('simple', $%d)", len(args)))
	}
	clause := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM tickets WHERE `+clause, args...).Scan(&total); err != nil {
		return TicketListResult{}, fmt.Errorf("count tickets: %w", err)
	}

	args = append(args, limit, offset)
	q := `SELECT ` + ticketColumns + ` FROM tickets WHERE ` + clause +
		fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return TicketListResult{}, fmt.Errorf("list tickets: %w", err)
	}
	defer rows.Close()

	tickets := []models.Ticket{}
	for rows.Next() {
		var t models.Ticket
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Category, &t.Priority, &t.Status,
			&t.CreatedBy, &t.AssignedTo, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return TicketListResult{}, fmt.Errorf("scan ticket: %w", err)
		}
		tickets = append(tickets, t)
	}
	return TicketListResult{Tickets: tickets, Total: total}, rows.Err()
}

// UpdateChanges holds the mutable fields of a ticket. Nil means "not changed".
type UpdateChanges struct {
	Title       *string
	Description *string
	Category    *string
	Priority    *string
	Status      *string
	AssignedTo  *uuid.UUID
	// ClearAssignment removes the assignee when AssignedTo is nil and the
	// caller explicitly asked for it.
	ClearAssignment bool
}

// Update applies changes to a ticket and writes the history entries for every
// changed field. The ticket update and the history rows share one transaction
// so the audit trail can never drift from the ticket state.
func (s *TicketStore) Update(ctx context.Context, id uuid.UUID, changes UpdateChanges, actorID uuid.UUID) (models.Ticket, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Ticket{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := getTicketTx(ctx, tx, id)
	if err != nil {
		return models.Ticket{}, err
	}

	sets := []string{"updated_at = now()"}
	args := []any{}
	set := func(column string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if changes.Title != nil {
		set("title", *changes.Title)
	}
	if changes.Description != nil {
		set("description", *changes.Description)
	}
	if changes.Category != nil {
		set("category", *changes.Category)
	}
	if changes.Priority != nil {
		set("priority", *changes.Priority)
	}
	if changes.Status != nil {
		set("status", *changes.Status)
	}
	if changes.AssignedTo != nil {
		set("assigned_to", *changes.AssignedTo)
	} else if changes.ClearAssignment {
		sets = append(sets, "assigned_to = NULL")
	}
	args = append(args, id)

	updateQuery := `UPDATE tickets SET ` + strings.Join(sets, ", ") +
		fmt.Sprintf(" WHERE id = $%d RETURNING ", len(args)) + ticketColumns
	var updated models.Ticket
	err = tx.QueryRow(ctx, updateQuery, args...).Scan(&updated.ID, &updated.Title, &updated.Description,
		&updated.Category, &updated.Priority, &updated.Status, &updated.CreatedBy, &updated.AssignedTo,
		&updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		return models.Ticket{}, fmt.Errorf("update ticket: %w", err)
	}

	// History entries, one per changed field.
	if changes.Status != nil && *changes.Status != current.Status {
		if err := insertHistory(ctx, tx, id, &actorID, "STATUS_CHANGED", ptr(current.Status), ptr(*changes.Status)); err != nil {
			return models.Ticket{}, err
		}
		// A terminal status also gets its own action so the audit trail can be
		// filtered on "was this ticket resolved" without parsing old values.
		switch *changes.Status {
		case models.StatusResolved:
			if err := insertHistory(ctx, tx, id, &actorID, "RESOLVED", ptr(current.Status), ptr(*changes.Status)); err != nil {
				return models.Ticket{}, err
			}
		case models.StatusClosed:
			if err := insertHistory(ctx, tx, id, &actorID, "CLOSED", ptr(current.Status), ptr(*changes.Status)); err != nil {
				return models.Ticket{}, err
			}
		}
	}
	if changes.Priority != nil && *changes.Priority != current.Priority {
		if err := insertHistory(ctx, tx, id, &actorID, "PRIORITY_CHANGED", ptr(current.Priority), ptr(*changes.Priority)); err != nil {
			return models.Ticket{}, err
		}
	}
	if changes.Title != nil && *changes.Title != current.Title {
		if err := insertHistory(ctx, tx, id, &actorID, "TITLE_CHANGED", ptr(current.Title), ptr(*changes.Title)); err != nil {
			return models.Ticket{}, err
		}
	}
	if changes.AssignedTo != nil && (current.AssignedTo == nil || *current.AssignedTo != *changes.AssignedTo) {
		var oldValue *string
		if current.AssignedTo != nil {
			oldValue = ptr(current.AssignedTo.String())
		}
		if err := insertHistory(ctx, tx, id, &actorID, "ASSIGNED", oldValue, ptr(changes.AssignedTo.String())); err != nil {
			return models.Ticket{}, err
		}
	} else if changes.ClearAssignment && current.AssignedTo != nil {
		if err := insertHistory(ctx, tx, id, &actorID, "ASSIGNED", ptr(current.AssignedTo.String()), nil); err != nil {
			return models.Ticket{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Ticket{}, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
}

// Delete removes a ticket and, through the foreign keys, its comments and
// history.
func (s *TicketStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM tickets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete ticket: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// History returns the audit trail of a ticket, oldest first.
func (s *TicketStore) History(ctx context.Context, ticketID uuid.UUID) ([]models.HistoryEntry, error) {
	const q = `SELECT id, ticket_id, actor_id, action, old_value, new_value, created_at
		FROM ticket_histories WHERE ticket_id = $1 ORDER BY created_at, id`
	rows, err := s.pool.Query(ctx, q, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}
	defer rows.Close()
	entries := []models.HistoryEntry{}
	for rows.Next() {
		var e models.HistoryEntry
		if err := rows.Scan(&e.ID, &e.TicketID, &e.ActorID, &e.Action, &e.OldValue, &e.NewValue, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// getTicketTx loads a ticket inside a transaction.
func getTicketTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (models.Ticket, error) {
	const q = `SELECT ` + ticketColumns + ` FROM tickets WHERE id = $1`
	var t models.Ticket
	err := tx.QueryRow(ctx, q, id).Scan(&t.ID, &t.Title, &t.Description, &t.Category, &t.Priority,
		&t.Status, &t.CreatedBy, &t.AssignedTo, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Ticket{}, ErrNotFound
	}
	if err != nil {
		return models.Ticket{}, fmt.Errorf("get ticket: %w", err)
	}
	return t, nil
}

// insertHistory writes one audit row using the given transaction.
func insertHistory(ctx context.Context, tx pgx.Tx, ticketID uuid.UUID, actorID *uuid.UUID, action string, oldValue, newValue *string) error {
	const q = `INSERT INTO ticket_histories (id, ticket_id, actor_id, action, old_value, new_value)
		VALUES ($1, $2, $3, $4, $5, $6)`
	if _, err := tx.Exec(ctx, q, uuid.New(), ticketID, actorID, action, oldValue, newValue); err != nil {
		return fmt.Errorf("insert history: %w", err)
	}
	return nil
}

// ptr returns a pointer to a copy of v.
func ptr(v string) *string { return &v }
