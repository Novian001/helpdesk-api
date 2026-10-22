// Package store contains the SQL data access code. Everything talks to
// PostgreSQL through pgx with explicit SQL, no ORM.
package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/novian001/helpdesk-api/internal/models"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// ErrEmailTaken is returned when a user with the same email already exists.
var ErrEmailTaken = errors.New("email already registered")

// UserStore reads and writes users.
type UserStore struct {
	pool *pgxpool.Pool
}

// NewUserStore builds a UserStore on top of a connection pool.
func NewUserStore(pool *pgxpool.Pool) *UserStore { return &UserStore{pool: pool} }

const userColumns = `id, name, email, password_hash, role, created_at, updated_at`

// Create inserts a new user. A duplicate email is reported as ErrEmailTaken.
func (s *UserStore) Create(ctx context.Context, u models.User) (models.User, error) {
	const q = `INSERT INTO users (id, name, email, password_hash, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + userColumns
	err := s.pool.QueryRow(ctx, q, u.ID, u.Name, strings.ToLower(u.Email), u.PasswordHash, u.Role).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return models.User{}, ErrEmailTaken
		}
		return models.User{}, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

// GetByID loads a user by primary key.
func (s *UserStore) GetByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	return scanUser(s.pool.QueryRow(ctx, q, id))
}

// GetByEmail loads a user by email. The lookup is case insensitive because the
// unique index is built on lower(email).
func (s *UserStore) GetByEmail(ctx context.Context, email string) (models.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE lower(email) = lower($1)`
	return scanUser(s.pool.QueryRow(ctx, q, email))
}

// UpdateRole changes the role of a user and returns the updated row.
func (s *UserStore) UpdateRole(ctx context.Context, id uuid.UUID, role models.Role) (models.User, error) {
	const q = `UPDATE users SET role = $1, updated_at = now() WHERE id = $2 RETURNING ` + userColumns
	var u models.User
	err := s.pool.QueryRow(ctx, q, role, id).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("update role: %w", err)
	}
	return u, nil
}

// ListUsers returns all users ordered by name. Used by the admin endpoints.
func (s *UserStore) ListUsers(ctx context.Context) ([]models.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users ORDER BY name`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// scanUser reads one user row. pgx.ErrNoRows is translated to ErrNotFound.
func scanUser(row pgx.Row) (models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}
