package tui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
	dashboardscreen "github.com/itzzritik/forged/cli/internal/tui/screens/dashboard"
	doctorscreen "github.com/itzzritik/forged/cli/internal/tui/screens/doctor"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type doctorRow struct {
	screen doctorscreen.Row
}

type doctorReportCopiedMsg struct {
	err error
}

func (m *model) isDoctorOverviewRoute() bool {
	return m.screen == screenDashboard && m.session.Current().ID == RouteDoctorOverview
}

func (m *model) isDoctorDashboardTab() bool {
	if m.screen != screenDashboard ||
		(!m.snapshot.VaultExists && strings.TrimSpace(m.snapshot.RuntimePathError) == "") ||
		m.session.Current().ID != RouteDashboardHome {
		return false
	}

	tabs := m.dashboardTabs()
	if len(tabs) == 0 {
		return false
	}
	m.normalizeDashboardSelection(tabs)
	return tabs[m.dashboardTabIndex].Label == "Doctor"
}

func (m *model) renderDoctorBody(contentWidth int, bodyHeight int) string {
	rows := m.doctorRows()
	sections := make([]string, 0, 2)
	if !m.isDoctorDashboardTab() {
		if status := dashboardscreen.Render(dashboardscreen.Screen{
			Notice: dashboardscreen.Notice{Message: m.notice.message, Tone: m.notice.tone},
		}, contentWidth); strings.TrimSpace(status) != "" {
			sections = append(sections, status)
			bodyHeight -= lipgloss.Height(status) + 1
		}
	}
	rows = m.visibleDoctorRows(rows, bodyHeight, contentWidth)
	screenRows := make([]doctorscreen.Row, 0, len(rows))
	for _, row := range rows {
		screenRows = append(screenRows, row.screen)
	}
	sections = append(sections, doctorscreen.Render(doctorscreen.Screen{Rows: screenRows}, contentWidth))
	return shell.IndentBlock(strings.Join(sections, "\n\n"), 2)
}

func (m *model) renderDoctorDashboardBody(contentWidth int, bodyHeight int) string {
	tabs, _, _ := m.dashboardRootScreen()
	tabBar := dashboardscreen.Render(dashboardscreen.Screen{
		Tabs: tabs,
		Notice: dashboardscreen.Notice{
			Message: m.notice.message,
			Tone:    m.notice.tone,
		},
	}, contentWidth)
	if strings.TrimSpace(tabBar) != "" {
		bodyHeight -= lipgloss.Height(tabBar) + 1
	}
	body := m.renderDoctorBody(contentWidth, bodyHeight)

	switch {
	case strings.TrimSpace(tabBar) == "":
		return body
	case strings.TrimSpace(body) == "":
		return tabBar
	default:
		return tabBar + "\n\n" + body
	}
}

func (m *model) visibleDoctorRows(rows []doctorRow, bodyHeight int, contentWidth int) []doctorRow {
	pageRows := max(1, min(len(rows), bodyHeight/doctorscreen.RowHeight(contentWidth)))
	m.doctorPageRows = pageRows
	maxOffset := max(0, len(rows)-pageRows)
	m.doctorOffset = max(0, min(m.doctorOffset, maxOffset))
	return rows[m.doctorOffset:min(len(rows), m.doctorOffset+pageRows)]
}

func (m *model) moveDoctorOffset(delta int) {
	pageRows := max(1, m.doctorPageRows)
	maxOffset := max(0, len(m.doctorRows())-pageRows)
	m.doctorOffset = max(0, min(m.doctorOffset+delta, maxOffset))
}

