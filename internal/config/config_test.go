package config

import (
	"testing"
	"time"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", "a-secret-that-is-definitely-long-enough")
	t.Setenv("SERVER_PORT", "")
	t.Setenv("APP_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ServerPort != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.ServerPort)
	}
	if cfg.JWTAccessTTL != 15*time.Minute {
		t.Fatalf("unexpected access TTL %s", cfg.JWTAccessTTL)
	}
}

func TestLoadRejectsMissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "a-secret-that-is-definitely-long-enough")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error when DATABASE_URL is missing")
	}
}

func TestLoadRejectsShortJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", "short")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a short JWT_SECRET")
	}
}

func TestLoadRejectsInvalidTTL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", "a-secret-that-is-definitely-long-enough")
	t.Setenv("JWT_ACCESS_TTL", "not-a-duration")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for an invalid duration")
	}
}
