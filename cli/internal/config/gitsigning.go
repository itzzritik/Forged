package config

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

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
