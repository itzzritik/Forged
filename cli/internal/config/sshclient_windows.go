//go:build windows

package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Win32-OpenSSH falls back to this pipe, which 1Password also serves.
const defaultAgentEndpoint = `\.\pipe\openssh-ssh-agent`

// Avoid Git's MSYS ssh, which may come first on PATH.
func sshClientPath() string {
	if path, err := exec.LookPath("ssh"); err == nil && !isMSYSExecutable(path) {
		return path
	}
	if dir, err := windows.GetSystemDirectory(); err == nil {
		if system := filepath.Join(dir, "OpenSSH", "ssh.exe"); fileExists(system) {
			return system
		}
	}
	return "ssh"
}

func isMSYSExecutable(path string) bool {
	dir := filepath.Dir(path)
	return fileExists(filepath.Join(dir, "msys-2.0.dll")) || fileExists(filepath.Join(dir, "cygwin1.dll"))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func InspectGitSSH() GitSSHStatus {
	status, _ := inspectGitSSH()
	return status
}

func EnsureGitUsesNativeSSH() error {
	status, git := inspectGitSSH()
	if !status.NeedsFix() {
		return nil
	}
	value := filepath.ToSlash(sshClientPath())
	if strings.ContainsAny(value, " \t") {
		value = `"` + value + `"`
	}
	if out, err := exec.Command(git, "config", "--global", "core.sshCommand", value).CombinedOutput(); err != nil {
		return fmt.Errorf("setting git core.sshCommand: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func inspectGitSSH() (GitSSHStatus, string) {
	git, err := exec.LookPath("git")
	if err != nil || !gitBundlesMSYSSSH(git) {
		return GitSSHStatus{}, ""
	}
	status := GitSSHStatus{Applicable: true, Fixable: sshClientPath() != "ssh"}
	for _, env := range []string{"GIT_SSH_COMMAND", "GIT_SSH"} {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			status.Source, status.Native = env, commandIsNativeSSH(value)
			return status, git
		}
	}
	value, err := gitGlobalSSHCommand(git)
	if err != nil {
		// Unknown is not unset: never overwrite a value we could not read.
		status.Fixable = false
	} else if value != "" {
		status.Source, status.Native = "core.sshCommand", commandIsNativeSSH(value)
	}
	return status, git
}

// --includes honours dotfile includes; repo-local config depends on cwd.
func gitGlobalSSHCommand(git string) (string, error) {
	for _, scope := range []string{"--global", "--system"} {
		out, err := exec.Command(git, "config", scope, "--includes", "--get", "core.sshCommand").Output()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			continue
		}
		if err != nil {
			return "", err
		}
		if value := strings.TrimSpace(string(out)); value != "" {
			return value, nil
		}
	}
	return "", nil
}

// git.exe lives in <root>\cmd or <root>\<mingw|ucrt>64\bin; MSYS in <root>\usr\bin.
func gitBundlesMSYSSSH(git string) bool {
	dir := filepath.Dir(git)
	for _, root := range []string{filepath.Dir(dir), filepath.Dir(filepath.Dir(dir))} {
		if ssh := filepath.Join(root, "usr", "bin", "ssh.exe"); fileExists(ssh) && isMSYSExecutable(ssh) {
			return true
		}
	}
	return false
}

func commandIsNativeSSH(command string) bool {
	program := strings.TrimSpace(command)
	if strings.HasPrefix(program, `"`) {
		if end := strings.Index(program[1:], `"`); end >= 0 {
			program = program[1 : end+1]
		}
	} else if fields := strings.Fields(program); len(fields) > 0 {
		program = fields[0]
	}
	program = filepath.FromSlash(program)
	// Git resolves bare names against its own usr\bin, i.e. MSYS ssh.
	return filepath.IsAbs(program) && !isMSYSExecutable(program)
}
