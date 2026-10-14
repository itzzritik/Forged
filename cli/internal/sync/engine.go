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

type rekeyContextAPI interface {
	RekeyContext(ctx context.Context, kdf vault.KDFParams, protectedKey string) error
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
	e.syncWrap(ctx, nil, nil)

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
	if errors.Is(err, vault.ErrDecrypt) {
		err = fmt.Errorf("%w: %w", ErrRemoteKeyMismatch, err)
	}
	if err != nil {
		return vault.VaultData{}, PullResult{}, err
	}

	var remote vault.VaultData
	if err := json.Unmarshal(plaintext, &remote); err != nil {
		return vault.VaultData{}, PullResult{}, err
	}
	state.RemoteMissing = false
	e.syncWrap(ctx, result.KDFParams, result.ProtectedSymmetricKey)

	now := time.Now().UTC()
	if !state.Dirty {
		// Merge against the last synced base: a plain union would bring back keys deleted
		// elsewhere. A remote whose version went backwards was recreated, so its base no
		// longer applies and missing keys there are not deletions.
		base, baseErr := e.decodeBaseBlob(state.LastSyncedBaseBlob)
		useBase := baseErr == nil && len(state.LastSyncedBaseBlob) > 0 && result.Version >= state.LastKnownServerVersion
		if err := e.updateLocal(ctx, func(local *vault.VaultData) error {
			if useBase {
				*local = MergeThreeWay(base, *local, remote, local.Metadata.DeviceID, remote.Metadata.DeviceID)
			} else {
				*local = MergeVaults(*local, remote)
			}
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
		if err := e.repairRemoteMetadata(ctx, state, result.Version); err != nil {
			err = fmt.Errorf("repairing missing remote vault metadata: %w", err)
			state.MarkDirty(err.Error(), time.Time{})
			return remote, result, err
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
	if result.Version < state.LastKnownServerVersion {
		base = vault.VaultData{}
	}

	if err := e.updateLocal(ctx, func(local *vault.VaultData) error {
		*local = MergeThreeWay(base, *local, remote, local.Metadata.DeviceID, remote.Metadata.DeviceID)
		return nil
	}); err != nil {
		return err
	}

	// The remote just merged is the base from now on, even if this push fails: keeping the older
	// base would make the next merge see that remote's edits as this device's.
	state.LastKnownServerVersion = result.Version
	state.LastSyncedBaseBlob = append([]byte(nil), result.Blob...)
	state.LastSyncedHash = hashBlob(result.Blob)
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
	var status StatusResult
	var err error
	if checker, ok := e.client.(statusContextAPI); ok {
		status, err = checker.StatusContext(ctx)
	} else if checker, ok := e.client.(statusAPI); ok {
		status, err = checker.Status()
	} else {
		return StatusResult{}, ErrStatusUnsupported
	}
	if err == nil && status.HasVault {
		e.syncWrap(ctx, status.KDFParams, status.ProtectedSymmetricKey)
	}
	return status, err
}

// syncWrap uploads this device's password change while it is still pending, otherwise stages a
// different server wrap for adoption at the next password unlock. Pushes no longer carry the wrap.
func (e *Engine) syncWrap(ctx context.Context, remoteKDF *kdfParamsJSON, remoteProtected *string) {
	path := e.vault.Path()
	if vault.RekeyPending(path) {
		rekeyer, ok := e.client.(rekeyContextAPI)
		if !ok {
			return
		}
		protectedKey := base64.StdEncoding.EncodeToString(e.vault.ProtectedKeyBytes())
		if err := rekeyer.RekeyContext(ctx, e.vault.KDFParams(), protectedKey); err != nil {
			e.logWrap("retrying password change upload failed", err)
			return
		}
		e.logWrap("clearing uploaded password change marker", vault.SetRekeyPending(path, false))
		return
	}
	if remoteKDF == nil || remoteProtected == nil || *remoteProtected == "" {
		return
	}
	kdf, err := remoteKDF.params()
	if err != nil {
		e.logWrap("ignoring remote wrap", err)
		return
	}
	protectedKey, err := base64.StdEncoding.DecodeString(*remoteProtected)
	if err != nil {
		e.logWrap("ignoring remote wrap", err)
		return
	}
	e.logWrap("staging remote wrap", e.vault.StagePendingWrap(kdf, protectedKey))
}

func (e *Engine) logWrap(msg string, err error) {
	if err != nil && e.logger != nil {
		e.logger.Warn(msg, "error", err)
	}
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
	if errors.Is(err, vault.ErrDecrypt) {
		err = fmt.Errorf("%w: %w", ErrRemoteKeyMismatch, err)
	}
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
