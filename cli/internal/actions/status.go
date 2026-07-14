package actions

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
)

type RuntimeStatus struct {
	Syncing              bool
	Dirty                bool
	Linked               bool
	LastSuccessfulPullAt time.Time
	LastSuccessfulPushAt time.Time
	Unlocked             bool
	SensitiveKnown       bool
	SensitiveReported    bool
	Error                string
}

func LoadRuntimeStatus(paths config.Paths) (RuntimeStatus, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdStatus, nil)
	if err != nil {
		return RuntimeStatus{}, fmt.Errorf("loading daemon status: %w", err)
	}

	var status struct {
		Sensitive *struct {
			Unlocked *bool `json:"unlocked"`
		} `json:"sensitive"`
		Sync struct {
			Dirty                bool      `json:"dirty"`
			LastErr              string    `json:"last_error"`
			Linked               bool      `json:"linked"`
			Syncing              bool      `json:"syncing"`
			LastSuccessfulPullAt time.Time `json:"last_successful_pull_at"`
			LastSuccessfulPushAt time.Time `json:"last_successful_push_at"`
		} `json:"sync"`
	}
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return RuntimeStatus{}, fmt.Errorf("parsing daemon status: %w", err)
	}

	result := RuntimeStatus{
		Syncing:              status.Sync.Syncing,
		Dirty:                status.Sync.Dirty,
		Linked:               status.Sync.Linked,
		LastSuccessfulPullAt: status.Sync.LastSuccessfulPullAt,
		LastSuccessfulPushAt: status.Sync.LastSuccessfulPushAt,
		Error:                status.Sync.LastErr,
		SensitiveReported:    status.Sensitive != nil && status.Sensitive.Unlocked != nil,
	}
	if result.SensitiveReported {
		result.Unlocked = *status.Sensitive.Unlocked
		result.SensitiveKnown = true
	}
	return result, nil
}
