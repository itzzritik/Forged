package core

import (
	"time"

	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/readiness"
)

type Health int

const (
	HealthChecking Health = iota
	HealthFixing
	HealthOK
	HealthBad
)

type Busy struct{ Maintenance, Sync, SSHToggle, Logout, Clipboard, PasswordChange bool }

type State struct {
	Deps           Deps
	Reporter       *Reporter
	Width, Height  int
	Snapshot       readiness.Snapshot
	Summary        readiness.RepairSummary
	Health         Health
	RepairErr      string
	Status         actions.RuntimeStatus
	StatusLoaded   bool
	StatusDown     bool
	Security       actions.SecurityState
	SecurityLoaded bool
	SecurityErr    string
	Signing        actions.CommitSigningStatus
	SigningLoaded  bool
	SigningErr     string
	AccountName    string
	AccountEmail   string
	Keys           []actions.KeySummary
	KeysLoaded     bool
	KeysErr        string
	Details        map[string]actions.KeyDetail
	Busy           Busy
	Locked         bool
	Clip           *Clip
	ProblemCount   int
	SpinFrame      int
}

type Clip struct {
	ID       ID
	Name     string
	Lease    ClipboardLease
	Until    time.Time
	Clearing bool
}