func (m *model) doctorFooterActions(includeTabs bool) []shell.FooterAction {
	actions := make([]shell.FooterAction, 0, 5)
	if includeTabs {
		actions = append(actions, shell.FooterAction{Key: theme.Glyphs.LeftRight, Label: "Tabs"})
	}
	actions = append(actions, shell.FooterAction{Key: theme.Glyphs.UpDown, Label: "Scroll"})
	if m.doctorCanFixIssues() && !m.maintenanceBusy {
		actions = append(actions, shell.FooterAction{Key: "Enter", Label: "Fix"})
	}
	actions = append(actions, shell.FooterAction{Key: "C", Label: "Copy"})
	if strings.TrimSpace(m.snapshot.RuntimePathError) == "" {
		actions = append(actions, shell.FooterAction{Key: "R", Label: "Refresh"})
	}
	actions = append(actions, shell.FooterAction{Key: "Esc", Label: m.session.EscLabel(EscAuto)})
	return actions
}

func (m *model) updateDoctorKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "r", "R":
		if strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
			return m, nil
		}
		return m, tea.Batch(m.refreshSnapshotCmd(), m.loadSecurityStateCmd(), m.invalidateSigningStatusCmd())
	case "up", "k":
		m.moveDoctorOffset(-1)
		return m, nil
	case "down", "j":
		m.moveDoctorOffset(1)
		return m, nil
	case "c", "C":
		return m, m.copyDoctorReportCmd()
	case "enter":
		if !m.doctorCanFixIssues() || m.maintenanceBusy {
			return m, nil
		}
		return m, m.startDoctorRepair(nil)
	default:
		return m, nil
	}
}

func (m *model) updateDoctorDashboardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	tabs := m.dashboardTabs()
	m.normalizeDashboardSelection(tabs)

	switch msg.String() {
	case "esc":
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "left", "h":
		return m, m.switchDashboardTab(-1, tabs)
	case "right", "l":
		return m, m.switchDashboardTab(1, tabs)
	case "up", "k":
		m.moveDoctorOffset(-1)
		return m, nil
	case "down", "j":
		m.moveDoctorOffset(1)
		return m, nil
	case "r", "R":
		if strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
			return m, nil
		}
		return m, tea.Batch(m.refreshSnapshotCmd(), m.loadSecurityStateCmd(), m.invalidateSigningStatusCmd())
	case "c", "C":
		return m, m.copyDoctorReportCmd()
	case "enter":
		if !m.doctorCanFixIssues() || m.maintenanceBusy {
			return m, nil
		}
		return m, m.startDoctorRepair(nil)
	default:
		return m, nil
	}
}

func (m *model) copyDoctorReportCmd() tea.Cmd {
	m.clipboardBusy = true
	m.notice = notice{}
	report := m.doctorReport()
	copyText := m.deps.CopyText
	return func() tea.Msg {
		return doctorReportCopiedMsg{err: copyText(report)}
	}
}

func (m *model) handleDoctorReportCopiedMsg(msg doctorReportCopiedMsg) (tea.Model, tea.Cmd) {
	m.clipboardBusy = false
	if msg.err != nil {
		message := m.reportError("doctor.copy-report", msg.err)
		m.notice = notice{message: "Couldn't copy doctor report: " + message, tone: dashboardscreen.ToneDanger}
		return m, nil
	}
	m.cancelPrivateClipboard()
	m.notice = notice{message: "Doctor report copied", tone: dashboardscreen.ToneSuccess}
	return m, nil
}

func (m *model) doctorReport() string {
	lines := []string{
		"Forged diagnostic report",
		"Generated: " + time.Now().UTC().Format(time.RFC3339),
		"Version: " + doctorReportValue(m.deps.AppVersion),
		"Platform: " + runtime.GOOS + "/" + runtime.GOARCH,
		"Readiness: " + doctorReportValue(string(m.snapshot.State)),
		"CLI build: " + doctorReportValue(m.snapshot.CurrentBuildID),
		"Daemon build: " + doctorReportValue(m.snapshot.DaemonBuildID),
		"",
		"Checks:",
	}
	for _, row := range m.doctorRows() {
		lines = append(lines, fmt.Sprintf(
			"- [%s] %s: %s",
			doctorReportTone(row.screen.Tone),
			strings.TrimSpace(row.screen.Check),
			doctorReportStatus(row.screen.Status),
		))
	}
	return strings.Join(lines, "\n") + "\n"
}

