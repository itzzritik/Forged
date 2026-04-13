package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/itzzritik/forged/cli/internal/importers"
)

func findSignBinary() (string, error) {
	if path, err := exec.LookPath("forged-sign"); err == nil {
		return path, nil
	}
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find forged-sign binary")
	}
	candidate := filepath.Join(filepath.Dir(self), "forged-sign")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("forged-sign not found in PATH or next to forged binary")
}

func writeAllowedSigners(publicKey string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	signerFile := filepath.Join(home, ".ssh", "allowed_signers")

	if data, err := os.ReadFile(signerFile); err == nil {
		if strings.Contains(string(data), publicKey) {
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(signerFile), 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(signerFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "* %s\n", publicKey)
	return err
}

func deriveKeyName(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.TrimPrefix(name, "id_")
	name = strings.TrimPrefix(name, "id-")
	name = strings.ReplaceAll(name, "_", " ")
	name = importers.SanitizeName(name)
	if name == "" {
		name = "Default"
	}
	return name
}

func applyGitSigningConfig(publicKey, signPath string) error {
	cmds := [][]string{
		{"git", "config", "--global", "user.signingkey", publicKey},
		{"git", "config", "--global", "gpg.format", "ssh"},
		{"git", "config", "--global", "gpg.ssh.program", signPath},
		{"git", "config", "--global", "commit.gpgsign", "true"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("running %v: %s: %w", args, string(out), err)
		}
	}
	return nil
}
