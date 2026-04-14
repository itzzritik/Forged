package config

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

type SSHAgentOwner struct {
	Name string
	Path string
}

func (o SSHAgentOwner) IsUnknown() bool { return o.Name == "" }
func (o SSHAgentOwner) IsForged() bool  { return o.Name == "Forged" }

func DetectSSHAgentOwner(paths Paths) (SSHAgentOwner, error) {
	cmd := exec.Command("ssh", "-G", "github.com")
	out, err := cmd.Output()
	if err != nil {
		return SSHAgentOwner{}, err
	}

	raw := parseIdentityAgent(out)
	if raw == "" || strings.EqualFold(raw, "none") {
		return SSHAgentOwner{Name: "None"}, nil
	}

	if usesEnvIdentityAgent(raw) {
		envPath := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
		if envPath != "" {
			raw = envPath
		} else {
			return SSHAgentOwner{Name: "SSH_AUTH_SOCK"}, nil
		}
	}

	raw = strings.Trim(raw, `"'`)
	lower := strings.ToLower(raw)

	switch {
	case raw == paths.AgentSocket():
		return SSHAgentOwner{Name: "Forged", Path: raw}, nil
	case strings.Contains(lower, "1password"):
		return SSHAgentOwner{Name: "1Password", Path: raw}, nil
	case strings.Contains(lower, "bitwarden"):
		return SSHAgentOwner{Name: "Bitwarden", Path: raw}, nil
	case strings.Contains(lower, "secretive"):
		return SSHAgentOwner{Name: "Secretive", Path: raw}, nil
	default:
		return SSHAgentOwner{Name: "Custom", Path: raw}, nil
	}
}

func parseIdentityAgent(out []byte) string {
	for _, line := range bytes.Split(out, []byte("\n")) {
		fields := strings.Fields(string(line))
		if len(fields) >= 2 && strings.EqualFold(fields[0], "identityagent") {
			return strings.Join(fields[1:], " ")
		}
	}
	return ""
}

func usesEnvIdentityAgent(value string) bool {
	value = strings.TrimSpace(value)
	return strings.EqualFold(value, "SSH_AUTH_SOCK") ||
		strings.EqualFold(value, "$SSH_AUTH_SOCK") ||
		strings.EqualFold(value, "${SSH_AUTH_SOCK}")
}
