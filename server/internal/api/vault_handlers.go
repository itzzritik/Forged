package api

import (
	"encoding/json"
	"net/http"

	"github.com/itzzritik/forged/server/internal/middleware"
)

func (s *Server) handleVaultRekey(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req struct {
		KDFParams             json.RawMessage `json:"kdf_params"`
		ProtectedSymmetricKey string          `json:"protected_symmetric_key"`
	}
	if err := readJSON(r, &req); err != nil || req.ProtectedSymmetricKey == "" {
		writeError(w, http.StatusBadRequest, "protected_symmetric_key required")
		return
	}

	if err := s.DB.UpdateRekey(r.Context(), userID, req.KDFParams, req.ProtectedSymmetricKey); err != nil {
		writeError(w, http.StatusInternalServerError, "rekey failed")
		return
	}

	s.DB.AuditLog(r.Context(), userID, "", "vault_rekey_success", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
