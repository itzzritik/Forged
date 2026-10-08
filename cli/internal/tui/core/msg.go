package core

import (
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type ID uint64

var lastID atomic.Uint64

func NextID() ID { return ID(lastID.Add(1)) }

type Tab int

const (
	TabOverview Tab = iota
	TabKeys
	TabSSH
	TabAccount
	TabHealth
)

type OpenOverlayMsg struct {
	Screen      Screen
	X, Y        int
	W, H        int
	Center, Dim bool
}

type CloseOverlayMsg struct{ Screen Screen }

type ToastMsg struct {
	Text string
	Tone ui.Tone
}

type SwitchTabMsg struct {
	Tab    Tab
	Select string
}

// LockedMsg is broadcast to every screen: wipe secrets and drop previews.
type LockedMsg struct{}

type KeysChangedMsg struct{}

type KeysMsg struct {
	ID   ID
	Keys []actions.KeySummary
	Err  error
}

type DetailMsg struct {
	ID     ID
	Name   string
	Detail actions.KeyDetail
	Err    error
}

type SigningMsg struct {
	ID     ID
	Status actions.CommitSigningStatus
	Err    error
}

type SecurityMsg struct {
	ID    ID
	State actions.SecurityState
	Err   error
}

type StatusMsg struct {
	ID     ID
	Status actions.RuntimeStatus
	Err    error
}

type SnapshotMsg struct {
	ID     ID
	Result readiness.RunResult
	Err    error
}

type RoutesMsg struct {
	ID    ID
	Debug actions.SSHRoutingDebug
	Err   error
}

type RequestLoginMsg struct{}

type RequestRepairMsg struct{ Password []byte }

type RequestLockMsg struct{}

type RequestSyncMsg struct{}

type RefreshMsg struct{}

// CancelClipMsg drops the lease without clearing: a normal copy started at Since already replaced the clipboard.
type CancelClipMsg struct{ Since ID }

type ClipStartMsg struct{ Clip *Clip }

type ClipTickMsg struct{ ID ID }

type ClipClearedMsg struct {
	ID      ID
	Cleared bool
	Err     error
}

type Spinner interface{ Spinning() bool }

func Send(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }

func Toast(text string, tone ui.Tone) tea.Cmd { return Send(ToastMsg{Text: text, Tone: tone}) }

func Close(s Screen) tea.Cmd { return Send(CloseOverlayMsg{Screen: s}) }
