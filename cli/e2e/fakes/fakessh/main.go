//go:build e2e && windows

// fakessh pins Win32-OpenSSH to %USERPROFILE%; it resolves "~" from the
// account profile, so an isolated profile would read the real ~/.ssh.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	profile := os.Getenv("USERPROFILE")
	args := append([]string{
		"-F", filepath.Join(profile, ".ssh", "config"),
		"-o", "UserKnownHostsFile=" + filepath.Join(profile, ".ssh", "known_hosts"),
	}, os.Args[1:]...)
	cmd := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "OpenSSH", "ssh.exe"), args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		os.Exit(255)
	}
}
