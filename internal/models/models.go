// Package models contains the domain types shared by the store, service and
// HTTP layers.
package models

import (
	"time"

	"github.com/google/uuid"
)

// Role is the access level of a user account.
type Role string

const (
	RoleAdmin Role = "ADMIN"
	RoleAgent Role = "AGENT"
	RoleUser  Role = "USER"
)

// Valid reports whether the role is one of the three supported values.
func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleAgent || r == RoleUser
}

// Ticket status values.
const (
	StatusOpen       = "OPEN"
	StatusInProgress = "IN_PROGRESS"
	StatusResolved   = "RESOLVED"
	StatusClosed     = "CLOSED"
)

// Ticket priority values.
const (
	PriorityLow      = "LOW"
	PriorityMedium   = "MEDIUM"
	PriorityHigh     = "HIGH"
	PriorityCritical = "CRITICAL"
)

// User is an account able to authenticate against the API.
type User struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Ticket is an incident tracked by the helpdesk.
type Ticket struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Category    string     `json:"category"`
	Priority    string     `json:"priority"`
	Status      string     `json:"status"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	AssignedTo  *uuid.UUID `json:"assigned_to"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Comment is a message in a ticket discussion.
type Comment struct {
	ID        uuid.UUID `json:"id"`
	TicketID  uuid.UUID `json:"ticket_id"`
	UserID    uuid.UUID `json:"user_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HistoryEntry records one important change on a ticket.
type HistoryEntry struct {
	ID        uuid.UUID  `json:"id"`
	TicketID  uuid.UUID  `json:"ticket_id"`
	ActorID   *uuid.UUID `json:"actor_id"`
	Action    string     `json:"action"`
	OldValue  *string    `json:"old_value"`
	NewValue  *string    `json:"new_value"`
	CreatedAt time.Time  `json:"created_at"`
}

// Pagination describes the slice of a list endpoint that was returned.
type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}
