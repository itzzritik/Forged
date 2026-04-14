package sshrouting

import (
	"fmt"
	"sort"
	"strings"
)

type AdvancedHost struct {
	Alias        string
	HostName     string
	IdentityFile string
}

func RenderBaseConfig(agentSocket, advancedPath string) string {
	return fmt.Sprintf("# Added by Forged\nHost *\n    IdentityAgent %q\n\nInclude %s\n", agentSocket, advancedPath)
}

func RenderAdvancedConfig(entries []AdvancedHost) string {
	if len(entries) == 0 {
		return ""
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Alias < entries[j].Alias
	})

	var b strings.Builder
	b.WriteString("# Managed by Forged\n")
	for _, entry := range entries {
		fmt.Fprintf(&b, "Host %s\n", entry.Alias)
		fmt.Fprintf(&b, "    HostName %s\n", entry.HostName)
		if entry.IdentityFile != "" {
			fmt.Fprintf(&b, "    IdentityFile %s\n", entry.IdentityFile)
		}
		b.WriteString("\n")
	}

	return b.String()
}
