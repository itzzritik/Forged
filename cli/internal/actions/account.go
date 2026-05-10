package actions

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/itzzritik/forged/cli/internal/accountauth"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/ipc"
)

type AccountCredentials = accountauth.Credentials

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
		return AccountCredentials{}, fmt.Errorf("Log-in flow is not ready")
	}
	return s.wait(ctx)
}

func CredentialsPath(paths config.Paths) string {
	return accountauth.CredentialsPath(paths)
}

func LoadCredentials(paths config.Paths) (AccountCredentials, error) {
	creds, err := accountauth.Load(paths)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, accountauth.ErrLoginRequired) {
		return AccountCredentials{}, fmt.Errorf("Not logged in. Open Forged and use Manage > Log In")
	}
	if err != nil {
		return AccountCredentials{}, fmt.Errorf("Could not load saved login: %w", err)
	}
	return creds, nil
}

func LoadFreshCredentials(ctx context.Context, paths config.Paths) (AccountCredentials, error) {
	creds, err := accountauth.EnsureFresh(ctx, paths)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, accountauth.ErrLoginRequired) {
		return AccountCredentials{}, fmt.Errorf("Not logged in. Open Forged and use Manage > Log In")
	}
	if err != nil {
		return AccountCredentials{}, err
	}
	return creds, nil
}

func SaveCredentials(paths config.Paths, creds AccountCredentials) error {
	if err := accountauth.Save(paths, creds); err != nil {
		return err
	}

	if _, running := daemon.IsRunning(paths); !running {
		return nil
	}
	if !daemonHasActiveVaultSession(paths) {
		return nil
	}

	_, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdSyncLink, ipc.SyncLinkArgs{
		ServerURL: creds.ServerURL,
		Token:     accountauth.CurrentToken(creds),
		UserID:    creds.UserID,
	})
	if err != nil {
		return fmt.Errorf("Linking running daemon: %w", err)
	}
	return nil
}

func daemonHasActiveVaultSession(paths config.Paths) bool {
	resp, err := ipc.NewClient(paths.CtlSocket()).CallWithTimeout(ipc.CmdStatus, nil, 3*time.Second)
	if err != nil {
		return false
	}
	var status struct {
		Sensitive *struct {
			Active bool `json:"active"`
		} `json:"sensitive"`
	}
	if err := json.Unmarshal(resp.Data, &status); err != nil || status.Sensitive == nil {
		return false
	}
	return status.Sensitive.Active
}

func ClearCredentials(paths config.Paths) error {
	if creds, err := accountauth.Load(paths); err == nil {
		revokeRemoteSession(creds)
	}

	if _, running := daemon.IsRunning(paths); running {
		if _, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdSyncUnlink, nil); err != nil {
			return fmt.Errorf("Unlinking running daemon: %w", err)
		}
	}

	if err := accountauth.Delete(paths); err != nil {
		return err
	}
	for _, path := range []string{paths.SyncStateFile(), paths.SyncDirtyFile()} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func revokeRemoteSession(creds AccountCredentials) {
	if strings.TrimSpace(creds.ServerURL) == "" || strings.TrimSpace(creds.RefreshToken) == "" {
		return
	}

	body, _ := json.Marshal(map[string]string{
		"refresh_token": creds.RefreshToken,
	})

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(creds.ServerURL, "/")+"/api/v1/auth/logout", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func BeginLogin(server string, openBrowser func(string)) (LoginSession, error) {
	return BeginLoginWithProgress(server, openBrowser, nil)
}

func BeginLoginWithProgress(server string, openBrowser func(string), progress func(LoginProgress)) (LoginSession, error) {
	code, err := randomHex(16)
	if err != nil {
		return LoginSession{}, fmt.Errorf("Generating code: %w", err)
	}

	verification, err := randomHex(2)
	if err != nil {
		return LoginSession{}, fmt.Errorf("Generating verification: %w", err)
	}
	codeVerifier, err := randomVerifier(32)
	if err != nil {
		return LoginSession{}, fmt.Errorf("Generating code verifier: %w", err)
	}

	payload, _ := json.Marshal(map[string]string{
		"code":             code,
		"verification":     verification,
		"code_challenge":   accountauth.CodeChallengeS256(codeVerifier),
		"challenge_method": "S256",
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
			return pollLogin(ctx, server, code, pollURL, codeVerifier)
		},
	}, nil
}

func pollLogin(ctx context.Context, server, code, pollURL, codeVerifier string) (AccountCredentials, error) {
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
			Name   string `json:"name"`
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
				Name:      strings.TrimSpace(result.Name),
			}, nil
		case "approved":
			return exchangeLogin(ctx, server, code, codeVerifier)
		case "error":
			return AccountCredentials{}, fmt.Errorf("Authentication failed: %s", result.Error)
		case "pending":
			continue
		}
	}

	return AccountCredentials{}, fmt.Errorf("Timed out while waiting to log in")
}

