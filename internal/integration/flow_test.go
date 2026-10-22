package integration

import (
	"net/http"
	"testing"
)

// TestRegisterLoginAndMe walks through the authentication flow against the
// real database.
func TestRegisterLoginAndMe(t *testing.T) {
	app := testApp(t)

	status, body := request(t, app, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"name": "Budi Santoso", "email": "budi@example.com", "password": "supersecret123",
	})
	if status != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d (%v)", status, body)
	}
	if body["access_token"] == "" || body["refresh_token"] == "" {
		t.Fatal("register must return an access and a refresh token")
	}
	access := body["access_token"].(string)

	status, body = request(t, app, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "budi@example.com", "password": "supersecret123",
	})
	if status != http.StatusOK {
		t.Fatalf("login: expected 200, got %d (%v)", status, body)
	}
	access = body["access_token"].(string)

	status, body = request(t, app, http.MethodGet, "/api/v1/me", access, nil)
	if status != http.StatusOK {
		t.Fatalf("me: expected 200, got %d (%v)", status, body)
	}
	if body["email"] != "budi@example.com" {
		t.Fatalf("unexpected profile: %v", body)
	}
	if _, leaked := body["password_hash"]; leaked {
		t.Fatal("the response must never contain the password hash")
	}
}

// TestLoginWithWrongPasswordIsUnauthorized checks the error envelope.
func TestLoginWithWrongPasswordIsUnauthorized(t *testing.T) {
	app := testApp(t)
	registeredUser(t, app, "Budi", "budi@example.com")

	status, body := request(t, app, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "budi@example.com", "password": "wrong-password",
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%v)", status, body)
	}
	errObj := body["error"].(map[string]any)
	if errObj["code"] != "UNAUTHORIZED" {
		t.Fatalf("unexpected error code: %v", errObj["code"])
	}
}

// TestProtectedRouteNeedsToken checks the authentication middleware.
func TestProtectedRouteNeedsToken(t *testing.T) {
	app := testApp(t)
	status, body := request(t, app, http.MethodGet, "/api/v1/tickets", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%v)", status, body)
	}
}

// TestRefreshTokenFlow checks rotation and revocation.
func TestRefreshTokenFlow(t *testing.T) {
	app := testApp(t)
	_, body := request(t, app, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"name": "Budi", "email": "budi@example.com", "password": "supersecret123",
	})
	refresh := body["refresh_token"].(string)

	status, body := request(t, app, http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": refresh,
	})
	if status != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d (%v)", status, body)
	}
	rotated := body["refresh_token"].(string)
	if rotated == refresh {
		t.Fatal("refresh must rotate the token")
	}

	// The old token is revoked and must not work a second time.
	status, _ = request(t, app, http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": refresh,
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a reused refresh token, got %d", status)
	}

	// Logout revokes the token.
	status, _ = request(t, app, http.MethodPost, "/api/v1/auth/logout", "", map[string]string{
		"refresh_token": rotated,
	})
	if status != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d", status)
	}
	status, _ = request(t, app, http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": rotated,
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", status)
	}
}

// TestCreateAndGetTicket creates a ticket and reads it back.
func TestCreateAndGetTicket(t *testing.T) {
	app := testApp(t)
	token, _ := registeredUser(t, app, "Budi", "budi@example.com")

	status, body := request(t, app, http.MethodPost, "/api/v1/tickets", token, map[string]string{
		"title": "Database timeout on checkout", "description": "Checkout times out after 30 seconds.",
		"category": "database", "priority": "HIGH",
	})
	if status != http.StatusCreated {
		t.Fatalf("create ticket: expected 201, got %d (%v)", status, body)
	}
	if body["status"] != "OPEN" {
		t.Fatalf("a new ticket must start OPEN, got %v", body["status"])
	}
	ticketID := body["id"].(string)

	status, body = request(t, app, http.MethodGet, "/api/v1/tickets/"+ticketID, token, nil)
	if status != http.StatusOK {
		t.Fatalf("get ticket: expected 200, got %d (%v)", status, body)
	}
	if body["title"] != "Database timeout on checkout" {
		t.Fatalf("unexpected ticket: %v", body)
	}
}

