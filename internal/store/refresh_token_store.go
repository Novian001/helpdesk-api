package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RefreshTokenStore persists refresh tokens as hashes. Storing only the hash
// means a database leak does not hand out usable sessions.
type RefreshTokenStore struct {
	pool *pgxpool.Pool
}

// NewRefreshTokenStore builds a RefreshTokenStore.
func NewRefreshTokenStore(pool *pgxpool.Pool) *RefreshTokenStore {
	return &RefreshTokenStore{pool: pool}
}

// Save stores the hash of a refresh token.
func (s *RefreshTokenStore) Save(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	const q = `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`
	if _, err := s.pool.Exec(ctx, q, uuid.New(), userID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

// FindActive returns the user id of an active, non revoked, non expired token.
func (s *RefreshTokenStore) FindActive(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	const q = `SELECT user_id FROM refresh_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`
	var userID uuid.UUID
	err := s.pool.QueryRow(ctx, q, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("find refresh token: %w", err)
	}
	return userID, nil
}

// Revoke marks a token as revoked. It is used by logout and by the rotation
// step of the refresh endpoint.
func (s *RefreshTokenStore) Revoke(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}
