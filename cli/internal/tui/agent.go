package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/tui/components"
	agentscreen "github.com/itzzritik/forged/cli/internal/tui/screens/agent"
	dashboardscreen "github.com/itzzritik/forged/cli/internal/tui/screens/dashboard"
	keyscreen "github.com/itzzritik/forged/cli/internal/tui/screens/keys"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type agentItemID string

const (
	agentItemSSHToggle     agentItemID = "ssh-toggle"
	agentItemCommitSigning agentItemID = "commit-signing"
	agentItemSSHRouting    agentItemID = "ssh-routing"
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
	loading      bool
	all          []actions.KeySummary
	rows         []actions.KeySummary
	selected     int
	offset       int
	searchActive bool
	input        textinput.Model
	err          string
	busy         bool
	busyMessage  string
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
		{ID: agentItemSSHRouting, Label: "SSH Routing", Summary: "Inspect and clear learned SSH and Git route memory"},
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
	bottomSections := make([]string, 0, 2)
	if item, ok := m.selectedAgentItem(); ok && strings.TrimSpace(item.Summary) != "" {
		bottomSections = append(bottomSections, theme.BodyMuted.Width(max(24, min(contentWidth, theme.HeroMaxWidth))).Render(item.Summary))
	}
	if errText := strings.TrimSpace(m.agent.statusErr); errText != "" {
		bottomSections = append(bottomSections, theme.Danger.Render("✕ "+errText))
	}

	top := strings.Join(sections, "\n")
	if len(bottomSections) == 0 {
		return top
	}
	return shell.DockBottom(top+"\n", strings.Join(bottomSections, "\n\n"))
}

func (m *model) renderAgentSigningBody(contentWidth int) string {
	m.ensureAgentSigningInput()

	rows := m.agentSigningVisibleRows()
	browserRows := make([]keyscreen.BrowserRow, 0, len(rows))
	for _, key := range rows {
		row := keyscreen.BrowserRow{
			Name:        key.Name,
			Type:        key.Type,
			Fingerprint: key.Fingerprint,
		}
		if m.isAgentSigningKeyApplied(key) {
			row.StatusIcon = "✓"
		}
		browserRows = append(browserRows, row)
	}

	return agentscreen.RenderSigning(agentscreen.SigningScreen{
		Loading:     m.agent.signing.loading,
		Busy:        m.agent.signing.busy,
		BusyMessage: m.agent.signing.busyMessage,
		Status:      m.signingStatus,
		Browser: keyscreen.BrowserScreen{
			SearchView:    m.agent.signing.input.View(),
			SearchQuery:   m.agent.signing.input.Value(),
			SearchActive:  m.agent.signing.searchActive,
			CountLabel:    m.agentSigningCountLabel(),
			Rows:          browserRows,
			SelectedIndex: m.agentSigningSelectedIndex(),
			Loading:       m.agent.signing.loading,
			Error:         m.agent.signing.err,
			ShowTopBorder: true,
			ShowStatus:    true,
			EmptySubtitle: "Generate or import a key to enable commit signing",
		},
	}, m.spinner.View(), contentWidth)
}

func (m *model) keyMatchesCurrentSigning(publicKey string) bool {
	if !m.signingLoaded || m.signingStatus.Mode != actions.CommitSigningForged {
		return false
	}
	return strings.TrimSpace(publicKey) != "" && strings.TrimSpace(publicKey) == strings.TrimSpace(m.signingStatus.PublicKey)
}

func (m *model) ensureAgentSigningInput() {
	if m.agent.signing.input.Cursor.BlinkSpeed != 0 {
		m.resizeAgentInputs()
		return
	}
	query := strings.TrimSpace(m.agent.signing.input.Value())
	active := m.agent.signing.searchActive
	m.agent.signing.input = newKeyInput("Search keys")
	if query != "" {
		m.agent.signing.input.SetValue(query)
	}
	if active {
		m.agent.signing.input.Focus()
	}
	m.resizeAgentInputs()
}

func (m *model) resizeAgentInputs() {
	m.resizeAgentSigningSearchInput()
}

