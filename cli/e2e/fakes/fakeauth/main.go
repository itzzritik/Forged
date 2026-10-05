//go:build e2e

// fakeauth replaces forged-auth during E2E runs so tests never raise a real
// Windows Hello prompt on the developer's desktop. Results are scripted
// through files in FORGED_E2E_AUTH_DIR:
//
//	result    authorize outcome (ok, canceled, failed, unavailable_by_environment)
//	lock      present => emit one session_locked event, then deleted
//	password  present => collect-password "types" its contents; absent => unavailable
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
)

func main() {
	dir := os.Getenv("FORGED_E2E_AUTH_DIR")
	var mu sync.Mutex
	emit := func(resp sensitiveauth.HelperResponse) {
		mu.Lock()
		defer mu.Unlock()
		resp.Provider = "e2e-fake"
		_ = json.NewEncoder(os.Stdout).Encode(resp)
	}
	go watchLocks(dir, emit)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req sensitiveauth.HelperRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		record(dir, req)
		resp := sensitiveauth.HelperResponse{ID: req.ID, Type: req.Type}
		switch req.Type {
		case "cancel":
			continue
		case "authorize":
			resp.Status = scripted(dir, "result", "ok")
		case "status", "subscribe-locks":
			resp.Status = "ok"
		case "collect-password":
			resp.Status = "unavailable_by_platform"
			if dir != "" {
				if password, err := os.ReadFile(filepath.Join(dir, "password")); err == nil {
					resp.Status = "ok"
					resp.Secret = base64.StdEncoding.EncodeToString(password)
				}
			}
		default:
			resp.Status = "failed"
			resp.Message = "unsupported request"
		}
		emit(resp)
	}
}

func scripted(dir, name, fallback string) string {
	if dir == "" {
		return fallback
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return fallback
	}
	if v := strings.TrimSpace(string(data)); v != "" {
		return v
	}
	return fallback
}

func record(dir string, req sensitiveauth.HelperRequest) {
	if dir == "" {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "requests.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(time.Now().Format(time.RFC3339Nano) + " " + req.Type + " " + req.Action + "\n")
}

func watchLocks(dir string, emit func(sensitiveauth.HelperResponse)) {
	if dir == "" {
		return
	}
	lock := filepath.Join(dir, "lock")
	for {
		if err := os.Remove(lock); err == nil {
			emit(sensitiveauth.HelperResponse{Type: "event", Status: "session_locked"})
		}
		time.Sleep(100 * time.Millisecond)
	}
}
