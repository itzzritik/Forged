package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itzzritik/forged/server/internal/api"
	"github.com/itzzritik/forged/server/internal/auth"
	"github.com/itzzritik/forged/server/internal/db"
	"github.com/itzzritik/forged/server/internal/middleware"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	databaseURL := envRequired("DATABASE_URL")
	jwtSecret := envRequired("JWT_SECRET")
	port := envDefault("PORT", "8080")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	database, err := db.Connect(ctx, databaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	srv := &api.Server{
		DB:      database,
		Secret:  jwtSecret,
		DevMode: os.Getenv("REDIRECT_BASE_URL") == "",
		OAuth: auth.OAuthConfig{
			GoogleClientID:     envDefault("GOOGLE_CLIENT_ID", ""),
			GoogleClientSecret: envDefault("GOOGLE_CLIENT_SECRET", ""),
			GitHubClientID:     envDefault("GITHUB_CLIENT_ID", ""),
			GitHubClientSecret: envDefault("GITHUB_CLIENT_SECRET", ""),
			RedirectBaseURL:    envDefault("REDIRECT_BASE_URL", "https://forged-api.ritik.me"),
		},
	}

	handler := middleware.CORS(middleware.Logger(logger, srv.Routes()))

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("server starting", "port", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	logger.Info("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	httpServer.Shutdown(shutdownCtx)
}

func envRequired(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "required env var %s is not set\n", key)
		os.Exit(1)
	}
	return v
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
