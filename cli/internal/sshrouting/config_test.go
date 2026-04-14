package sshrouting

import (
	"strings"
	"testing"
)

func TestRenderBaseConfigUsesIdentityAgentAndIncludesAdvancedConfig(t *testing.T) {
	got := RenderBaseConfig("/tmp/agent.sock", "/Users/test/.ssh/forged/config")
	if !strings.Contains(got, "IdentityAgent \"/tmp/agent.sock\"") {
		t.Fatalf("missing identity agent: %s", got)
	}
	if !strings.Contains(got, "Include /Users/test/.ssh/forged/config") {
		t.Fatalf("missing advanced include: %s", got)
	}
}

func TestRenderAdvancedConfigEmptyWhenNoEntries(t *testing.T) {
	if got := RenderAdvancedConfig(nil); strings.TrimSpace(got) != "" {
		t.Fatalf("expected empty config, got %q", got)
	}
}
