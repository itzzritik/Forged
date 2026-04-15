package readiness

import (
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
)

func (e *Engine) Repair(snapshot Snapshot) (Snapshot, RepairSummary, error) {
	current := snapshot
	var summary RepairSummary

	if !current.ConfigExists {
		if err := e.ensureConfigFile(e.Paths); err != nil {
			summary.Failed = append(summary.Failed, "config")
		} else {
			updated, err := e.Assess()
			if err != nil {
				return updated, summary, err
			}
			current = updated
			if current.ConfigExists {
				summary.Fixed = append(summary.Fixed, "config")
			} else {
				summary.Failed = append(summary.Failed, "config")
			}
		}
	}

	if (!current.SSHEnabled || !current.ManagedConfigReady) && len(summary.Failed) == 0 {
		if err := e.enableSSHConfig(e.Paths); err != nil {
			summary.Failed = append(summary.Failed, "ssh")
		} else {
			updated, err := e.Assess()
			if err != nil {
				return updated, summary, err
			}
			current = updated
			if current.SSHEnabled && current.ManagedConfigReady {
				summary.Fixed = append(summary.Fixed, "ssh")
			} else {
				summary.Failed = append(summary.Failed, "ssh")
			}
		}
	}

	if current.VaultExists &&
		current.Service.Installed &&
		current.Service.Repairable &&
		(!current.Service.Running || !current.IPCSocketReady || !current.AgentSocketReady) {
		if err := e.restartUserService(); err != nil {
			summary.Failed = append(summary.Failed, "service")
		} else {
			updated, err := e.waitForServiceReady()
			if err != nil {
				return updated, summary, err
			}
			current = updated
			if current.Service.Running && current.IPCSocketReady && current.AgentSocketReady {
				summary.Fixed = append(summary.Fixed, "service")
			} else {
				summary.Failed = append(summary.Failed, "service")
			}
		}
	}

	current.State = classifyState(current)
	return current, summary, nil
}

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

func (e *Engine) restartUserService() error {
	if e != nil && e.restartService != nil {
		return e.restartService()
	}
	return daemon.RestartService()
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
		if updated.Service.Running && updated.IPCSocketReady && updated.AgentSocketReady {
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
