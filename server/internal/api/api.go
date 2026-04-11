package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/itzzritik/forged/server/internal/auth"
	"github.com/itzzritik/forged/server/internal/db"
	"github.com/itzzritik/forged/server/internal/middleware"
)

type Server struct {
	DB         *db.DB
	Secret     string
	OAuth      auth.OAuthConfig
	DevMode    bool
	Logger     *slog.Logger
	HTTPClient *http.Client
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	if s.DevMode {
		mux.HandleFunc("POST /api/v1/auth/dev", s.handleDevAuth)
	}
	mux.HandleFunc("GET /api/v1/auth/google", s.handleGoogleRedirect)
	mux.HandleFunc("GET /api/v1/auth/google/callback", s.handleGoogleCallback)
	mux.HandleFunc("GET /api/v1/auth/github", s.handleGitHubRedirect)
	mux.HandleFunc("GET /api/v1/auth/github/callback", s.handleGitHubCallback)

	authed := http.NewServeMux()
	authed.HandleFunc("POST /api/v1/sync/push", s.handleSyncPush)
	authed.HandleFunc("GET /api/v1/sync/pull", s.handleSyncPull)
	authed.HandleFunc("GET /api/v1/sync/status", s.handleSyncStatus)
	authed.HandleFunc("GET /api/v1/devices", s.handleListDevices)
	authed.HandleFunc("POST /api/v1/devices", s.handleRegisterDevice)
	authed.HandleFunc("DELETE /api/v1/devices/{id}", s.handleDeleteDevice)
	authed.HandleFunc("POST /api/v1/devices/{id}/approve", s.handleApproveDevice)
	authed.HandleFunc("GET /api/v1/account", s.handleGetAccount)
	authed.HandleFunc("POST /api/v1/account/delete", s.handleDeleteAccount)
	authed.HandleFunc("POST /api/v1/vault/verify", s.handleVaultVerify)
	authed.HandleFunc("POST /api/v1/vault/rekey", s.handleVaultRekey)

	mux.HandleFunc("POST /api/v1/auth/sessions", middleware.RateLimit(10, s.handleCreateSession))
	mux.HandleFunc("GET /api/v1/auth/sessions/{code}", middleware.RateLimit(30, s.handlePollSession))
	mux.HandleFunc("GET /api/v1/auth/sessions/{code}/verification", middleware.RateLimit(30, s.handleGetVerification))

	mux.Handle("/api/v1/", middleware.Auth(s.Secret, authed))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return mux
}

func (s *Server) StartSessionCleanup() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			count, err := s.DB.CleanupAuthSessions(context.Background())
			if err != nil && s.Logger != nil {
				s.Logger.Error("auth session cleanup failed", "error", err)
			} else if count > 0 && s.Logger != nil {
				s.Logger.Debug("cleaned auth sessions", "count", count)
			}
			auditCount, auditErr := s.DB.CleanupAuditLog(context.Background())
			if auditErr != nil && s.Logger != nil {
				s.Logger.Error("audit log cleanup failed", "error", auditErr)
			} else if auditCount > 0 && s.Logger != nil {
				s.Logger.Debug("cleaned audit log", "count", auditCount)
			}
		}
	}()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
