package tui

import (
	"fmt"

	"github.com/itzzritik/forged/cli/internal/readiness"
)

type SummaryRow struct {
	Label string
	Value string
}

type State struct {
	Snapshot readiness.Snapshot
}

func NewState(snapshot readiness.Snapshot) State {
	return State{Snapshot: snapshot}
}

func (s State) DashboardSubtitle() string {
	switch s.Snapshot.State {
	case readiness.StateUninitialized:
		return "Setup and recovery move here next."
	case readiness.StateReadyEmpty:
		return "The machine is healthy. The new shell is taking over in batches."
	case readiness.StateReady:
		return "Forged is healthy. The new shell is now the interactive entrypoint."
	case readiness.StateDegraded:
		return "Forged needs attention. Recovery flows move here next."
	case readiness.StateBlocked:
		return "Forged is blocked on additional work."
	default:
		return "Shared shell foundation active."
	}
}

func (s State) StatusLabel() string {
	switch s.Snapshot.State {
	case readiness.StateReady:
		return "READY"
	case readiness.StateReadyEmpty:
		return "READY-EMPTY"
	case readiness.StateDegraded:
		return "DEGRADED"
	case readiness.StateBlocked:
		return "BLOCKED"
	case readiness.StateUninitialized:
		return "SETUP"
	case readiness.StateSealed:
		return "SEALED"
	default:
		return "UNKNOWN"
	}
}

func (s State) StatusTone() string {
	switch s.Snapshot.State {
	case readiness.StateReady, readiness.StateReadyEmpty:
		return "success"
	case readiness.StateUninitialized, readiness.StateSealed:
		return "accent"
	case readiness.StateDegraded:
		return "warning"
	case readiness.StateBlocked:
		return "danger"
	default:
		return "neutral"
	}
}

func (s State) SummaryRows() []SummaryRow {
	syncState := "Not linked"
	if s.Snapshot.LoggedIn {
		syncState = "Linked"
	}

	daemonState := "Offline"
	if s.Snapshot.Service.Running {
		daemonState = "Running"
	}

	socketState := "Waiting"
	if s.Snapshot.IPCSocketReady && s.Snapshot.AgentSocketReady {
		socketState = "Ready"
	}

	sshState := "Not active"
	if s.Snapshot.SSHEnabled && s.Snapshot.ManagedConfigReady {
		sshState = "Active"
	}

	return []SummaryRow{
		{Label: "State", Value: string(s.Snapshot.State)},
		{Label: "Keys", Value: fmt.Sprintf("%d loaded", s.Snapshot.KeyCount)},
		{Label: "Daemon", Value: daemonState},
		{Label: "Sockets", Value: socketState},
		{Label: "SSH", Value: sshState},
		{Label: "Sync", Value: syncState},
	}
}
