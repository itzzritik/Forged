package health

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const (
	capAvailable  = "available"
	capByPlatform = "unavailable_by_platform"
	capByEnv      = "unavailable_by_environment"
)

type Check struct {
	Name, Status, Detail string
	Tone                 ui.Tone
}

func Checks(st *core.State, paths config.Paths) []Check {
	c := checker{st: st, snap: st.Snapshot, paths: paths, recovery: st.Recovery()}
	out := []Check{
		c.vault(), c.config(), c.service(), c.daemon(), c.ipcSocket(), c.agentSocket(),
		c.sshAgent(), c.sshConfig(), c.identityAgent(),
	}
	if c.snap.GitSSH.Applicable {
		out = append(out, c.gitSSH())
	}
	return append(out, c.systemAuth(), c.secureStore(), c.syncAccount())
}

func Count(st *core.State, paths config.Paths) int {
	n := 0
	for _, c := range Checks(st, paths) {
		if c.Tone == ui.ToneBad {
			n++
		}
	}
	return n
}

func Report(st *core.State, paths config.Paths) string {
	lines := []string{
		"Forged diagnostic report",
		"Generated: " + time.Now().UTC().Format(time.RFC3339),
		"Version: " + orUnknown(st.Deps.AppVersion),
		"Platform: " + runtime.GOOS + "/" + runtime.GOARCH,
		"Readiness: " + orUnknown(string(st.Snapshot.State)),
		"CLI build: " + orUnknown(st.Snapshot.CurrentBuildID),
		"Daemon build: " + orUnknown(st.Snapshot.DaemonBuildID),
		"",
		"Checks:",
	}
	for _, c := range Checks(st, paths) {
		lines = append(lines, fmt.Sprintf("- [%s] %s: %s", reportTone(c.Tone), c.Name, c.Status))
	}
	return strings.Join(lines, "\n") + "\n"
}

func reportTone(t ui.Tone) string {
	switch t {
	case ui.ToneGood:
		return "ok"
	case ui.ToneBad:
		return "error"
	}
	return "warning"
}

func orUnknown(s string) string {
	if s = strings.TrimSpace(s); s == "" {
		return "unknown"
	}
	return s
}

type checker struct {
	st       *core.State
	snap     readiness.Snapshot
	paths    config.Paths
	recovery bool
}

func (c checker) pathError() string { return ui.Sanitize(strings.TrimSpace(c.snap.RuntimePathError)) }

func (c checker) unavailable(name string) (Check, bool) {
	if c.recovery {
		return Check{name, "Unavailable", c.pathError(), ui.ToneBad}, true
	}
	return Check{}, false
}

func (c checker) fixDetail(detail string) string {
	if c.recovery {
		return "Resolve the daemon endpoint identity before repair"
	}
	return detail
}

func clean(s string) string { return ui.Sanitize(strings.TrimSpace(s)) }

func osName(darwin, windows, other string) string {
	switch runtime.GOOS {
	case "darwin":
		return darwin
	case "windows":
		return windows
	}
	return other
}

func (c checker) vault() Check {
	if c.snap.VaultExists {
		return Check{"Vault", "Present", c.paths.VaultFile(), ui.ToneGood}
	}
	detail := "Resolve the daemon endpoint identity before setup or restore"
	switch {
	case c.recovery:
	case c.st.CredentialError() != "":
		detail = "Repair saved account credentials before setting up or restoring this device"
	case c.snap.LoggedIn:
		detail = "Fix issues can restore this device"
	default:
		detail = "Set up or restore this device"
	}
	return Check{"Vault", "Missing", detail, ui.ToneBad}
}

func (c checker) config() Check {
	switch {
	case c.snap.ConfigExists && !c.snap.ConfigValid:
		detail := clean(c.snap.ConfigError)
		if detail == "" {
			detail = "Edit " + c.paths.ConfigFile()
		}
		return Check{"Config file", "Invalid", detail, ui.ToneBad}
	case c.snap.ConfigExists:
		return Check{"Config file", "Ready", c.paths.ConfigFile(), ui.ToneGood}
	}
	return Check{"Config file", "Missing", c.fixDetail("Run Fix issues"), ui.ToneBad}
}

func (c checker) service() Check {
	if ch, ok := c.unavailable("Service"); ok {
		return ch
	}
	svc := c.snap.Service
	switch {
	case svc.OwnershipBlocked:
		detail := clean(svc.Detail)
		if detail == "" {
			detail = "Stop the running Forged daemon or service, then refresh Health."
		}
		return Check{"Service", "Needs manual stop", c.fixDetail(detail), ui.ToneBad}
	case !svc.Repairable:
		detail := clean(svc.Detail)
		if detail == "" {
			detail = "Service needs manual repair."
		}
		return Check{"Service", "Blocked", c.fixDetail(detail), ui.ToneBad}
	case svc.Installed && svc.ConfigValid:
		return Check{"Service", "Installed", "System service ready", ui.ToneGood}
	case svc.Installed:
		detail := clean(svc.Detail)
		if detail == "" {
			detail = "Service configuration is invalid"
		}
		return Check{"Service", "Invalid", detail, ui.ToneBad}
	}
	return Check{"Service", "Not installed", "Run Fix issues", ui.ToneBad}
}

