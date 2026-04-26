package sshrouting

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	probePerKeyTimeout = 4 * time.Second
	probeTotalTimeout  = 20 * time.Second
)

type ProbeStatus string

const (
	ProbeSuccess      ProbeStatus = "success"
	ProbeDenied       ProbeStatus = "denied"
	ProbeSkipped      ProbeStatus = "skipped"
	ProbeInconclusive ProbeStatus = "inconclusive"
)

type ProbeResult struct {
	Status      ProbeStatus
	Fingerprint string
	Message     string
}

type ProviderProber struct {
	agentSocket string
	command     string
}

func NewProviderProber(agentSocket string) ProviderProber {
	return ProviderProber{
		agentSocket: agentSocket,
		command:     "ssh",
	}
}

func (p ProviderProber) Probe(ctx context.Context, target Target, operation OperationClass, ref KeyRef) ProbeResult {
	provider, ok := DetectProvider(target)
	if !ok {
		return ProbeResult{Status: ProbeSkipped, Fingerprint: ref.Fingerprint, Message: "unknown provider"}
	}
	if strings.TrimSpace(ref.Path) == "" {
		return ProbeResult{Status: ProbeSkipped, Fingerprint: ref.Fingerprint, Message: "missing identity hint"}
	}

	repoPath := providerRepoPath(target)
	if repoPath == "" {
		return ProbeResult{Status: ProbeSkipped, Fingerprint: ref.Fingerprint, Message: "missing repo path"}
	}

	probeCommand := "git-upload-pack"
	if operation == OperationWrite {
		probeCommand = "git-receive-pack"
	}
	probeOperation, err := operationFromProbeCommand(probeCommand)
	if err != nil {
		return ProbeResult{Status: ProbeSkipped, Fingerprint: ref.Fingerprint, Message: err.Error()}
	}
	port := target.Port
	if port <= 0 {
		port = 22
	}

	perKeyCtx, cancel := context.WithTimeout(ctx, probePerKeyTimeout)
	defer cancel()

	args := []string{
		"-F", "none",
		"-o", "BatchMode=yes",
		"-o", "IdentitiesOnly=yes",
		"-o", "PreferredAuthentications=publickey",
		"-o", "PasswordAuthentication=no",
		"-o", "KbdInteractiveAuthentication=no",
		"-o", "NumberOfPasswordPrompts=0",
		"-o", "StrictHostKeyChecking=yes",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=4",
		"-o", "IdentityAgent=" + p.agentSocket,
		"-o", "IdentityFile=" + ref.Path,
		"-p", fmt.Sprintf("%d", port),
		provider.User + "@" + provider.Host,
		probeCommand,
		repoPath,
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(perKeyCtx, p.command, args...)
	cmd.Stdin = strings.NewReader("")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "FORGED_SSH_ROUTE_SKIP=1")

	err = cmd.Run()
	output := stdout.String()
	message := strings.TrimSpace(stderr.String())
	if hasGitAdvertisement(output, probeOperation) {
		return ProbeResult{Status: ProbeSuccess, Fingerprint: ref.Fingerprint}
	}
	if perKeyCtx.Err() != nil || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ProbeResult{Status: ProbeInconclusive, Fingerprint: ref.Fingerprint, Message: "probe timed out"}
	}
	if err == nil {
		return ProbeResult{Status: ProbeInconclusive, Fingerprint: ref.Fingerprint, Message: "probe returned no refs"}
	}
	if isHostTrustFailure(message) {
		return ProbeResult{Status: ProbeSkipped, Fingerprint: ref.Fingerprint, Message: "host key is not trusted"}
	}
	if isAccessDenied(message) {
		return ProbeResult{Status: ProbeDenied, Fingerprint: ref.Fingerprint, Message: message}
	}
	return ProbeResult{Status: ProbeInconclusive, Fingerprint: ref.Fingerprint, Message: message}
}

func hasGitAdvertisement(output string, operation OperationClass) bool {
	if output == "" {
		return false
	}
	if strings.Contains(output, "# service=git-upload-pack") {
		return true
	}
	if looksLikeGitPktLine(output) && (strings.Contains(output, " HEAD") || strings.Contains(output, "\x00")) {
		return true
	}
	if operation == OperationWrite && (strings.Contains(output, "\x00report-status") || strings.Contains(output, "\x00delete-refs")) {
		return true
	}
	return strings.HasPrefix(output, "00") || strings.Contains(output, " refs/")
}

func looksLikeGitPktLine(output string) bool {
	if len(output) < 4 {
		return false
	}
	for _, r := range output[:4] {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return output[:4] != "0000"
}

func isHostTrustFailure(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "host key verification failed") ||
		strings.Contains(lower, "remote host identification has changed") ||
		strings.Contains(lower, "no hostkey alg")
}

func isAccessDenied(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "permission denied") ||
		strings.Contains(lower, "repository not found") ||
		strings.Contains(lower, "project not found") ||
		strings.Contains(lower, "access denied") ||
		strings.Contains(lower, "could not read from remote repository")
}
