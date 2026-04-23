package accountauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
)

var ErrLoginRequired = errors.New("log-in required")

type Credentials struct {
	ServerURL        string    `json:"server_url"`
	Token            string    `json:"token,omitempty"`
	AccessToken      string    `json:"access_token,omitempty"`
	AccessExpiresAt  time.Time `json:"access_expires_at,omitempty"`
	RefreshToken     string    `json:"refresh_token,omitempty"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at,omitempty"`
	UserID           string    `json:"user_id"`
	Email            string    `json:"email"`
	Name             string    `json:"name,omitempty"`
}

func CredentialsPath(paths config.Paths) string {
	return paths.CredentialsFile()
}

func Load(paths config.Paths) (Credentials, error) {
	data, err := os.ReadFile(CredentialsPath(paths))
	if err != nil {
		return Credentials{}, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return Credentials{}, err
	}

	normalizeCredentials(&creds)
	return creds, nil
}

func Save(paths config.Paths, creds Credentials) error {
	path := CredentialsPath(paths)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	normalizeCredentials(&creds)
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func EnsureFresh(ctx context.Context, paths config.Paths) (Credentials, error) {
	creds, err := Load(paths)
	if err != nil {
		return Credentials{}, err
	}
	if !NeedsRefresh(creds, time.Now()) {
		return creds, nil
	}
	if creds.RefreshToken == "" || (!creds.RefreshExpiresAt.IsZero() && time.Now().After(creds.RefreshExpiresAt)) {
		return Credentials{}, ErrLoginRequired
	}

	refreshed, err := refresh(ctx, creds)
	if err != nil {
		return Credentials{}, err
	}
	if err := Save(paths, refreshed); err != nil {
		return Credentials{}, fmt.Errorf("Saving refreshed credentials: %w", err)
	}
	return refreshed, nil
}

func NeedsRefresh(creds Credentials, now time.Time) bool {
	if creds.AccessToken == "" && creds.Token == "" {
		return true
	}
	if creds.AccessExpiresAt.IsZero() {
		return false
	}
	return !creds.AccessExpiresAt.After(now.Add(time.Minute))
}

func CurrentToken(creds Credentials) string {
	if strings.TrimSpace(creds.AccessToken) != "" {
		return strings.TrimSpace(creds.AccessToken)
	}
	return strings.TrimSpace(creds.Token)
}

func normalizeCredentials(creds *Credentials) {
	if creds == nil {
		return
	}
	creds.ServerURL = strings.TrimSpace(creds.ServerURL)
	creds.Email = strings.TrimSpace(creds.Email)
	creds.UserID = strings.TrimSpace(creds.UserID)
	creds.Name = strings.TrimSpace(creds.Name)
	creds.RefreshToken = strings.TrimSpace(creds.RefreshToken)

	if token := strings.TrimSpace(creds.AccessToken); token != "" {
		creds.AccessToken = token
		creds.Token = token
	} else {
		creds.Token = strings.TrimSpace(creds.Token)
		creds.AccessToken = creds.Token
	}

	if creds.Name == "" {
		if name := decodeAccountName(creds.AccessToken); name != "" {
			creds.Name = name
		} else {
			creds.Name = fallbackAccountName(creds.Email)
		}
	}
}

func refresh(ctx context.Context, creds Credentials) (Credentials, error) {
	body, _ := json.Marshal(map[string]string{
		"refresh_token": creds.RefreshToken,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(creds.ServerURL, "/")+"/api/v1/auth/refresh", bytes.NewReader(body))
	if err != nil {
		return Credentials{}, fmt.Errorf("Creating refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Credentials{}, fmt.Errorf("Refreshing credentials: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return Credentials{}, ErrLoginRequired
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return Credentials{}, fmt.Errorf("Refreshing credentials failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
		return Credentials{}, fmt.Errorf("Decoding refresh response: %w", err)
	}

	accessExpiry, err := parseOptionalTime(result.AccessExpiresAt)
	if err != nil {
		return Credentials{}, fmt.Errorf("Parsing access expiry: %w", err)
	}
	refreshExpiry, err := parseOptionalTime(result.RefreshExpiresAt)
	if err != nil {
		return Credentials{}, fmt.Errorf("Parsing refresh expiry: %w", err)
	}

	refreshed := Credentials{
		ServerURL:        creds.ServerURL,
		AccessToken:      result.AccessToken,
		Token:            result.AccessToken,
		AccessExpiresAt:  accessExpiry,
		RefreshToken:     result.RefreshToken,
		RefreshExpiresAt: refreshExpiry,
		UserID:           result.UserID,
		Email:            result.Email,
		Name:             result.Name,
	}
	normalizeCredentials(&refreshed)
	return refreshed, nil
}

func parseOptionalTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

func decodeAccountName(token string) string {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) < 2 {
		return ""
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	var claims struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return strings.TrimSpace(claims.Name)
}

func fallbackAccountName(email string) string {
	local := strings.TrimSpace(email)
	if local == "" {
		return ""
	}
	if at := strings.Index(local, "@"); at > 0 {
		local = local[:at]
	}
	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	local = strings.ReplaceAll(local, "-", " ")
	words := strings.Fields(local)
	if len(words) == 0 {
		return ""
	}
	for index, word := range words {
		runes := []rune(strings.ToLower(word))
		if len(runes) == 0 {
			continue
		}
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		words[index] = string(runes)
	}
	return strings.Join(words, " ")
}

func CodeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