func (c checker) daemon() Check {
	if ch, ok := c.unavailable("Daemon"); ok {
		return ch
	}
	snap := c.snap
	switch {
	case snap.Service.OwnershipBlocked:
		detail := "Running daemon or service owns the runtime"
		if snap.DaemonPID > 0 {
			detail = fmt.Sprintf("PID %d is not service-owned", snap.DaemonPID)
		}
		return Check{"Daemon", "Needs manual stop", detail, ui.ToneBad}
	case snap.Service.Running:
		if cur := strings.TrimSpace(snap.CurrentBuildID); cur != "" && strings.TrimSpace(snap.DaemonBuildID) != cur {
			detail := fmt.Sprintf("The background service is running build %s. This app is build %s. Fix issues restarts the service on the current build.", orUnknown(snap.DaemonBuildID), cur)
			return Check{"Daemon", "Outdated", detail, ui.ToneBad}
		}
		detail := "Running"
		if snap.DaemonPID > 0 {
			detail = fmt.Sprintf("PID %d", snap.DaemonPID)
		}
		return Check{"Daemon", "Running", detail, ui.ToneGood}
	}
	return Check{"Daemon", "Not running", "Run Fix issues", ui.ToneBad}
}

func (c checker) socket(name string, ready bool, path string) Check {
	if ch, ok := c.unavailable(name); ok {
		return ch
	}
	if ready {
		return Check{name, "Ready", path, ui.ToneGood}
	}
	return Check{name, "Not responding", path, ui.ToneBad}
}

func (c checker) ipcSocket() Check {
	return c.socket("Control socket", c.snap.IPCSocketReady, c.paths.CtlSocket())
}

func (c checker) agentSocket() Check {
	return c.socket("Agent socket", c.snap.AgentSocketReady, c.paths.AgentSocket())
}

func (c checker) sshAgent() Check {
	switch {
	case c.snap.RequiresManualSSHConfigurationChange():
		return Check{"SSH agent", "Active externally", "Forged remains active through external SSH configuration; update it manually, then refresh", ui.ToneWarn}
	case c.snap.AgentDisabled:
		return Check{"SSH agent", "Disabled", c.fixDetail("Turn on the SSH agent in SSH & Git"), ui.ToneWarn}
	}
	if ch, ok := c.unavailable("SSH agent"); ok {
		return ch
	}
	if c.snap.SSHEnabled {
		detail := "Forged SSH include is configured"
		if !platform.SSHRoutingSupported() {
			detail = "Forged SSH agent is active; automatic SSH routing is unavailable on this platform"
		}
		return Check{"SSH agent", "Active", detail, ui.ToneGood}
	}
	return Check{"SSH agent", "Not active", c.fixDetail("Run Fix issues"), ui.ToneBad}
}

func (c checker) sshConfig() Check {
	switch {
	case c.snap.RequiresManualSSHConfigurationChange():
		return Check{"SSH config", "Disabled", "Forged-managed SSH integration is disabled", ui.ToneWarn}
	case c.snap.AgentDisabled:
		return Check{"SSH config", "Disabled", c.fixDetail("Not needed while SSH integration is disabled"), ui.ToneWarn}
	}
	if ch, ok := c.unavailable("SSH config"); ok {
		return ch
	}
	if c.snap.ManagedConfigReady {
		return Check{"SSH config", "Ready", c.paths.SSHManagedConfig(), ui.ToneGood}
	}
	return Check{"SSH config", "Missing", c.paths.SSHManagedConfig(), ui.ToneBad}
}

func (c checker) identityAgent() Check {
	owner := c.snap.IdentityAgentOwner
	ownerPath := clean(owner.Path)
	agentPath := c.paths.AgentSocket()
	if ownerPath != "" {
		agentPath = ownerPath
	}
	switch {
	case c.snap.RequiresManualSSHConfigurationChange():
		return Check{"IdentityAgent", "Forged (external)", agentPath + "; update external SSH configuration manually", ui.ToneWarn}
	case c.snap.AgentDisabled:
		return Check{"IdentityAgent", "Disabled", c.fixDetail("Not needed while SSH integration is disabled"), ui.ToneWarn}
	}
	if ch, ok := c.unavailable("IdentityAgent"); ok {
		return ch
	}
	if owner.IsForged() {
		return Check{"IdentityAgent", "Forged", agentPath, ui.ToneGood}
	}
	switch name := clean(owner.Name); name {
	case "":
		return Check{"IdentityAgent", "Unknown", "Could not inspect active ssh configuration", ui.ToneBad}
	case "None":
		return Check{"IdentityAgent", "Not configured", "No active IdentityAgent is configured", ui.ToneBad}
	default:
		if ownerPath != "" {
			name += " (" + ownerPath + ")"
		}
		return Check{"IdentityAgent", "Not Forged", name, ui.ToneBad}
	}
}

