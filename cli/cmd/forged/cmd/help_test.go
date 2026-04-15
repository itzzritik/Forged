package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelpShowsGroupedSectionsOnly(t *testing.T) {
	cmd := newRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Run forged with no arguments to open the interactive launcher.",
		"Get Started",
		"Account",
		"Keys",
		"Vault",
		"Agent",
		"Recovery",
		"forged key generate",
		"forged agent signing",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q:\n%s", want, out)
		}
	}

	for _, unwanted := range []string{"daemon", "start", "stop", "status", "logs", "completion", "benchmark"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("help unexpectedly exposed %q:\n%s", unwanted, out)
		}
	}
}

func TestKeyHelpShowsGroupedExamples(t *testing.T) {
	cmd := newKeyCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Run without a subcommand to open the interactive manager.",
		"generate",
		"list",
		"view <name>",
		"rename <old> <new>",
		"delete <name>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("key help missing %q:\n%s", want, out)
		}
	}
}

func TestAgentHelpShowsEnableDisableAndSigning(t *testing.T) {
	cmd := newAgentCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	out := buf.String()
	for _, want := range []string{"enable", "disable", "signing"} {
		if !strings.Contains(out, want) {
			t.Fatalf("agent help missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "forged signing") {
		t.Fatalf("agent help leaked old root signing command:\n%s", out)
	}
}

func TestVaultHelpShowsLockUnlockAndChangePassword(t *testing.T) {
	cmd := newVaultCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	out := buf.String()
	for _, want := range []string{"lock", "unlock", "change-password"} {
		if !strings.Contains(out, want) {
			t.Fatalf("vault help missing %q:\n%s", want, out)
		}
	}
}