func (m *model) resizeAgentSigningSearchInput() {
	m.ensureAgentSigningInputState()
	rowWidth := shell.BodyWidth(m.width)
	searchWidth := max(1, rowWidth-keyBrowserSearchPrefixWidth-keyBrowserSearchCursorWidth)
	if countLabel := strings.TrimSpace(m.agentSigningCountLabelForWidth(rowWidth)); countLabel != "" {
		searchWidth = max(1, rowWidth-lipgloss.Width(countLabel)-keyBrowserSearchPrefixWidth-keyBrowserSearchGapWidth-keyBrowserSearchCursorWidth)
	}
	m.agent.signing.input.Width = searchWidth
}

func (m *model) ensureAgentSigningInputState() {
	if m.agent.signing.input.Cursor.BlinkSpeed == 0 {
		m.agent.signing.input = newKeyInput("Search keys")
	}
}

func (m *model) refreshAgentSigningRows() {
	query := strings.TrimSpace(m.agent.signing.input.Value())
	resolution := actions.ResolveKeyQuery(m.agent.signing.all, query)
	m.agent.signing.rows = resolution.Matches
	if query == "" {
		m.agent.signing.rows = actions.ResolveKeyQuery(m.agent.signing.all, "").Matches
	}
	m.resizeAgentSigningSearchInput()

	if len(m.agent.signing.rows) == 0 {
		m.agent.signing.selected = 0
		m.agent.signing.offset = 0
		return
	}
	if m.agent.signing.selected >= len(m.agent.signing.rows) {
		m.agent.signing.selected = len(m.agent.signing.rows) - 1
	}
	if m.agent.signing.selected < 0 {
		m.agent.signing.selected = 0
	}
	m.ensureAgentSigningVisible()
}

func (m *model) moveAgentSigningSelection(delta int) {
	if len(m.agent.signing.rows) == 0 {
		return
	}
	m.agent.signing.selected = min(max(m.agent.signing.selected+delta, 0), len(m.agent.signing.rows)-1)
	m.ensureAgentSigningVisible()
}

func (m *model) ensureAgentSigningVisible() {
	if m.agent.signing.selected < m.agent.signing.offset {
		m.agent.signing.offset = m.agent.signing.selected
	}
	if m.agent.signing.selected >= m.agent.signing.offset+keyscreen.VisibleRows() {
		m.agent.signing.offset = m.agent.signing.selected - keyscreen.VisibleRows() + 1
	}
	if m.agent.signing.offset < 0 {
		m.agent.signing.offset = 0
	}
}

func (m *model) agentSigningVisibleRows() []actions.KeySummary {
	if len(m.agent.signing.rows) == 0 {
		return nil
	}
	start := min(max(m.agent.signing.offset, 0), len(m.agent.signing.rows))
	end := min(len(m.agent.signing.rows), start+keyscreen.VisibleRows())
	return m.agent.signing.rows[start:end]
}

func (m *model) agentSigningSelectedIndex() int {
	if m.agent.signing.selected < m.agent.signing.offset {
		return 0
	}
	return m.agent.signing.selected - m.agent.signing.offset
}

func (m *model) agentSigningCountLabel() string {
	return m.agentSigningCountLabelForWidth(shell.BodyWidth(m.width))
}

func (m *model) agentSigningCountLabelForWidth(width int) string {
	total := len(m.agent.signing.all)
	if total == 0 {
		if m.agent.signing.loading {
			return ""
		}
		return chooseKeyBrowserCountLabel(width, "0 keys", "0")
	}

	filtered := len(m.agent.signing.rows)
	if strings.TrimSpace(m.agent.signing.input.Value()) != "" {
		return chooseKeyBrowserCountLabel(
			width,
			strings.TrimSpace(strings.Join([]string{strconv.Itoa(filtered), "of", strconv.Itoa(total), "keys"}, " ")),
			strconv.Itoa(filtered)+"/"+strconv.Itoa(total)+" keys",
			strconv.Itoa(filtered)+"/"+strconv.Itoa(total),
		)
	}
	if total == 1 {
		return chooseKeyBrowserCountLabel(width, "1 key", "1")
	}
	return chooseKeyBrowserCountLabel(width, strconv.Itoa(total)+" keys", strconv.Itoa(total))
}

func (m *model) selectedAgentSigningKey() (actions.KeySummary, bool) {
	if len(m.agent.signing.rows) == 0 || m.agent.signing.selected < 0 || m.agent.signing.selected >= len(m.agent.signing.rows) {
		return actions.KeySummary{}, false
	}
	return m.agent.signing.rows[m.agent.signing.selected], true
}