func doctorReportTone(tone doctorscreen.Tone) string {
	switch tone {
	case doctorscreen.ToneSuccess:
		return "ok"
	case doctorscreen.ToneWarning:
		return "warning"
	default:
		return "error"
	}
}

func doctorReportStatus(status string) string {
	status = strings.TrimSpace(status)
	for _, prefix := range []string{theme.Glyphs.Check, theme.Glyphs.Cross, theme.Glyphs.Pending, "!"} {
		if strings.HasPrefix(status, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(status, prefix))
		}
	}
	return status
}

func doctorReportValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func (m *model) startDoctorRepair(password []byte) tea.Cmd {
	return m.startMaintenance(
		maintenanceTriggerDoctor,
		password,
		false,
		"Fixing Issues",
		"",
	)
}

func (m *model) doctorCanFixIssues() bool {
	s := m.snapshot
	if strings.TrimSpace(s.RuntimePathError) != "" {
		return false
	}
	if !s.VaultExists {
		return false
	}
	if !s.ConfigExists {
		return true
	}
	if !s.ConfigValid {
		return false
	}
	if !s.Service.Repairable {
		return false
	}
	if !s.Service.Installed || !s.Service.ConfigValid || !s.Service.Running {
		return true
	}
	if s.Service.PID > 0 && (s.DaemonPID <= 0 || s.Service.PID != s.DaemonPID) {
		return true
	}
	if current := strings.TrimSpace(s.CurrentBuildID); current != "" && strings.TrimSpace(s.DaemonBuildID) != current {
		return true
	}
	if !s.IPCSocketReady || !s.AgentSocketReady {
		return true
	}
	if !s.AgentDisabled && (!s.SSHEnabled || !s.ManagedConfigReady || !s.IdentityAgentOwner.IsForged()) {
		return true
	}
	return false
}

func (m *model) doctorRows() []doctorRow {
	paths := config.DefaultPaths()
	rows := []doctorRow{
		m.doctorVaultRow(paths),
		m.doctorConfigRow(paths),
		m.doctorServiceRow(),
		m.doctorDaemonRow(),
		m.doctorIPCSocketRow(paths),
		m.doctorAgentSocketRow(paths),
		m.doctorSSHAgentRow(),
		m.doctorSSHConfigRow(paths),
		m.doctorIdentityAgentRow(paths),
		m.doctorSystemAuthRow(),
		m.doctorSecureStoreRow(),
		m.doctorSyncAccountRow(),
	}

	return rows
}

func (m *model) doctorRepairDetail(detail string) string {
	if strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
		return "Resolve the daemon endpoint identity before repair"
	}
	return detail
}

func (m *model) doctorRuntimePathUnavailableRow(check string) (doctorRow, bool) {
	if runtimePathError := strings.TrimSpace(m.snapshot.RuntimePathError); runtimePathError != "" {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  check,
				Status: theme.Glyphs.Cross + " Unavailable",
				Detail: runtimePathError,
				Tone:   doctorscreen.ToneDanger,
			},
		}, true
	}
	return doctorRow{}, false
}

func (m *model) doctorVaultRow(paths config.Paths) doctorRow {
	if m.snapshot.VaultExists {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Vault",
				Status: theme.Glyphs.Check + " Present",
				Detail: paths.VaultFile(),
				Tone:   doctorscreen.ToneSuccess,
			},
		}
	}

	detail := "Resolve the daemon endpoint identity before setup or restore"
	if strings.TrimSpace(m.snapshot.RuntimePathError) == "" && m.snapshot.LoggedIn {
		detail = "Fix Issues can restore this device"
	} else if strings.TrimSpace(m.snapshot.RuntimePathError) == "" {
		detail = "Set up or restore this device"
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "Vault",
			Status: theme.Glyphs.Cross + " Missing",
			Detail: detail,
			Tone:   doctorscreen.ToneDanger,
		},
	}
}

