package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/itzzritik/forged/server/internal/middleware"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) handleVaultVerify(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req struct {
		MasterPasswordHash string `json:"master_password_hash"`
	}
	if err := readJSON(r, &req); err != nil || req.MasterPasswordHash == "" {
		writeError(w, http.StatusBadRequest, "master_password_hash required")
		return
	}

	hash, protectedKey, _, lockedUntil, err := s.DB.GetUserVaultAuth(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "no vault found")
		return
	}

	// Check lockout
	if lockedUntil != nil && time.Now().Before(*lockedUntil) {
		s.DB.AuditLog(r.Context(), userID, "", "vault_unlock_failed_locked", r.RemoteAddr)
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"locked_until": lockedUntil})
		return
	}

	// If no hash set yet, return protected key directly (backward compat)
	if hash == nil || *hash == "" {
		w.Header().Set("Cache-Control", "no-store")
		resp := map[string]any{"verified": true}
		if protectedKey != nil {
			resp["protected_symmetric_key"] = *protectedKey
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Verify password hash
	if err := bcrypt.CompareHashAndPassword([]byte(*hash), []byte(req.MasterPasswordHash)); err != nil {
		newAttempts, _ := s.DB.IncrementUnlockAttempts(r.Context(), userID)
		s.DB.AuditLog(r.Context(), userID, "", "vault_unlock_failed", r.RemoteAddr)

		if newAttempts >= 5 {
			lockUntil := time.Now().Add(15 * time.Minute)
			s.DB.LockVaultUnlock(r.Context(), userID, lockUntil)
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"locked_until": lockUntil})
			return
		}

		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"verified":           false,
			"attempts_remaining": 5 - newAttempts,
		})
		return
	}

	// Success
	s.DB.ResetUnlockAttempts(r.Context(), userID)
	s.DB.AuditLog(r.Context(), userID, "", "vault_unlock_success", r.RemoteAddr)

	w.Header().Set("Cache-Control", "no-store")
	resp := map[string]any{"verified": true}
	if protectedKey != nil {
		resp["protected_symmetric_key"] = *protectedKey
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleVaultRekey(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req struct {
		OldMasterPasswordHash string          `json:"old_master_password_hash"`
		KDFParams             json.RawMessage `json:"kdf_params"`
		ProtectedSymmetricKey string          `json:"protected_symmetric_key"`
		MasterPasswordHash    string          `json:"master_password_hash"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	// Verify old password
	hash, _, _, _, err := s.DB.GetUserVaultAuth(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "no vault found")
		return
	}

	if hash != nil && *hash != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(*hash), []byte(req.OldMasterPasswordHash)); err != nil {
			s.DB.AuditLog(r.Context(), userID, "", "vault_rekey_failed", r.RemoteAddr)
			writeError(w, http.StatusUnauthorized, "wrong password")
			return
		}
	}

	// Hash new password hash with bcrypt before storage
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.MasterPasswordHash), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hashing failed")
		return
	}

	if err := s.DB.UpdateRekey(r.Context(), userID, req.KDFParams, req.ProtectedSymmetricKey, string(newHash)); err != nil {
		writeError(w, http.StatusInternalServerError, "rekey failed")
		return
	}

	s.DB.AuditLog(r.Context(), userID, "", "vault_rekey_success", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