func (m *model) isAgentSigningKeyApplied(key actions.KeySummary) bool {
	if m.signingStatus.Mode != actions.CommitSigningForged {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(key.Name), strings.TrimSpace(m.signingStatus.KeyName))
}

func (m *model) selectedAgentSigningKeyApplied() bool {
	key, ok := m.selectedAgentSigningKey()
	if !ok {
		return false
	}
	return m.isAgentSigningKeyApplied(key)
}

func (m *model) selectAgentSigningByName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for index, key := range m.agent.signing.rows {
		if strings.EqualFold(strings.TrimSpace(key.Name), name) {
			m.agent.signing.selected = index
			m.ensureAgentSigningVisible()
			return true
		}
	}
	return false
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
	m.ensureAgentSigningInput()

	if m.agent.signing.busy {
		switch msg.String() {
		case "esc":
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		}
		return m, nil
	}

	if m.agent.signing.loading {
		if msg.String() == "esc" {
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		}
		return m, nil
	}

	if m.agent.signing.searchActive {
		switch msg.String() {
		case "esc":
			m.agent.signing.searchActive = false
			m.agent.signing.input.Blur()
			m.agent.signing.input.SetValue("")
			m.refreshAgentSigningRows()
			return m, nil
		case "enter":
			m.agent.signing.searchActive = false
			m.agent.signing.input.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.agent.signing.input, cmd = m.agent.signing.input.Update(msg)
		m.refreshAgentSigningRows()
		return m, cmd
	}

	if m.agent.signing.err != "" {
		switch msg.String() {
		case "esc":
			m.agent.signing.err = ""
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		case "enter":
			return m, m.startAgentSigningRoute()
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
	case "/":
		m.agent.signing.err = ""
		m.agent.signing.searchActive = true
		m.agent.signing.input.Focus()
		return m, textinput.Blink
	case "up", "k":
		m.agent.signing.err = ""
		m.moveAgentSigningSelection(-1)
		return m, nil
	case "down", "j":
		m.agent.signing.err = ""
		m.moveAgentSigningSelection(1)
		return m, nil
	case "d":
		if !m.signingStatus.Enabled() {
			return m, nil
		}
		m.agent.signing.err = ""
		return m, m.runDisableCommitSigning()
	case "enter":
		key, ok := m.selectedAgentSigningKey()
		if !ok || m.isAgentSigningKeyApplied(key) {
			return m, nil
		}
		m.agent.signing.err = ""
		return m, m.runEnableCommitSigning(key.Name)
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
	case agentItemSSHRouting:
		if m.session.Current().ID != RouteAgentRouting {
			m.session.Push(Route{ID: RouteAgentRouting})
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
	m.agent.signing.all = nil
	m.agent.signing.rows = nil
	m.agent.signing.selected = 0
	m.agent.signing.offset = 0
	m.agent.signing.searchActive = false
	m.agent.signing.input = newKeyInput("Search keys")
	m.agent.signing.input.Blur()
	m.resizeAgentInputs()
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
		m.signingLoaded = true
		m.signingError = msg.err.Error()
		return m, nil
	}
	m.signingStatus = msg.status
	m.signingLoaded = true
	m.signingError = ""
	if m.isAgentSigningRoute() && msg.status.Mode == actions.CommitSigningForged {
		m.selectAgentSigningByName(msg.status.KeyName)
	}
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
		m.agent.signing.all = nil
		m.agent.signing.rows = nil
		return m, nil
	}

	m.agent.signing.err = ""
	m.agent.signing.all = msg.keys
	m.refreshAgentSigningRows()
	if routeName := strings.TrimSpace(m.session.Current().Params["name"]); routeName != "" {
		if m.selectAgentSigningByName(routeName) {
			return m, nil
		}
	}
	if m.signingStatus.Mode == actions.CommitSigningForged && m.selectAgentSigningByName(m.signingStatus.KeyName) {
		return m, nil
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
	m.signingError = ""
	if msg.status.Mode == actions.CommitSigningForged {
		m.selectAgentSigningByName(msg.status.KeyName)
	}
	return m, nil
}
