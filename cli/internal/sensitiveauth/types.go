package sensitiveauth

import (
	"fmt"
	"time"
)

type Action string

const (
	ActionView     Action = "view"
	ActionExport   Action = "export"
	ActionExternal Action = "external"
)

const (
	ExportTokenTTL   = time.Minute
	SharedSessionTTL = 4 * time.Hour
)

type AuthorizeResult struct {
	Authorized       bool   `json:"authorized"`
	PasswordRequired bool   `json:"password_required"`
	Prompt           string `json:"prompt,omitempty"`
	ExportToken      string `json:"export_token,omitempty"`
}

func ParseAction(raw string) (Action, error) {
	switch Action(raw) {
	case ActionView, ActionExport, ActionExternal:
		return Action(raw), nil
	default:
		return "", fmt.Errorf("unsupported sensitive action %q", raw)
	}
}

func (a Action) PasswordPrompt() string {
	switch a {
	case ActionExport:
		return "Native authentication unavailable. Enter your master password to unlock private-key access and export:"
	case ActionExternal:
		return ""
	default:
		return "Native authentication unavailable. Enter your master password to unlock private-key access:"
	}
}

func (a Action) NativeReason() string {
	switch a {
	case ActionExport:
		return "Authenticate to export private keys from Forged"
	case ActionExternal:
		return "Authenticate to use private keys through Forged"
	default:
		return "Authenticate to unlock private-key access in Forged"
	}
}
