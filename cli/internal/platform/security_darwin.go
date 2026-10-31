//go:build darwin

package platform

import (
	"context"
	"errors"
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

// SecurityDeleteAllGenericPasswords deletes every item for service, including
// ones that older installs (other install ids) left behind.
func SecurityDeleteAllGenericPasswords(ctx context.Context, service string) error {
	// security deletes one match per call and exits 44 once none is left.
	for range 1000 {
		out, err := exec.CommandContext(ctx, "security", "delete-generic-password", "-s", service).CombinedOutput()
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 44 {
			return nil
		}
		if err != nil {
			return fmt.Errorf("deleting keychain items for %s: %s: %w", service, strings.TrimSpace(string(out)), err)
		}
	}
	return fmt.Errorf("deleting keychain items for %s: too many items", service)
}

func quoteSecurityInteractiveArg(value string) (string, error) {
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", fmt.Errorf("invalid security interactive argument")
	}
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "'", "\\'")
	return "'" + value + "'", nil
}
