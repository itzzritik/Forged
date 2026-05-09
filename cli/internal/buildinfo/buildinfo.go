package buildinfo

import (
	"runtime/debug"
	"strings"
)

var ID string

func CurrentID() string {
	if id := strings.TrimSpace(ID); id != "" {
		return id
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	revision := ""
	modified := ""
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = strings.TrimSpace(setting.Value)
		case "vcs.modified":
			modified = strings.TrimSpace(setting.Value)
		}
	}
	if revision != "" {
		if modified == "true" {
			return revision + "-modified"
		}
		return revision
	}
	if strings.TrimSpace(info.Main.Version) != "" && info.Main.Version != "(devel)" {
		return strings.TrimSpace(info.Main.Version)
	}
	return "dev"
}
