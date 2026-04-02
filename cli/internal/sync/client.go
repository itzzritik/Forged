package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	ServerURL  string
	Token      string
	DeviceID   string
	HTTPClient *http.Client
}

func NewClient(serverURL, token, deviceID string) *Client {
	return &Client{
		ServerURL: serverURL,
		Token:     token,
		DeviceID:  deviceID,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type PushResult struct {
	Version int64 `json:"version"`
}

func (c *Client) Push(blob []byte, expectedVersion int64) (PushResult, error) {
	req, err := http.NewRequest("POST", c.ServerURL+"/api/v1/sync/push", bytes.NewReader(blob))
	if err != nil {
		return PushResult{}, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Vault-Version", strconv.FormatInt(expectedVersion, 10))
	req.Header.Set("X-Device-ID", c.DeviceID)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return PushResult{}, fmt.Errorf("push request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return PushResult{}, fmt.Errorf("version conflict: vault was updated by another device")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return PushResult{}, fmt.Errorf("push failed (%d): %s", resp.StatusCode, string(body))
	}

	var result PushResult
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

type PullResult struct {
	Blob    []byte
	Version int64
}

func (c *Client) Pull() (PullResult, error) {
	req, err := http.NewRequest("GET", c.ServerURL+"/api/v1/sync/pull", nil)
	if err != nil {
		return PullResult{}, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("X-Device-ID", c.DeviceID)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return PullResult{}, fmt.Errorf("pull request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return PullResult{}, fmt.Errorf("no vault on server")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return PullResult{}, fmt.Errorf("pull failed (%d): %s", resp.StatusCode, string(body))
	}

	blob, err := io.ReadAll(resp.Body)
	if err != nil {
		return PullResult{}, fmt.Errorf("reading response: %w", err)
	}

	version, _ := strconv.ParseInt(resp.Header.Get("X-Vault-Version"), 10, 64)

	return PullResult{Blob: blob, Version: version}, nil
}

type StatusResult struct {
	HasVault  bool   `json:"has_vault"`
	Version   int64  `json:"version"`
	UpdatedAt string `json:"updated_at"`
}

func (c *Client) Status() (StatusResult, error) {
	req, err := http.NewRequest("GET", c.ServerURL+"/api/v1/sync/status", nil)
	if err != nil {
		return StatusResult{}, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return StatusResult{}, fmt.Errorf("status request failed: %w", err)
	}
	defer resp.Body.Close()

	var result StatusResult
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

type AuthResult struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}

func Register(serverURL, email, password string) (AuthResult, error) {
	return authRequest(serverURL+"/api/v1/auth/register", email, password)
}

func Login(serverURL, email, password string) (AuthResult, error) {
	return authRequest(serverURL+"/api/v1/auth/login", email, password)
}

func authRequest(url, email, password string) (AuthResult, error) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return AuthResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		msg := errResp["error"]
		if msg == "" {
			msg = fmt.Sprintf("request failed with status %d", resp.StatusCode)
		}
		return AuthResult{}, fmt.Errorf("%s", msg)
	}

	var result AuthResult
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}
