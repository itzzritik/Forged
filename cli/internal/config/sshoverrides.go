package config

import (
	"strings"

	"github.com/itzzritik/forged/cli/internal/platform"
)

// SSHHostsOverriddenByForged lists Host or Match blocks after the Forged include whose IdentityAgent
// (or, with routing, ControlMaster/ControlPath) never applies: ssh keeps the first value, and Forged's
// Host * comes first. Catch-all blocks are left out, since Forged replacing them is the point.
// ponytail: reads ~/.ssh/config only, not files it includes.
func SSHHostsOverriddenByForged(paths Paths) []string {
	content, err := readConfigFile(paths.SSHUserConfig())
	if err != nil {
		return nil
	}
	overridden := map[string]bool{"identityagent": true}
	if platform.SSHRoutingSupported() {
		overridden["controlmaster"] = true
		overridden["controlpath"] = true
	}
	includes := forgedIncludeLines(paths)
	afterInclude := false
	header := ""
	var hosts []string
	seen := map[string]bool{}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if _, ok := includes[trimmed]; ok {
			afterInclude = true
			continue
		}
		keyword, value, _ := strings.Cut(strings.Replace(trimmed, "=", " ", 1), " ")
		keyword = strings.ToLower(keyword)
		value = strings.TrimSpace(value)
		switch {
		case keyword == "host" || keyword == "match":
			header = value
			if value == "*" || strings.EqualFold(value, "all") {
				header = ""
			}
		case afterInclude && header != "" && overridden[keyword] && !seen[header]:
			seen[header] = true
			hosts = append(hosts, header)
		}
	}
	return hosts
}
