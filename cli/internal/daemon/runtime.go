package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/buildinfo"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/platform"
)

type RuntimeSpec struct {
	Binary  string
	Args    []string
	BuildID string
}

var ErrDaemonServiceOwnership = errors.New("running Forged daemon is not owned by the installed service")

func DefaultRuntimeSpec() (RuntimeSpec, error) {
	binary, err := findBinary()
	if err != nil {
		return RuntimeSpec{}, err
	}
	return normalizeRuntimeSpec(RuntimeSpec{
		Binary:  binary,
		Args:    []string{"daemon"},
		BuildID: buildinfo.CurrentID(),
	})
}

func EnsureService(paths config.Paths, runtime RuntimeSpec) error {
	if err := paths.ValidateRuntimePaths(); err != nil {
		return err
	}
	runtime, err := normalizeRuntimeSpec(runtime)
	if err != nil {
		return err
	}
	if err := RequireServiceOwnership(paths); err != nil {
		return err
	}
	if err := InstallService(paths, runtime); err != nil {
		return err
	}
	if err := RequireServiceOwnership(paths); err != nil {
		return err
	}
	return RestartService()
}

func RequireServiceOwnership(paths config.Paths) error {
	return requireServiceOwnership(paths, nil)
}

// status, when set, is an inspection the caller just made.
func requireServiceOwnership(paths config.Paths, status *ServiceStatus) error {
	daemonPID, running := IsRunning(paths)
	if !running {
		if platform.IsSocketAlive(paths.CtlSocket()) {
			return fmt.Errorf("%w; stop the running Forged daemon or service and try again", ErrDaemonServiceOwnership)
		}
		return nil
	}

	if status == nil {
		inspected, err := InspectService(paths)
		if err != nil {
			return fmt.Errorf("Inspecting local service: %w", err)
		}
		status = &inspected
	}
	if status.Installed && status.PIDKnown && status.PID == daemonPID {
		return nil
	}
	return fmt.Errorf("%w; stop the running Forged daemon or service and try again", ErrDaemonServiceOwnership)
}

func normalizeRuntimeSpec(runtime RuntimeSpec) (RuntimeSpec, error) {
	if runtime.Binary == "" {
		defaultRuntime, err := DefaultRuntimeSpec()
		if err != nil {
			return RuntimeSpec{}, err
		}
		runtime.Binary = defaultRuntime.Binary
		if len(runtime.Args) == 0 {
			runtime.Args = append([]string(nil), defaultRuntime.Args...)
		}
	}
	if len(runtime.Args) == 0 {
		runtime.Args = []string{"daemon"}
	}
	if strings.TrimSpace(runtime.BuildID) == "" {
		runtime.BuildID = buildinfo.CurrentID()
	}
	return runtime, nil
}

func validateDaemonServiceCommand(binary string, args []string) error {
	if strings.TrimSpace(binary) == "" {
		return errors.New("service has no executable")
	}
	if len(args) != 1 || args[0] != "daemon" {
		return errors.New("service does not invoke daemon exactly")
	}
	return nil
}

func RefreshInstalledServiceIfStale(paths config.Paths, runtime RuntimeSpec) (bool, error) {
	runtime, err := normalizeRuntimeSpec(runtime)
	if err != nil {
		return false, err
	}
	installed, err := ServiceInstalled()
	if err != nil {
		return false, fmt.Errorf("checking installed service: %w", err)
	}
	if !installed {
		return false, nil
	}

	fresh, err := ServiceFresh(paths, runtime.BuildID)
	if err == nil && fresh {
		if err := WaitForServiceReady(paths, runtime.BuildID, 8*time.Second); err != nil {
			return false, err
		}
		return false, nil
	}
	if errors.Is(err, ipc.ErrDaemonIdentity) {
		return false, fmt.Errorf("verifying daemon identity: %w", err)
	}
	if errors.Is(err, ErrDaemonServiceOwnership) {
		if err := WaitForBuildID(paths, runtime.BuildID, 8*time.Second); err != nil {
			return false, err
		}
		return false, nil
	}

	if err := EnsureService(paths, runtime); err != nil {
		return false, err
	}
	if err := WaitForBuildID(paths, runtime.BuildID, 8*time.Second); err != nil {
		return true, err
	}
	return true, nil
}

func ServiceFresh(paths config.Paths, expectedBuildID string) (bool, error) {
	expectedBuildID = strings.TrimSpace(expectedBuildID)
	if expectedBuildID == "" {
		expectedBuildID = buildinfo.CurrentID()
	}

	status, err := InspectService(paths)
	if err != nil {
		return false, err
	}
	// Older installs ran the service straight from the npm install.
	if !status.Installed || !status.ConfigValid || !status.Running || !InInstallDir(status.BinaryPath) {
		return false, nil
	}

	buildID, err := RunningBuildID(paths)
	if err != nil {
		return false, err
	}
	if buildID == "" || buildID != expectedBuildID {
		return false, nil
	}
	if err := requireServiceOwnership(paths, &status); err != nil {
		return false, err
	}
	return true, nil
}

func RunningBuildID(paths config.Paths) (string, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdStatus, nil)
	if err != nil {
		return "", err
	}

	var status struct {
		BuildID string `json:"build_id"`
		Build   struct {
			ID string `json:"id"`
		} `json:"build"`
	}
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return "", fmt.Errorf("parsing daemon status: %w", err)
	}
	if id := strings.TrimSpace(status.BuildID); id != "" {
		return id, nil
	}
	return strings.TrimSpace(status.Build.ID), nil
}

func WaitForBuildID(paths config.Paths, expectedBuildID string, timeout time.Duration) error {
	return WaitForServiceReady(paths, expectedBuildID, timeout)
}

func WaitForServiceReady(paths config.Paths, expectedBuildID string, timeout time.Duration) error {
	expectedBuildID = strings.TrimSpace(expectedBuildID)
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		status, err := InspectService(paths)
		if err != nil {
			lastErr = err
			time.Sleep(150 * time.Millisecond)
			continue
		}
		if !status.Installed || !status.ConfigValid || !status.Running ||
			!platform.IsSocketAlive(paths.CtlSocket()) || !platform.IsSocketAlive(paths.AgentSocket()) {
			time.Sleep(150 * time.Millisecond)
			continue
		}

		buildID, err := RunningBuildID(paths)
		if err != nil {
			lastErr = err
			time.Sleep(150 * time.Millisecond)
			continue
		}
		if buildID == "" || (expectedBuildID != "" && buildID != expectedBuildID) {
			time.Sleep(150 * time.Millisecond)
			continue
		}
		if err := requireServiceOwnership(paths, &status); err != nil {
			lastErr = err
			time.Sleep(150 * time.Millisecond)
			continue
		}
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("waiting for ready daemon: %w", lastErr)
	}
	if expectedBuildID != "" {
		return fmt.Errorf("waiting for ready daemon build %s", expectedBuildID)
	}
	return fmt.Errorf("waiting for ready daemon service")
}
