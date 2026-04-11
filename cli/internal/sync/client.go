package sync

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
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

func kdfToJSON(kdf vault.KDFParams) kdfParamsJSON {
	return kdfParamsJSON{
		Salt:        base64.StdEncoding.EncodeToString(kdf.Salt[:]),
		Time:        kdf.TimeCost,
		Memory:      kdf.MemoryCost,
		Parallelism: kdf.Parallelism,
	}
}

func (c *Client) Push(blob []byte, kdf vault.KDFParams, protectedKey string, masterPasswordHash string, expectedVersion int64) (PushResult, error) {
	body, _ := json.Marshal(map[string]any{
		"blob":                    base64.StdEncoding.EncodeToString(blob),
		"kdf_params":              kdfToJSON(kdf),
		"protected_symmetric_key": protectedKey,
		"master_password_hash":    masterPasswordHash,
		"expected_version":        expectedVersion,
		"device_id":               c.DeviceID,
	})

	req, err := http.NewRequest("POST", c.ServerURL+"/api/v1/sync/push", bytes.NewReader(body))
	if err != nil {
		return PushResult{}, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return PushResult{}, fmt.Errorf("push request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return PushResult{}, fmt.Errorf("version conflict: vault was updated by another device")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return PushResult{}, fmt.Errorf("push failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result PushResult
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func (c *Client) Rekey(oldMasterPasswordHash string, kdf vault.KDFParams, protectedKey string, masterPasswordHash string) error {
	body, _ := json.Marshal(map[string]any{
		"old_master_password_hash": oldMasterPasswordHash,
		"kdf_params":              kdfToJSON(kdf),
		"protected_symmetric_key": protectedKey,
		"master_password_hash":    masterPasswordHash,
	})

	req, err := http.NewRequest("POST", c.ServerURL+"/api/v1/vault/rekey", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("rekey request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("wrong password")
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("rekey failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

type PullResult struct {
	Blob      []byte
	Version   int64
	KDFParams *kdfParamsJSON
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
		respBody, _ := io.ReadAll(resp.Body)
		return PullResult{}, fmt.Errorf("pull failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var jsonResp struct {
		Blob      string         `json:"blob"`
		Version   int64          `json:"version"`
		KDFParams *kdfParamsJSON `json:"kdf_params"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		return PullResult{}, fmt.Errorf("parsing pull response: %w", err)
	}

	blob, err := base64.StdEncoding.DecodeString(jsonResp.Blob)
	if err != nil {
		return PullResult{}, fmt.Errorf("decoding blob: %w", err)
	}

	return PullResult{Blob: blob, Version: jsonResp.Version, KDFParams: jsonResp.KDFParams}, nil
}

type StatusResult struct {
	HasVault  bool           `json:"has_vault"`
	Version   int64          `json:"version"`
	UpdatedAt string         `json:"updated_at"`
	KDFParams *kdfParamsJSON `json:"kdf_params,omitempty"`
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
