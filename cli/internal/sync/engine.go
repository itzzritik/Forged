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

type statusAPI interface {
	Status() (StatusResult, error)
}

type statusContextAPI interface {
	StatusContext(ctx context.Context) (StatusResult, error)
}

type pushContextAPI interface {
	PushContext(ctx context.Context, blob []byte, kdf vault.KDFParams, protectedKey string, expectedVersion int64) (PushResult, error)
}

type pullContextAPI interface {
	PullContext(ctx context.Context) (PullResult, error)
}

type Engine struct {
	vault      *vault.Vault
	client     API
	logger     *slog.Logger
	vaultApply VaultApply
}

// VaultApply commits a local vault update while honoring the operation context.
type VaultApply func(context.Context, func(*vault.VaultData) error) error

func NewEngine(v *vault.Vault, client API, logger *slog.Logger) *Engine {
	return NewEngineWithVaultApply(v, client, logger, nil)
}

func NewEngineWithVaultApply(v *vault.Vault, client API, logger *slog.Logger, vaultApply VaultApply) *Engine {
	if vaultApply == nil {
		vaultApply = func(ctx context.Context, update func(*vault.VaultData) error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return v.UpdateData(update)
		}
	}
	return &Engine{
		vault:      v,
		client:     client,
		logger:     logger,
		vaultApply: vaultApply,
	}
}

func (e *Engine) updateLocal(ctx context.Context, update func(*vault.VaultData) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.vaultApply(ctx, update); err != nil {
		return err
	}
	return nil
}

