package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/config"
	dashboardscreen "github.com/itzzritik/forged/cli/internal/tui/screens/dashboard"
	doctorscreen "github.com/itzzritik/forged/cli/internal/tui/screens/doctor"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
)

const (
	doctorSeverityDanger = iota
	doctorSeverityWarning
	doctorSeveritySuccess
)

type doctorRow struct {
	screen   doctorscreen.Row
	severity int
	order    int
}

func (m *model) isDoctorOverviewRoute() bool {
	return m.screen == screenDashboard && m.snapshot.VaultExists && m.session.Current().ID == RouteDoctorOverview
}

func (m *model) isDoctorDashboardTab() bool {
	if m.screen != screenDashboard ||
		!m.snapshot.VaultExists ||
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

func (m *model) renderDoctorBody(contentWidth int) string {
	rows := m.doctorRows()
	screenRows := make([]doctorscreen.Row, 0, len(rows))
	for _, row := range rows {
		screenRows = append(screenRows, row.screen)
	}
	return shell.IndentBlock(doctorscreen.Render(doctorscreen.Screen{Rows: screenRows}, contentWidth), 2)
}

func (m *model) renderDoctorDashboardBody(contentWidth int) string {
	tabs, _, _ := m.dashboardRootScreen()
	tabBar := dashboardscreen.Render(dashboardscreen.Screen{
		Tabs: tabs,
		Notice: dashboardscreen.Notice{
			Message: m.notice.message,
			Tone:    m.notice.tone,
		},
	}, contentWidth)
	body := m.renderDoctorBody(contentWidth)

	switch {
	case strings.TrimSpace(tabBar) == "":
		return body
	case strings.TrimSpace(body) == "":
		return tabBar
	default:
		return tabBar + "\n\n" + body
	}
}

func (m *model) doctorFooterActions(includeTabs bool) []shell.FooterAction {
	actions := make([]shell.FooterAction, 0, 4)
	if includeTabs {
		actions = append(actions, shell.FooterAction{Key: "←/→", Label: "Tabs"})
	}
	if m.doctorCanFixIssues() && m.maintenanceProgress == nil {
		actions = append(actions, shell.FooterAction{Key: "Enter", Label: "Fix Issues"})
	}
	actions = append(actions,
		shell.FooterAction{Key: "R", Label: "Refresh"},
		shell.FooterAction{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
	)
	return actions
}

func (m *model) updateDoctorKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "r":
		return m, m.refreshSnapshotCmd()
	case "enter":
		if !m.doctorCanFixIssues() || m.maintenanceProgress != nil {
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
	case "r":
		return m, m.refreshSnapshotCmd()
	case "enter":
		if !m.doctorCanFixIssues() || m.maintenanceProgress != nil {
			return m, nil
		}
		return m, m.startDoctorRepair(nil)
	default:
		return m, nil
	}
}

func (m *model) startDoctorRepair(password []byte) tea.Cmd {
	return m.startMaintenance(
		maintenanceTriggerDoctor,
		password,
		false,
		"Fixing Issues",
		"Reviewing this machine and repairing detected issues.",
		"",
	)
}

func (m *model) doctorCanFixIssues() bool {
	s := m.snapshot
	if !s.VaultExists || !s.ConfigExists {
		return true
	}
	if !s.Service.Installed || !s.Service.ConfigValid || !s.Service.Running {
		return true
	}
	if !s.IPCSocketReady || !s.AgentSocketReady {
		return true
	}
	if s.AgentDisabled {
		return true
	}
	if !s.SSHEnabled || !s.ManagedConfigReady || !s.IdentityAgentOwner.IsForged() {
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
	if m.shouldShowExternalUsePolicy() {
		rows = append(rows, m.doctorExternalUsePolicyRow())
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].severity != rows[j].severity {
			return rows[i].severity < rows[j].severity
		}
		return rows[i].order < rows[j].order
	})
	return rows
}

func (m *model) doctorVaultRow(paths config.Paths) doctorRow {
	if m.snapshot.VaultExists {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Vault",
				Status: "✓ Present",
				Detail: paths.VaultFile(),
				Tone:   doctorscreen.ToneSuccess,
			},
			severity: doctorSeveritySuccess,
			order:    0,
		}
	}

	detail := "Set up or restore this device"
	if m.snapshot.LoggedIn {
		detail = "Fix Issues can restore this device"
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "Vault",
			Status: "✕ Missing",
			Detail: detail,
			Tone:   doctorscreen.ToneDanger,
		},
		severity: doctorSeverityDanger,
		order:    0,
	}
}

func (m *model) doctorConfigRow(paths config.Paths) doctorRow {
	if m.snapshot.ConfigExists {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Config",
				Status: "✓ Ready",
				Detail: paths.ConfigFile(),
				Tone:   doctorscreen.ToneSuccess,
			},
			severity: doctorSeveritySuccess,
			order:    1,
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "Config",
			Status: "✕ Missing",
			Detail: "Run Fix Issues",
			Tone:   doctorscreen.ToneDanger,
		},
		severity: doctorSeverityDanger,
		order:    1,
	}
}

