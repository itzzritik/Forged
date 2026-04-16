package readiness

import (
	"fmt"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/vault"
)

func (e *Engine) ensureConfigFile(paths config.Paths) error {
	if e != nil && e.ensureConfig != nil {
		return e.ensureConfig(paths)
	}
	return ensureDefaultConfigFile(paths)
}

func (e *Engine) enableSSHConfig(paths config.Paths) error {
	if e != nil && e.enableSSH != nil {
		return e.enableSSH(paths)
	}
	return config.EnableSSHAgent(paths)
}

func (e *Engine) ensureServiceWithPassword(password []byte) error {
	runtime, err := e.serviceRuntimeSpec()
	if err != nil {
		return err
	}
	if e != nil && e.ensureService != nil {
		return e.ensureService(e.Paths, daemon.ServiceCredentials{
			MasterPassword: string(password),
		}, runtime)
	}
	return daemon.EnsureService(e.Paths, daemon.ServiceCredentials{
		MasterPassword: string(password),
	}, runtime)
}

func (e *Engine) installedServicePassword() (string, error) {
	if e != nil && e.readInstalledServicePassword != nil {
		return e.readInstalledServicePassword()
	}
	return daemon.ReadInstalledServicePassword()
}

func (e *Engine) serviceRuntimeSpec() (daemon.RuntimeSpec, error) {
	if e != nil && e.serviceRuntime != nil {
		return e.serviceRuntime()
	}
	return daemon.DefaultRuntimeSpec()
}

func (e *Engine) waitForServiceReady() (Snapshot, error) {
	retries := 1
	if e != nil && e.serviceRetries > 0 {
		retries = e.serviceRetries
	}

	var last Snapshot
	for attempt := 0; attempt < retries; attempt++ {
		updated, err := e.Assess()
		if err != nil {
			return updated, err
		}
		last = updated
		if serviceHealthy(updated) {
			return updated, nil
		}
		if attempt == retries-1 {
			break
		}
		e.pauseForServiceRetry()
	}

	return last, nil
}

func (e *Engine) pauseForServiceRetry() {
	if e != nil && e.sleep != nil {
		e.sleep()
		return
	}
	time.Sleep(500 * time.Millisecond)
}

func appendUnique(items []string, item string) []string {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func (e *Engine) markFixed(summary *RepairSummary, item string) {
	summary.Fixed = appendUnique(summary.Fixed, item)
}

func (e *Engine) markFailed(summary *RepairSummary, item string) {
	summary.Failed = appendUnique(summary.Failed, item)
}

func serviceHealthy(snapshot Snapshot) bool {
	return snapshot.Service.Installed &&
		snapshot.Service.ConfigValid &&
		snapshot.Service.Running &&
		snapshot.IPCSocketReady &&
		snapshot.AgentSocketReady
}

func serviceNeedsRepair(snapshot Snapshot) bool {
	if !snapshot.VaultExists {
		return false
	}
	return !serviceHealthy(snapshot)
}

func createEmptyVaultForRestore(paths config.Paths, password []byte) error {
	v, err := vault.Create(paths.VaultFile(), password)
	if err != nil {
		return err
	}
	v.Close()
	return nil
}

func passwordUnlocksVault(paths config.Paths, password []byte) (bool, error) {
	if len(password) == 0 {
		return false, nil
	}

	v, err := vault.Open(paths.VaultFile(), password)
	if err == nil {
		v.Close()
		return true, nil
	}

	message := err.Error()
	switch {
	case strings.Contains(message, "message authentication failed"):
		return false, nil
	case strings.Contains(message, "vault is locked by another process"):
		return true, nil
	default:
		return false, fmt.Errorf("verifying master password: %w", err)
	}
}
