package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"eneba-backend/internal/config"
	"eneba-backend/internal/database"
	"eneba-backend/internal/handlers"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/server"
	"eneba-backend/internal/services"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// run is the real entry point. Returning an error instead of calling
// os.Exit everywhere keeps deferred cleanup working.
func run() error {
	// Load .env for local development. On Render the file doesn't exist
	// and the real environment variables are used instead.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	slog.SetDefault(newLogger(cfg))

	// ctx is cancelled when the process gets Ctrl+C or SIGTERM (Render sends SIGTERM on redeploy).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Bring the schema up to date before serving traffic.
	if err := database.Migrate(cfg.DatabaseURL, "migrations"); err != nil {
		return err
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	slog.Info("database connected")
	// Wire dependencies by hand: repositories and shared services -> handlers.
	userRepo := repository.NewUserRepository(pool)
	refreshTokenRepo := repository.NewRefreshTokenRepository(pool)

	tokenService := services.NewTokenService(cfg)

	router := server.NewRouter(server.Deps{
		Config: cfg,
		Health: handlers.NewHealthHandler(pool),
		Auth:   handlers.NewAuthHandler(pool, userRepo, refreshTokenRepo, tokenService, cfg.IsProduction()),
		Users:  handlers.NewUserHandler(userRepo),
		Tokens: tokenService,
	})
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Run the server in a goroutine so main can wait for a shutdown signal.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err // server failed to start, e.g. port already in use
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	// Graceful shutdown: stop accepting new requests and give
	// in-flight ones up to 10 seconds to finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	slog.Info("server stopped cleanly")
	return nil
}

// newLogger returns readable text logs locally and JSON logs in production,
// which Render's log viewer can search and filter.
func newLogger(cfg *config.Config) *slog.Logger {
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
