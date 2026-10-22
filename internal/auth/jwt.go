package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/novian001/helpdesk-api/internal/models"
)

// TokenKind distinguishes access tokens from refresh tokens so a refresh token
// can never be replayed as an access token.
type TokenKind string

const (
	KindAccess  TokenKind = "access"
	KindRefresh TokenKind = "refresh"
)

// Claims is the JWT payload. Role travels in the token so the middleware can
// authorize without a database round trip.
type Claims struct {
	Role models.Role `json:"role"`
	Kind TokenKind   `json:"kind"`
	Name string      `json:"name,omitempty"`
	jwt.RegisteredClaims
}

// Manager issues and verifies access and refresh tokens.
type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewManager builds a token manager.
func NewManager(secret string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

// AccessTTL exposes the configured access token lifetime.
func (m *Manager) AccessTTL() time.Duration { return m.accessTTL }

// RefreshTTL exposes the configured refresh token lifetime.
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

// IssueAccessToken returns a signed access token for a user.
func (m *Manager) IssueAccessToken(userID uuid.UUID, role models.Role, name string) (string, error) {
	return m.issue(userID, role, name, KindAccess, m.accessTTL)
}

// IssueRefreshToken returns a signed refresh token for a user.
func (m *Manager) IssueRefreshToken(userID uuid.UUID, role models.Role, name string) (string, error) {
	return m.issue(userID, role, name, KindRefresh, m.refreshTTL)
}

func (m *Manager) issue(userID uuid.UUID, role models.Role, name string, kind TokenKind, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		Role: role,
		Kind: kind,
		Name: name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        uuid.NewString(),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse validates the signature and expiry of a token and checks its kind.
func (m *Manager) Parse(tokenString string, kind TokenKind) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("token is not valid")
	}
	if claims.Kind != kind {
		return nil, fmt.Errorf("expected %s token, got %s", kind, claims.Kind)
	}
	return claims, nil
}