func (m *model) doctorServiceRow() doctorRow {
	if m.snapshot.Service.Installed && m.snapshot.Service.ConfigValid {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Service",
				Status: "✓ Installed",
				Detail: "System service ready",
				Tone:   doctorscreen.ToneSuccess,
			},
			severity: doctorSeveritySuccess,
			order:    2,
		}
	}

	status := "✕ Not installed"
	detail := "Run Fix Issues"
	if m.snapshot.Service.Installed && !m.snapshot.Service.ConfigValid {
		status = "✕ Invalid"
		detail = strings.TrimSpace(m.snapshot.Service.Detail)
		if detail == "" {
			detail = "Service configuration is invalid"
		}
	}

	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "Service",
			Status: status,
			Detail: detail,
			Tone:   doctorscreen.ToneDanger,
		},
		severity: doctorSeverityDanger,
		order:    2,
	}
}

func (m *model) doctorDaemonRow() doctorRow {
	if m.snapshot.Service.Running {
		detail := "Running"
		if m.snapshot.DaemonPID > 0 {
			detail = fmt.Sprintf("PID %d", m.snapshot.DaemonPID)
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Daemon",
				Status: "✓ Running",
				Detail: detail,
				Tone:   doctorscreen.ToneSuccess,
			},
			severity: doctorSeveritySuccess,
			order:    3,
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "Daemon",
			Status: "✕ Not running",
			Detail: "Run Fix Issues",
			Tone:   doctorscreen.ToneDanger,
		},
		severity: doctorSeverityDanger,
		order:    3,
	}
}

func (m *model) doctorIPCSocketRow(paths config.Paths) doctorRow {
	if m.snapshot.IPCSocketReady {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "IPC Socket",
				Status: "✓ Ready",
				Detail: paths.CtlSocket(),
				Tone:   doctorscreen.ToneSuccess,
			},
			severity: doctorSeveritySuccess,
			order:    4,
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "IPC Socket",
			Status: "✕ Not responding",
			Detail: paths.CtlSocket(),
			Tone:   doctorscreen.ToneDanger,
		},
		severity: doctorSeverityDanger,
		order:    4,
	}
}

func (m *model) doctorAgentSocketRow(paths config.Paths) doctorRow {
	if m.snapshot.AgentSocketReady {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Agent Socket",
				Status: "✓ Ready",
				Detail: paths.AgentSocket(),
				Tone:   doctorscreen.ToneSuccess,
			},
			severity: doctorSeveritySuccess,
			order:    5,
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "Agent Socket",
			Status: "✕ Not responding",
			Detail: paths.AgentSocket(),
			Tone:   doctorscreen.ToneDanger,
		},
		severity: doctorSeverityDanger,
		order:    5,
	}
}

func (m *model) doctorSSHAgentRow() doctorRow {
	if m.snapshot.AgentDisabled {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "SSH Agent",
				Status: "! Disabled",
				Detail: "Fix Issues will re-enable it",
				Tone:   doctorscreen.ToneWarning,
			},
			severity: doctorSeverityWarning,
			order:    6,
		}
	}
	if m.snapshot.SSHEnabled {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "SSH Agent",
				Status: "✓ Active",
				Detail: "Forged SSH include is configured",
				Tone:   doctorscreen.ToneSuccess,
			},
			severity: doctorSeveritySuccess,
			order:    6,
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "SSH Agent",
			Status: "✕ Not active",
			Detail: "Run Fix Issues",
			Tone:   doctorscreen.ToneDanger,
		},
		severity: doctorSeverityDanger,
		order:    6,
	}
}

func (m *model) doctorSSHConfigRow(paths config.Paths) doctorRow {
	if m.snapshot.AgentDisabled {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "SSH Config",
				Status: "! Disabled",
				Detail: "Fix Issues will re-enable it",
				Tone:   doctorscreen.ToneWarning,
			},
			severity: doctorSeverityWarning,
			order:    7,
		}
	}
	if m.snapshot.ManagedConfigReady {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "SSH Config",
				Status: "✓ Ready",
				Detail: paths.SSHManagedConfig(),
				Tone:   doctorscreen.ToneSuccess,
			},
			severity: doctorSeveritySuccess,
			order:    7,
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "SSH Config",
			Status: "✕ Missing",
			Detail: paths.SSHManagedConfig(),
			Tone:   doctorscreen.ToneDanger,
		},
		severity: doctorSeverityDanger,
		order:    7,
	}
}

