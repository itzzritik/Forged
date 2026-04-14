package sshrouting

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var githubAccountPattern = regexp.MustCompile(`Hi ([^!]+)!`)

func ProbeGitHubAccount(agentSocket, identityFile string) (string, error) {
	sshBinary, err := exec.LookPath("ssh")
	if err != nil {
		return "", err
	}

	args := []string{
		"-F", os.DevNull,
		"-o", "BatchMode=yes",
		"-o", "PasswordAuthentication=no",
		"-o", "KbdInteractiveAuthentication=no",
		"-o", "IdentitiesOnly=yes",
		"-o", fmt.Sprintf("IdentityAgent=%s", agentSocket),
		"-o", fmt.Sprintf("IdentityFile=%s", identityFile),
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=5",
		"-T",
		"git@github.com",
	}

	cmd := exec.Command(sshBinary, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()
	text := strings.TrimSpace(output.String())
	match := githubAccountPattern.FindStringSubmatch(text)
	if len(match) == 2 {
		return strings.ToLower(strings.TrimSpace(match[1])), nil
	}
	if err == nil {
		return "", fmt.Errorf("github account not reported")
	}
	return "", fmt.Errorf("%s", text)
}

func HintFilePath(dir, keyRef string) string {
	return filepath.Join(dir, keyRef+".pub")
}
