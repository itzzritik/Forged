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
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/accountauth"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/ipc"
)

type AccountCredentials = accountauth.Credentials

var (
	ErrAccountChangeSyncCleanupPending             = errors.New("account saved, but sync cleanup is pending")
	ErrAccountChangeCredentialSecretCleanupPending = errors.New("account saved, but credential secret cleanup is pending")
	ErrAccountChangeCommittedUnconfirmed           = errors.New("account saved, but local cleanup could not be confirmed")
	ErrAccountClearSyncCleanupPending              = errors.New("account cleared, but sync cleanup is pending")
	ErrAccountClearCommittedUnconfirmed            = errors.New("account cleared, but local cleanup could not be confirmed")
	ErrAccountClearCredentialSecretCleanupPending  = errors.New("account cleared, but credential secret cleanup is pending")
)

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
	if err := accountauth.ValidateCredentials(creds); err != nil {
		return err
	}
	changeID, err := randomHex(16)
	if err != nil {
		return fmt.Errorf("Preparing account change: %w", err)
	}
	creds.ChangeID = changeID
	args := accountCredentialsArgs(creds)
	resp, err := callDaemonAccountCommand(paths, ipc.CmdAccountReplace, args)
	if err != nil {
		if saved, loadErr := accountauth.Load(paths); loadErr == nil && saved.ChangeID == creds.ChangeID {
			return ErrAccountChangeCommittedUnconfirmed
		}
		return err
	}
	result, resultErr := accountChangeResult(resp)
	if resultErr != nil {
		if saved, loadErr := accountauth.Load(paths); loadErr == nil && saved.ChangeID == creds.ChangeID {
			return ErrAccountChangeCommittedUnconfirmed
		}
		return resultErr
	}
	var cleanupErr error
	if result.SyncCleanupPending {
		cleanupErr = errors.Join(cleanupErr, ErrAccountChangeSyncCleanupPending)
	}
	if result.CredentialSecretCleanupPending {
		cleanupErr = errors.Join(cleanupErr, ErrAccountChangeCredentialSecretCleanupPending)
	}
	return cleanupErr
}

func accountCredentialsArgs(creds AccountCredentials) ipc.AccountCredentialsArgs {
	return ipc.AccountCredentialsArgs{
		ServerURL:        creds.ServerURL,
		Token:            creds.Token,
		AccessToken:      creds.AccessToken,
		AccessExpiresAt:  creds.AccessExpiresAt,
		RefreshToken:     creds.RefreshToken,
		RefreshExpiresAt: creds.RefreshExpiresAt,
		UserID:           creds.UserID,
		Email:            creds.Email,
		Name:             creds.Name,
		ChangeID:         creds.ChangeID,
	}
}

func ClearCredentials(paths config.Paths) error {
	resp, err := callDaemonAccountCommand(paths, ipc.CmdAccountClear, nil)
	if err != nil {
		if credentialsCleared(paths) {
			return ErrAccountClearCommittedUnconfirmed
		}
		return err
	}
	result, err := accountChangeResult(resp)
	if err != nil {
		if credentialsCleared(paths) {
			return ErrAccountClearCommittedUnconfirmed
		}
		return err
	}
	if !credentialsCleared(paths) {
		return fmt.Errorf("daemon reported logout but credentials remain")
	}
	var cleanupErr error
	if result.SyncCleanupPending {
		cleanupErr = errors.Join(cleanupErr, ErrAccountClearSyncCleanupPending)
	}
	if result.CredentialSecretCleanupPending {
		cleanupErr = errors.Join(cleanupErr, ErrAccountClearCredentialSecretCleanupPending)
	}
	return cleanupErr
}

func accountChangeResult(resp ipc.Response) (ipc.AccountChangeResult, error) {
	if resp.Status != "ok" {
		return ipc.AccountChangeResult{}, fmt.Errorf("unexpected account change response")
	}
	var result ipc.AccountChangeResult
	if len(resp.Data) == 0 {
		return result, nil
	}
	if bytes.Equal(bytes.TrimSpace(resp.Data), []byte("null")) {
		return ipc.AccountChangeResult{}, fmt.Errorf("unexpected account change response")
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return ipc.AccountChangeResult{}, fmt.Errorf("Reading account change result: %w", err)
	}
	return result, nil
}

func credentialsCleared(paths config.Paths) bool {
	_, err := accountauth.Load(paths)
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, accountauth.ErrLoginRequired)
}

func callDaemonAccountCommand(paths config.Paths, command string, args any) (ipc.Response, error) {
	if err := ensureAccountDaemon(paths); err != nil {
		return ipc.Response{}, err
	}
	resp, err := ipc.NewClient(paths.CtlSocket()).CallWithTimeout(command, args, time.Minute)
	if err != nil {
		return resp, fmt.Errorf("Changing daemon account: %w", err)
	}
	return resp, nil
}