func (m *model) doctorConfigRow(paths config.Paths) doctorRow {
	if m.snapshot.ConfigExists && !m.snapshot.ConfigValid {
		detail := strings.TrimSpace(m.snapshot.ConfigError)
		if detail == "" {
			detail = "Edit " + paths.ConfigFile()
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Config",
				Status: theme.Glyphs.Cross + " Invalid",
				Detail: detail,
				Tone:   doctorscreen.ToneDanger,
			},
		}
	}
	if m.snapshot.ConfigExists {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Config",
				Status: theme.Glyphs.Check + " Ready",
				Detail: paths.ConfigFile(),
				Tone:   doctorscreen.ToneSuccess,
			},
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "Config",
			Status: theme.Glyphs.Cross + " Missing",
			Detail: m.doctorRepairDetail("Run Fix Issues"),
			Tone:   doctorscreen.ToneDanger,
		},
	}
}

func (m *model) doctorServiceRow() doctorRow {
	if row, unavailable := m.doctorRuntimePathUnavailableRow("Service"); unavailable {
		return row
	}
	if m.snapshot.Service.OwnershipBlocked {
		detail := strings.TrimSpace(m.snapshot.Service.Detail)
		if detail == "" {
			detail = "Stop the running Forged daemon or service, then refresh Doctor."
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Service",
				Status: theme.Glyphs.Cross + " Needs manual stop",
				Detail: m.doctorRepairDetail(detail),
				Tone:   doctorscreen.ToneDanger,
			},
		}
	}
	if !m.snapshot.Service.Repairable {
		detail := strings.TrimSpace(m.snapshot.Service.Detail)
		if detail == "" {
			detail = "Service needs manual repair."
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Service",
				Status: theme.Glyphs.Cross + " Blocked",
				Detail: m.doctorRepairDetail(detail),
				Tone:   doctorscreen.ToneDanger,
			},
		}
	}
	if m.snapshot.Service.Installed && m.snapshot.Service.ConfigValid {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Service",
				Status: theme.Glyphs.Check + " Installed",
				Detail: "System service ready",
				Tone:   doctorscreen.ToneSuccess,
			},
		}
	}

	status := theme.Glyphs.Cross + " Not installed"
	detail := "Run Fix Issues"
	if m.snapshot.Service.Installed && !m.snapshot.Service.ConfigValid {
		status = theme.Glyphs.Cross + " Invalid"
		detail = strings.TrimSpace(m.snapshot.Service.Detail)
		if detail == "" {
			detail = "Service configuration is invalid"
		}
	}
	detail = m.doctorRepairDetail(detail)

	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "Service",
			Status: status,
			Detail: detail,
			Tone:   doctorscreen.ToneDanger,
		},
	}
}

func (m *model) doctorDaemonRow() doctorRow {
	if row, unavailable := m.doctorRuntimePathUnavailableRow("Daemon"); unavailable {
		return row
	}
	if m.snapshot.Service.OwnershipBlocked {
		detail := "Running daemon or service owns the runtime"
		if m.snapshot.DaemonPID > 0 {
			detail = fmt.Sprintf("PID %d is not service-owned", m.snapshot.DaemonPID)
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Daemon",
				Status: theme.Glyphs.Cross + " Needs manual stop",
				Detail: detail,
				Tone:   doctorscreen.ToneDanger,
			},
		}
	}
	if m.snapshot.Service.Running {
		detail := "Running"
		if m.snapshot.DaemonPID > 0 {
			detail = fmt.Sprintf("PID %d", m.snapshot.DaemonPID)
		}
		if strings.TrimSpace(m.snapshot.CurrentBuildID) != "" && strings.TrimSpace(m.snapshot.DaemonBuildID) != strings.TrimSpace(m.snapshot.CurrentBuildID) {
			return doctorRow{
				screen: doctorscreen.Row{
					Check:  "Daemon",
					Status: theme.Glyphs.Cross + " Outdated",
					Detail: m.doctorRepairDetail("Run Fix Issues"),
					Tone:   doctorscreen.ToneDanger,
				},
			}
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Daemon",
				Status: theme.Glyphs.Check + " Running",
				Detail: detail,
				Tone:   doctorscreen.ToneSuccess,
			},
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "Daemon",
			Status: theme.Glyphs.Cross + " Not running",
			Detail: m.doctorRepairDetail("Run Fix Issues"),
			Tone:   doctorscreen.ToneDanger,
		},
	}
}

