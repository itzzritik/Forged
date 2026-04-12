package sync

import (
	"fmt"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type FirstLinkAction string

const (
	FirstLinkAdoptRemote  FirstLinkAction = "adopt_remote"
	FirstLinkPushLocal    FirstLinkAction = "push_local"
	FirstLinkMergeAndPush FirstLinkAction = "merge_and_push"
	FirstLinkNoop         FirstLinkAction = "noop"
)

func DecideFirstLinkAction(state SyncState, linkedUserID string, local, remote vault.VaultData, remoteExists bool) (vault.VaultData, FirstLinkAction, error) {
	if state.LinkedUserID != "" && state.LinkedUserID != linkedUserID {
		return vault.VaultData{}, "", fmt.Errorf("local vault is linked to a different account")
	}

	alreadyLinked := state.LinkedUserID == linkedUserID && (state.LastKnownServerVersion > 0 || len(state.LastSyncedBaseBlob) > 0)
	if alreadyLinked {
		return local, FirstLinkNoop, nil
	}

	localEmpty := len(local.Keys) == 0
	remoteEmpty := !remoteExists || len(remote.Keys) == 0

	switch {
	case localEmpty && remoteEmpty:
		return local, FirstLinkNoop, nil
	case localEmpty && !remoteEmpty:
		return remote, FirstLinkAdoptRemote, nil
	case !localEmpty && remoteEmpty:
		return local, FirstLinkPushLocal, nil
	default:
		merged := BootstrapMerge(local, remote, state.DeviceID, remote.Metadata.DeviceID)
		return merged, FirstLinkMergeAndPush, nil
	}
}
