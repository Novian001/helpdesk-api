package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds every runtime setting. All values come from the environment so
// the same binary can run locally, in Docker and in production unchanged.
type Config struct {
	AppEnv          string
	ServerPort      int
	DatabaseURL     string
	JWTSecret       string
	JWTAccessTTL    time.Duration
	JWTRefreshTTL   time.Duration
	ShutdownTimeout time.Duration
}

// IsProduction reports whether the process runs with production defaults.
func (c Config) IsProduction() bool { return c.AppEnv == "production" }

// Load reads configuration from the environment and fails fast on invalid or
// missing values. Secrets are never given a usable default: a missing
// JWT_SECRET must stop the process, not silently weaken authentication.
func Load() (Config, error) {
	cfg := Config{
		AppEnv:          envOr("APP_ENV", "development"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		ShutdownTimeout: 10 * time.Second,
	}

	port, err := strconv.Atoi(envOr("SERVER_PORT", "8080"))
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("SERVER_PORT must be a valid TCP port, got %q", os.Getenv("SERVER_PORT"))
	}
	cfg.ServerPort = port

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	if cfg.JWTAccessTTL, err = durationEnv("JWT_ACCESS_TTL", 15*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.JWTRefreshTTL, err = durationEnv("JWT_REFRESH_TTL", 7*24*time.Hour); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a Go duration (e.g. 15m): %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return d, nil
}
