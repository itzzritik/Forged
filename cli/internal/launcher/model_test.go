package launcher

import (
	"testing"

	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/readiness"
)

func TestBuildMenuOrdersLoginLastWhenVaultIsEmpty(t *testing.T) {
	t.Parallel()

	menu := BuildMenu(readiness.Snapshot{
		State:              readiness.StateReadyEmpty,
		KeyCount:           0,
		LoggedIn:           false,
		VaultExists:        true,
		ConfigExists:       true,
		Service:            daemon.ServiceStatus{Installed: true, ConfigValid: true, Loaded: true, Running: true},
		IPCSocketReady:     true,
		AgentSocketReady:   true,
		SSHEnabled:         true,
		ManagedConfigReady: true,
	})

	if len(menu) != 4 {
		t.Fatalf("expected 4 menu items, got %d", len(menu))
	}
	if menu[0].ID != ActionGenerate || menu[0].Label != "Generate your first key" {
		t.Fatalf("unexpected first item: %#v", menu[0])
	}
	if menu[len(menu)-1].ID != ActionLogin {
		t.Fatalf("expected login last, got %#v", menu)
	}
}

func TestBuildMenuOrdersLoginFirstWhenKeysExist(t *testing.T) {
	t.Parallel()

	menu := BuildMenu(readiness.Snapshot{
		State:              readiness.StateReady,
		KeyCount:           2,
		LoggedIn:           false,
		VaultExists:        true,
		ConfigExists:       true,
		Service:            daemon.ServiceStatus{Installed: true, ConfigValid: true, Loaded: true, Running: true},
		IPCSocketReady:     true,
		AgentSocketReady:   true,
		SSHEnabled:         true,
		ManagedConfigReady: true,
	})

	if len(menu) != 4 {
		t.Fatalf("expected 4 menu items, got %d", len(menu))
	}
	if menu[0].ID != ActionLogin {
		t.Fatalf("expected login first, got %#v", menu[0])
	}
	if menu[1].ID != ActionGenerate || menu[1].Label != "Generate a new key" {
		t.Fatalf("unexpected second item: %#v", menu[1])
	}
}

func TestBuildMenuHidesLoginWhenAlreadyAuthenticated(t *testing.T) {
	t.Parallel()

	menu := BuildMenu(readiness.Snapshot{
		State:              readiness.StateReady,
		KeyCount:           1,
		LoggedIn:           true,
		VaultExists:        true,
		ConfigExists:       true,
		Service:            daemon.ServiceStatus{Installed: true, ConfigValid: true, Loaded: true, Running: true},
		IPCSocketReady:     true,
		AgentSocketReady:   true,
		SSHEnabled:         true,
		ManagedConfigReady: true,
	})

	for _, item := range menu {
		if item.ID == ActionLogin {
			t.Fatalf("did not expect login item when already authenticated: %#v", menu)
		}
	}
}
