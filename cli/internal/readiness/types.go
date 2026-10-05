package readiness

import (
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
)

type State string

const (
	StateUninitialized State = "uninitialized"
	StateSealed        State = "sealed"
	StateReadyEmpty    State = "ready-empty"
	StateReady         State = "ready"
	StateDegraded      State = "degraded"
	StateBlocked       State = "blocked"
)

type RowState string

const (
	RowChecking RowState = "checking"
	RowFixing   RowState = "fixing"
	RowReady    RowState = "ready"
	RowNeedsYou RowState = "needs_you"
	RowBlocked  RowState = "blocked"
)

type Mode string

const (
	ModeAssessOnly          Mode = "assess_only"
	ModeInteractiveLauncher Mode = "interactive_launcher"
	ModeInteractiveDoctor   Mode = "interactive_doctor"
	ModeNonInteractiveFix   Mode = "non_interactive_fix"
)

type NextAction string

const (
	NextActionNone                  NextAction = "none"
	NextActionNeedsPassword         NextAction = "needs_password"
	NextActionNeedsInteractiveSetup NextAction = "needs_interactive_setup"
	NextActionNeedsCredentialRepair NextAction = "needs_credential_repair"
)

type PasswordPrompt func(reason string) ([]byte, error)

type Snapshot struct {
	State                 State
	RuntimePathError      string
	KeyCount              int
	CurrentBuildID        string
	DaemonBuildID         string
	LoggedIn              bool
	LoginCheckError       string
	VaultExists           bool
	ConfigExists          bool
	ConfigValid           bool
	ConfigError           string
	Service               daemon.ServiceStatus
	DaemonPID             int
	IPCSocketReady        bool
	AgentSocketReady      bool
	AgentDisabled         bool
	SSHEnabled            bool
	ManagedSSHIntegration bool
	ManagedConfigReady    bool
	IdentityAgentOwner    config.SSHAgentOwner
	GitSSH                config.GitSSHStatus
}

func (s Snapshot) SSHHealthy() bool {
	return sshConfigHealthy(s) && !s.GitSSH.NeedsFix()
}

func (s Snapshot) RequiresManualSSHConfigurationChange() bool {
	return s.AgentDisabled && !s.ManagedSSHIntegration && s.IdentityAgentOwner.IsForged()
}

type RepairSummary struct {
	Fixed  []string
	Failed []string
}

type RunOptions struct {
	Mode           Mode
	PromptPassword PasswordPrompt
}

type RunResult struct {
	Snapshot Snapshot
	Summary  RepairSummary
	Next     NextAction
}
