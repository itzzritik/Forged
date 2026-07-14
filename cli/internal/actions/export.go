package actions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
)

type ExportResult struct {
	Path     string
	KeyCount int
}

type exportedKey struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	PrivateKey  string `json:"private_key"`
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment"`
	GitSigning  bool   `json:"git_signing"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func DefaultExportPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Sprintf("forged-export-%s.json", time.Now().Format("2006-01-02"))
	}
	return filepath.Join(home, "Desktop", fmt.Sprintf("forged-export-%s.json", time.Now().Format("2006-01-02")))
}

func AuthorizeExport(paths config.Paths, password []byte) (string, error) {
	authResult, err := authorizeSensitiveResult(paths, sensitiveauth.ActionExport, password)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(authResult.ExportToken) == "" {
		return "", fmt.Errorf("Export authorization did not return a token")
	}
	return strings.TrimSpace(authResult.ExportToken), nil
}

func ExportVaultWithToken(paths config.Paths, outPath string, token string) (ExportResult, error) {
	outPath = strings.TrimSpace(outPath)
	if outPath == "" {
		return ExportResult{}, fmt.Errorf("Enter an export path")
	}
	outPath = expandUserPath(outPath)
	if strings.TrimSpace(token) == "" {
		return ExportResult{}, &SensitiveAuthRequiredError{Prompt: sensitiveauth.ActionExport.PasswordPrompt()}
	}

	client := ipc.NewClient(paths.CtlSocket())
	resp, err := client.Call(ipc.CmdExportAll, map[string]string{"token": strings.TrimSpace(token)})
	if err != nil {
		if strings.Contains(err.Error(), "sensitive export requires fresh authentication") {
			return ExportResult{}, &SensitiveAuthRequiredError{Prompt: sensitiveauth.ActionExport.PasswordPrompt()}
		}
		return ExportResult{}, err
	}
	defer clear(resp.Data)

	var keys []exportedKey
	if err := json.Unmarshal(resp.Data, &keys); err != nil {
		return ExportResult{}, fmt.Errorf("Parsing export payload: %w", err)
	}

	items := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		items = append(items, map[string]any{
			"type": "ssh_key",
			"name": key.Name,
			"ssh_key": map[string]any{
				"private_key": key.PrivateKey,
				"public_key":  key.PublicKey,
				"fingerprint": key.Fingerprint,
				"key_type":    key.Type,
				"comment":     key.Comment,
				"git_signing": key.GitSigning,
			},
			"created_at": key.CreatedAt,
			"updated_at": key.UpdatedAt,
		})
	}

	export := map[string]any{
		"format":      "forged-export",
		"version":     1,
		"exported_at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"items":       items,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		clear(data)
		return ExportResult{}, fmt.Errorf("Marshaling export: %w", err)
	}
	defer clear(data)
	if err := writePrivateExport(outPath, data); err != nil {
		return ExportResult{}, fmt.Errorf("Writing export file: %w", err)
	}

	return ExportResult{Path: outPath, KeyCount: len(keys)}, nil
}

func writePrivateExport(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".forged-export-*")
	if err != nil {
		return fmt.Errorf("Creating temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
		}
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("Setting private permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("Writing temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("Syncing temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("Closing temporary file: %w", err)
	}
	closed = true
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("Replacing destination: %w", err)
	}
	tmpPath = ""
	return nil
}
