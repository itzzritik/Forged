package actions

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	forgedsync "github.com/itzzritik/forged/cli/internal/sync"
	"github.com/itzzritik/forged/cli/internal/vault"
)

type UnlockResult struct {
	PasswordRequired bool
}

type ChangePasswordResult struct {
	Detail string
	Synced bool
}

func LockSensitive(paths config.Paths) error {
	_, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdSensitiveLock, nil)
	if err != nil && strings.Contains(err.Error(), "daemon is not running") {
		return nil
	}
	return err
}

func UnlockSensitive(paths config.Paths, password []byte) (UnlockResult, error) {
	if len(password) == 0 {
		_, err := authorizeSensitiveResult(paths, sensitiveauth.ActionView, nil)
		switch {
		case err == nil:
			return UnlockResult{}, nil
		case IsSensitiveAuthRequired(err):
			return UnlockResult{PasswordRequired: true}, nil
		case strings.Contains(err.Error(), "authentication canceled"):
			return UnlockResult{PasswordRequired: true}, nil
		case strings.Contains(err.Error(), "authentication failed"):
			return UnlockResult{PasswordRequired: true}, nil
		default:
			return UnlockResult{}, err
		}
	}

	if _, err := authorizeSensitiveResult(paths, sensitiveauth.ActionView, password); err != nil {
		return UnlockResult{}, err
	}
	return UnlockResult{}, nil
}

func ChangePassword(paths config.Paths, currentPassword []byte, newPassword []byte) (ChangePasswordResult, error) {
	v, err := vault.Open(paths.VaultFile(), currentPassword)
	if err != nil {
		return ChangePasswordResult{}, fmt.Errorf("wrong password or corrupted vault")
	}
	closed := false
	defer func() {
		if !closed {
			v.Close()
		}
	}()

	if err := v.ChangePassword(newPassword); err != nil {
		return ChangePasswordResult{}, fmt.Errorf("changing password: %w", err)
	}

	kdf := v.KDFParams()
	protectedKey := base64.StdEncoding.EncodeToString(v.ProtectedKeyBytes())
	v.Close()
	closed = true

	runtime, err := daemon.DefaultRuntimeSpec()
	if err != nil {
		return ChangePasswordResult{
			Detail: "Local vault updated. The local service could not be refreshed automatically. Run Forged Doctor to finish setup.",
		}, nil
	}
	if err := daemon.EnsureService(paths, daemon.ServiceCredentials{
		MasterPassword: string(newPassword),
	}, runtime); err != nil {
		return ChangePasswordResult{
			Detail: "Local vault updated. The local service needs repair with your new password. Run Forged Doctor to finish setup.",
		}, nil
	}

	creds, err := LoadCredentials(paths)
	if err != nil {
		return ChangePasswordResult{
			Detail: "Local vault updated. Sign in and sync later to update recovery.",
		}, nil
	}

	client := forgedsync.NewClient(creds.ServerURL, creds.Token, "")
	if err := client.Rekey(kdf, protectedKey); err != nil {
		return ChangePasswordResult{
			Detail: "Local vault updated. Remote recovery was not updated. Run sync later to retry.",
		}, nil
	}

	return ChangePasswordResult{
		Detail: "Local vault and remote recovery were updated.",
		Synced: true,
	}, nil
}
