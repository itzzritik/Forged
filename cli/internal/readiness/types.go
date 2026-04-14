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

type Snapshot struct {
	State              State
	KeyCount           int
	LoggedIn           bool
	VaultExists        bool
	ConfigExists       bool
	Service            daemon.ServiceStatus
	DaemonPID          int
	IPCSocketReady     bool
	AgentSocketReady   bool
	SSHEnabled         bool
	ManagedConfigReady bool
	IdentityAgentOwner config.SSHAgentOwner
}

type RepairSummary struct {
	Fixed  []string
	Failed []string
}
