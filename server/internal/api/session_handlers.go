package api

import (
	"net/http"

	"github.com/jackc/pgx/v5"
)

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code         string `json:"code"`
		Verification string `json:"verification"`
	}
	if err := readJSON(r, &req); err != nil || req.Code == "" || req.Verification == "" {
		writeError(w, http.StatusBadRequest, "code and verification required")
		return
	}

	err := s.DB.CreateAuthSession(r.Context(), req.Code, req.Verification)
	if err != nil {
		writeError(w, http.StatusConflict, "session already exists")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (s *Server) handlePollSession(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	w.Header().Set("Cache-Control", "no-store")

	session, err := s.DB.GetAuthSession(r.Context(), code)
	if err == pgx.ErrNoRows || session == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
		return
	}

	if session.Error != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "error",
			"error":  *session.Error,
		})
		return
	}

	if session.CompletedAt == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "complete",
		"token":   *session.Token,
		"user_id": *session.UserID,
		"email":   *session.Email,
	})
}

func (s *Server) handleGetVerification(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	w.Header().Set("Cache-Control", "no-store")

	session, err := s.DB.GetAuthSession(r.Context(), code)
	if err != nil || session == nil {
		writeJSON(w, http.StatusOK, map[string]string{"verification": ""})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"verification": session.Verification})
}