func (m *model) doctorIPCSocketRow(paths config.Paths) doctorRow {
	if row, unavailable := m.doctorRuntimePathUnavailableRow("IPC Socket"); unavailable {
		return row
	}
	if m.snapshot.IPCSocketReady {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "IPC Socket",
				Status: theme.Glyphs.Check + " Ready",
				Detail: paths.CtlSocket(),
				Tone:   doctorscreen.ToneSuccess,
			},
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "IPC Socket",
			Status: theme.Glyphs.Cross + " Not responding",
			Detail: paths.CtlSocket(),
			Tone:   doctorscreen.ToneDanger,
		},
	}
}

func (m *model) doctorAgentSocketRow(paths config.Paths) doctorRow {
	if row, unavailable := m.doctorRuntimePathUnavailableRow("Agent Socket"); unavailable {
		return row
	}
	if m.snapshot.AgentSocketReady {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Agent Socket",
				Status: theme.Glyphs.Check + " Ready",
				Detail: paths.AgentSocket(),
				Tone:   doctorscreen.ToneSuccess,
			},
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "Agent Socket",
			Status: theme.Glyphs.Cross + " Not responding",
			Detail: paths.AgentSocket(),
			Tone:   doctorscreen.ToneDanger,
		},
	}
}

func (m *model) doctorSSHAgentRow() doctorRow {
	if m.snapshot.AgentDisabled {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "SSH Agent",
				Status: "! Disabled",
				Detail: m.doctorRepairDetail("Enable SSH Agent from the Agent tab"),
				Tone:   doctorscreen.ToneWarning,
			},
		}
	}
	if row, unavailable := m.doctorRuntimePathUnavailableRow("SSH Agent"); unavailable {
		return row
	}
	if m.snapshot.SSHEnabled {
		detail := "Forged SSH include is configured"
		if !platform.SSHRoutingSupported() {
			detail = "Forged SSH agent is active; automatic SSH routing is unavailable on this platform"
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "SSH Agent",
				Status: theme.Glyphs.Check + " Active",
				Detail: detail,
				Tone:   doctorscreen.ToneSuccess,
			},
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "SSH Agent",
			Status: theme.Glyphs.Cross + " Not active",
			Detail: m.doctorRepairDetail("Run Fix Issues"),
			Tone:   doctorscreen.ToneDanger,
		},
	}
}

func (m *model) doctorSSHConfigRow(paths config.Paths) doctorRow {
	if m.snapshot.AgentDisabled {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "SSH Config",
				Status: "! Disabled",
				Detail: m.doctorRepairDetail("Not needed while SSH integration is disabled"),
				Tone:   doctorscreen.ToneWarning,
			},
		}
	}
	if row, unavailable := m.doctorRuntimePathUnavailableRow("SSH Config"); unavailable {
		return row
	}
	if m.snapshot.ManagedConfigReady {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "SSH Config",
				Status: theme.Glyphs.Check + " Ready",
				Detail: paths.SSHManagedConfig(),
				Tone:   doctorscreen.ToneSuccess,
			},
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "SSH Config",
			Status: theme.Glyphs.Cross + " Missing",
			Detail: paths.SSHManagedConfig(),
			Tone:   doctorscreen.ToneDanger,
		},
	}
}

