package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/buildinfo"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
)

type RuntimeSpec struct {
	Binary  string
	Args    []string
	BuildID string
}

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
	runtime, err := normalizeRuntimeSpec(runtime)
	if err != nil {
		return err
	}
	if err := InstallService(paths, runtime); err != nil {
		return err
	}
	return RestartService()
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

func RefreshInstalledServiceIfStale(paths config.Paths, runtime RuntimeSpec) (bool, error) {
	runtime, err := normalizeRuntimeSpec(runtime)
	if err != nil {
		return false, err
	}
	if !ServiceInstalled() {
		return false, nil
	}

	fresh, err := ServiceFresh(paths, runtime.BuildID)
	if err == nil && fresh {
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
	if !status.Installed || !status.ConfigValid || !status.Running {
		return false, nil
	}

	buildID, err := RunningBuildID(paths)
	if err != nil {
		return false, err
	}
	return buildID != "" && buildID == expectedBuildID, nil
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
	expectedBuildID = strings.TrimSpace(expectedBuildID)
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		buildID, err := RunningBuildID(paths)
		if err == nil && buildID != "" && buildID == expectedBuildID {
			return nil
		}
		if err != nil {
			lastErr = err
		}
		time.Sleep(150 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("waiting for fresh daemon: %w", lastErr)
	}
	return fmt.Errorf("waiting for fresh daemon build %s", expectedBuildID)
}
