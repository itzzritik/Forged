package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/tui/components"
	agentscreen "github.com/itzzritik/forged/cli/internal/tui/screens/agent"
	dashboardscreen "github.com/itzzritik/forged/cli/internal/tui/screens/dashboard"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type agentItemID string

const (
	agentItemSSHToggle     agentItemID = "ssh-toggle"
	agentItemCommitSigning agentItemID = "commit-signing"
	agentListMinHeight                 = 6
)

type agentItem struct {
	ID      agentItemID
	Label   string
	Summary string
}

type agentState struct {
	selected     int
	statusErr    string
	sshBusy      bool
	sshActionID  int
	signing      agentSigningState
	signingKeyID int
	actionID     int
}

type agentSigningState struct {
	loading     bool
	keys        []actions.KeySummary
	selected    int
	err         string
	busy        bool
	busyMessage string
}

type agentSSHFinishedMsg struct {
	id  int
	err error
}

type agentSigningKeysMsg struct {
	id   int
	keys []actions.KeySummary
	err  error
}

type agentSigningFinishedMsg struct {
	id     int
	status actions.CommitSigningStatus
	err    error
}

func (m *model) isAgentHomeRoute() bool {
	return m.screen == screenDashboard && m.snapshot.VaultExists && m.session.Current().ID == RouteAgentHome
}

func (m *model) isAgentSigningRoute() bool {
	return m.screen == screenDashboard && m.snapshot.VaultExists && m.session.Current().ID == RouteAgentSigning
}

func (m *model) agentUsesSpinner() bool {
	if !m.snapshot.VaultExists {
		return false
	}
	return m.agent.sshBusy || m.agent.signing.loading || m.agent.signing.busy || !m.signingLoaded
}

func (m *model) agentItems() []agentItem {
	sshLabel := "Enable SSH Agent"
	sshSummary := "Use Forged as your active SSH agent on this machine"
	if m.snapshot.IdentityAgentOwner.IsForged() {
		sshLabel = "Disable SSH Agent"
		sshSummary = "Stop using Forged as your active SSH agent on this machine"
	}

	signingSummary := "Review signing status and choose the key used for Git commit signing"
	switch m.signingStatus.Mode {
	case actions.CommitSigningForged:
		signingSummary = "Review or change the Forged key used to sign Git commits"
	case actions.CommitSigningExternal:
		signingSummary = "Review the current external signing key or switch commit signing to Forged"
	}

	return []agentItem{
		{ID: agentItemSSHToggle, Label: sshLabel, Summary: sshSummary},
		{ID: agentItemCommitSigning, Label: "Commit Signing", Summary: signingSummary},
	}
}

func (m *model) agentDashboardPages() []dashboardPage {
	items := m.agentItems()
	pages := make([]dashboardPage, 0, len(items))
	for _, item := range items {
		pages = append(pages, dashboardPage{
			Label:   item.Label,
			Summary: item.Summary,
		})
	}
	return pages
}

func (m *model) normalizeAgentSelection(items []agentItem) {
	if len(items) == 0 {
		m.agent.selected = 0
		return
	}
	if m.agent.selected < 0 {
		m.agent.selected = 0
	}
	if m.agent.selected >= len(items) {
		m.agent.selected = len(items) - 1
	}
}

func (m *model) selectedAgentItem() (agentItem, bool) {
	items := m.agentItems()
	if len(items) == 0 {
		return agentItem{}, false
	}
	m.normalizeAgentSelection(items)
	return items[m.agent.selected], true
}

func (m *model) renderAgentBody(contentWidth int) string {
	items := m.agentItems()
	if len(items) == 0 {
		return ""
	}
	m.normalizeAgentSelection(items)

	listItems := make([]components.SelectionListItem, 0, len(items))
	for index, item := range items {
		listItems = append(listItems, components.SelectionListItem{
			Label:    item.Label,
			Selected: index == m.agent.selected,
		})
	}

	sections := []string{components.RenderSelectionList(listItems, contentWidth, agentListMinHeight)}
	if item, ok := m.selectedAgentItem(); ok && strings.TrimSpace(item.Summary) != "" {
		sections = append(sections, "", theme.BodyMuted.Width(max(24, min(contentWidth, theme.HeroMaxWidth))).Render(item.Summary))
	}
	if errText := strings.TrimSpace(m.agent.statusErr); errText != "" {
		sections = append(sections, "", theme.Danger.Render("✕ "+errText))
	}
	return strings.Join(sections, "\n")
}

