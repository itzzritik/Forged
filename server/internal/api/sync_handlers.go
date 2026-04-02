package api

import (
	"io"
	"net/http"
	"strconv"

	"github.com/itzzritik/forged/server/internal/middleware"
)

func (s *Server) handleSyncPush(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	versionStr := r.Header.Get("X-Vault-Version")
	version, _ := strconv.ParseInt(versionStr, 10, 64)

	deviceID := r.Header.Get("X-Device-ID")

	blob, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB max
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read body")
		return
	}
	defer r.Body.Close()

	if len(blob) == 0 {
		writeError(w, http.StatusBadRequest, "empty vault blob")
		return
	}

	newVersion, err := s.DB.PushVault(r.Context(), userID, blob, version, deviceID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	s.DB.AuditLog(r.Context(), userID, deviceID, "sync_push", r.RemoteAddr)

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

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Vault-Version", strconv.FormatInt(vault.Version, 10))
	w.WriteHeader(http.StatusOK)
	w.Write(vault.EncryptedBlob)
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

	writeJSON(w, http.StatusOK, map[string]any{
		"has_vault":  true,
		"version":    vault.Version,
		"updated_at": vault.UpdatedAt,
	})
}
