//go:build linux

package sensitiveauth

import (
	"context"
	"errors"
	"os/exec"
)

type linuxNativeProvider struct{}

func NewNativeProvider() NativeProvider {
	return linuxNativeProvider{}
}

func (linuxNativeProvider) Name() string { return "pkexec" }

func (linuxNativeProvider) Authorize(ctx context.Context, action Action) error {
	path, err := exec.LookPath("pkexec")
	if err != nil {
		return ErrNativeUnavailable
	}

	cmd := exec.CommandContext(ctx, path, "--disable-internal-agent", "/bin/true")
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 126 || exitErr.ExitCode() == 127 {
				return ErrNativeUnavailable
			}
		}
		return ErrAuthenticationFailed
	}

	return nil
}
