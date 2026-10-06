package readiness

import (
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
)

const serviceReadyTimeout = 8 * time.Second

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

func (e *Engine) ensureServiceInstalled() error {
	runtime, err := e.serviceRuntimeSpec()
	if err != nil {
		return err
	}
	if e != nil && e.ensureService != nil {
		return e.ensureService(e.Paths, runtime)
	}
	return daemon.EnsureService(e.Paths, runtime)
}

func (e *Engine) serviceRuntimeSpec() (daemon.RuntimeSpec, error) {
	if e != nil && e.serviceRuntime != nil {
		return e.serviceRuntime()
	}
	return daemon.DefaultRuntimeSpec()
}

func (e *Engine) waitForServiceReady() (Snapshot, error) {
	return e.waitForServiceReadyWhile(nil)
}

func (e *Engine) waitForServiceReadyWhile(keepWaiting func(Snapshot) bool) (Snapshot, error) {
	retries := 1
	if e != nil && e.serviceRetries > 0 {
		retries = e.serviceRetries
	}
	deadline := time.Now().Add(serviceReadyTimeout)

	var last Snapshot
	for attempt := 0; attempt < retries; attempt++ {
		if attempt > 0 && !time.Now().Before(deadline) {
			break
		}
		updated, err := e.Assess()
		if err != nil {
			return updated, err
		}
		last = updated
		if serviceHealthy(updated) {
			return updated, nil
		}
		if keepWaiting != nil && !keepWaiting(updated) {
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
		serviceOwnsDaemon(snapshot) &&
		serviceBuildFresh(snapshot) &&
		serviceRunsInstalledCopy(snapshot) &&
		snapshot.IPCSocketReady &&
		snapshot.AgentSocketReady
}

func serviceNeedsRepair(snapshot Snapshot) bool {
	if !snapshot.VaultExists {
		return false
	}
	return !serviceHealthy(snapshot)
}

func serviceMayBeBooting(snapshot Snapshot) bool {
	if !snapshot.Service.Installed ||
		!snapshot.Service.ConfigValid ||
		!snapshot.Service.Repairable ||
		snapshot.Service.BinaryMissing ||
		!snapshot.Service.Running {
		return false
	}
	if snapshot.Service.PID > 0 && snapshot.DaemonPID > 0 && snapshot.Service.PID != snapshot.DaemonPID {
		return false
	}
	currentBuild := strings.TrimSpace(snapshot.CurrentBuildID)
	daemonBuild := strings.TrimSpace(snapshot.DaemonBuildID)
	if currentBuild != "" && daemonBuild != "" && daemonBuild != currentBuild {
		return false
	}
	return !snapshot.IPCSocketReady ||
		!snapshot.AgentSocketReady ||
		(snapshot.Service.PID > 0 && snapshot.DaemonPID == 0) ||
		(currentBuild != "" && daemonBuild == "")
}