// TestUpdateTicketAndHistory checks the update flow and the audit trail.
func TestUpdateTicketAndHistory(t *testing.T) {
	app := testApp(t)
	token, _ := registeredUser(t, app, "Budi", "budi@example.com")

	_, body := request(t, app, http.MethodPost, "/api/v1/tickets", token, map[string]string{
		"title": "Timeout", "description": "Times out.", "category": "database", "priority": "LOW",
	})
	ticketID := body["id"].(string)

	status, body := request(t, app, http.MethodPatch, "/api/v1/tickets/"+ticketID, token, map[string]string{
		"status": "IN_PROGRESS", "priority": "CRITICAL",
	})
	if status != http.StatusOK {
		t.Fatalf("update: expected 200, got %d (%v)", status, body)
	}
	if body["status"] != "IN_PROGRESS" || body["priority"] != "CRITICAL" {
		t.Fatalf("update did not apply: %v", body)
	}

	status, body = request(t, app, http.MethodGet, "/api/v1/tickets/"+ticketID+"/history", token, nil)
	if status != http.StatusOK {
		t.Fatalf("history: expected 200, got %d (%v)", status, body)
	}
	entries := body["data"].([]any)
	actions := map[string]bool{}
	for _, e := range entries {
		actions[e.(map[string]any)["action"].(string)] = true
	}
	for _, want := range []string{"TICKET_CREATED", "STATUS_CHANGED", "PRIORITY_CHANGED"} {
		if !actions[want] {
			t.Fatalf("history is missing %s: %v", want, actions)
		}
	}
}

// TestAssignTicket checks the assignment rules: staff can assign a ticket to
// an agent, a plain user cannot assign at all.
func TestAssignTicket(t *testing.T) {
	app := testApp(t)
	adminToken, _ := registeredUser(t, app, "Admin", "admin@example.com")
	// The second registered user starts as a plain USER; the admin promotes
	// them to AGENT so they become a valid assignee.
	_, agentID := registeredUser(t, app, "Agent", "agent@example.com")
	status, body := request(t, app, http.MethodPatch, "/api/v1/users/"+agentID+"/role", adminToken, map[string]string{
		"role": "AGENT",
	})
	if status != http.StatusOK {
		t.Fatalf("promote to AGENT: expected 200, got %d (%v)", status, body)
	}

	_, body = request(t, app, http.MethodPost, "/api/v1/tickets", adminToken, map[string]string{
		"title": "Timeout", "description": "Times out.", "category": "database", "priority": "HIGH",
	})
	ticketID := body["id"].(string)

	status, body = request(t, app, http.MethodPost, "/api/v1/tickets/"+ticketID+"/assign", adminToken, map[string]string{
		"user_id": agentID,
	})
	if status != http.StatusOK {
		t.Fatalf("assign: expected 200, got %d (%v)", status, body)
	}
	if body["assigned_to"] != agentID {
		t.Fatalf("assigned_to was not set: %v", body)
	}
}

// TestCommentFlow checks adding, listing and deleting comments.
func TestCommentFlow(t *testing.T) {
	app := testApp(t)
	token, _ := registeredUser(t, app, "Budi", "budi@example.com")

	_, body := request(t, app, http.MethodPost, "/api/v1/tickets", token, map[string]string{
		"title": "Timeout", "description": "Times out.", "category": "database", "priority": "HIGH",
	})
	ticketID := body["id"].(string)

	status, body := request(t, app, http.MethodPost, "/api/v1/tickets/"+ticketID+"/comments", token, map[string]string{
		"body": "Reproduced in staging.",
	})
	if status != http.StatusCreated {
		t.Fatalf("comment: expected 201, got %d (%v)", status, body)
	}
	commentID := body["id"].(string)

	status, body = request(t, app, http.MethodGet, "/api/v1/tickets/"+ticketID+"/comments", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list comments: expected 200, got %d (%v)", status, body)
	}
	if len(body["data"].([]any)) != 1 {
		t.Fatalf("expected one comment, got %v", body["data"])
	}

	status, _ = request(t, app, http.MethodDelete, "/api/v1/comments/"+commentID, token, nil)
	if status != http.StatusOK {
		t.Fatalf("delete comment: expected 200, got %d", status)
	}
}

// TestUserCannotSeeOtherUsersTickets checks the RBAC rule on the list and get
// endpoints.
func TestUserCannotSeeOtherUsersTickets(t *testing.T) {
	app := testApp(t)
	adminToken, _ := registeredUser(t, app, "Admin", "admin@example.com")
	userToken, _ := registeredUser(t, app, "Budi", "budi@example.com")

	_, body := request(t, app, http.MethodPost, "/api/v1/tickets", adminToken, map[string]string{
		"title": "Admin ticket", "description": "Owned by the admin.", "category": "network", "priority": "LOW",
	})
	ticketID := body["id"].(string)

	status, body := request(t, app, http.MethodGet, "/api/v1/tickets/"+ticketID, userToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (%v)", status, body)
	}

	status, body = request(t, app, http.MethodGet, "/api/v1/tickets", userToken, nil)
	if status != http.StatusOK {
		t.Fatalf("list: expected 200, got %d (%v)", status, body)
	}
	data := body["data"].([]any)
	if len(data) != 0 {
		t.Fatalf("a plain user must only see their own tickets, got %d", len(data))
	}
}

