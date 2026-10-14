//go:build !windows

package sync

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func deviceName(hostname string) string {
	if runtime.GOOS == "darwin" {
		if name := commandOutput("scutil", "--get", "ComputerName"); name != "" {
			return name
		}
	}
	return hostname
}

func osVersion() string {
	if runtime.GOOS == "darwin" {
		return strings.TrimSpace("macOS " + commandOutput("sw_vers", "-productVersion"))
	}
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(value, `"'`)
		}
	}
	return ""
}

func commandOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
