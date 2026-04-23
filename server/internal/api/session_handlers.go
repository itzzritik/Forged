package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	serverauth "github.com/itzzritik/forged/server/internal/auth"
	"github.com/itzzritik/forged/server/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code            string `json:"code"`
		Verification    string `json:"verification"`
		CodeChallenge   string `json:"code_challenge"`
		ChallengeMethod string `json:"challenge_method"`
	}
	if err := readJSON(r, &req); err != nil || req.Code == "" || req.Verification == "" {
		writeError(w, http.StatusBadRequest, "Code and verification required")
		return
	}
	if (strings.TrimSpace(req.CodeChallenge) == "") != (strings.TrimSpace(req.ChallengeMethod) == "") {
		writeError(w, http.StatusBadRequest, "Code challenge and challenge method must be provided together")
		return
	}
	if method := strings.TrimSpace(req.ChallengeMethod); method != "" && strings.ToUpper(method) != "S256" {
		writeError(w, http.StatusBadRequest, "Only S256 PKCE challenges are supported")
		return
	}

	err := s.DB.CreateAuthSession(r.Context(), req.Code, req.Verification, req.CodeChallenge, strings.ToUpper(strings.TrimSpace(req.ChallengeMethod)))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "Session already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "Could not create auth session")
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

	if session.ApprovedAt != nil && session.CompletedAt == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
		return
	}

	if session.CompletedAt == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
		return
	}

	if session.Token != nil && session.UserID != nil && session.Email != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "complete",
			"token":   *session.Token,
			"user_id": *session.UserID,
			"email":   *session.Email,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
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

