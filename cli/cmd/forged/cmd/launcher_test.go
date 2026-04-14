package cmd

import "testing"

func TestShouldLaunchBareForged(t *testing.T) {
	t.Parallel()

	interactive := isInteractiveTerminal
	defer func() { isInteractiveTerminal = interactive }()
	isInteractiveTerminal = func() bool { return true }

	jsonOutput = false
	if !shouldLaunchBareForged(nil) {
		t.Fatal("expected bare forged to launch UI")
	}
}

func TestShouldNotLaunchBareForgedWhenJSONRequested(t *testing.T) {
	t.Parallel()

	interactive := isInteractiveTerminal
	defer func() { isInteractiveTerminal = interactive }()
	isInteractiveTerminal = func() bool { return true }

	jsonOutput = true
	defer func() { jsonOutput = false }()

	if shouldLaunchBareForged(nil) {
		t.Fatal("did not expect launcher when --json is set")
	}
}

func TestShouldNotLaunchBareForgedWhenArgsProvided(t *testing.T) {
	t.Parallel()

	interactive := isInteractiveTerminal
	defer func() { isInteractiveTerminal = interactive }()
	isInteractiveTerminal = func() bool { return true }

	jsonOutput = false
	if shouldLaunchBareForged([]string{"status"}) {
		t.Fatal("did not expect launcher when subcommand args are provided")
	}
}

func TestShouldNotLaunchBareForgedWhenTerminalIsNotInteractive(t *testing.T) {
	t.Parallel()

	interactive := isInteractiveTerminal
	defer func() { isInteractiveTerminal = interactive }()
	isInteractiveTerminal = func() bool { return false }

	jsonOutput = false
	if shouldLaunchBareForged(nil) {
		t.Fatal("did not expect launcher in non-interactive terminals")
	}
}
