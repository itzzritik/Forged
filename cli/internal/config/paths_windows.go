//go:build windows

package config

import (
	"fmt"

	"github.com/itzzritik/forged/cli/internal/platform"
)

const legacyAgentSocket = `\\.\pipe\forged-agent`

func (Paths) AgentSocket() string {
	agent, _, err := platform.CurrentUserPipePaths()
	if err != nil {
		return ""
	}
	return agent
}

func (Paths) CtlSocket() string {
	_, ctl, err := platform.CurrentUserPipePaths()
	if err != nil {
		return ""
	}
	return ctl
}

func (Paths) ValidateRuntimePaths() error {
	if _, _, err := platform.CurrentUserPipePaths(); err != nil {
		return fmt.Errorf("resolving current Windows user pipe identity: %w", err)
	}
	return nil
}

func isLegacyAgentSocket(value string) bool {
	return value == legacyAgentSocket
}
