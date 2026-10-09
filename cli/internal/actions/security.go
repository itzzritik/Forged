package actions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
)

type SecurityState struct {
	MasterPasswordInterval string
	SystemAuthCapability   string
	SecureStoreCapability  string
	HeadlessUnlock         bool
	HeadlessSupported      bool
	HeadlessOffered        bool
}

func LoadSecurityState(paths config.Paths) (SecurityState, error) {
	cfg, err := config.Load(paths.ConfigFile())
	if err != nil {
		return SecurityState{}, err
	}

	return SecurityState{
		MasterPasswordInterval: config.NormalizeMasterPasswordInterval(cfg.Security.MasterPasswordInterval),
		SystemAuthCapability:   string(inspectNativeCapability(helperBinaryPath())),
		SecureStoreCapability:  string(sensitiveauth.NewSecureStore(paths).Capability(context.Background())),
		HeadlessUnlock:         sensitiveauth.HeadlessModeEnabled(paths),
		HeadlessSupported:      sensitiveauth.HeadlessModeSupported(paths),
		HeadlessOffered:        cfg.Security.HeadlessOffered,
	}, nil
}

func SetMasterPasswordInterval(paths config.Paths, interval string) error {
	return config.SetMasterPasswordInterval(paths, interval)
}

func EnableHeadlessUnlock(paths config.Paths) (enrolled bool, err error) {
	if !sensitiveauth.HeadlessModeSupported(paths) {
		return false, fmt.Errorf("Headless mode is not supported on this platform")
	}
	if err := config.SetHeadlessUnlock(paths, true); err != nil {
		return false, fmt.Errorf("Saving headless mode: %w", err)
	}
	_, err = ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdSensitiveEnroll, nil)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, ipc.ErrDaemonNotRunning), err.Error() == sensitiveauth.ErrLocked.Error(),
		strings.HasPrefix(err.Error(), "Unknown command"):
		return false, nil
	}
	return false, fmt.Errorf("Enrolling headless unlock: %w", err)
}

func DisableHeadlessUnlock(paths config.Paths) error {
	if err := config.SetHeadlessUnlock(paths, false); err != nil {
		return fmt.Errorf("Saving headless mode: %w", err)
	}
	if err := sensitiveauth.InvalidateHeadlessEnrollment(paths); err != nil {
		return fmt.Errorf("Removing headless unlock trust: %w", err)
	}
	return nil
}

func inspectNativeCapability(helperPath string) sensitiveauth.CapabilityState {
	if helperPath == "" {
		return sensitiveauth.CapabilityBroken
	}

	client := sensitiveauth.NewHelperClient(helperPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Start(ctx, nil, nil); err != nil {
		return sensitiveauth.CapabilityBroken
	}
	defer client.Close()

	capability, err := client.Capability(ctx)
	if err != nil {
		return sensitiveauth.CapabilityBroken
	}
	return capability
}

func helperBinaryPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}

	name := "forged-auth"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(filepath.Dir(exe), name)
}