func (c checker) gitSSH() Check {
	g := c.snap.GitSSH
	switch {
	case g.Native:
		return Check{"Git SSH", "Win32-OpenSSH", "Git uses Win32-OpenSSH", ui.ToneGood}
	case g.NeedsFix():
		return Check{"Git SSH", "Bundled ssh", c.fixDetail("Git's bundled ssh can't reach the Forged agent; Fix issues sets core.sshCommand"), ui.ToneWarn}
	case g.Source != "":
		return Check{"Git SSH", clean(g.Source), "Git's configured ssh can't reach the Forged agent; point it at Win32-OpenSSH", ui.ToneWarn}
	}
	return Check{"Git SSH", "Bundled ssh", "Git's bundled ssh can't reach the Forged agent; install the Windows OpenSSH Client", ui.ToneWarn}
}

func (c checker) securityGate(name string) (Check, bool) {
	if ch, ok := c.unavailable(name); ok {
		return ch, true
	}
	switch {
	case !c.st.SecurityLoaded && strings.TrimSpace(c.st.SecurityErr) == "":
		return Check{name, "Checking", "Loading security state", ui.ToneBusy}, true
	case strings.TrimSpace(c.st.SecurityErr) != "":
		return Check{name, "Check failed", "Security inspection failed: " + clean(c.st.SecurityErr), ui.ToneBad}, true
	}
	return Check{}, false
}

func (c checker) systemAuth() Check {
	name := osName("Touch ID", "Windows Hello", "System Auth")
	if ch, done := c.securityGate(name); done {
		return ch
	}
	sec := c.st.Security
	if sec.HeadlessUnlock {
		return Check{name, "Headless mode", "System Auth is intentionally skipped on this device", ui.ToneWarn}
	}
	switch sec.SystemAuthCapability {
	case capAvailable:
		if runtime.GOOS == "linux" {
			return Check{name, "Detected", "Desktop session and pkexec detected in this terminal", ui.ToneGood}
		}
		return Check{name, "Available", "System Auth is ready for sensitive actions", ui.ToneGood}
	case capByPlatform:
		return Check{name, "Not available", systemAuthHint(false), ui.ToneWarn}
	case capByEnv:
		return Check{name, "Unavailable here", systemAuthHint(true), ui.ToneWarn}
	}
	detail := "System Auth is expected but not working"
	if runtime.GOOS == "linux" {
		detail = "Forged auth helper or pkexec is not working"
	}
	return Check{name, "Broken", detail, ui.ToneBad}
}

func (c checker) secureStore() Check {
	name := osName("Keychain", "Credential Manager", "Secure store")
	if ch, done := c.securityGate(name); done {
		return ch
	}
	sec := c.st.Security
	if sec.HeadlessUnlock {
		return Check{name, "File-backed", "Device unlock trust is stored in a local file", ui.ToneWarn}
	}
	switch sec.SecureStoreCapability {
	case capAvailable:
		return Check{name, "Available", "Local unlock trust can be stored securely", ui.ToneGood}
	case capByPlatform, capByEnv:
		status := "Unavailable"
		if runtime.GOOS == "linux" {
			status = "Not supported"
		}
		return Check{name, status, secureStoreHint(), ui.ToneWarn}
	}
	return Check{name, "Broken", "Local unlock trust cannot be persisted", ui.ToneBad}
}

func (c checker) syncAccount() Check {
	const name = "Sync account"
	if ch, ok := c.unavailable(name); ok {
		return ch
	}
	if issue := c.st.SyncIssue(); issue != "" {
		return Check{name, "Sync error", clean(issue), ui.ToneBad}
	}
	if cred := c.st.CredentialError(); cred != "" {
		return Check{name, "Credentials unavailable", clean(cred), ui.ToneBad}
	}
	if c.st.StatusLoaded {
		switch {
		case c.st.Status.Syncing:
			return Check{name, "Syncing", "Multi-device sync in progress", ui.ToneBusy}
		case c.st.Status.Linked && c.st.Status.Dirty:
			return Check{name, "Pending changes", "Local changes have not synced yet", ui.ToneWarn}
		}
	}
	if !c.snap.LoggedIn {
		return Check{name, "Not logged in", "Multi-device sync unavailable", ui.ToneWarn}
	}
	return Check{name, "Logged in", "Multi-device sync available", ui.ToneGood}
}

func systemAuthHint(byEnv bool) string {
	if byEnv {
		switch runtime.GOOS {
		case "windows":
			return "Windows Hello can't prompt here"
		case "linux":
			return "No desktop prompt here; use --headless only on a trusted headless device"
		case "darwin":
			return "No desktop prompt (often SSH)"
		}
		return "System Auth can't prompt here"
	}
	switch runtime.GOOS {
	case "windows":
		return "Check Windows Hello setup/policy"
	case "darwin":
		return "Touch ID/device auth unavailable"
	}
	return "Unsupported on this platform"
}

func secureStoreHint() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows DPAPI is unavailable for this user"
	case "linux":
		return "Linux secure device-key storage is not implemented"
	}
	return "Master-password trust cannot be remembered securely"
}
