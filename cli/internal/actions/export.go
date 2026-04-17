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

func ExportVault(paths config.Paths, outPath string, password []byte) (ExportResult, error) {
	outPath = strings.TrimSpace(outPath)
	if outPath == "" {
		return ExportResult{}, fmt.Errorf("enter an export path")
	}
	outPath = expandUserPath(outPath)

	authResult, err := authorizeSensitiveResult(paths, sensitiveauth.ActionExport, password)
	if err != nil {
		return ExportResult{}, err
	}
	if strings.TrimSpace(authResult.ExportToken) == "" {
		return ExportResult{}, fmt.Errorf("export authorization did not return a token")
	}

	client := ipc.NewClient(paths.CtlSocket())
	resp, err := client.Call(ipc.CmdExportAll, map[string]string{"token": authResult.ExportToken})
	if err != nil {
		return ExportResult{}, err
	}

	var keys []exportedKey
	if err := json.Unmarshal(resp.Data, &keys); err != nil {
		return ExportResult{}, fmt.Errorf("parsing export payload: %w", err)
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
		return ExportResult{}, fmt.Errorf("marshaling export: %w", err)
	}
	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		return ExportResult{}, fmt.Errorf("writing export file: %w", err)
	}

	return ExportResult{Path: outPath, KeyCount: len(keys)}, nil
}