func (s *Server) handleExchangeSession(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	var req struct {
		CodeVerifier string `json:"code_verifier"`
	}
	if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.CodeVerifier) == "" {
		writeError(w, http.StatusBadRequest, "Code verifier required")
		return
	}

	session, err := s.DB.ConsumeApprovedAuthSession(r.Context(), code)
	if errors.Is(err, db.ErrSessionNotFound) || errors.Is(err, pgx.ErrNoRows) || session == nil {
		s.logAuthSecurityEvent("auth session exchange rejected", "reason", "session_not_ready", "code", code, "remote_addr", r.RemoteAddr)
		writeError(w, http.StatusNotFound, "Auth session not found or not ready")
		return
	}
	if err != nil {
		s.logAuthSecurityEvent("auth session exchange failed", "reason", "db_error", "code", code, "remote_addr", r.RemoteAddr, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not consume auth session")
		return
	}
	if session.CodeChallenge == nil || session.ChallengeMethod == nil || session.ApprovedUserID == nil {
		s.logAuthSecurityEvent("auth session exchange rejected", "reason", "missing_pkce", "code", code, "remote_addr", r.RemoteAddr)
		writeError(w, http.StatusBadRequest, "Auth session does not support exchange")
		return
	}
	if !serverauth.VerifyPKCE(req.CodeVerifier, *session.CodeChallenge, *session.ChallengeMethod) {
		s.logAuthSecurityEvent("auth session exchange rejected", "reason", "invalid_verifier", "code", code, "remote_addr", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "Code verifier is invalid")
		return
	}

	user, err := s.DB.GetUserByID(r.Context(), *session.ApprovedUserID)
	if err != nil {
		s.logAuthSecurityEvent("auth session exchange failed", "reason", "user_not_found", "code", code, "remote_addr", r.RemoteAddr, "user_id", *session.ApprovedUserID)
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	response, err := issueTokenPairResponse(r.Context(), s, user)
	if err != nil {
		s.logAuthSecurityEvent("auth session exchange failed", "reason", "token_issue_failed", "code", code, "remote_addr", r.RemoteAddr, "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not issue token pair")
		return
	}
	s.logAuthSecurityInfoEvent("auth session exchange completed", "code", code, "remote_addr", r.RemoteAddr, "user_id", user.ID)
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
		writeError(w, http.StatusBadRequest, "Refresh token required")
		return
	}

	sessionID, secret, err := serverauth.DecodeRefreshToken(req.RefreshToken)
	if err != nil {
		s.logAuthSecurityEvent("refresh rejected", "reason", "invalid_token_format", "remote_addr", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "Refresh token is invalid")
		return
	}

	newSecret, newSecretHash, err := serverauth.GenerateRefreshSecret()
	if err != nil {
		s.logAuthSecurityEvent("refresh failed", "reason", "secret_generation_failed", "remote_addr", r.RemoteAddr, "session_id", sessionID, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not rotate refresh token")
		return
	}

	next, user, err := s.DB.RotateRefreshSession(
		r.Context(),
		sessionID,
		serverauth.HashRefreshSecret(secret),
		newSecretHash,
		time.Now().Add(serverauth.RefreshTokenTTL).UTC(),
	)
	if errors.Is(err, db.ErrRefreshSessionReplay) {
		s.logAuthSecurityEvent("refresh replay detected", "remote_addr", r.RemoteAddr, "session_id", sessionID)
		writeError(w, http.StatusUnauthorized, "Refresh token is not valid")
		return
	}
	if errors.Is(err, db.ErrRefreshSessionExpired) || errors.Is(err, db.ErrRefreshSessionRevoked) || errors.Is(err, db.ErrRefreshSessionInvalid) || errors.Is(err, db.ErrRefreshSessionNotFound) {
		s.logAuthSecurityEvent("refresh rejected", "reason", refreshRejectReason(err), "remote_addr", r.RemoteAddr, "session_id", sessionID)
		writeError(w, http.StatusUnauthorized, "Refresh token is not valid")
		return
	}
	if err != nil {
		s.logAuthSecurityEvent("refresh failed", "reason", "db_error", "remote_addr", r.RemoteAddr, "session_id", sessionID, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not refresh session")
		return
	}

	response, err := issueTokenPairResponseWithSession(s, user, next, serverauth.EncodeRefreshToken(next.ID, newSecret))
	if err != nil {
		s.logAuthSecurityEvent("refresh failed", "reason", "token_issue_failed", "remote_addr", r.RemoteAddr, "session_id", next.ID, "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not issue token pair")
		return
	}
	s.logAuthSecurityInfoEvent("refresh rotated", "remote_addr", r.RemoteAddr, "session_id", next.ID, "user_id", user.ID)
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
		writeError(w, http.StatusBadRequest, "Refresh token required")
		return
	}

	sessionID, secret, err := serverauth.DecodeRefreshToken(req.RefreshToken)
	if err != nil {
		s.logAuthSecurityEvent("logout handled", "reason", "invalid_token_format", "remote_addr", r.RemoteAddr)
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
		return
	}

	err = s.DB.RevokeRefreshSession(r.Context(), sessionID, serverauth.HashRefreshSecret(secret), "logout")
	if errors.Is(err, db.ErrRefreshSessionNotFound) || errors.Is(err, db.ErrRefreshSessionInvalid) {
		s.logAuthSecurityEvent("logout handled", "reason", "already_invalid", "remote_addr", r.RemoteAddr, "session_id", sessionID)
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
		return
	}
	if err != nil {
		s.logAuthSecurityEvent("logout failed", "reason", "db_error", "remote_addr", r.RemoteAddr, "session_id", sessionID, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not revoke session")
		return
	}

	s.logAuthSecurityInfoEvent("logout revoked", "remote_addr", r.RemoteAddr, "session_id", sessionID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func issueTokenPairResponse(ctx context.Context, s *Server, user db.User) (map[string]any, error) {
	refreshSecret, refreshHash, err := serverauth.GenerateRefreshSecret()
	if err != nil {
		writeErrorNoReply(s, "Generating refresh secret failed", err)
		return nil, err
	}

	session, err := s.DB.CreateRefreshSession(ctx, user.ID, "", refreshHash, time.Now().Add(serverauth.RefreshTokenTTL).UTC(), nil)
	if err != nil {
		writeErrorNoReply(s, "Creating refresh session failed", err)
		return nil, err
	}

	return issueTokenPairResponseWithSession(s, user, session, serverauth.EncodeRefreshToken(session.ID, refreshSecret))
}

func issueTokenPairResponseWithSession(s *Server, user db.User, session db.RefreshSession, refreshToken string) (map[string]any, error) {
	accessToken, accessExpiresAt, err := serverauth.GenerateAccessToken(user.ID, user.Email, user.Name, s.Secret, serverauth.AccessTokenTTL)
	if err != nil {
		writeErrorNoReply(s, "Generating access token failed", err)
		return nil, err
	}

	return map[string]any{
		"status":             "complete",
		"token_type":         "Bearer",
		"access_token":       accessToken,
		"access_expires_at":  accessExpiresAt.Format(time.RFC3339),
		"refresh_token":      refreshToken,
		"refresh_expires_at": session.ExpiresAt.Format(time.RFC3339),
		"user_id":            user.ID,
		"email":              user.Email,
		"name":               user.Name,
	}, nil
}

func writeErrorNoReply(s *Server, message string, err error) {
	if s.Logger != nil {
		s.Logger.Error(message, "error", err)
	}
}

func (s *Server) logAuthSecurityEvent(message string, args ...any) {
	if s.Logger != nil {
		s.Logger.Warn(message, args...)
	}
}

func (s *Server) logAuthSecurityInfoEvent(message string, args ...any) {
	if s.Logger != nil {
		s.Logger.Info(message, args...)
	}
}

func refreshRejectReason(err error) string {
	switch {
	case errors.Is(err, db.ErrRefreshSessionExpired):
		return "expired"
	case errors.Is(err, db.ErrRefreshSessionRevoked):
		return "revoked"
	case errors.Is(err, db.ErrRefreshSessionInvalid):
		return "invalid_secret"
	case errors.Is(err, db.ErrRefreshSessionNotFound):
		return "not_found"
	default:
		return "unknown"
	}
}
