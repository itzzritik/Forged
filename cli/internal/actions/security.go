package actions

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
)

type SecurityState struct {
	MasterPasswordInterval string
	ExternalUsePolicy      string
	SystemAuthCapability   string
	SecureStoreCapability  string
}

func LoadSecurityState(paths config.Paths) (SecurityState, error) {
	cfg, err := config.Load(paths.ConfigFile())
	if err != nil {
		return SecurityState{}, err
	}

	return SecurityState{
		MasterPasswordInterval: config.NormalizeMasterPasswordInterval(cfg.Security.MasterPasswordInterval),
		ExternalUsePolicy:      cfg.Security.ExternalUsePolicy,
		SystemAuthCapability:   string(inspectNativeCapability(helperBinaryPath())),
		SecureStoreCapability:  string(sensitiveauth.NewSecureStore().Capability(context.Background())),
	}, nil
}

func SetMasterPasswordInterval(paths config.Paths, interval string) error {
	return config.SetMasterPasswordInterval(paths, interval)
}

func SetExternalUsePolicy(paths config.Paths, policy string) error {
	return config.SetExternalUsePolicy(paths, policy)
}

func inspectNativeCapability(helperPath string) sensitiveauth.CapabilityState {
	if helperPath == "" {
		return sensitiveauth.CapabilityBroken
	}

	client := sensitiveauth.NewHelperClient(helperPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Start(ctx, nil); err != nil {
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
