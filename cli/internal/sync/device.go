package sync

import (
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/itzzritik/forged/cli/internal/buildinfo"
)

type deviceInfo struct {
	name, hostname, osVersion string
}

var currentDevice = sync.OnceValue(func() deviceInfo {
	hostname, _ := os.Hostname()
	hostname = strings.TrimSuffix(hostname, ".local")
	return deviceInfo{name: deviceName(hostname), hostname: hostname, osVersion: osVersion()}
})

func (c *Client) setDeviceHeaders(req *http.Request) {
	info := currentDevice()
	req.Header.Set("X-Device-ID", c.DeviceID)
	// Encoded so names like "Ritik’s MacBook Pro" survive proxies.
	req.Header.Set("X-Device-Name", url.QueryEscape(info.name))
	req.Header.Set("X-Device-Hostname", info.hostname)
	req.Header.Set("X-Device-Platform", runtime.GOOS)
	req.Header.Set("X-Device-OS", info.osVersion)
	req.Header.Set("X-Device-Arch", runtime.GOARCH)
	req.Header.Set("X-Forged-Version", buildinfo.Version)
}

func commandOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
