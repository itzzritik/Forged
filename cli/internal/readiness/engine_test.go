package readiness

import (
	"testing"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
)

func TestClassifyState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Snapshot
		want State
	}{
		{
			name: "no vault is uninitialized",
			in:   Snapshot{},
			want: StateUninitialized,
		},
		{
			name: "healthy empty vault is ready-empty",
			in: Snapshot{
				VaultExists:        true,
				ConfigExists:       true,
				Service:            daemon.ServiceStatus{Installed: true, ConfigValid: true, Loaded: true, Running: true, Repairable: true},
				IPCSocketReady:     true,
				AgentSocketReady:   true,
				SSHEnabled:         true,
				ManagedConfigReady: true,
				KeyCount:           0,
			},
			want: StateReadyEmpty,
		},
		{
			name: "healthy vault with keys is ready",
			in: Snapshot{
				VaultExists:        true,
				ConfigExists:       true,
				Service:            daemon.ServiceStatus{Installed: true, ConfigValid: true, Loaded: true, Running: true, Repairable: true},
				IPCSocketReady:     true,
				AgentSocketReady:   true,
				SSHEnabled:         true,
				ManagedConfigReady: true,
				KeyCount:           2,
			},
			want: StateReady,
		},
		{
			name: "invalid service is blocked",
			in: Snapshot{
				VaultExists:  true,
				ConfigExists: true,
				Service: daemon.ServiceStatus{
					Installed:   true,
					ConfigValid: false,
					Repairable:  false,
				},
			},
			want: StateBlocked,
		},
		{
			name: "dead ipc keeps machine degraded",
			in: Snapshot{
				VaultExists:        true,
				ConfigExists:       true,
				Service:            daemon.ServiceStatus{Installed: true, ConfigValid: true, Loaded: true, Running: true, Repairable: true},
				IPCSocketReady:     false,
				AgentSocketReady:   true,
				SSHEnabled:         true,
				ManagedConfigReady: true,
				KeyCount:           1,
			},
			want: StateDegraded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := classifyState(tt.in); got != tt.want {
				t.Fatalf("classifyState() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyStateIgnoresLoginState(t *testing.T) {
	t.Parallel()

	base := Snapshot{
		VaultExists:        true,
		ConfigExists:       true,
		Service:            daemon.ServiceStatus{Installed: true, ConfigValid: true, Loaded: true, Running: true, Repairable: true},
		IPCSocketReady:     true,
		AgentSocketReady:   true,
		SSHEnabled:         true,
		ManagedConfigReady: true,
		KeyCount:           1,
	}

	loggedOut := base
	loggedIn := base
	loggedIn.LoggedIn = true

	if got := classifyState(loggedOut); got != StateReady {
		t.Fatalf("logged out classifyState() = %q, want %q", got, StateReady)
	}
	if got := classifyState(loggedIn); got != StateReady {
		t.Fatalf("logged in classifyState() = %q, want %q", got, StateReady)
	}
}

func TestRepairMarksConfigFixedOnlyAfterReassessment(t *testing.T) {
	t.Parallel()

	configExists := false
	engine := &Engine{
		Paths: config.Paths{},
		statPath: func(path string) bool {
			return configExists
		},
		inspectService: func(paths config.Paths) (daemon.ServiceStatus, error) {
			return daemon.ServiceStatus{Repairable: true}, nil
		},
		isRunning:    func(paths config.Paths) (int, bool) { return 0, false },
		socketReady:  func(path string) bool { return false },
		isSSHEnabled: func(paths config.Paths) bool { return true },
		detectOwner: func(paths config.Paths) (config.SSHAgentOwner, error) {
			return config.SSHAgentOwner{Name: "None"}, nil
		},
		loadCredentials: func(path string) (bool, error) { return false, nil },
		loadKeyCount:    func(socketPath string) (int, error) { return 0, nil },
		ensureConfig: func(paths config.Paths) error {
			configExists = true
			return nil
		},
		enableSSH:      func(paths config.Paths) error { return nil },
		restartService: func() error { return nil },
	}

	snapshot, summary, err := engine.Repair(Snapshot{})
	if err != nil {
		t.Fatalf("Repair() error = %v", err)
	}

	if snapshot.ConfigExists != true {
		t.Fatalf("expected config to exist after repair")
	}
	if len(summary.Fixed) != 1 || summary.Fixed[0] != "config" {
		t.Fatalf("unexpected fixed summary: %#v", summary.Fixed)
	}
	if len(summary.Failed) != 0 {
		t.Fatalf("unexpected failed summary: %#v", summary.Failed)
	}
}

func TestRepairDoesNotClaimConfigFixedWhenReassessmentStillFails(t *testing.T) {
	t.Parallel()

	engine := &Engine{
		Paths: config.Paths{},
		statPath: func(path string) bool {
			return false
		},
		inspectService: func(paths config.Paths) (daemon.ServiceStatus, error) {
			return daemon.ServiceStatus{Repairable: true}, nil
		},
		isRunning:    func(paths config.Paths) (int, bool) { return 0, false },
		socketReady:  func(path string) bool { return false },
		isSSHEnabled: func(paths config.Paths) bool { return true },
		detectOwner: func(paths config.Paths) (config.SSHAgentOwner, error) {
			return config.SSHAgentOwner{Name: "None"}, nil
		},
		loadCredentials: func(path string) (bool, error) { return false, nil },
		loadKeyCount:    func(socketPath string) (int, error) { return 0, nil },
		ensureConfig: func(paths config.Paths) error {
			return nil
		},
		enableSSH:      func(paths config.Paths) error { return nil },
		restartService: func() error { return nil },
	}

	_, summary, err := engine.Repair(Snapshot{})
	if err != nil {
		t.Fatalf("Repair() error = %v", err)
	}

	if len(summary.Fixed) != 0 {
		t.Fatalf("unexpected fixed summary: %#v", summary.Fixed)
	}
	if len(summary.Failed) != 1 || summary.Failed[0] != "config" {
		t.Fatalf("unexpected failed summary: %#v", summary.Failed)
	}
}
