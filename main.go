// Command helpdesk-api runs the helpdesk REST API.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/novian001/helpdesk-api/internal/auth"
	"github.com/novian001/helpdesk-api/internal/config"
	"github.com/novian001/helpdesk-api/internal/db"
	"github.com/novian001/helpdesk-api/internal/httpapi"
	"github.com/novian001/helpdesk-api/internal/service"
	"github.com/novian001/helpdesk-api/internal/store"
)

func main() {
	log := newLogger()
	if err := run(log); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

// run wires everything and blocks until the process is asked to stop.
func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	log.Info("database ready", "env", cfg.AppEnv)

	manager := auth.NewManager(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	userStore := store.NewUserStore(pool)
	ticketStore := store.NewTicketStore(pool)
	commentStore := store.NewCommentStore(pool)
	tokenStore := store.NewRefreshTokenStore(pool)

	authSvc := service.NewAuthService(userStore, tokenStore, manager)
	ticketSvc := service.NewTicketService(ticketStore, userStore)
	commentSvc := service.NewCommentService(commentStore, ticketStore)

	app := httpapi.NewApp(httpapi.NewHandlers(authSvc, ticketSvc, commentSvc), manager, log)

	serverErr := make(chan error, 1)
	go func() {
		log.Info("server listening", "port", cfg.ServerPort, "env", cfg.AppEnv)
		if err := app.Listen(":" + strconv.Itoa(cfg.ServerPort)); err != nil {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	// Graceful shutdown: stop accepting new connections and let in flight
	// requests finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		return err
	}
	log.Info("server stopped")
	return nil
}

// newLogger builds the structured logger used by the whole process.
func newLogger() *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("APP_ENV") == "development" {
		level = slog.LevelDebug
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