// TestUserCannotAssignOrDelete checks the forbidden staff operations.
func TestUserCannotAssignOrDelete(t *testing.T) {
	app := testApp(t)
	adminToken, adminID := registeredUser(t, app, "Admin", "admin@example.com")
	userToken, _ := registeredUser(t, app, "Budi", "budi@example.com")

	_, body := request(t, app, http.MethodPost, "/api/v1/tickets", adminToken, map[string]string{
		"title": "Admin ticket", "description": "Owned by the admin.", "category": "network", "priority": "LOW",
	})
	ticketID := body["id"].(string)

	status, body := request(t, app, http.MethodPost, "/api/v1/tickets/"+ticketID+"/assign", userToken, map[string]string{
		"user_id": adminID,
	})
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 on assign, got %d (%v)", status, body)
	}

	status, body = request(t, app, http.MethodDelete, "/api/v1/tickets/"+ticketID, userToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 on delete, got %d (%v)", status, body)
	}
}

// TestPaginationFilterAndSearch checks the list query behaviour.
func TestPaginationFilterAndSearch(t *testing.T) {
	app := testApp(t)
	token, _ := registeredUser(t, app, "Budi", "budi@example.com")

	for i := 0; i < 5; i++ {
		request(t, app, http.MethodPost, "/api/v1/tickets", token, map[string]string{
			"title":       "Database timeout",
			"description": "Checkout database timeout while saving the order.",
			"category":    "database", "priority": "HIGH",
		})
	}
	request(t, app, http.MethodPost, "/api/v1/tickets", token, map[string]string{
		"title": "Printer jam", "description": "Office printer jams on duplex.", "category": "hardware", "priority": "LOW",
	})

	status, body := request(t, app, http.MethodGet, "/api/v1/tickets?page=1&limit=2", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list: expected 200, got %d (%v)", status, body)
	}
	pagination := body["pagination"].(map[string]any)
	if len(body["data"].([]any)) != 2 {
		t.Fatalf("expected 2 tickets on the page, got %d", len(body["data"].([]any)))
	}
	if pagination["total"].(float64) != 6 {
		t.Fatalf("expected total 6, got %v", pagination["total"])
	}
	if pagination["total_pages"].(float64) != 3 {
		t.Fatalf("expected 3 total pages, got %v", pagination["total_pages"])
	}

	status, body = request(t, app, http.MethodGet, "/api/v1/tickets?priority=LOW", token, nil)
	if status != http.StatusOK {
		t.Fatalf("filter: expected 200, got %d (%v)", status, body)
	}
	if body["pagination"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("priority filter did not apply: %v", body["pagination"])
	}

	status, body = request(t, app, http.MethodGet, "/api/v1/tickets?search=database", token, nil)
	if status != http.StatusOK {
		t.Fatalf("search: expected 200, got %d (%v)", status, body)
	}
	if body["pagination"].(map[string]any)["total"].(float64) != 5 {
		t.Fatalf("search did not apply: %v", body["pagination"])
	}
}

// TestValidationErrors checks the validation error envelope.
func TestValidationErrors(t *testing.T) {
	app := testApp(t)
	token, _ := registeredUser(t, app, "Budi", "budi@example.com")

	status, body := request(t, app, http.MethodPost, "/api/v1/tickets", token, map[string]string{
		"title": "", "description": "d", "category": "database", "priority": "URGENT",
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%v)", status, body)
	}
	errObj := body["error"].(map[string]any)
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Fatalf("unexpected error code: %v", errObj["code"])
	}
	details := errObj["details"].(map[string]any)
	if _, ok := details["title"]; !ok {
		t.Fatalf("expected a title detail, got %v", details)
	}
	if _, ok := details["priority"]; !ok {
		t.Fatalf("expected a priority detail, got %v", details)
	}

	status, body = request(t, app, http.MethodGet, "/api/v1/tickets?page=0", token, nil)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for page=0, got %d (%v)", status, body)
	}
}

// TestNotFoundError checks the 404 envelope.
func TestNotFoundError(t *testing.T) {
	app := testApp(t)
	token, _ := registeredUser(t, app, "Budi", "budi@example.com")
	status, body := request(t, app, http.MethodGet, "/api/v1/tickets/6d1b9a1e-0c4e-4b6e-9a1e-0f1b2c3d4e5f", token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%v)", status, body)
	}
	errObj := body["error"].(map[string]any)
	if errObj["code"] != "TICKET_NOT_FOUND" {
		t.Fatalf("unexpected error code: %v", errObj["code"])
	}
}
