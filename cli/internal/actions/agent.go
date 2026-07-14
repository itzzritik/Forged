package actions

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
	"golang.org/x/crypto/ssh"
)

type CommitSigningMode string

const (
	CommitSigningOff      CommitSigningMode = "off"
	CommitSigningForged   CommitSigningMode = "forged"
	CommitSigningExternal CommitSigningMode = "external"
)

type CommitSigningStatus struct {
	Mode        CommitSigningMode
	KeyName     string
	Fingerprint string
	PublicKey   string
	Program     string
}

func (s CommitSigningStatus) Enabled() bool {
	return s.Mode != CommitSigningOff
}

func LoadCommitSigningStatus(paths config.Paths) (CommitSigningStatus, error) {
	gitConfig, err := loadGlobalGitSigningConfig()
	if err != nil {
		return CommitSigningStatus{}, err
	}
	signingKey := strings.TrimSpace(gitConfig["user.signingkey"])
	gpgFormat := strings.ToLower(strings.TrimSpace(gitConfig["gpg.format"]))
	signProgram := strings.TrimSpace(gitConfig["gpg.ssh.program"])
	commitValue, commitPresent := gitConfig["commit.gpgsign"]
	commitSign, err := parseGitBool(commitValue, commitPresent)
	if err != nil {
		return CommitSigningStatus{}, fmt.Errorf("Reading commit.gpgsign: %w", err)
	}

	if !commitSign || signingKey == "" {
		return CommitSigningStatus{Mode: CommitSigningOff}, nil
	}

	status := CommitSigningStatus{
		Mode:      CommitSigningExternal,
		PublicKey: signingKey,
		Program:   signProgram,
	}

	if signProgram == "" || gpgFormat != "ssh" {
		return status, nil
	}
	if !isForgedSigningProgram(signProgram) {
		return status, nil
	}

	match, err := matchForgedSigningKey(paths, signingKey)
	if err != nil {
		return status, err
	}
	if match == nil {
		return status, fmt.Errorf("Configured signing key is not available in Forged")
	}

	status.Mode = CommitSigningForged
	status.KeyName = match.Name
	status.Fingerprint = match.Fingerprint
	status.PublicKey = match.PublicKey
	return status, nil
}

func EnableCommitSigning(paths config.Paths, keyName string) (CommitSigningStatus, error) {
	exported, err := ExportPublicKey(paths, keyName)
	if err != nil {
		return CommitSigningStatus{}, err
	}

	signPath, err := findSignBinary()
	if err != nil {
		return CommitSigningStatus{}, err
	}
	if err := applyGitSigningConfig(exported.PublicKey, signPath); err != nil {
		return CommitSigningStatus{}, err
	}
	if err := writeAllowedSigners(exported.PublicKey); err != nil {
		return CommitSigningStatus{}, err
	}

	return LoadCommitSigningStatus(paths)
}

func DisableCommitSigning(paths config.Paths) (CommitSigningStatus, error) {
	cmd := exec.Command("git", "config", "--global", "--replace-all", "commit.gpgsign", "false")
	if out, err := cmd.CombinedOutput(); err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return CommitSigningStatus{}, fmt.Errorf("Disabling commit signing: %s: %w", detail, err)
		}
		return CommitSigningStatus{}, fmt.Errorf("Disabling commit signing: %w", err)
	}
	return LoadCommitSigningStatus(paths)
}

func EnableSSHAgent(paths config.Paths) error {
	return config.EnableSSHAgent(paths)
}

func DisableSSHAgent(paths config.Paths) error {
	return config.DisableSSHAgent(paths)
}

type matchedSigningKey struct {
	Name        string
	Fingerprint string
	PublicKey   string
}

