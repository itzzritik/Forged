package sync

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type API interface {
	Push(blob []byte, kdf vault.KDFParams, protectedKey string, expectedVersion int64) (PushResult, error)
	Pull() (PullResult, error)
}

type Engine struct {
	vault  *vault.Vault
	client API
	logger *slog.Logger
}

func NewEngine(v *vault.Vault, client API, logger *slog.Logger) *Engine {
	return &Engine{
		vault:  v,
		client: client,
		logger: logger,
	}
}

func (e *Engine) PushCurrent(ctx context.Context, state *SyncState) error {
	if state == nil {
		return fmt.Errorf("sync state required")
	}

	_ = ctx

	blob, err := e.vault.ExportForSync()
	if err != nil {
		return err
	}

	protectedKey := base64.StdEncoding.EncodeToString(e.vault.ProtectedKeyBytes())
	result, err := e.client.Push(blob, e.vault.KDFParams(), protectedKey, state.LastKnownServerVersion)
	if err != nil {
		return err
	}

	state.MarkClean(result.Version, blob, hashBlob(blob))
	return nil
}

func (e *Engine) PullLatest(ctx context.Context, state *SyncState) (vault.VaultData, PullResult, error) {
	if state == nil {
		return vault.VaultData{}, PullResult{}, fmt.Errorf("sync state required")
	}

	_ = ctx

	result, err := e.client.Pull()
	if err != nil {
		return vault.VaultData{}, PullResult{}, err
	}

	plaintext, err := vault.DecryptCombined(e.vault.Key(), result.Blob)
	if err != nil {
		return vault.VaultData{}, PullResult{}, err
	}

	var remote vault.VaultData
	if err := json.Unmarshal(plaintext, &remote); err != nil {
		return vault.VaultData{}, PullResult{}, err
	}

	if !state.Dirty {
		original := e.vault.Data
		e.vault.Data = MergeVaults(e.vault.Data, remote)
		if err := e.vault.Save(); err != nil {
			e.vault.Data = original
			return vault.VaultData{}, PullResult{}, err
		}
		state.LastSyncedBaseBlob = append([]byte(nil), result.Blob...)
		state.LastSyncedHash = hashBlob(result.Blob)
		state.LastError = ""
		state.NextRetryAt = time.Time{}
	}

	state.LastKnownServerVersion = result.Version
	state.LastSuccessfulPullAt = time.Now().UTC()
	return remote, result, nil
}

func (e *Engine) MergeAndRetry(ctx context.Context, state *SyncState) error {
	if state == nil {
		return fmt.Errorf("sync state required")
	}

	local := e.vault.Data
	remote, result, err := e.PullLatest(ctx, state)
	if err != nil {
		return err
	}

	base, err := e.decodeBaseBlob(state.LastSyncedBaseBlob)
	if err != nil {
		return err
	}

	original := e.vault.Data
	e.vault.Data = MergeThreeWay(base, local, remote, e.vault.DeviceID(), remote.Metadata.DeviceID)
	if err := e.vault.Save(); err != nil {
		e.vault.Data = original
		return err
	}

	state.LastKnownServerVersion = result.Version
	return e.PushCurrent(ctx, state)
}

func (e *Engine) ReconcileOnLink(ctx context.Context, state *SyncState, userID, serverURL string) error {
	if state == nil {
		return fmt.Errorf("sync state required")
	}

	local := e.vault.Data
	remote, result, remoteExists, err := e.fetchRemote(ctx)
	if err != nil {
		return err
	}

	merged, action, err := DecideFirstLinkAction(*state, userID, local, remote, remoteExists)
	if err != nil {
		return err
	}

	state.LinkedUserID = userID
	state.ServerURL = serverURL

	switch action {
	case FirstLinkNoop:
		return nil
	case FirstLinkAdoptRemote:
		original := e.vault.Data
		e.vault.Data = merged
		if err := e.vault.Save(); err != nil {
			e.vault.Data = original
			return err
		}
		if remoteExists {
			state.LastKnownServerVersion = result.Version
			state.LastSuccessfulPullAt = time.Now().UTC()
			state.LastSyncedBaseBlob = append([]byte(nil), result.Blob...)
			state.LastSyncedHash = hashBlob(result.Blob)
			state.Dirty = false
			state.LastError = ""
			state.NextRetryAt = time.Time{}
		}
		return nil
	case FirstLinkPushLocal:
		state.LastKnownServerVersion = 0
		state.MarkDirty("", time.Time{})
		return e.PushCurrent(ctx, state)
	case FirstLinkMergeAndPush:
		original := e.vault.Data
		e.vault.Data = merged
		if err := e.vault.Save(); err != nil {
			e.vault.Data = original
			return err
		}
		if remoteExists {
			state.LastKnownServerVersion = result.Version
			state.LastSuccessfulPullAt = time.Now().UTC()
			state.LastSyncedBaseBlob = append([]byte(nil), result.Blob...)
			state.LastSyncedHash = hashBlob(result.Blob)
		}
		state.MarkDirty("", time.Time{})
		return e.PushCurrent(ctx, state)
	default:
		return nil
	}
}

func (e *Engine) Pull() error {
	state := DefaultSyncState("")
	_, _, err := e.PullLatest(context.Background(), &state)
	return err
}

func (e *Engine) push() error {
	state := DefaultSyncState("")
	return e.PushCurrent(context.Background(), &state)
}

func (e *Engine) retryWithBackoff(fn func() error) {
	delays := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second,
		60 * time.Second,
		5 * time.Minute,
	}

	for _, delay := range delays {
		time.Sleep(delay)

		if err := fn(); err == nil {
			if e.logger != nil {
				e.logger.Info("sync retry succeeded")
			}
			return
		}
		if e.logger != nil {
			e.logger.Debug("sync retry failed, backing off", "next_delay", delay*2)
		}
	}
	if e.logger != nil {
		e.logger.Warn("sync retries exhausted, will try again next interval")
	}
}

func (e *Engine) decodeBaseBlob(blob []byte) (vault.VaultData, error) {
	if len(blob) == 0 {
		return vault.VaultData{}, nil
	}

	plaintext, err := vault.DecryptCombined(e.vault.Key(), blob)
	if err != nil {
		return vault.VaultData{}, err
	}

	var base vault.VaultData
	if err := json.Unmarshal(plaintext, &base); err != nil {
		return vault.VaultData{}, err
	}
	return base, nil
}

func hashBlob(blob []byte) string {
	sum := sha256.Sum256(blob)
	return hex.EncodeToString(sum[:])
}

func (e *Engine) fetchRemote(ctx context.Context) (vault.VaultData, PullResult, bool, error) {
	_ = ctx

	result, err := e.client.Pull()
	if errors.Is(err, ErrNoRemoteVault) {
		return vault.VaultData{}, PullResult{}, false, nil
	}
	if err != nil {
		return vault.VaultData{}, PullResult{}, false, err
	}

	plaintext, err := vault.DecryptCombined(e.vault.Key(), result.Blob)
	if err != nil {
		return vault.VaultData{}, PullResult{}, false, err
	}

	var remote vault.VaultData
	if err := json.Unmarshal(plaintext, &remote); err != nil {
		return vault.VaultData{}, PullResult{}, false, err
	}

	return remote, result, true, nil
}