func (m *model) doctorIdentityAgentRow(paths config.Paths) doctorRow {
	if m.snapshot.AgentDisabled {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "IdentityAgent",
				Status: "! Disabled",
				Detail: "Fix Issues will re-enable it",
				Tone:   doctorscreen.ToneWarning,
			},
			severity: doctorSeverityWarning,
			order:    8,
		}
	}
	if m.snapshot.IdentityAgentOwner.IsForged() {
		detail := paths.AgentSocket()
		if ownerPath := strings.TrimSpace(m.snapshot.IdentityAgentOwner.Path); ownerPath != "" {
			detail = ownerPath
		}
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "IdentityAgent",
				Status: "✓ Forged",
				Detail: detail,
				Tone:   doctorscreen.ToneSuccess,
			},
			severity: doctorSeveritySuccess,
			order:    8,
		}
	}

	status := "✕ Not Forged"
	detail := strings.TrimSpace(m.snapshot.IdentityAgentOwner.Name)
	switch detail {
	case "":
		status = "✕ Unknown"
		detail = "Could not inspect active ssh configuration"
	case "None":
		status = "✕ Not configured"
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
		severity: doctorSeverityDanger,
		order:    8,
	}
}

func (m *model) doctorSyncAccountRow() doctorRow {
	if m.snapshot.LoggedIn {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Sync Account",
				Status: "✓ Logged in",
				Detail: "Multi-device sync available",
				Tone:   doctorscreen.ToneSuccess,
			},
			severity: doctorSeveritySuccess,
			order:    12,
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "Sync Account",
			Status: "! Not logged in",
			Detail: "Multi-device sync unavailable",
			Tone:   doctorscreen.ToneWarning,
		},
		severity: doctorSeverityWarning,
		order:    12,
	}
}

func (m *model) doctorSystemAuthRow() doctorRow {
	if !m.securityLoaded {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "System Auth",
				Status: "! Checking",
				Detail: "Loading security state",
				Tone:   doctorscreen.ToneWarning,
			},
			severity: doctorSeverityWarning,
			order:    9,
		}
	}
	switch m.securityState.SystemAuthCapability {
	case securityCapabilityAvailable:
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "System Auth",
				Status: "✓ Available",
				Detail: "System authentication is ready for sensitive actions",
				Tone:   doctorscreen.ToneSuccess,
			},
			severity: doctorSeveritySuccess,
			order:    9,
		}
	case securityCapabilityUnavailableByPlatform, securityCapabilityUnavailableByEnv:
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "System Auth",
				Status: "! Unavailable",
				Detail: "External use follows your configured policy on this machine",
				Tone:   doctorscreen.ToneWarning,
			},
			severity: doctorSeverityWarning,
			order:    9,
		}
	default:
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "System Auth",
				Status: "✕ Broken",
				Detail: "System authentication is expected but not working",
				Tone:   doctorscreen.ToneDanger,
			},
			severity: doctorSeverityDanger,
			order:    9,
		}
	}
}

func (m *model) doctorSecureStoreRow() doctorRow {
	if !m.securityLoaded {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Secure Store",
				Status: "! Checking",
				Detail: "Loading security state",
				Tone:   doctorscreen.ToneWarning,
			},
			severity: doctorSeverityWarning,
			order:    10,
		}
	}
	switch m.securityState.SecureStoreCapability {
	case securityCapabilityAvailable:
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Secure Store",
				Status: "✓ Available",
				Detail: "Local unlock trust can be stored securely",
				Tone:   doctorscreen.ToneSuccess,
			},
			severity: doctorSeveritySuccess,
			order:    10,
		}
	case securityCapabilityUnavailableByPlatform, securityCapabilityUnavailableByEnv:
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Secure Store",
				Status: "! Unavailable",
				Detail: "Master-password trust cannot be remembered securely",
				Tone:   doctorscreen.ToneWarning,
			},
			severity: doctorSeverityWarning,
			order:    10,
		}
	default:
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "Secure Store",
				Status: "✕ Broken",
				Detail: "Local unlock trust cannot be persisted",
				Tone:   doctorscreen.ToneDanger,
			},
			severity: doctorSeverityDanger,
			order:    10,
		}
	}
}

func (m *model) doctorExternalUsePolicyRow() doctorRow {
	if !m.securityLoaded {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "External Use",
				Status: "! Checking",
				Detail: "Loading security state",
				Tone:   doctorscreen.ToneWarning,
			},
			severity: doctorSeverityWarning,
			order:    11,
		}
	}
	if m.securityState.ExternalUsePolicy == config.ExternalUsePolicyAllow {
		return doctorRow{
			screen: doctorscreen.Row{
				Check:  "External Use",
				Status: "! Allow external",
				Detail: "SSH auth and signing may proceed without system auth on unsupported environments",
				Tone:   doctorscreen.ToneWarning,
			},
			severity: doctorSeverityWarning,
			order:    11,
		}
	}
	return doctorRow{
		screen: doctorscreen.Row{
			Check:  "External Use",
			Status: "✓ Deny by default",
			Detail: "SSH auth and signing are blocked when system auth is unavailable",
			Tone:   doctorscreen.ToneSuccess,
		},
		severity: doctorSeveritySuccess,
		order:    11,
	}
}