func (m *model) renderAgentSigningBody(contentWidth int) string {
	rows := make([]agentscreen.SigningKeyRow, 0, len(m.agent.signing.keys))
	for index, key := range m.agent.signing.keys {
		rows = append(rows, agentscreen.SigningKeyRow{
			Name:        key.Name,
			Fingerprint: key.Fingerprint,
			Selected:    index == m.agent.signing.selected,
		})
	}

	return agentscreen.RenderSigning(agentscreen.SigningScreen{
		Loading:     m.agent.signing.loading,
		Error:       m.agent.signing.err,
		Busy:        m.agent.signing.busy,
		BusyMessage: m.agent.signing.busyMessage,
		Status:      m.signingStatus,
		Rows:        rows,
	}, m.spinner.View(), contentWidth)
}

func (m *model) keyMatchesCurrentSigning(publicKey string) bool {
	if !m.signingLoaded || m.signingStatus.Mode != actions.CommitSigningForged {
		return false
	}
	return strings.TrimSpace(publicKey) != "" && strings.TrimSpace(publicKey) == strings.TrimSpace(m.signingStatus.PublicKey)
}

func (m *model) updateAgentKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.agentItems()
	m.normalizeAgentSelection(items)

	switch msg.String() {
	case "esc":
		m.agent.statusErr = ""
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "up", "k":
		m.agent.statusErr = ""
		if m.agent.selected > 0 {
			m.agent.selected--
		}
		return m, nil
	case "down", "j":
		m.agent.statusErr = ""
		if len(items) > 0 && m.agent.selected < len(items)-1 {
			m.agent.selected++
		}
		return m, nil
	case "enter":
		m.agent.statusErr = ""
		item, ok := m.selectedAgentItem()
		if !ok {
			return m, nil
		}
		return m.openAgentItem(item)
	}

	return m, nil
}

func (m *model) updateAgentSigningKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.agent.signing.busy {
		switch msg.String() {
		case "esc":
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.agent.signing.err = ""
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "up", "k":
		m.agent.signing.err = ""
		if m.agent.signing.selected > 0 {
			m.agent.signing.selected--
		}
		return m, nil
	case "down", "j":
		m.agent.signing.err = ""
		if len(m.agent.signing.keys) > 0 && m.agent.signing.selected < len(m.agent.signing.keys)-1 {
			m.agent.signing.selected++
		}
		return m, nil
	case "d":
		if !m.signingStatus.Enabled() {
			return m, nil
		}
		m.agent.signing.err = ""
		return m, m.runDisableCommitSigning()
	case "enter":
		if len(m.agent.signing.keys) == 0 {
			return m, nil
		}
		m.agent.signing.err = ""
		return m, m.runEnableCommitSigning(m.agent.signing.keys[m.agent.signing.selected].Name)
	}

	return m, nil
}

func (m *model) openAgentItem(item agentItem) (tea.Model, tea.Cmd) {
	switch item.ID {
	case agentItemSSHToggle:
		return m, m.runAgentSSHToggle()
	case agentItemCommitSigning:
		if m.session.Current().ID != RouteAgentSigning {
			m.session.Push(Route{ID: RouteAgentSigning})
		}
		return m, m.showCurrentRoute()
	default:
		return m, nil
	}
}

func (m *model) startAgentSigningRoute() tea.Cmd {
	m.agent.signing.loading = true
	m.agent.signing.err = ""
	m.agent.signing.busy = false
	m.agent.signing.busyMessage = ""
	return tea.Batch(
		m.spinner.Tick,
		m.loadSigningStatusCmd(),
		m.listAgentSigningKeysCmd(),
	)
}

func (m *model) runAgentSSHToggle() tea.Cmd {
	if m.snapshot.IdentityAgentOwner.IsForged() {
		return m.disableSSHAgentCmd()
	}
	return m.enableSSHAgentCmd()
}

func (m *model) enableSSHAgentCmd() tea.Cmd {
	enable := m.enableSSHAgent
	m.agent.sshActionID++
	actionID := m.agent.sshActionID
	m.agent.sshBusy = true
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			return agentSSHFinishedMsg{id: actionID, err: enable()}
		},
	)
}

