package api

import (
	"encoding/json"
	"net/http"

	"github.com/itzzritik/forged/server/internal/auth"
	"github.com/itzzritik/forged/server/internal/db"
	"github.com/itzzritik/forged/server/internal/middleware"
)

type Server struct {
	DB      *db.DB
	Secret  string
	OAuth   auth.OAuthConfig
	DevMode bool
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

	mux.Handle("/api/v1/", middleware.Auth(s.Secret, authed))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return mux
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
