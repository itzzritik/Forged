package actions

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	forgedsync "github.com/itzzritik/forged/cli/internal/sync"
	"github.com/itzzritik/forged/cli/internal/vault"
)

type UnlockResult struct {
	PasswordRequired bool
	Prompt           string
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
	return unlockSensitive(paths, password, false)
}

func UnlockSensitiveLaunch(paths config.Paths, password []byte) (UnlockResult, error) {
	return unlockSensitive(paths, password, false)
}

func unlockSensitive(paths config.Paths, password []byte, force bool) (UnlockResult, error) {
	if len(password) == 0 {
		_, err := authorizeSensitiveResultWithOptions(paths, sensitiveauth.ActionView, nil, force)
		switch {
		case err == nil:
			return UnlockResult{}, nil
		case IsSensitiveAuthRequired(err):
			return UnlockResult{PasswordRequired: true, Prompt: unlockPrompt(err, sensitiveauth.ActionView.PasswordPrompt())}, nil
		default:
			return UnlockResult{}, err
		}
	}

	if _, err := authorizeSensitiveResultWithOptions(paths, sensitiveauth.ActionView, password, force); err != nil {
		return UnlockResult{}, err
	}
	_, _ = sensitiveauth.VerifyAndRefreshLocalEnrollment(paths, password)
	return UnlockResult{}, nil
}

func unlockPrompt(err error, fallback string) string {
	var target *SensitiveAuthRequiredError
	if errors.As(err, &target) && strings.TrimSpace(target.Prompt) != "" {
		return strings.TrimSpace(target.Prompt)
	}
	return fallback
}

func ChangePassword(paths config.Paths, currentPassword []byte, newPassword []byte) (ChangePasswordResult, error) {
	check, err := vault.OpenReadOnly(paths.VaultFile(), currentPassword)
	if err != nil {
		return ChangePasswordResult{}, fmt.Errorf("Wrong password or corrupted vault")
	}
	check.Close()

	serviceStopped, err := stopDaemonForPasswordChange(paths)
	if err != nil {
		return ChangePasswordResult{}, err
	}

	passwordChanged := false
	defer func() {
		if serviceStopped && !passwordChanged {
			_ = daemon.StartService()
		}
	}()

	v, err := vault.Open(paths.VaultFile(), currentPassword)
	if err != nil {
		if strings.Contains(err.Error(), "vault is locked by another process") {
			return ChangePasswordResult{}, fmt.Errorf("Vault is busy. Try again.")
		}
		return ChangePasswordResult{}, fmt.Errorf("Wrong password or corrupted vault")
	}
	closed := false
	defer func() {
		if !closed {
			v.Close()
		}
	}()

	if err := v.ChangePassword(newPassword); err != nil {
		return ChangePasswordResult{}, fmt.Errorf("Changing password: %w", err)
	}
	passwordChanged = true
	_ = sensitiveauth.InvalidateLocalEnrollment(paths)

	kdf := v.KDFParams()
	protectedKey := base64.StdEncoding.EncodeToString(v.ProtectedKeyBytes())
	enrollmentResult, enrollmentErr := sensitiveauth.RefreshLocalEnrollment(paths, v.Key())
	v.Close()
	closed = true

	runtime, err := daemon.DefaultRuntimeSpec()
	if err != nil {
		result := ChangePasswordResult{
			Detail: "Local vault updated. The local service could not be refreshed automatically. Run Forged Doctor to finish setup.",
		}
		applyEnrollmentDetail(&result, enrollmentResult, enrollmentErr)
		return result, nil
	}
	if err := daemon.EnsureService(paths, runtime); err != nil {
		result := ChangePasswordResult{
			Detail: "Local vault updated. The local service needs repair with your new password. Run Forged Doctor to finish setup.",
		}
		applyEnrollmentDetail(&result, enrollmentResult, enrollmentErr)
		return result, nil
	}

	creds, err := LoadFreshCredentials(context.Background(), paths)
	if err != nil {
		result := ChangePasswordResult{
			Detail: "Local vault updated. Log in and sync later to update recovery.",
		}
		applyEnrollmentDetail(&result, enrollmentResult, enrollmentErr)
		return result, nil
	}

	client := forgedsync.NewClient(creds.ServerURL, creds.Token, "")
	if err := client.Rekey(kdf, protectedKey); err != nil {
		result := ChangePasswordResult{
			Detail: "Local vault updated. Remote recovery was not updated. Run sync later to retry.",
		}
		applyEnrollmentDetail(&result, enrollmentResult, enrollmentErr)
		return result, nil
	}

	result := ChangePasswordResult{
		Detail: "Local vault and remote recovery were updated.",
		Synced: true,
	}
	applyEnrollmentDetail(&result, enrollmentResult, enrollmentErr)
	return result, nil
}

func stopDaemonForPasswordChange(paths config.Paths) (bool, error) {
	if _, running := daemon.IsRunning(paths); !running {
		return false, nil
	}

	if !daemon.ServiceInstalled() {
		return false, fmt.Errorf("Stop the running Forged daemon and try again.")
	}

	if err := daemon.StopService(); err != nil {
		return false, fmt.Errorf("Stopping local service: %w", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, running := daemon.IsRunning(paths); !running {
			return true, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return false, fmt.Errorf("Waiting for local service to stop")
}

func applyEnrollmentDetail(result *ChangePasswordResult, enrollment sensitiveauth.EnrollmentResult, err error) {
	switch {
	case err != nil:
		result.Detail += " Local unlock trust could not be refreshed."
	case enrollment.Refreshed:
		return
	case enrollment.Reason != "":
		result.Detail += " Local unlock trust needs re-enrollment."
	}
}