func (m *model) disableSSHAgentCmd() tea.Cmd {
	disable := m.disableSSHAgent
	m.agent.sshActionID++
	actionID := m.agent.sshActionID
	m.agent.sshBusy = true
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			return agentSSHFinishedMsg{id: actionID, err: disable()}
		},
	)
}

func (m *model) listAgentSigningKeysCmd() tea.Cmd {
	m.agent.signingKeyID++
	loadID := m.agent.signingKeyID
	return func() tea.Msg {
		keys, err := actions.ListKeys(config.DefaultPaths())
		return agentSigningKeysMsg{id: loadID, keys: keys, err: err}
	}
}

func (m *model) runEnableCommitSigning(name string) tea.Cmd {
	enable := m.enableCommitSigning
	m.agent.actionID++
	actionID := m.agent.actionID
	m.agent.signing.busy = true
	m.agent.signing.busyMessage = "Updating signing configuration"
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			status, err := enable(name)
			return agentSigningFinishedMsg{id: actionID, status: status, err: err}
		},
	)
}

func (m *model) runDisableCommitSigning() tea.Cmd {
	disable := m.disableCommitSigning
	m.agent.actionID++
	actionID := m.agent.actionID
	m.agent.signing.busy = true
	m.agent.signing.busyMessage = "Disabling commit signing"
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			status, err := disable()
			return agentSigningFinishedMsg{id: actionID, status: status, err: err}
		},
	)
}

func (m *model) loadSigningStatusCmd() tea.Cmd {
	if m.loadSigningStatus == nil {
		return nil
	}
	load := m.loadSigningStatus
	m.signingLoadID++
	loadID := m.signingLoadID
	return func() tea.Msg {
		status, err := load()
		return signingStatusMsg{id: loadID, status: status, err: err}
	}
}

func (m *model) refreshSnapshotCmd() tea.Cmd {
	if m.loadSnapshot == nil {
		return nil
	}
	load := m.loadSnapshot
	return func() tea.Msg {
		snapshot, err := load()
		return snapshotRefreshMsg{snapshot: snapshot, err: err}
	}
}

func (m *model) handleSnapshotRefreshMsg(msg snapshotRefreshMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, nil
	}
	m.snapshot = msg.snapshot
	m.systemHeader = m.systemHeaderForSnapshot(msg.snapshot)
	return m, nil
}

func (m *model) handleSigningStatusMsg(msg signingStatusMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.signingLoadID {
		return m, nil
	}
	if msg.err != nil {
		return m, nil
	}
	m.signingStatus = msg.status
	m.signingLoaded = true
	return m, nil
}

func (m *model) handleAgentSSHFinishedMsg(msg agentSSHFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.agent.sshActionID {
		return m, nil
	}
	m.agent.sshBusy = false
	if msg.err != nil {
		m.agent.statusErr = msg.err.Error()
		m.notice = notice{message: msg.err.Error(), tone: dashboardscreen.ToneDanger}
		return m, nil
	}

	m.notice = notice{}
	m.agent.statusErr = ""
	return m, m.refreshSnapshotCmd()
}

func (m *model) handleAgentSigningKeysMsg(msg agentSigningKeysMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.agent.signingKeyID {
		return m, nil
	}
	m.agent.signing.loading = false
	if msg.err != nil {
		m.agent.signing.err = msg.err.Error()
		m.agent.signing.keys = nil
		return m, nil
	}

	m.agent.signing.err = ""
	m.agent.signing.keys = msg.keys
	if routeName := strings.TrimSpace(m.session.Current().Params["name"]); routeName != "" {
		for index, key := range msg.keys {
			if strings.EqualFold(strings.TrimSpace(key.Name), routeName) {
				m.agent.signing.selected = index
				return m, nil
			}
		}
	}
	if m.agent.signing.selected >= len(msg.keys) {
		m.agent.signing.selected = max(0, len(msg.keys)-1)
	}
	return m, nil
}

func (m *model) handleAgentSigningFinishedMsg(msg agentSigningFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.agent.actionID {
		return m, nil
	}

	m.agent.signing.busy = false
	m.agent.signing.busyMessage = ""
	if msg.err != nil {
		m.agent.signing.err = msg.err.Error()
		return m, nil
	}

	m.agent.signing.err = ""
	m.signingStatus = msg.status
	m.signingLoaded = true
	return m, nil
}
