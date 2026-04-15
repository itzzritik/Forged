package cmd

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootVersionFlagsPrintVersionWithoutVersionCommand(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-v"}} {
		cmd := newRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs(args)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v", args, err)
		}
		if !strings.Contains(buf.String(), "forged ") {
			t.Fatalf("missing version output for %v: %s", args, buf.String())
		}
	}
}

func TestRewriteCLIErrorSuggestsReplacementForRemovedCommands(t *testing.T) {
	err := rewriteCLIError(fmt.Errorf("unknown command \"start\" for \"forged\""))
	if !strings.Contains(err.Error(), "Run `forged` or `forged doctor --fix`") {
		t.Fatalf("unexpected error rewrite: %v", err)
	}
}

func TestVisibleRootCommandsMatchPublicContract(t *testing.T) {
	cmd := newRootCmd()
	got := visibleSubcommandNames(cmd)
	want := []string{"agent", "doctor", "export", "help", "import", "key", "login", "logout", "sync", "vault"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visible root command mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestKeyGroupContainsOnlyGroupedKeyCommands(t *testing.T) {
	got := visibleSubcommandNames(newKeyCmd())
	want := []string{"delete", "generate", "help", "list", "rename", "view"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("key subcommand mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestShouldLaunchKeyManager(t *testing.T) {
	interactive := isInteractiveTerminal
	defer func() { isInteractiveTerminal = interactive }()
	isInteractiveTerminal = func() bool { return true }
	jsonOutput = false

	if !shouldLaunchKeyManager(nil) {
		t.Fatal("expected key manager to launch for interactive no-arg use")
	}
	if shouldLaunchKeyManager([]string{"list"}) {
		t.Fatal("did not expect key manager when args are provided")
	}
	jsonOutput = true
	if shouldLaunchKeyManager(nil) {
		t.Fatal("did not expect key manager when --json is enabled")
	}
	jsonOutput = false
}

func TestShouldLaunchVaultManager(t *testing.T) {
	interactive := isInteractiveTerminal
	defer func() { isInteractiveTerminal = interactive }()
	isInteractiveTerminal = func() bool { return true }
	jsonOutput = false

	if !shouldLaunchVaultManager(nil) {
		t.Fatal("expected vault manager to launch for interactive no-arg use")
	}
	if shouldLaunchVaultManager([]string{"lock"}) {
		t.Fatal("did not expect vault manager when args are provided")
	}
}

func TestShouldLaunchAgentManager(t *testing.T) {
	interactive := isInteractiveTerminal
	defer func() { isInteractiveTerminal = interactive }()
	isInteractiveTerminal = func() bool { return true }
	jsonOutput = false

	if !shouldLaunchAgentManager(nil) {
		t.Fatal("expected agent manager to launch for interactive no-arg use")
	}
	if shouldLaunchAgentManager([]string{"enable"}) {
		t.Fatal("did not expect agent manager when args are provided")
	}
}

func visibleSubcommandNames(cmd *cobra.Command) []string {
	var names []string
	for _, child := range cmd.Commands() {
		if child.Hidden {
			continue
		}
		names = append(names, child.Name())
	}
	return names
}
