// Package service holds the business rules. Services receive their
// dependencies through the constructor, which keeps them easy to test.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/novian001/helpdesk-api/internal/apperr"
	"github.com/novian001/helpdesk-api/internal/auth"
	"github.com/novian001/helpdesk-api/internal/models"
	"github.com/novian001/helpdesk-api/internal/store"
)

// ErrInvalidCredentials is returned for both an unknown email and a wrong
// password so the API never reveals which emails exist.
var ErrInvalidCredentials = errors.New("invalid credentials")

// AuthService implements registration, login, refresh and logout.
type AuthService struct {
	users   *store.UserStore
	tokens  *store.RefreshTokenStore
	manager *auth.Manager
}

// NewAuthService wires the auth service.
func NewAuthService(users *store.UserStore, tokens *store.RefreshTokenStore, manager *auth.Manager) *AuthService {
	return &AuthService{users: users, tokens: tokens, manager: manager}
}

// TokenPair is what the auth endpoints return.
type TokenPair struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int64       `json:"expires_in"`
	User         models.User `json:"user"`
}

// Register creates a new account. The first registered account becomes ADMIN
// so a fresh database is usable without a manual seed script.
func (s *AuthService) Register(ctx context.Context, name, email, password string) (TokenPair, error) {
	if err := validateEmail(email); err != nil {
		return TokenPair{}, err
	}
	if err := validatePassword(password); err != nil {
		return TokenPair{}, err
	}
	if strings.TrimSpace(name) == "" {
		return TokenPair{}, apperr.Unprocessable("name is required").WithDetails(map[string]string{"name": "must not be empty"})
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return TokenPair{}, apperr.Internal(err)
	}

	role := models.RoleUser
	existing, err := s.users.ListUsers(ctx)
	if err != nil {
		return TokenPair{}, apperr.Internal(err)
	}
	if len(existing) == 0 {
		role = models.RoleAdmin
	}

	user, err := s.users.Create(ctx, models.User{
		ID:           uuid.New(),
		Name:         strings.TrimSpace(name),
		Email:        strings.TrimSpace(email),
		PasswordHash: hash,
		Role:         role,
	})
	if errors.Is(err, store.ErrEmailTaken) {
		return TokenPair{}, apperr.Conflict("EMAIL_ALREADY_REGISTERED", "email is already registered")
	}
	if err != nil {
		return TokenPair{}, apperr.Internal(err)
	}
	return s.issueTokens(ctx, user)
}

// Login verifies the password and issues a new token pair.
func (s *AuthService) Login(ctx context.Context, email, password string) (TokenPair, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		// Run a dummy verification so a missing account and a wrong password
		// take a comparable amount of time.
		_, _ = auth.VerifyPassword(password, dummyHash)
		return TokenPair{}, apperr.Unauthorized("invalid email or password")
	}
	if err != nil {
		return TokenPair{}, apperr.Internal(err)
	}
	ok, err := auth.VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return TokenPair{}, apperr.Internal(err)
	}
	if !ok {
		return TokenPair{}, apperr.Unauthorized("invalid email or password")
	}
	return s.issueTokens(ctx, user)
}

// Refresh rotates a refresh token: the old one is revoked and a new pair is
// issued. Reusing a revoked token fails.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	if refreshToken == "" {
		return TokenPair{}, apperr.Unprocessable("refresh_token is required")
	}
	claims, err := s.manager.Parse(refreshToken, auth.KindRefresh)
	if err != nil {
		return TokenPair{}, apperr.Unauthorized("invalid refresh token")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return TokenPair{}, apperr.Unauthorized("invalid refresh token")
	}
	if _, err := s.tokens.FindActive(ctx, hashToken(refreshToken)); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return TokenPair{}, apperr.Unauthorized("refresh token is not active")
		}
		return TokenPair{}, apperr.Internal(err)
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return TokenPair{}, apperr.Unauthorized("invalid refresh token")
	}
	if err := s.tokens.Revoke(ctx, hashToken(refreshToken)); err != nil {
		return TokenPair{}, apperr.Internal(err)
	}
	return s.issueTokens(ctx, user)
}

// Logout revokes a refresh token.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return apperr.Unprocessable("refresh_token is required")
	}
	if err := s.tokens.Revoke(ctx, hashToken(refreshToken)); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// Me returns the profile of the authenticated user.
func (s *AuthService) Me(ctx context.Context, userID uuid.UUID) (models.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return models.User{}, apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	if err != nil {
		return models.User{}, apperr.Internal(err)
	}
	return user, nil
}

// SetRole changes the role of a user. Admin only, enforced by the HTTP layer.
func (s *AuthService) SetRole(ctx context.Context, userID uuid.UUID, role models.Role) (models.User, error) {
	if !role.Valid() {
		return models.User{}, apperr.Unprocessable("role is not valid").WithDetails(map[string]string{
			"role": "must be one of ADMIN, AGENT, USER",
		})
	}
	user, err := s.users.UpdateRole(ctx, userID, role)
	if errors.Is(err, store.ErrNotFound) {
		return models.User{}, apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	if err != nil {
		return models.User{}, apperr.Internal(err)
	}
	return user, nil
}

// ListUsers returns all users. Admin only, enforced by the HTTP layer.
func (s *AuthService) ListUsers(ctx context.Context) ([]models.User, error) {
	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return users, nil
}

// issueTokens creates an access token and stores the hash of a refresh token.
func (s *AuthService) issueTokens(ctx context.Context, user models.User) (TokenPair, error) {
	access, err := s.manager.IssueAccessToken(user.ID, user.Role, user.Name)
	if err != nil {
		return TokenPair{}, apperr.Internal(fmt.Errorf("issue access token: %w", err))
	}
	refresh, err := s.manager.IssueRefreshToken(user.ID, user.Role, user.Name)
	if err != nil {
		return TokenPair{}, apperr.Internal(fmt.Errorf("issue refresh token: %w", err))
	}
	expiresAt := time.Now().Add(s.manager.RefreshTTL())
	if err := s.tokens.Save(ctx, user.ID, hashToken(refresh), expiresAt); err != nil {
		return TokenPair{}, apperr.Internal(err)
	}
	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.manager.AccessTTL().Seconds()),
		User:         user,
	}, nil
}

// hashToken hashes a raw token with SHA-256 for storage.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
