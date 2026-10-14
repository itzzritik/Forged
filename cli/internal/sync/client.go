package sync

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type Client struct {
	ServerURL   string
	Token       string
	TokenSource func(context.Context) (string, error)
	DeviceID    string
	HTTPClient  *http.Client
	// OnUnauthorized lets the token owner force a refresh after the server refuses a token.
	OnUnauthorized func(token string)
}

var _ API = (*Client)(nil)

var (
	ErrNoRemoteVault = errors.New("No vault on server")
	// ErrRemoteKeyMismatch is a remote vault this device's vault key cannot open; retrying cannot fix it.
	ErrRemoteKeyMismatch = errors.New("The vault saved in your account cannot be opened with this device's vault key")
	ErrVersionConflict   = errors.New("Version conflict")
	ErrStatusUnsupported = errors.New("Status unsupported")
)

const (
	maxSyncPullResponseBytes    = 64 << 20
	maxSyncControlResponseBytes = 64 << 10
	maxSyncErrorBodyBytes       = 8 << 10
)

func readBoundedSyncResponse(body io.Reader, limit int64) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, false, err
	}
	return data, int64(len(data)) > limit, nil
}

func decodeSyncResponse(body io.Reader, limit int64, result any) error {
	data, oversized, err := readBoundedSyncResponse(body, limit)
	if err != nil {
		return err
	}
	if oversized {
		return fmt.Errorf("response body exceeds %d byte limit", limit)
	}
	return json.NewDecoder(bytes.NewReader(data)).Decode(result)
}

func syncErrorBody(body io.Reader) string {
	data, oversized, err := readBoundedSyncResponse(body, maxSyncErrorBodyBytes)
	if err != nil {
		return fmt.Sprintf("unable to read response: %v", err)
	}
	if oversized {
		return string(data[:maxSyncErrorBodyBytes]) + " (truncated)"
	}
	return string(data)
}

func validateServerVersion(operation string, version int64) error {
	if version <= 0 {
		return fmt.Errorf("Invalid %s response version %d", operation, version)
	}
	return nil
}

