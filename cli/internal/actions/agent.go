package actions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
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
	signingKey := strings.TrimSpace(gitGlobalConfig("user.signingkey"))
	gpgFormat := strings.ToLower(strings.TrimSpace(gitGlobalConfig("gpg.format")))
	signProgram := strings.TrimSpace(gitGlobalConfig("gpg.ssh.program"))
	commitSign := strings.ToLower(strings.TrimSpace(gitGlobalConfig("commit.gpgsign")))

	if !gitBoolEnabled(commitSign) || signingKey == "" {
		return CommitSigningStatus{Mode: CommitSigningOff}, nil
	}

	status := CommitSigningStatus{
		Mode:      CommitSigningExternal,
		PublicKey: signingKey,
		Program:   signProgram,
	}

	if signProgram == "" || (gpgFormat != "" && gpgFormat != "ssh") {
		return status, nil
	}
	if !isForgedSigningProgram(signProgram) {
		return status, nil
	}

	status.Mode = CommitSigningForged
	match, err := matchForgedSigningKey(paths, signingKey)
	if err != nil {
		return status, nil
	}
	if match == nil {
		return status, nil
	}

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
	for _, args := range [][]string{
		{"git", "config", "--global", "--unset", "user.signingkey"},
		{"git", "config", "--global", "--unset", "gpg.format"},
		{"git", "config", "--global", "--unset", "gpg.ssh.program"},
		{"git", "config", "--global", "--unset", "commit.gpgsign"},
	} {
		_ = exec.Command(args[0], args[1:]...).Run()
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
		return nil, err
	}

	for _, key := range keys {
		exported, err := ExportPublicKey(paths, key.Name)
		if err != nil {
			continue
		}
		if !samePublicKey(exported.PublicKey, publicKey) {
			continue
		}
		return &matchedSigningKey{
			Name:        exported.Name,
			Fingerprint: exported.Fingerprint,
			PublicKey:   exported.PublicKey,
		}, nil
	}

	return nil, nil
}

func samePublicKey(left string, right string) bool {
	return strings.TrimSpace(left) == strings.TrimSpace(right)
}

func gitGlobalConfig(key string) string {
	out, err := exec.Command("git", "config", "--global", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitBoolEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "on", "1":
		return true
	default:
		return false
	}
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
			return fmt.Errorf("running %v: %s: %w", args, string(out), err)
		}
	}
	return nil
}
