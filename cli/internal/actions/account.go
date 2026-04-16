package actions

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/ipc"
)

type AccountCredentials struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
}

type LoginSession struct {
	VerificationCode string
	URL              string
	wait             func(context.Context) (AccountCredentials, error)
}

func (s LoginSession) Wait(ctx context.Context) (AccountCredentials, error) {
	if s.wait == nil {
		return AccountCredentials{}, fmt.Errorf("login session is not ready")
	}
	return s.wait(ctx)
}

func CredentialsPath(paths config.Paths) string {
	return paths.CredentialsFile()
}

func LoadCredentials(paths config.Paths) (AccountCredentials, error) {
	data, err := os.ReadFile(CredentialsPath(paths))
	if err != nil {
		return AccountCredentials{}, fmt.Errorf("not logged in. Run: forged login")
	}

	var creds AccountCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return AccountCredentials{}, fmt.Errorf("corrupted credentials file")
	}
	return creds, nil
}

func SaveCredentials(paths config.Paths, creds AccountCredentials) error {
	path := CredentialsPath(paths)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, _ := json.MarshalIndent(creds, "", "  ")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}

	if _, running := daemon.IsRunning(paths); !running {
		return nil
	}

	_, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdSyncLink, ipc.SyncLinkArgs{
		ServerURL: creds.ServerURL,
		Token:     creds.Token,
		UserID:    creds.UserID,
	})
	if err != nil {
		return fmt.Errorf("linking running daemon: %w", err)
	}
	return nil
}

func ClearCredentials(paths config.Paths) error {
	if err := os.Remove(CredentialsPath(paths)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func BeginLogin(server string, openBrowser func(string)) (LoginSession, error) {
	code, err := randomHex(16)
	if err != nil {
		return LoginSession{}, fmt.Errorf("generating code: %w", err)
	}

	verification, err := randomHex(2)
	if err != nil {
		return LoginSession{}, fmt.Errorf("generating verification: %w", err)
	}

	payload, _ := json.Marshal(map[string]string{
		"code":         code,
		"verification": verification,
	})

	resp, err := http.Post(server+"/api/v1/auth/sessions", "application/json", bytes.NewReader(payload))
	if err != nil {
		return LoginSession{}, fmt.Errorf("could not reach server: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return LoginSession{}, fmt.Errorf("could not create auth session (status %d)", resp.StatusCode)
	}

	authURL := ipc.DefaultWebApp + "/login?code=" + code
	if openBrowser != nil {
		openBrowser(authURL)
	}

	pollURL := server + "/api/v1/auth/sessions/" + code
	displayCode := fmt.Sprintf("FORGE-%s", strings.ToUpper(verification))

	return LoginSession{
		VerificationCode: displayCode,
		URL:              authURL,
		wait: func(ctx context.Context) (AccountCredentials, error) {
			return pollLogin(ctx, server, pollURL)
		},
	}, nil
}

func pollLogin(ctx context.Context, server string, pollURL string) (AccountCredentials, error) {
	deadline := time.Now().Add(5 * time.Minute)
	interval := 2 * time.Second

	for time.Now().Before(deadline) {
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return AccountCredentials{}, ctx.Err()
		case <-timer.C:
		}

		resp, err := http.Get(pollURL)
		if err != nil {
			interval = min(interval*2, 10*time.Second)
			continue
		}

		var result struct {
			Status string `json:"status"`
			Token  string `json:"token"`
			UserID string `json:"user_id"`
			Email  string `json:"email"`
			Error  string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		interval = 2 * time.Second

		switch result.Status {
		case "complete":
			return AccountCredentials{
				ServerURL: server,
				Token:     result.Token,
				UserID:    result.UserID,
				Email:     result.Email,
			}, nil
		case "error":
			return AccountCredentials{}, fmt.Errorf("authentication failed: %s", result.Error)
		case "pending":
			continue
		}
	}

	return AccountCredentials{}, fmt.Errorf("login timed out after 5 minutes")
}

func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}

	if err := cmd.Start(); err == nil {
		go cmd.Wait()
	}
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