func exchangeLogin(ctx context.Context, server, code, codeVerifier string) (AccountCredentials, error) {
	payload, _ := json.Marshal(map[string]string{
		"code_verifier": codeVerifier,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server+"/api/v1/auth/sessions/"+code+"/exchange", bytes.NewReader(payload))
	if err != nil {
		return AccountCredentials{}, fmt.Errorf("Creating auth exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return AccountCredentials{}, fmt.Errorf("Exchanging approved session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readLoginErrorBody(resp.Body)
		if body == "" {
			body = resp.Status
		}
		return AccountCredentials{}, fmt.Errorf("Authentication failed: %s", body)
	}

	var result struct {
		AccessToken      string `json:"access_token"`
		AccessExpiresAt  string `json:"access_expires_at"`
		RefreshToken     string `json:"refresh_token"`
		RefreshExpiresAt string `json:"refresh_expires_at"`
		UserID           string `json:"user_id"`
		Email            string `json:"email"`
		Name             string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return AccountCredentials{}, fmt.Errorf("Decoding auth exchange response: %w", err)
	}

	accessExpiry, err := time.Parse(time.RFC3339, strings.TrimSpace(result.AccessExpiresAt))
	if err != nil {
		return AccountCredentials{}, fmt.Errorf("Parsing access expiry: %w", err)
	}
	refreshExpiry, err := time.Parse(time.RFC3339, strings.TrimSpace(result.RefreshExpiresAt))
	if err != nil {
		return AccountCredentials{}, fmt.Errorf("Parsing refresh expiry: %w", err)
	}

	return AccountCredentials{
		ServerURL:        server,
		Token:            result.AccessToken,
		AccessToken:      result.AccessToken,
		AccessExpiresAt:  accessExpiry,
		RefreshToken:     result.RefreshToken,
		RefreshExpiresAt: refreshExpiry,
		UserID:           result.UserID,
		Email:            result.Email,
		Name:             strings.TrimSpace(result.Name),
	}, nil
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
	backoffSchedule := []time.Duration{
		1 * time.Second,
		3 * time.Second,
		7 * time.Second,
	}
	maxAttempts := len(backoffSchedule) + 1

	client := &http.Client{Timeout: 15 * time.Second}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest(http.MethodPost, server+"/api/v1/auth/sessions", bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("Creating auth session request: %w", err)
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

		lastErr = loginAttemptError(err, statusCode, responseBody, attempt)
		logLoginAttempt("auth session create failed", attempt, server, statusCode, responseBody, err)

		if attempt == maxAttempts || !shouldRetryLoginAttempt(err, statusCode) {
			break
		}

		delay := backoffSchedule[attempt-1]
		if progress != nil {
			progress(LoginProgress{
				Status: fmt.Sprintf("Attempt %d failed. Retrying in %s (%d/%d)", attempt, humanizeDuration(delay), attempt+1, maxAttempts),
			})
		}
		time.Sleep(delay)
	}

	return nil, lastErr
}

func shouldRetryLoginAttempt(err error, statusCode int) bool {
	if err != nil {
		return true
	}
	switch statusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return statusCode >= 500
	}
}

func loginAttemptError(err error, statusCode int, responseBody string, attempt int) error {
	suffix := fmt.Sprintf(" after %d attempts", attempt)
	if err != nil {
		return fmt.Errorf("Could not reach server: %v%s", err, suffix)
	}
	if trimmed := strings.TrimSpace(responseBody); trimmed != "" {
		return fmt.Errorf("%s%s", sentenceCase(trimmed), suffix)
	}
	return fmt.Errorf("Could not create auth session (status %d)%s", statusCode, suffix)
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

func humanizeDuration(delay time.Duration) string {
	seconds := int(delay.Seconds())
	if seconds < 60 {
		if seconds == 1 {
			return "1 second"
		}
		return fmt.Sprintf("%d seconds", seconds)
	}
	minutes := seconds / 60
	if minutes == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", minutes)
}

func sentenceCase(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) == 0 || !unicode.IsLower(runes[0]) {
		return trimmed
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func randomVerifier(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
