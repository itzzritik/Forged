//go:build !windows

package config

import "path/filepath"

func (p Paths) AgentSocket() string {
	return filepath.Join(p.RuntimeDir, "agent.sock")
}

func (p Paths) CtlSocket() string {
	return filepath.Join(p.RuntimeDir, "ctl.sock")
}

func (Paths) ValidateRuntimePaths() error {
	return nil
}

func isLegacyAgentSocket(string) bool {
	return false
}