func (e *Engine) PushCurrent(ctx context.Context, state *SyncState) error {
	if state == nil {
		return fmt.Errorf("Sync state required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	blob, kdf, protectedKeyBytes, err := e.vault.ExportForSync()
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	protectedKey := base64.StdEncoding.EncodeToString(protectedKeyBytes)
	var result PushResult
	if client, ok := e.client.(pushContextAPI); ok {
		result, err = client.PushContext(ctx, blob, kdf, protectedKey, state.LastKnownServerVersion)
	} else {
		result, err = e.client.Push(blob, kdf, protectedKey, state.LastKnownServerVersion)
	}
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	state.MarkClean(result.Version, blob, hashBlob(blob))
	return nil
}

func (e *Engine) PullLatest(ctx context.Context, state *SyncState) (vault.VaultData, PullResult, error) {
	if state == nil {
		return vault.VaultData{}, PullResult{}, fmt.Errorf("Sync state required")
	}
	if err := ctx.Err(); err != nil {
		return vault.VaultData{}, PullResult{}, err
	}

	var result PullResult
	var err error
	if client, ok := e.client.(pullContextAPI); ok {
		result, err = client.PullContext(ctx)
	} else {
		result, err = e.client.Pull()
	}
	if err != nil {
		return vault.VaultData{}, PullResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return vault.VaultData{}, PullResult{}, err
	}

	plaintext, err := e.vault.DecryptSyncBlob(result.Blob)
	if err != nil {
		return vault.VaultData{}, PullResult{}, err
	}

	var remote vault.VaultData
	if err := json.Unmarshal(plaintext, &remote); err != nil {
		return vault.VaultData{}, PullResult{}, err
	}

	now := time.Now().UTC()
	if !state.Dirty {
		if err := e.updateLocal(ctx, func(local *vault.VaultData) error {
			*local = MergeVaults(*local, remote)
			return nil
		}); err != nil {
			return vault.VaultData{}, PullResult{}, err
		}
		state.LastSyncedBaseBlob = append([]byte(nil), result.Blob...)
		state.LastSyncedHash = hashBlob(result.Blob)
		state.LastError = ""
		state.NextRetryAt = time.Time{}
		state.LastKnownServerVersion = result.Version
	}

	state.LastRemoteCheckAt = now
	state.LastSuccessfulPullAt = now
	if !state.Dirty && remoteMetadataMissing(result) {
		if err := e.repairRemoteMetadata(ctx, state, result.Version); err != nil && e.logger != nil {
			e.logger.Warn("repairing missing remote vault metadata failed", "error", err)
		}
	}
	return remote, result, nil
}

func (e *Engine) MergeAndRetry(ctx context.Context, state *SyncState) error {
	if state == nil {
		return fmt.Errorf("Sync state required")
	}

	remote, result, err := e.PullLatest(ctx, state)
	if err != nil {
		return err
	}

	base, err := e.decodeBaseBlob(state.LastSyncedBaseBlob)
	if err != nil {
		return err
	}

	if err := e.updateLocal(ctx, func(local *vault.VaultData) error {
		*local = MergeThreeWay(base, *local, remote, local.Metadata.DeviceID, remote.Metadata.DeviceID)
		return nil
	}); err != nil {
		return err
	}

	state.LastKnownServerVersion = result.Version
	return e.PushCurrent(ctx, state)
}

func (e *Engine) ReconcileOnLink(ctx context.Context, state *SyncState, userID, serverURL string) error {
	if state == nil {
		return fmt.Errorf("Sync state required")
	}

	remote, result, remoteExists, err := e.fetchRemote(ctx)
	if err != nil {
		return err
	}

	var action FirstLinkAction
	if err := e.updateLocal(ctx, func(local *vault.VaultData) error {
		merged, nextAction, err := DecideFirstLinkAction(*state, userID, *local, remote, remoteExists)
		if err != nil {
			return err
		}
		action = nextAction
		switch action {
		case FirstLinkAdoptRemote, FirstLinkMergeAndPush:
			*local = merged
		}
		return nil
	}); err != nil {
		return err
	}

	state.LinkedUserID = userID
	state.ServerURL = serverURL

	switch action {
	case FirstLinkNoop:
		return nil
	case FirstLinkAdoptRemote:
		if remoteExists {
			now := time.Now().UTC()
			state.LastKnownServerVersion = result.Version
			state.LastRemoteCheckAt = now
			state.LastSuccessfulPullAt = now
			state.LastSyncedBaseBlob = append([]byte(nil), result.Blob...)
			state.LastSyncedHash = hashBlob(result.Blob)
			state.Dirty = false
			state.LastError = ""
			state.NextRetryAt = time.Time{}
		}
		return nil
	case FirstLinkPushLocal:
		state.LastKnownServerVersion = 0
		if remoteExists {
			state.LastKnownServerVersion = result.Version
		}
		state.MarkDirty("", time.Time{})
		return e.PushCurrent(ctx, state)
	case FirstLinkMergeAndPush:
		if remoteExists {
			now := time.Now().UTC()
			state.LastKnownServerVersion = result.Version
			state.LastRemoteCheckAt = now
			state.LastSuccessfulPullAt = now
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

func (e *Engine) RemoteStatus(ctx context.Context, state *SyncState) (StatusResult, error) {
	if state == nil {
		return StatusResult{}, fmt.Errorf("Sync state required")
	}
	if checker, ok := e.client.(statusContextAPI); ok {
		return checker.StatusContext(ctx)
	}
	if checker, ok := e.client.(statusAPI); ok {
		_ = ctx
		return checker.Status()
	}
	return StatusResult{}, ErrStatusUnsupported
}

func (e *Engine) decodeBaseBlob(blob []byte) (vault.VaultData, error) {
	if len(blob) == 0 {
		return vault.VaultData{}, nil
	}

	plaintext, err := e.vault.DecryptSyncBlob(blob)
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
	if err := ctx.Err(); err != nil {
		return vault.VaultData{}, PullResult{}, false, err
	}

	var result PullResult
	var err error
	if client, ok := e.client.(pullContextAPI); ok {
		result, err = client.PullContext(ctx)
	} else {
		result, err = e.client.Pull()
	}
	if errors.Is(err, ErrNoRemoteVault) {
		return vault.VaultData{}, PullResult{}, false, nil
	}
	if err != nil {
		return vault.VaultData{}, PullResult{}, false, err
	}
	if err := ctx.Err(); err != nil {
		return vault.VaultData{}, PullResult{}, false, err
	}

	plaintext, err := e.vault.DecryptSyncBlob(result.Blob)
	if err != nil {
		return vault.VaultData{}, PullResult{}, false, err
	}

	var remote vault.VaultData
	if err := json.Unmarshal(plaintext, &remote); err != nil {
		return vault.VaultData{}, PullResult{}, false, err
	}

	return remote, result, true, nil
}

func remoteMetadataMissing(result PullResult) bool {
	return result.KDFParams == nil || result.ProtectedSymmetricKey == nil || *result.ProtectedSymmetricKey == ""
}

func (e *Engine) repairRemoteMetadata(ctx context.Context, state *SyncState, version int64) error {
	if state == nil {
		return fmt.Errorf("Sync state required")
	}

	if e.logger != nil {
		e.logger.Info("remote vault metadata missing, repairing with local metadata", "version", version)
	}

	state.LastKnownServerVersion = version
	return e.PushCurrent(ctx, state)
}
