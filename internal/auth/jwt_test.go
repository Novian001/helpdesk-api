package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/novian001/helpdesk-api/internal/models"
)

func newTestManager() *Manager {
	return NewManager("test-secret-that-is-long-enough-1234", 15*time.Minute, 24*time.Hour)
}

func TestIssueAndParseAccessToken(t *testing.T) {
	m := newTestManager()
	userID := uuid.New()
	token, err := m.IssueAccessToken(userID, models.RoleAgent, "Citra")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := m.Parse(token, KindAccess)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Subject != userID.String() {
		t.Fatalf("unexpected subject %q", claims.Subject)
	}
	if claims.Role != models.RoleAgent {
		t.Fatalf("unexpected role %q", claims.Role)
	}
}

func TestRefreshTokenCannotBeUsedAsAccessToken(t *testing.T) {
	m := newTestManager()
	refresh, err := m.IssueRefreshToken(uuid.New(), models.RoleUser, "Budi")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := m.Parse(refresh, KindAccess); err == nil {
		t.Fatal("a refresh token must not pass as an access token")
	}
}

func TestTokenSignedWithAnotherSecretIsRejected(t *testing.T) {
	token, err := newTestManager().IssueAccessToken(uuid.New(), models.RoleUser, "Budi")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	other := NewManager("a-completely-different-secret-value", time.Minute, time.Hour)
	if _, err := other.Parse(token, KindAccess); err == nil {
		t.Fatal("a token signed with a different secret must be rejected")
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	m := NewManager("test-secret-that-is-long-enough-1234", -time.Minute, time.Hour)
	token, err := m.IssueAccessToken(uuid.New(), models.RoleUser, "Budi")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := m.Parse(token, KindAccess); err == nil {
		t.Fatal("an expired token must be rejected")
	}
}