func ensureAccountDaemon(paths config.Paths) error {
	if protocol, err := runningAccountProtocol(paths); err == nil && protocol >= ipc.AccountChangeProtocol {
		return nil
	}
	runtimeSpec, err := daemon.DefaultRuntimeSpec()
	if err != nil {
		return fmt.Errorf("Finding current daemon: %w", err)
	}
	if err := daemon.EnsureService(paths, runtimeSpec); err != nil {
		return fmt.Errorf("Starting current daemon: %w", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if protocol, err := runningAccountProtocol(paths); err == nil && protocol >= ipc.AccountChangeProtocol {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("Current daemon did not become ready; run `forged doctor` and try again")
}

func runningAccountProtocol(paths config.Paths) (int, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).CallWithTimeout(ipc.CmdStatus, nil, 3*time.Second)
	if err != nil {
		return 0, err
	}
	var status struct {
		Protocol int `json:"account_change_protocol"`
	}
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return 0, err
	}
	return status.Protocol, nil
}

func BeginLogin(server string, openBrowser func(string)) (LoginSession, error) {
	return BeginLoginWithProgressContext(context.Background(), server, openBrowser, nil)
}

func BeginLoginWithProgress(server string, openBrowser func(string), progress func(LoginProgress)) (LoginSession, error) {
	return BeginLoginWithProgressContext(context.Background(), server, openBrowser, progress)
}

func BeginLoginWithProgressContext(ctx context.Context, server string, openBrowser func(string), progress func(LoginProgress)) (LoginSession, error) {
	if err := ctx.Err(); err != nil {
		return LoginSession{}, err
	}
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

	resp, err := createAuthSessionWithRetry(ctx, server, payload, progress)
	if err != nil {
		return LoginSession{}, err
	}
	resp.Body.Close()

	authURL := ipc.DefaultWebApp + "/login?code=" + code
	if err := ctx.Err(); err != nil {
		return LoginSession{}, err
	}
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
	client := &http.Client{Timeout: 15 * time.Second}

	for time.Now().Before(deadline) {
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return AccountCredentials{}, ctx.Err()
		case <-timer.C:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
		if err != nil {
			return AccountCredentials{}, fmt.Errorf("Creating login status request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return AccountCredentials{}, ctx.Err()
			}
			interval = min(interval*2, 10*time.Second)
			continue
		}

		var result struct {
			Status string `json:"status"`
			Token  string `json:"token"`
			UserID string `json:"user_id"`
			Email  string `json:"email"`
			Name   string `json:"name"`
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
			return AccountCredentials{}, fmt.Errorf("Authentication failed")
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
		return AccountCredentials{}, fmt.Errorf("Authentication failed (status %d)", resp.StatusCode)
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

func createAuthSessionWithRetry(ctx context.Context, server string, payload []byte, progress func(LoginProgress)) (*http.Response, error) {
	backoffSchedule := []time.Duration{
		1 * time.Second,
		3 * time.Second,
		7 * time.Second,
	}
	maxAttempts := len(backoffSchedule) + 1

	client := &http.Client{Timeout: 15 * time.Second}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server+"/api/v1/auth/sessions", bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("Creating auth session request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if ctx.Err() != nil {
			if resp != nil {
				resp.Body.Close()
			}
			return nil, ctx.Err()
		}
		if err == nil && resp.StatusCode == http.StatusCreated {
			if attempt > 1 {
				logLoginAttempt("auth session created after retry", attempt, server, http.StatusCreated, nil)
			}
			return resp, nil
		}

		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
			resp.Body.Close()
		}

		lastErr = loginAttemptError(err, statusCode, attempt)
		logLoginAttempt("auth session create failed", attempt, server, statusCode, err)

		if attempt == maxAttempts || !shouldRetryLoginAttempt(err, statusCode) {
			break
		}

		delay := backoffSchedule[attempt-1]
		if progress != nil {
			progress(LoginProgress{
				Status: fmt.Sprintf("Attempt %d failed. Retrying in %s (%d/%d)", attempt, humanizeDuration(delay), attempt+1, maxAttempts),
			})
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
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

func loginAttemptError(err error, statusCode int, attempt int) error {
	suffix := fmt.Sprintf(" after %d attempts", attempt)
	if err != nil {
		return fmt.Errorf("Could not reach server: %v%s", err, suffix)
	}
	return fmt.Errorf("Could not create auth session (status %d)%s", statusCode, suffix)
}

func logLoginAttempt(event string, attempt int, server string, statusCode int, err error) {
	paths := config.DefaultPaths()
	line := fmt.Sprintf("%s login %s attempt=%d server=%s", time.Now().Format(time.RFC3339), event, attempt, sanitizeDiagnosticError(server))
	if statusCode > 0 {
		line += fmt.Sprintf(" status=%d", statusCode)
	}
	if err != nil {
		line += fmt.Sprintf(" error=%q", sanitizeDiagnosticError(err.Error()))
	}
	_ = appendTUILogLine(paths, line+"\n")
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
