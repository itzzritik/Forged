package sshrouting

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func CurrentRemote() (Remote, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Remote{}, err
	}

	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = cwd
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return Remote{}, fmt.Errorf("current repo has no origin")
	}

	return NormalizeRemote(strings.TrimSpace(stdout.String()))
}