func (m *model) doctorIdentityAgentRow(paths config.Paths) doctorRow {
	if m.snapshot.AgentDisabled {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "IdentityAgent",
				Status: "! Disabled",
				Detail: m.doctorRepairDetail("Not needed while SSH integration is disabled"),
				Tone:   doctorscreen.ToneWarning,
			},
		}
	}
	if row, unavailable := m.doctorRuntimePathUnavailableRow("IdentityAgent"); unavailable {
		return row
	}
	if m.snapshot.IdentityAgentOwner.IsForged() {
		detail := paths.AgentSocket()
		if ownerPath := strings.TrimSpace(m.snapshot.IdentityAgentOwner.Path); ownerPath != "" {
			detail = ownerPath
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "IdentityAgent",
				Status: theme.Glyphs.Check + " Forged",
				Detail: detail,
				Tone:   doctorscreen.ToneSuccess,
			},
		}
	}

	status := theme.Glyphs.Cross + " Not Forged"
	detail := strings.TrimSpace(m.snapshot.IdentityAgentOwner.Name)
	switch detail {
	case "":
		status = theme.Glyphs.Cross + " Unknown"
		detail = "Could not inspect active ssh configuration"
	case "None":
		status = theme.Glyphs.Cross + " Not configured"
		detail = "No active IdentityAgent is configured"
	default:
		if ownerPath := strings.TrimSpace(m.snapshot.IdentityAgentOwner.Path); ownerPath != "" {
			detail += " (" + ownerPath + ")"
		}
	}

	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "IdentityAgent",
			Status: status,
			Detail: detail,
			Tone:   doctorscreen.ToneDanger,
		},
	}
}

func (m *model) doctorSyncAccountRow() doctorRow {
	if row, unavailable := m.doctorRuntimePathUnavailableRow("Sync Account"); unavailable {
		return row
	}
	if m.runtimeLoaded {
		if syncErr := strings.TrimSpace(m.runtimeStatus.Error); syncErr != "" {
			return doctorRow{
				screen: doctorscreen.Row{
					Check:  "Sync Account",
					Status: theme.Glyphs.Cross + " Sync error",
					Detail: syncErr,
					Tone:   doctorscreen.ToneDanger,
				},
			}
		}
		if m.runtimeStatus.Syncing {
			return doctorRow{
				screen: doctorscreen.Row{
					Check:  "Sync Account",
					Status: theme.Glyphs.Pending + " Syncing",
					Detail: "Multi-device sync in progress",
					Tone:   doctorscreen.ToneWarning,
				},
			}
		}
		if m.runtimeStatus.Linked && m.runtimeStatus.Dirty {
			return doctorRow{
				screen: doctorscreen.Row{
					Check:  "Sync Account",
					Status: "! Pending changes",
					Detail: "Local changes have not synced yet",
					Tone:   doctorscreen.ToneWarning,
				},
			}
		}
	}

	if !m.snapshot.LoggedIn {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Sync Account",
				Status: "! Not logged in",
				Detail: "Multi-device sync unavailable",
				Tone:   doctorscreen.ToneWarning,
			},
		}
	}

	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "Sync Account",
			Status: theme.Glyphs.Check + " Logged in",
			Detail: "Multi-device sync available",
			Tone:   doctorscreen.ToneSuccess,
		},
	}
}

