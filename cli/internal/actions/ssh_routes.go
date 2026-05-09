package actions

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
)

type SSHRoutingDebug struct {
	Routes          []SSHRouteDebug          `json:"routes"`
	RuntimeAttempts []SSHRouteRuntimeAttempt `json:"runtime_attempts"`
	PublicHints     []SSHRoutePublicHint     `json:"public_hints"`
}

type SSHRouteDebug struct {
	Target        string                 `json:"target"`
	Kind          string                 `json:"kind,omitempty"`
	Host          string                 `json:"host,omitempty"`
	User          string                 `json:"user,omitempty"`
	Port          int                    `json:"port,omitempty"`
	Owner         string                 `json:"owner,omitempty"`
	Repo          string                 `json:"repo,omitempty"`
	Fingerprint   string                 `json:"fingerprint,omitempty"`
	KeyName       string                 `json:"key_name,omitempty"`
	KeyRef        string                 `json:"key_ref,omitempty"`
	ProvenBy      string                 `json:"proven_by,omitempty"`
	Operation     string                 `json:"operation,omitempty"`
	SuccessCount  int                    `json:"success_count,omitempty"`
	LastSuccessAt *time.Time             `json:"last_success_at,omitempty"`
	Updated       time.Time              `json:"updated"`
	Attempts      []SSHRouteDebugAttempt `json:"attempts,omitempty"`
}

type SSHRouteDebugAttempt struct {
	Fingerprint string    `json:"fingerprint,omitempty"`
	KeyName     string    `json:"key_name,omitempty"`
	KeyRef      string    `json:"key_ref,omitempty"`
	AttemptedAt time.Time `json:"attempted_at"`
}

type SSHRouteRuntimeAttempt struct {
	Token         string                  `json:"token"`
	Path          string                  `json:"path"`
	Updated       time.Time               `json:"updated"`
	AgeSeconds    int64                   `json:"age_seconds"`
	IdentityFiles []SSHRouteIdentityFile  `json:"identity_files,omitempty"`
	Clients       []SSHRouteRuntimeClient `json:"clients,omitempty"`
}

type SSHRouteRuntimeClient struct {
	ClientPID  int      `json:"client_pid,omitempty"`
	Target     string   `json:"target,omitempty"`
	Operation  string   `json:"operation,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
	HadExact   bool     `json:"had_exact,omitempty"`
}

type SSHRouteIdentityFile struct {
	Path        string `json:"path"`
	Ref         string `json:"ref,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	KeyName     string `json:"key_name,omitempty"`
}

type SSHRoutePublicHint struct {
	Ref         string    `json:"ref"`
	Path        string    `json:"path"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	KeyName     string    `json:"key_name,omitempty"`
	Updated     time.Time `json:"updated"`
	Stale       bool      `json:"stale,omitempty"`
}

func LoadSSHRoutingDebug(paths config.Paths) (SSHRoutingDebug, error) {
	snapshot, err := loadSSHRoutingDebug(paths)
	if err == nil {
		return snapshot, nil
	}
	if isUnknownIPCCommand(err, ipc.CmdSSHRoutesList) {
		return SSHRoutingDebug{}, fmt.Errorf("SSH routing diagnostics need a fresh Forged daemon; run Doctor > Fix Issues")
	}
	return SSHRoutingDebug{}, err
}

func loadSSHRoutingDebug(paths config.Paths) (SSHRoutingDebug, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdSSHRoutesList, nil)
	if err != nil {
		return SSHRoutingDebug{}, err
	}
	var snapshot SSHRoutingDebug
	if err := json.Unmarshal(resp.Data, &snapshot); err != nil {
		return SSHRoutingDebug{}, fmt.Errorf("parsing SSH routing debug payload: %w", err)
	}
	return snapshot, nil
}

func isUnknownIPCCommand(err error, command string) bool {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "unknown command") && strings.Contains(message, strings.ToLower(command))
}

func ClearSSHRoute(paths config.Paths, target string) error {
	_, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdSSHRouteClear, ipc.SSHRouteClearArgs{Target: target})
	return err
}

func ClearAllSSHRoutes(paths config.Paths) error {
	_, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdSSHRoutesClearAll, nil)
	return err
}