func matchForgedSigningKey(paths config.Paths, publicKey string) (*matchedSigningKey, error) {
	keys, err := ListKeys(paths)
	if err != nil {
		return nil, fmt.Errorf("Listing keys for commit signing: %w", err)
	}
	configuredKey, err := parseConfiguredSigningKey(publicKey)
	if err != nil {
		return nil, err
	}
	fingerprint := ssh.FingerprintSHA256(configuredKey)
	canonicalPublicKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(configuredKey)))

	for _, key := range keys {
		if strings.TrimSpace(key.Fingerprint) != fingerprint {
			continue
		}
		return &matchedSigningKey{
			Name:        key.Name,
			Fingerprint: key.Fingerprint,
			PublicKey:   canonicalPublicKey,
		}, nil
	}

	return nil, nil
}

func loadGlobalGitSigningConfig() (map[string]string, error) {
	out, err := exec.Command(
		"git", "config", "--global", "--null", "--get-regexp",
		"^(user\\.signingkey|gpg\\.format|gpg\\.ssh\\.program|commit\\.gpgsign)$",
	).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return map[string]string{}, nil
		}
		detail := ""
		if exitErr != nil {
			detail = strings.TrimSpace(string(exitErr.Stderr))
		}
		if detail != "" {
			return nil, fmt.Errorf("Reading global Git signing config: %s: %w", detail, err)
		}
		return nil, fmt.Errorf("Reading global Git signing config: %w", err)
	}

	values := make(map[string]string, 4)
	for _, record := range bytes.Split(out, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		key, value, hasValue := bytes.Cut(record, []byte{'\n'})
		normalizedKey := strings.ToLower(strings.TrimSpace(string(key)))
		if !hasValue && normalizedKey == "commit.gpgsign" {
			value = []byte("true")
		}
		values[normalizedKey] = strings.TrimSpace(string(value))
	}
	return values, nil
}

func parseConfiguredSigningKey(value string) (ssh.PublicKey, error) {
	value = strings.TrimSpace(value)
	if literal, ok := strings.CutPrefix(value, "key::"); ok {
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(strings.TrimSpace(literal)))
		if err != nil {
			return nil, fmt.Errorf("Parsing configured signing key: %w", err)
		}
		return key, nil
	}

	key, _, _, _, parseErr := ssh.ParseAuthorizedKey([]byte(value))
	if parseErr == nil {
		return key, nil
	}
	path := value
	if value == "~" || strings.HasPrefix(value, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil, fmt.Errorf("Reading configured signing key file %q: %w", path, readErr)
	}
	key, _, _, _, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return nil, fmt.Errorf("Parsing configured signing key file: %w", err)
	}
	return key, nil
}

func parseGitBool(value string, present bool) (bool, error) {
	if !present {
		return false, nil
	}
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "true", "yes", "on":
		return true, nil
	case "", "false", "no", "off":
		return false, nil
	}
	if strings.HasSuffix(value, "k") || strings.HasSuffix(value, "m") || strings.HasSuffix(value, "g") {
		value = value[:len(value)-1]
	}
	number, err := strconv.ParseInt(value, 0, 64)
	if err != nil {
		return false, fmt.Errorf("Invalid boolean value %q", value)
	}
	return number != 0, nil
}

func isForgedSigningProgram(program string) bool {
	program = strings.TrimSpace(program)
	if program == "" {
		return false
	}

	binary := filepath.Base(program)
	return binary == "forged-sign" || binary == "forged-sign.exe"
}

func findSignBinary() (string, error) {
	if path, err := exec.LookPath("forged-sign"); err == nil {
		return path, nil
	}

	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("Cannot find forged-sign binary")
	}

	candidate := filepath.Join(filepath.Dir(self), "forged-sign")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("Forged-sign not found in PATH or next to the Forged binary")
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

	if err := os.MkdirAll(filepath.Dir(signerFile), 0o700); err != nil {
		return err
	}

	file, err := os.OpenFile(signerFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "* %s\n", publicKey)
	return err
}

func applyGitSigningConfig(publicKey string, signPath string) error {
	for _, args := range [][]string{
		{"git", "config", "--global", "user.signingkey", publicKey},
		{"git", "config", "--global", "gpg.format", "ssh"},
		{"git", "config", "--global", "gpg.ssh.program", signPath},
		{"git", "config", "--global", "commit.gpgsign", "true"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("Running %v: %s: %w", args, string(out), err)
		}
	}
	return nil
}
