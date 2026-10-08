package core

import (
	"strings"

	"github.com/itzzritik/forged/cli/internal/accountauth"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

func (s *State) Recovery() bool { return strings.TrimSpace(s.Snapshot.RuntimePathError) != "" }

func (s *State) SyncIssue() string {
	if !s.StatusLoaded {
		return ""
	}
	diagnostic := strings.TrimSpace(s.Status.Error)
	if diagnostic == "" || accountauth.IsCredentialDiagnostic(diagnostic) {
		return ""
	}
	return diagnostic
}

func (s *State) CredentialError() string {
	if s.SyncIssue() != "" {
		return ""
	}
	if diagnostic := strings.TrimSpace(s.Snapshot.LoginCheckError); diagnostic != "" {
		return diagnostic
	}
	if !s.StatusLoaded {
		return ""
	}
	if diagnostic := strings.TrimSpace(s.Status.Error); accountauth.IsCredentialDiagnostic(diagnostic) {
		return diagnostic
	}
	return ""
}

func (s *State) SyncPending() bool {
	return s.Status.Syncing || s.StatusLoaded && s.Status.Dirty && strings.TrimSpace(s.Status.Error) == ""
}

func (s *State) Chip() ui.Chip {
	chip := func(tone ui.Tone, label string) ui.Chip {
		glyph := ui.G.Warn
		if tone == ui.ToneBusy {
			glyph = ui.G.Spinner[0]
		}
		return ui.Chip{Glyph: glyph, Label: label, Tone: tone}
	}
	switch {
	case s.Recovery():
		return chip(ui.ToneBad, "Daemon unavailable")
	case strings.TrimSpace(s.RepairErr) != "":
		return chip(ui.ToneBad, "Repair failed")
	case s.Health == HealthBad:
		return chip(ui.ToneBad, "Health check failed")
	case s.Health == HealthFixing:
		return chip(ui.ToneBusy, "Fixing")
	case s.Health == HealthChecking:
		return chip(ui.ToneBusy, "Checking")
	case s.StatusDown:
		return chip(ui.ToneBad, "Vault unavailable")
	case s.SyncIssue() != "":
		return chip(ui.ToneBad, "Sync issue")
	case s.CredentialError() != "":
		return chip(ui.ToneWarn, "Account needs attention")
	case s.Problems() > 0:
		return chip(ui.ToneWarn, Plural(s.Problems(), "problem"))
	case s.Snapshot.RequiresManualSSHConfigurationChange():
		return chip(ui.ToneWarn, "External SSH config")
	case s.Snapshot.AgentDisabled:
		return chip(ui.ToneWarn, "Agent off")
	case strings.TrimSpace(s.SigningErr) != "":
		return chip(ui.ToneWarn, "Signing issue")
	// The daemon marks local-only vaults dirty too.
	case s.Snapshot.LoggedIn && s.SyncPending():
		return chip(ui.ToneBusy, "Syncing")
	}
	return ui.Chip{Glyph: ui.G.Dot, Label: "All good", Tone: ui.ToneGood}
}

func (s *State) Problems() int { return s.ProblemCount }

func (s *State) RepairBusy() bool { return s.Busy.Maintenance || s.Busy.SSHToggle }

func (s *State) CanFix() bool {
	n := s.Snapshot
	switch {
	case s.RepairBusy(), strings.TrimSpace(n.RuntimePathError) != "", !n.VaultExists:
		return false
	case !n.ConfigExists:
		return true
	case !n.ConfigValid, !n.Service.Repairable:
		return false
	case !n.Service.Installed, !n.Service.ConfigValid, !n.Service.Running:
		return true
	case n.Service.PID > 0 && (n.DaemonPID <= 0 || n.Service.PID != n.DaemonPID):
		return true
	}
	if current := strings.TrimSpace(n.CurrentBuildID); current != "" && strings.TrimSpace(n.DaemonBuildID) != current {
		return true
	}
	return !n.IPCSocketReady || !n.AgentSocketReady || !n.AgentDisabled && !n.SSHHealthy()
}

func (s *State) KeyIsSigning(publicKey string) bool {
	if !s.SigningLoaded || s.Signing.Mode != actions.CommitSigningForged {
		return false
	}
	publicKey = strings.TrimSpace(publicKey)
	return publicKey != "" && publicKey == strings.TrimSpace(s.Signing.PublicKey)
}

func (s *State) Tabs() []Tab {
	if s.Recovery() {
		return []Tab{TabSSH, TabHealth}
	}
	return []Tab{TabOverview, TabKeys, TabSSH, TabAccount, TabHealth}
}
