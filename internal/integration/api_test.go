// Package integration exercises the real API against a real PostgreSQL
// database. The tests are skipped when TEST_DATABASE_URL is not set, so the
// unit test suite stays runnable without a database.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/novian001/helpdesk-api/internal/auth"
	"github.com/novian001/helpdesk-api/internal/db"
	"github.com/novian001/helpdesk-api/internal/httpapi"
	"github.com/novian001/helpdesk-api/internal/service"
	"github.com/novian001/helpdesk-api/internal/store"
)

const testSecret = "integration-test-secret-value-long-enough"

// testApp builds the full application on a migrated test database.
func testApp(t *testing.T) *fiber.App {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set, skipping integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	// Start every test from a clean schema.
	if _, err := pool.Exec(ctx, `TRUNCATE tickets, comments, ticket_histories, refresh_tokens, users CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	t.Cleanup(pool.Close)

	manager := auth.NewManager(testSecret, 15*time.Minute, time.Hour)
	userStore := store.NewUserStore(pool)
	ticketStore := store.NewTicketStore(pool)
	commentStore := store.NewCommentStore(pool)
	tokenStore := store.NewRefreshTokenStore(pool)

	authSvc := service.NewAuthService(userStore, tokenStore, manager)
	ticketSvc := service.NewTicketService(ticketStore, userStore)
	commentSvc := service.NewCommentService(commentStore, ticketStore)

	return httpapi.NewApp(httpapi.NewHandlers(authSvc, ticketSvc, commentSvc), manager, testLogger())
}

// testLogger returns a logger that discards output so the test log stays
// readable.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// request performs a JSON request against the app.
func request(t *testing.T, app *fiber.App, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := app.Test(req, 10000)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	parsed := map[string]any{}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil && res.StatusCode != http.StatusNoContent {
		t.Fatalf("%s %s: decode response: %v (status %d)", method, path, err, res.StatusCode)
	}
	return res.StatusCode, parsed
}

// registeredUser registers a user and returns the access token and the user id.
func registeredUser(t *testing.T, app *fiber.App, name, email string) (string, string) {
	t.Helper()
	status, body := request(t, app, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"name": name, "email": email, "password": "supersecret123",
	})
	if status != http.StatusCreated {
		t.Fatalf("register %s: status %d, body %v", email, status, body)
	}
	user := body["user"].(map[string]any)
	return body["access_token"].(string), user["id"].(string)
}
