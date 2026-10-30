package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// GitSigningKeys are the global git settings Forged signing sets, saves and restores.
var GitSigningKeys = []string{"user.signingkey", "gpg.format", "gpg.ssh.program", "gpg.ssh.allowedSignersFile", "commit.gpgsign"}

// RestoreGitSigning puts back the setup saved when Forged signing was turned on, if Forged's
// signing is still the one configured.
func RestoreGitSigning(paths Paths) (bool, error) {
	data, err := os.ReadFile(paths.GitSigningBackupFile())
	if err != nil {
		return false, nil
	}
	if program := GlobalGitConfigValue("gpg.ssh.program"); program == nil || !IsForgedSignProgram(*program) {
		return false, nil
	}
	var previous map[string]*string
	if err := json.Unmarshal(data, &previous); err != nil {
		return false, nil
	}
	for _, key := range GitSigningKeys {
		if err := SetGlobalGitConfig(key, previous[key]); err != nil {
			return false, err
		}
	}
	_ = os.Remove(paths.GitSigningBackupFile())
	return true, nil
}

// RemoveGitSigning takes Forged out of global git signing for uninstall, so
// git commit keeps working and nothing points at a deleted forged-sign.
func RemoveGitSigning(paths Paths) error {
	if program := GlobalGitConfigValue("gpg.ssh.program"); program == nil || !IsForgedSignProgram(*program) {
		return nil
	}
	if restored, err := RestoreGitSigning(paths); err != nil || restored {
		return err
	}
	for _, key := range []string{"gpg.ssh.program", "commit.gpgsign"} {
		if err := SetGlobalGitConfig(key, nil); err != nil {
			return err
		}
	}
	return nil
}

// GlobalGitConfigValue is nil when key is unset.
func GlobalGitConfigValue(key string) *string {
	out, err := exec.Command("git", "config", "--global", "--get", key).Output()
	if err != nil {
		return nil
	}
	value := strings.TrimRight(string(out), "\r\n")
	return &value
}

// SetGlobalGitConfig sets key, or unsets it when value is nil.
func SetGlobalGitConfig(key string, value *string) error {
	args := []string{"config", "--global", "--unset-all", key}
	if value != nil {
		args = []string{"config", "--global", "--replace-all", key, *value}
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	var exit *exec.ExitError
	// git exits 5 when unsetting a key that is not set.
	if value == nil && errors.As(err, &exit) && exit.ExitCode() == 5 {
		return nil
	}
	if err != nil {
		return fmt.Errorf("Running git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

// IsForgedSignProgram reports whether a gpg.ssh.program value is forged-sign.
func IsForgedSignProgram(program string) bool {
	switch filepath.Base(strings.TrimSpace(program)) {
	case "forged-sign", "forged-sign.exe":
		return true
	}
	return false
}

// GitSigningProgramStale reports whether global git signs through a
// forged-sign other than installed, e.g. one inside an npm install that a
// Node switch or reinstall can remove.
func GitSigningProgramStale(installed string) bool {
	program := globalGitConfig("gpg.ssh.program")
	return IsForgedSignProgram(program) && !samePath(program, installed)
}

// RepointGitSigningProgram points a Forged gpg.ssh.program at installed.
// Other signing programs are the user's choice and are left alone.
func RepointGitSigningProgram(installed string) error {
	if !GitSigningProgramStale(installed) {
		return nil
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return err
	}
	if out, err := exec.Command(git, "config", "--global", "gpg.ssh.program", installed).CombinedOutput(); err != nil {
		return fmt.Errorf("setting gpg.ssh.program: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func globalGitConfig(key string) string {
	git, err := exec.LookPath("git")
	if err != nil {
		return ""
	}
	out, err := exec.Command(git, "config", "--global", "--get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(strings.TrimSpace(a)), filepath.Clean(strings.TrimSpace(b))
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
