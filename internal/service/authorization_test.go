package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/novian001/helpdesk-api/internal/models"
)

func TestCanView(t *testing.T) {
	owner := uuid.New()
	ticket := models.Ticket{ID: uuid.New(), CreatedBy: owner}

	admin := models.User{ID: uuid.New(), Role: models.RoleAdmin}
	agent := models.User{ID: uuid.New(), Role: models.RoleAgent}
	reporter := models.User{ID: owner, Role: models.RoleUser}
	stranger := models.User{ID: uuid.New(), Role: models.RoleUser}

	if !canView(admin, ticket) {
		t.Fatal("admin must see every ticket")
	}
	if !canView(agent, ticket) {
		t.Fatal("agent must see every ticket")
	}
	if !canView(reporter, ticket) {
		t.Fatal("the creator must see their own ticket")
	}
	if canView(stranger, ticket) {
		t.Fatal("another user must not see the ticket")
	}
}

func TestValidationRejectsBadInput(t *testing.T) {
	if err := validateTitle("  "); err == nil {
		t.Fatal("an empty title must be rejected")
	}
	if err := validateDescription(""); err == nil {
		t.Fatal("an empty description must be rejected")
	}
	if err := validateEmail("not-an-email"); err == nil {
		t.Fatal("an invalid email must be rejected")
	}
	if err := validatePassword("short"); err == nil {
		t.Fatal("a short password must be rejected")
	}
	if err := validateEmail("user@example.com"); err != nil {
		t.Fatalf("a valid email must pass: %v", err)
	}
}
