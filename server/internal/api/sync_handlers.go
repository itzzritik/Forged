package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/itzzritik/forged/server/internal/middleware"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) handleSyncPush(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req struct {
		Blob                  string          `json:"blob"`
		KDFParams             json.RawMessage `json:"kdf_params"`
		ProtectedSymmetricKey string          `json:"protected_symmetric_key"`
		MasterPasswordHash    string          `json:"master_password_hash"`
		ExpectedVersion       int64           `json:"expected_version"`
		DeviceID              string          `json:"device_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	blob, err := base64.StdEncoding.DecodeString(req.Blob)
	if err != nil || len(blob) == 0 {
		writeError(w, http.StatusBadRequest, "invalid or empty blob")
		return
	}

	newVersion, err := s.DB.PushVault(r.Context(), userID, blob, req.ExpectedVersion, req.DeviceID, req.KDFParams, req.ProtectedSymmetricKey)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	if req.MasterPasswordHash != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(req.MasterPasswordHash), bcrypt.DefaultCost)
		if err == nil {
			s.DB.SetMasterPasswordHash(r.Context(), userID, string(hashed))
		}
	}

	s.DB.AuditLog(r.Context(), userID, req.DeviceID, "sync_push", r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]any{
		"version": newVersion,
	})
}

func (s *Server) handleSyncPull(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	vault, err := s.DB.GetVault(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "no vault found")
		return
	}

	deviceID := r.Header.Get("X-Device-ID")
	s.DB.AuditLog(r.Context(), userID, deviceID, "sync_pull", r.RemoteAddr)

	resp := map[string]any{
		"blob":    base64.StdEncoding.EncodeToString(vault.EncryptedBlob),
		"version": vault.Version,
	}
	if vault.KDFParams != nil {
		resp["kdf_params"] = json.RawMessage(vault.KDFParams)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	vault, err := s.DB.GetVault(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"has_vault": false,
		})
		return
	}

	resp := map[string]any{
		"has_vault":  true,
		"version":    vault.Version,
		"updated_at": vault.UpdatedAt,
	}
	if vault.KDFParams != nil {
		resp["kdf_params"] = json.RawMessage(vault.KDFParams)
	}

	writeJSON(w, http.StatusOK, resp)
}
