package sensitiveauth

import (
	"fmt"
	"time"
)

type Action string

const (
	ActionView   Action = "view"
	ActionExport Action = "export"
)

const (
	ViewLeaseTTL   = 4 * time.Hour
	ExportTokenTTL = time.Minute
)

type AuthorizeResult struct {
	Authorized       bool   `json:"authorized"`
	PasswordRequired bool   `json:"password_required"`
	Prompt           string `json:"prompt,omitempty"`
	ExportToken      string `json:"export_token,omitempty"`
}

func ParseAction(raw string) (Action, error) {
	switch Action(raw) {
	case ActionView, ActionExport:
		return Action(raw), nil
	default:
		return "", fmt.Errorf("unsupported sensitive action %q", raw)
	}
}

func (a Action) PasswordPrompt() string {
	switch a {
	case ActionExport:
		return "Native authentication unavailable. Enter your master password to export:"
	default:
		return "Native authentication unavailable. Enter your master password to continue:"
	}
}

func (a Action) NativeReason() string {
	switch a {
	case ActionExport:
		return "Authenticate to export your Forged vault"
	default:
		return "Authenticate to reveal a private key in Forged"
	}
}
