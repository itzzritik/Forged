package api

import (
	"net/http"

	"github.com/itzzritik/forged/server/internal/db"
	"github.com/itzzritik/forged/server/internal/middleware"
)

type registerDeviceRequest struct {
	Name            string `json:"name"`
	Platform        string `json:"platform"`
	Hostname        string `json:"hostname"`
	DevicePublicKey string `json:"device_public_key"`
}

func (s *Server) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req registerDeviceRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Platform == "" || req.DevicePublicKey == "" {
		writeError(w, http.StatusBadRequest, "name, platform, and device_public_key required")
		return
	}

	dev, err := s.DB.CreateDevice(r.Context(), userID, req.Name, req.Platform, req.Hostname, req.DevicePublicKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not register device")
		return
	}

	s.DB.AuditLog(r.Context(), userID, dev.ID, "device_register", r.RemoteAddr)

	writeJSON(w, http.StatusCreated, dev)
}

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	devices, err := s.DB.ListDevices(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list devices")
		return
	}

	if devices == nil {
		devices = []db.Device{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (s *Server) handleApproveDevice(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	deviceID := r.PathValue("id")

	if err := s.DB.ApproveDevice(r.Context(), userID, deviceID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (s *Server) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	deviceID := r.PathValue("id")

	if err := s.DB.DeleteDevice(r.Context(), userID, deviceID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	user, err := s.DB.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	if err := s.DB.DeleteUser(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete account")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
