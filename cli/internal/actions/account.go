package actions

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

type LoginProgress struct {
	Status string
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
	return BeginLoginWithProgress(server, openBrowser, nil)
}

func BeginLoginWithProgress(server string, openBrowser func(string), progress func(LoginProgress)) (LoginSession, error) {
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

	resp, err := createAuthSessionWithRetry(server, payload, progress)
	if err != nil {
		return LoginSession{}, err
	}
	resp.Body.Close()

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

func createAuthSessionWithRetry(server string, payload []byte, progress func(LoginProgress)) (*http.Response, error) {
	const maxAttempts = 3

	client := &http.Client{Timeout: 15 * time.Second}
	backoff := time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest(http.MethodPost, server+"/api/v1/auth/sessions", bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("creating auth session request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusCreated {
			if attempt > 1 {
				logLoginAttempt("auth session created after retry", attempt, server, http.StatusCreated, "", nil)
			}
			return resp, nil
		}

		statusCode := 0
		responseBody := ""
		if resp != nil {
			statusCode = resp.StatusCode
			responseBody = readLoginErrorBody(resp.Body)
			resp.Body.Close()
		}

		lastErr = loginAttemptError(err, statusCode, attempt)
		logLoginAttempt("auth session create failed", attempt, server, statusCode, responseBody, err)

		if attempt == maxAttempts {
			break
		}

		if progress != nil {
			progress(LoginProgress{
				Status: fmt.Sprintf("Attempt %d failed. Retrying (%d/%d)", attempt, attempt+1, maxAttempts),
			})
		}
		time.Sleep(backoff)
		backoff *= 2
	}

	return nil, lastErr
}

func loginAttemptError(err error, statusCode int, attempt int) error {
	suffix := fmt.Sprintf(" after %d attempts", attempt)
	if err != nil {
		return fmt.Errorf("could not reach server: %v%s", err, suffix)
	}
	return fmt.Errorf("could not create auth session (status %d)%s", statusCode, suffix)
}

func readLoginErrorBody(body io.Reader) string {
	if body == nil {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(body, 2048))
	if err != nil {
		return ""
	}
	text := strings.Join(strings.Fields(string(data)), " ")
	if len(text) > 300 {
		return text[:300] + "..."
	}
	return text
}

func logLoginAttempt(event string, attempt int, server string, statusCode int, responseBody string, err error) {
	paths := config.DefaultPaths()
	path := paths.LogFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()

	line := fmt.Sprintf("%s login %s attempt=%d server=%s", time.Now().Format(time.RFC3339), event, attempt, server)
	if statusCode > 0 {
		line += fmt.Sprintf(" status=%d", statusCode)
	}
	if err != nil {
		line += fmt.Sprintf(" error=%q", err.Error())
	}
	if trimmed := strings.TrimSpace(responseBody); trimmed != "" {
		line += fmt.Sprintf(" body=%q", trimmed)
	}
	_, _ = fmt.Fprintln(file, line)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
