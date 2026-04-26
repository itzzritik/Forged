package sshrouting

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type ProcessContext struct {
	ClientPID     int
	Command       string
	ParentPID     int
	ParentCommand string
	Operation     OperationClass
	RepoPath      string
}

func InspectProcess(clientPID int) ProcessContext {
	ctx := ProcessContext{ClientPID: clientPID, Operation: OperationUnknown}
	if clientPID <= 0 {
		return ctx
	}
	ctx.Command = processCommand(clientPID)
	ctx.ParentPID = parentPID(clientPID)
	if ctx.ParentPID > 0 {
		ctx.ParentCommand = processCommand(ctx.ParentPID)
	}
	ctx.Operation = inferOperation(ctx.Command, ctx.ParentCommand)
	ctx.RepoPath = gitCommandRepoPath(ctx.Command)
	if ctx.RepoPath == "" {
		ctx.RepoPath = gitCommandRepoPath(ctx.ParentCommand)
	}
	return ctx
}

func ResolveProcessGitTarget(clientPID int, input PrepareInput) (Target, OperationClass, bool) {
	ctx := InspectProcess(clientPID)
	if ctx.RepoPath == "" {
		return Target{}, ctx.Operation, false
	}
	target, err := targetFromRepoPath(input, ctx.RepoPath)
	if err != nil {
		return Target{}, ctx.Operation, false
	}
	return target, ctx.Operation, true
}

func inferOperation(commands ...string) OperationClass {
	for _, command := range commands {
		lower := strings.ToLower(command)
		switch {
		case strings.Contains(lower, "git-receive-pack"),
			strings.Contains(lower, " git push"),
			strings.HasPrefix(lower, "git push"):
			return OperationWrite
		case strings.Contains(lower, "git-upload-pack"),
			strings.Contains(lower, " git fetch"),
			strings.HasPrefix(lower, "git fetch"),
			strings.Contains(lower, " git pull"),
			strings.HasPrefix(lower, "git pull"),
			strings.Contains(lower, " git clone"),
			strings.HasPrefix(lower, "git clone"),
			strings.Contains(lower, " git ls-remote"),
			strings.HasPrefix(lower, "git ls-remote"):
			return OperationRead
		}
	}
	for _, command := range commands {
		lower := strings.ToLower(strings.TrimSpace(command))
		if strings.Contains(lower, "ssh ") || strings.HasPrefix(lower, "ssh") {
			return OperationSSHAuth
		}
	}
	return OperationUnknown
}

func gitCommandRepoPath(command string) string {
	fields := shellFields(command)
	for i, field := range fields {
		base := strings.Trim(strings.ToLower(field), `"'`)
		if base != "git-upload-pack" && base != "git-receive-pack" {
			continue
		}
		if i+1 >= len(fields) {
			return ""
		}
		return fields[i+1]
	}
	return ""
}

func shellFields(command string) []string {
	var fields []string
	var current strings.Builder
	var quote rune
	escaped := false
	for _, r := range command {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}
	return fields
}

func processCommand(pid int) string {
	if pid <= 0 || runtime.GOOS == "windows" {
		return ""
	}
	out, err := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func parentPID(pid int) int {
	if pid <= 0 || runtime.GOOS == "windows" {
		return 0
	}
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return value
}

func (o OperationClass) String() string {
	if o == "" {
		return string(OperationUnknown)
	}
	return string(o)
}

func parseOperation(raw string) OperationClass {
	switch OperationClass(strings.TrimSpace(raw)) {
	case OperationRead:
		return OperationRead
	case OperationWrite:
		return OperationWrite
	case OperationSSHAuth:
		return OperationSSHAuth
	default:
		return OperationUnknown
	}
}

func operationFromProbeCommand(command string) (OperationClass, error) {
	switch command {
	case "git-upload-pack":
		return OperationRead, nil
	case "git-receive-pack":
		return OperationWrite, nil
	default:
		return OperationUnknown, fmt.Errorf("Unsupported probe command %q", command)
	}
}
