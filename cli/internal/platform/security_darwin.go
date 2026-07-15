//go:build darwin

package platform

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const securityInteractiveMaxLine = 4096

// SecurityAddGenericPassword sends a quoted command through security's stdin
// command loop so the password stays out of process arguments. A bare final
// -w instead prompts on a tty.
func SecurityAddGenericPassword(ctx context.Context, service, account, password string) ([]byte, error) {
	service, err := quoteSecurityInteractiveArg(service)
	if err != nil {
		return nil, err
	}
	account, err = quoteSecurityInteractiveArg(account)
	if err != nil {
		return nil, err
	}
	password, err = quoteSecurityInteractiveArg(password)
	if err != nil {
		return nil, err
	}
	line := "add-generic-password -U -s " + service + " -a " + account + " -w " + password + "\n"
	if len(line) >= securityInteractiveMaxLine {
		return nil, fmt.Errorf("security interactive command is too long")
	}
	cmd := exec.CommandContext(ctx, "security", "-i")
	cmd.Stdin = strings.NewReader(line)
	return cmd.CombinedOutput()
}

func quoteSecurityInteractiveArg(value string) (string, error) {
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", fmt.Errorf("invalid security interactive argument")
	}
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "'", "\\'")
	return "'" + value + "'", nil
}