func NewClient(serverURL, token, deviceID string) *Client {
	return &Client{
		ServerURL: serverURL,
		Token:     token,
		DeviceID:  deviceID,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        5,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func NewClientWithTokenSource(serverURL, deviceID string, tokenSource func(context.Context) (string, error)) *Client {
	return &Client{
		ServerURL:   serverURL,
		TokenSource: tokenSource,
		DeviceID:    deviceID,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        5,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

type PushResult struct {
	Version int64 `json:"version"`
}

type kdfParamsJSON struct {
	Salt        string `json:"salt"`
	Time        uint32 `json:"time"`
	Memory      uint32 `json:"memory"`
	Parallelism uint8  `json:"parallelism"`
}

func (k kdfParamsJSON) params() (vault.KDFParams, error) {
	var kdf vault.KDFParams
	salt, err := base64.StdEncoding.DecodeString(k.Salt)
	if err != nil || len(salt) != vault.SaltSize {
		return kdf, fmt.Errorf("invalid remote KDF salt")
	}
	copy(kdf.Salt[:], salt)
	kdf.TimeCost, kdf.MemoryCost, kdf.Parallelism = k.Time, k.Memory, k.Parallelism
	return kdf, nil
}

func kdfToJSON(kdf vault.KDFParams) kdfParamsJSON {
	return kdfParamsJSON{
		Salt:        base64.StdEncoding.EncodeToString(kdf.Salt[:]),
		Time:        kdf.TimeCost,
		Memory:      kdf.MemoryCost,
		Parallelism: kdf.Parallelism,
	}
}

func (c *Client) Push(blob []byte, kdf vault.KDFParams, protectedKey string, expectedVersion int64) (PushResult, error) {
	return c.PushContext(context.Background(), blob, kdf, protectedKey, expectedVersion)
}

func (c *Client) PushContext(ctx context.Context, blob []byte, kdf vault.KDFParams, protectedKey string, expectedVersion int64) (PushResult, error) {
	body, _ := json.Marshal(map[string]any{
		"blob":                    base64.StdEncoding.EncodeToString(blob),
		"kdf_params":              kdfToJSON(kdf),
		"protected_symmetric_key": protectedKey,
		"expected_version":        expectedVersion,
		"device_id":               c.DeviceID,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", c.ServerURL+"/api/v1/sync/push", bytes.NewReader(body))
	if err != nil {
		return PushResult{}, err
	}

	token, err := c.authToken(ctx)
	if err != nil {
		return PushResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	c.setDeviceHeaders(req)

	resp, err := c.do(req, token)
	if err != nil {
		return PushResult{}, fmt.Errorf("Push request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return PushResult{}, fmt.Errorf("%w: vault was updated by another device", ErrVersionConflict)
	}

	if resp.StatusCode != http.StatusOK {
		return PushResult{}, fmt.Errorf("Push failed (%d): %s", resp.StatusCode, syncErrorBody(resp.Body))
	}

	var result PushResult
	if err := decodeSyncResponse(resp.Body, maxSyncControlResponseBytes, &result); err != nil {
		return PushResult{}, fmt.Errorf("Parsing push response: %w", err)
	}
	if err := validateServerVersion("push", result.Version); err != nil {
		return PushResult{}, err
	}
	return result, nil
}

func (c *Client) Rekey(kdf vault.KDFParams, protectedKey string) error {
	return c.RekeyContext(context.Background(), kdf, protectedKey)
}

func (c *Client) RekeyContext(ctx context.Context, kdf vault.KDFParams, protectedKey string) error {
	body, _ := json.Marshal(map[string]any{
		"kdf_params":              kdfToJSON(kdf),
		"protected_symmetric_key": protectedKey,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", c.ServerURL+"/api/v1/vault/rekey", bytes.NewReader(body))
	if err != nil {
		return err
	}
	token, err := c.authToken(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req, token)
	if err != nil {
		return fmt.Errorf("Rekey request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Rekey failed (%d): %s", resp.StatusCode, syncErrorBody(resp.Body))
	}
	return nil
}

type PullResult struct {
	Blob                  []byte
	Version               int64
	KDFParams             *kdfParamsJSON
	ProtectedSymmetricKey *string
}

func (c *Client) Pull() (PullResult, error) {
	return c.PullContext(context.Background())
}

func (c *Client) PullContext(ctx context.Context) (PullResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.ServerURL+"/api/v1/sync/pull", nil)
	if err != nil {
		return PullResult{}, err
	}

	token, err := c.authToken(ctx)
	if err != nil {
		return PullResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	c.setDeviceHeaders(req)

	resp, err := c.do(req, token)
	if err != nil {
		return PullResult{}, fmt.Errorf("Pull request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Only the server's own answer means no vault; a proxy or a wrong server URL also says 404.
		body := syncErrorBody(resp.Body)
		if strings.Contains(strings.ToLower(body), "no vault") {
			return PullResult{}, ErrNoRemoteVault
		}
		return PullResult{}, fmt.Errorf("Pull failed (404): %s", body)
	}

	if resp.StatusCode != http.StatusOK {
		return PullResult{}, fmt.Errorf("Pull failed (%d): %s", resp.StatusCode, syncErrorBody(resp.Body))
	}

	var jsonResp struct {
		Blob                  string         `json:"blob"`
		Version               int64          `json:"version"`
		KDFParams             *kdfParamsJSON `json:"kdf_params"`
		ProtectedSymmetricKey *string        `json:"protected_symmetric_key"`
	}
	if err := decodeSyncResponse(resp.Body, maxSyncPullResponseBytes, &jsonResp); err != nil {
		return PullResult{}, fmt.Errorf("Parsing pull response: %w", err)
	}
	if err := validateServerVersion("pull", jsonResp.Version); err != nil {
		return PullResult{}, err
	}

	blob, err := base64.StdEncoding.DecodeString(jsonResp.Blob)
	if err != nil {
		return PullResult{}, fmt.Errorf("Decoding blob: %w", err)
	}

	return PullResult{
		Blob:                  blob,
		Version:               jsonResp.Version,
		KDFParams:             jsonResp.KDFParams,
		ProtectedSymmetricKey: jsonResp.ProtectedSymmetricKey,
	}, nil
}

type StatusResult struct {
	HasVault              bool           `json:"has_vault"`
	Version               int64          `json:"version"`
	UpdatedAt             string         `json:"updated_at"`
	KDFParams             *kdfParamsJSON `json:"kdf_params,omitempty"`
	ProtectedSymmetricKey *string        `json:"protected_symmetric_key,omitempty"`
}

func (c *Client) Status() (StatusResult, error) {
	return c.StatusContext(context.Background())
}

func (c *Client) StatusContext(ctx context.Context) (StatusResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.ServerURL+"/api/v1/sync/status", nil)
	if err != nil {
		return StatusResult{}, err
	}

	token, err := c.authToken(ctx)
	if err != nil {
		return StatusResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	c.setDeviceHeaders(req)

	resp, err := c.do(req, token)
	if err != nil {
		return StatusResult{}, fmt.Errorf("Status request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return StatusResult{}, fmt.Errorf("Status failed (%d): %s", resp.StatusCode, syncErrorBody(resp.Body))
	}

	var jsonResp struct {
		HasVault              *bool          `json:"has_vault"`
		Version               int64          `json:"version"`
		UpdatedAt             string         `json:"updated_at"`
		KDFParams             *kdfParamsJSON `json:"kdf_params,omitempty"`
		ProtectedSymmetricKey *string        `json:"protected_symmetric_key,omitempty"`
	}
	if err := decodeSyncResponse(resp.Body, maxSyncControlResponseBytes, &jsonResp); err != nil {
		return StatusResult{}, fmt.Errorf("Parsing status response: %w", err)
	}
	if jsonResp.HasVault == nil {
		return StatusResult{}, fmt.Errorf("Parsing status response: missing has_vault")
	}
	result := StatusResult{
		HasVault:              *jsonResp.HasVault,
		Version:               jsonResp.Version,
		UpdatedAt:             jsonResp.UpdatedAt,
		KDFParams:             jsonResp.KDFParams,
		ProtectedSymmetricKey: jsonResp.ProtectedSymmetricKey,
	}
	if result.HasVault {
		if err := validateServerVersion("status", result.Version); err != nil {
			return StatusResult{}, err
		}
	} else if result.Version != 0 {
		return StatusResult{}, fmt.Errorf("Invalid status response version %d without a vault", result.Version)
	}
	return result, nil
}

// do sends req, and after a 401 has the token owner refresh once and retries: a slow local clock
// or a rotated server key can invalidate a token the client still trusts.
func (c *Client) do(req *http.Request, token string) (*http.Response, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusUnauthorized || c.OnUnauthorized == nil {
		return resp, err
	}
	resp.Body.Close()
	c.OnUnauthorized(token)
	fresh, err := c.authToken(req.Context())
	if err != nil {
		return nil, err
	}
	retry := req.Clone(req.Context())
	if req.GetBody != nil {
		if retry.Body, err = req.GetBody(); err != nil {
			return nil, err
		}
	}
	retry.Header.Set("Authorization", "Bearer "+fresh)
	return c.HTTPClient.Do(retry)
}

func (c *Client) authToken(ctx context.Context) (string, error) {
	if c.TokenSource != nil {
		token, err := c.TokenSource(ctx)
		if err != nil {
			return "", err
		}
		if token == "" {
			return "", fmt.Errorf("Missing account access token")
		}
		return token, nil
	}
	if c.Token == "" {
		return "", fmt.Errorf("Missing account access token")
	}
	return c.Token, nil
}