func (m *model) doctorSystemAuthRow() doctorRow {
	if row, unavailable := m.doctorRuntimePathUnavailableRow("System Auth"); unavailable {
		return row
	}
	if !m.securityLoaded {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "System Auth",
				Status: "! Checking",
				Detail: "Loading security state",
				Tone:   doctorscreen.ToneWarning,
			},
		}
	}
	if m.securityLoadErr != "" {
		return m.doctorSecurityLoadFailureRow("System Auth")
	}
	if m.securityState.HeadlessUnlock {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "System Auth",
				Status: "! Headless mode",
				Detail: "System Auth is intentionally skipped on this device",
				Tone:   doctorscreen.ToneWarning,
			},
		}
	}
	switch m.securityState.SystemAuthCapability {
	case securityCapabilityAvailable:
		status := theme.Glyphs.Check + " Available"
		detail := "System Auth is ready for sensitive actions"
		if runtime.GOOS == "linux" {
			status = theme.Glyphs.Check + " Detected"
			detail = "Desktop session and pkexec detected in this terminal"
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "System Auth",
				Status: status,
				Detail: detail,
				Tone:   doctorscreen.ToneSuccess,
			},
		}
	case securityCapabilityUnavailableByPlatform, securityCapabilityUnavailableByEnv:
		status := "! Not available"
		if m.securityState.SystemAuthCapability == securityCapabilityUnavailableByEnv {
			status = "! Unavailable here"
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "System Auth",
				Status: status,
				Detail: systemAuthUnavailableHint(m.securityState.SystemAuthCapability),
				Tone:   doctorscreen.ToneWarning,
			},
		}
	default:
		detail := "System Auth is expected but not working"
		if runtime.GOOS == "linux" {
			detail = "Forged auth helper or pkexec is not working"
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "System Auth",
				Status: theme.Glyphs.Cross + " Broken",
				Detail: detail,
				Tone:   doctorscreen.ToneDanger,
			},
		}
	}
}

func (m *model) doctorSecureStoreRow() doctorRow {
	if row, unavailable := m.doctorRuntimePathUnavailableRow("Secure Store"); unavailable {
		return row
	}
	if !m.securityLoaded {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Secure Store",
				Status: "! Checking",
				Detail: "Loading security state",
				Tone:   doctorscreen.ToneWarning,
			},
		}
	}
	if m.securityLoadErr != "" {
		return m.doctorSecurityLoadFailureRow("Secure Store")
	}
	if m.securityState.HeadlessUnlock {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Secure Store",
				Status: "! File-backed",
				Detail: "Device unlock trust is stored in a local file",
				Tone:   doctorscreen.ToneWarning,
			},
		}
	}
	switch m.securityState.SecureStoreCapability {
	case securityCapabilityAvailable:
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Secure Store",
				Status: theme.Glyphs.Check + " Available",
				Detail: "Local unlock trust can be stored securely",
				Tone:   doctorscreen.ToneSuccess,
			},
		}
	case securityCapabilityUnavailableByPlatform, securityCapabilityUnavailableByEnv:
		status := "! Unavailable"
		if runtime.GOOS == "linux" {
			status = "! Not supported"
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Secure Store",
				Status: status,
				Detail: secureStoreUnavailableHint(),
				Tone:   doctorscreen.ToneWarning,
			},
		}
	default:
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Secure Store",
				Status: theme.Glyphs.Cross + " Broken",
				Detail: "Local unlock trust cannot be persisted",
				Tone:   doctorscreen.ToneDanger,
			},
		}
	}
}

func (m *model) doctorSecurityLoadFailureRow(check string) doctorRow {
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  check,
			Status: theme.Glyphs.Cross + " Check failed",
			Detail: "Security inspection failed: " + m.securityLoadErr,
			Tone:   doctorscreen.ToneDanger,
		},
	}
}

func systemAuthUnavailableHint(capability string) string {
	if capability == securityCapabilityUnavailableByEnv {
		switch runtime.GOOS {
		case "windows":
			return "Windows Hello can't prompt here"
		case "linux":
			return "No desktop prompt here; use --headless only on a trusted headless device"
		case "darwin":
			return "No desktop prompt (often SSH)"
		default:
			return "System Auth can't prompt here"
		}
	}

	switch runtime.GOOS {
	case "windows":
		return "Check Windows Hello setup/policy"
	case "linux":
		return "Unsupported on this platform"
	case "darwin":
		return "Touch ID/device auth unavailable"
	default:
		return "Unsupported on this platform"
	}
}

func secureStoreUnavailableHint() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows DPAPI is unavailable for this user"
	case "linux":
		return "Linux secure device-key storage is not implemented"
	default:
		return "Master-password trust cannot be remembered securely"
	}
}
