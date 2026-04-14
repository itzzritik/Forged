package sshrouting

import (
	"fmt"
	"sort"
	"strings"
)

type ProviderRouteEntry struct {
	MatchHost    string
	Provider     string
	AccountSlug  string
	IdentityFile string
	MatchExec    string
}

func RenderBaseConfig(agentSocket, advancedPath string) string {
	return fmt.Sprintf("# Added by Forged\nHost *\n    IdentityAgent %q\n\nInclude %s\n", agentSocket, advancedPath)
}

func RenderAdvancedConfig(entries []ProviderRouteEntry) string {
	if len(entries) == 0 {
		return ""
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].MatchHost != entries[j].MatchHost {
			return entries[i].MatchHost < entries[j].MatchHost
		}
		if entries[i].Provider != entries[j].Provider {
			return entries[i].Provider < entries[j].Provider
		}
		return entries[i].AccountSlug < entries[j].AccountSlug
	})

	var b strings.Builder
	b.WriteString("# Managed by Forged\n")
	for _, entry := range entries {
		fmt.Fprintf(&b, "Match host %s exec %s\n", entry.MatchHost, entry.MatchExec)
		b.WriteString("    User git\n")
		b.WriteString("    IdentitiesOnly yes\n")
		fmt.Fprintf(&b, "    IdentityFile %q\n\n", entry.IdentityFile)
	}

	return b.String()
}
