package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/actions"
	keyscreen "github.com/itzzritik/forged/cli/internal/tui/screens/keys"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

const (
	labRoutingPollInterval = 2 * time.Second
	labRouteVisibleRows    = 10
)

type labState struct {
	routing     actions.SSHRoutingDebug
	loading     bool
	refreshing  bool
	busy        bool
	err         string
	notice      string
	selected    int
	offset      int
	clearTarget string
	clearAll    bool
	requestID   int
}

type labRoutingLoadedMsg struct {
	id      int
	routing actions.SSHRoutingDebug
	err     error
}

type labRoutingClearedMsg struct {
	id  int
	err error
}

type labRoutingPollMsg struct {
	id int
}

func (m *model) isLabRoutingRoute() bool {
	return m.screen == screenDashboard &&
		m.snapshot.VaultExists &&
		m.session.Current().ID == RouteAgentRouting
}

func (m *model) startLabRoutingRoute() tea.Cmd {
	m.lab.loading = len(m.lab.routing.Routes) == 0
	m.lab.refreshing = !m.lab.loading
	m.lab.busy = false
	m.lab.err = ""
	m.lab.notice = ""
	m.lab.clearTarget = ""
	m.lab.clearAll = false
	m.lab.requestID++
	id := m.lab.requestID
	return tea.Batch(m.spinner.Tick, m.loadLabRoutingCmd(id))
}

func (m *model) loadLabRoutingCmd(id int) tea.Cmd {
	load := m.loadSSHRoutingDebug
	return func() tea.Msg {
		routing, err := load()
		return labRoutingLoadedMsg{id: id, routing: routing, err: err}
	}
}

func (m *model) clearLabRoutingCmd(id int, target string, all bool) tea.Cmd {
	clearOne := m.clearSSHRoute
	clearAll := m.clearAllSSHRoutes
	return func() tea.Msg {
		var err error
		if all {
			err = clearAll()
		} else {
			err = clearOne(target)
		}
		return labRoutingClearedMsg{id: id, err: err}
	}
}

func (m *model) handleLabRoutingLoadedMsg(msg labRoutingLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.lab.requestID {
		return m, nil
	}
	m.lab.loading = false
	m.lab.refreshing = false
	m.lab.busy = false
	if msg.err != nil {
		m.lab.err = msg.err.Error()
		return m, m.scheduleLabRoutingPoll()
	}
	m.lab.routing = msg.routing
	m.lab.err = ""
	m.normalizeLabSelection()
	return m, m.scheduleLabRoutingPoll()
}

func (m *model) handleLabRoutingClearedMsg(msg labRoutingClearedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.lab.requestID {
		return m, nil
	}
	m.lab.busy = false
	m.lab.clearTarget = ""
	m.lab.clearAll = false
	if msg.err != nil {
		m.lab.err = msg.err.Error()
		return m, m.scheduleLabRoutingPoll()
	}
	m.lab.notice = "Route memory cleared"
	m.lab.loading = false
	m.lab.refreshing = true
	m.lab.requestID++
	id := m.lab.requestID
	return m, m.loadLabRoutingCmd(id)
}

func (m *model) handleLabRoutingPollMsg(msg labRoutingPollMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.lab.requestID || !m.isLabRoutingRoute() {
		return m, nil
	}
	if m.lab.busy || m.lab.clearTarget != "" || m.lab.clearAll {
		return m, m.scheduleLabRoutingPoll()
	}
	m.lab.refreshing = true
	return m, m.loadLabRoutingCmd(msg.id)
}

func (m *model) scheduleLabRoutingPoll() tea.Cmd {
	if !m.isLabRoutingRoute() {
		return nil
	}
	id := m.lab.requestID
	return tea.Tick(labRoutingPollInterval, func(time.Time) tea.Msg {
		return labRoutingPollMsg{id: id}
	})
}

func (m *model) updateLabRoutingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.lab.busy {
		switch msg.String() {
		case "esc":
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		}
		return m, nil
	}

	if m.lab.clearTarget != "" || m.lab.clearAll {
		switch msg.String() {
		case "enter":
			m.lab.busy = true
			m.lab.err = ""
			m.lab.notice = ""
			m.lab.requestID++
			id := m.lab.requestID
			return m, tea.Batch(m.spinner.Tick, m.clearLabRoutingCmd(id, m.lab.clearTarget, m.lab.clearAll))
		case "esc", "c", "C":
			m.lab.clearTarget = ""
			m.lab.clearAll = false
			m.lab.notice = ""
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "up", "k":
		if m.lab.selected > 0 {
			m.lab.selected--
			m.normalizeLabSelection()
		}
		return m, nil
	case "down", "j":
		if m.lab.selected < len(m.lab.routing.Routes)-1 {
			m.lab.selected++
			m.normalizeLabSelection()
		}
		return m, nil
	case "c":
		if route, ok := m.selectedLabRoute(); ok {
			m.lab.clearTarget = route.Target
			m.lab.notice = "Press Enter to clear the selected learned route."
		}
		return m, nil
	case "C":
		if len(m.lab.routing.Routes) > 0 {
			m.lab.clearAll = true
			m.lab.notice = "Press Enter to clear all learned routes."
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m *model) normalizeLabSelection() {
	if len(m.lab.routing.Routes) == 0 {
		m.lab.selected = 0
		m.lab.offset = 0
		return
	}
	if m.lab.selected < 0 {
		m.lab.selected = 0
	}
	if m.lab.selected >= len(m.lab.routing.Routes) {
		m.lab.selected = len(m.lab.routing.Routes) - 1
	}
	if m.lab.offset > m.lab.selected {
		m.lab.offset = m.lab.selected
	}
	if m.lab.selected >= m.lab.offset+labRouteVisibleRows {
		m.lab.offset = m.lab.selected - labRouteVisibleRows + 1
	}
	if m.lab.offset < 0 {
		m.lab.offset = 0
	}
}

func (m *model) selectedLabRoute() (actions.SSHRouteDebug, bool) {
	if len(m.lab.routing.Routes) == 0 {
		return actions.SSHRouteDebug{}, false
	}
	m.normalizeLabSelection()
	return m.lab.routing.Routes[m.lab.selected], true
}

func (m *model) renderLabRoutingBody(contentWidth int) string {
	width := max(36, min(contentWidth, theme.HeroMaxWidth+10))
	sections := []string{
		m.renderLabRouteSummary(width),
		"",
		m.renderLabRouteBrowser(width),
	}
	return strings.Join(sections, "\n")
}

func (m *model) renderLabRouteSummary(width int) string {
	route, ok := m.selectedLabRoute()
	if !ok {
		if m.lab.loading {
			return theme.BodyStrong.Render(m.spinner.View() + " Reading SSH route memory")
		}
		return strings.Join([]string{
			theme.Warning.Render("! No learned SSH routes"),
			theme.BodyMuted.Render("Routes appear here after successful Git or SSH authentication."),
		}, "\n")
	}

	status := labRouteSummaryTitle(route, width)
	if m.lab.loading {
		status = theme.BodyStrong.Render(m.spinner.View() + " Updating route memory")
	}

	return status + "\n" + labRouteSummaryMeta(route)
}

func (m *model) renderLabRouteBrowser(width int) string {
	return keyscreen.RenderBrowser(keyscreen.BrowserScreen{
		SearchView:       "Live route memory",
		SearchNotice:     m.labBrowserNotice(),
		CountLabel:       m.labRouteCountLabel(),
		NameHeader:       "TARGET",
		TypeHeader:       "KEY",
		DetailHeader:     "SERVICE",
		NameWidth:        34,
		TypeWidth:        24,
		MinDetailWidth:   12,
		VisibleRows:      labRouteVisibleRows,
		PreserveTypeCase: true,
		Rows:             m.labRouteBrowserRows(),
		SelectedIndex:    m.labRouteSelectedIndex(),
		ShowTopBorder:    true,
		HideFooter:       true,
		EmptyTitle:       "No routes to show",
		EmptySubtitle:    m.labEmptySubtitle(),
	}, m.spinner.View(), width)
}

func (m *model) labRouteBrowserRows() []keyscreen.BrowserRow {
	routes := m.labVisibleRoutes()
	rows := make([]keyscreen.BrowserRow, 0, len(routes))
	for _, route := range routes {
		rows = append(rows, keyscreen.BrowserRow{
			Name:        routeLabel(route),
			Type:        labRouteKeyLabel(route),
			Fingerprint: labRouteServiceLabel(route),
		})
	}
	return rows
}

func (m *model) labVisibleRoutes() []actions.SSHRouteDebug {
	routes := m.lab.routing.Routes
	if len(routes) == 0 {
		return nil
	}
	m.normalizeLabSelection()
	start := min(max(m.lab.offset, 0), len(routes))
	end := min(len(routes), start+labRouteVisibleRows)
	return routes[start:end]
}

func (m *model) labRouteSelectedIndex() int {
	if m.lab.selected < m.lab.offset {
		return 0
	}
	return m.lab.selected - m.lab.offset
}

func (m *model) labRouteCountLabel() string {
	total := len(m.lab.routing.Routes)
	if total == 0 {
		if m.lab.loading {
			return "loading"
		}
		return "0 routes"
	}
	start := min(total, m.lab.offset+1)
	end := min(total, m.lab.offset+labRouteVisibleRows)
	if total <= labRouteVisibleRows {
		if total == 1 {
			return "1 route"
		}
		return fmt.Sprintf("%d routes", total)
	}
	return fmt.Sprintf("%d-%d of %d routes", start, end, total)
}

func (m *model) labBrowserNotice() string {
	if m.lab.busy {
		return "Clearing route memory"
	}
	if strings.TrimSpace(m.lab.err) != "" {
		return "✕ " + m.lab.err
	}
	if strings.TrimSpace(m.lab.notice) != "" {
		return m.lab.notice
	}
	return ""
}

func (m *model) labEmptySubtitle() string {
	if m.lab.loading {
		return "Reading learned routes from the daemon"
	}
	if strings.TrimSpace(m.lab.err) != "" {
		return "Forged will retry while this screen is open"
	}
	return "Use Git or SSH once and learned routes will appear here"
}

func (m *model) labFooterActions() []shell.FooterAction {
	if m.lab.busy {
		return []shell.FooterAction{{Key: "Esc", Label: m.session.EscLabel(EscAuto)}}
	}
	if m.lab.clearTarget != "" || m.lab.clearAll {
		return []shell.FooterAction{
			{Key: "Enter", Label: "Confirm"},
			{Key: "Esc", Label: "Cancel"},
		}
	}
	return []shell.FooterAction{
		{Key: "↑/↓", Label: "Select"},
		{Key: "C", Label: "Clear Route"},
		{Key: "Shift+C", Label: "Clear All"},
		{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
	}
}

func routeLabel(route actions.SSHRouteDebug) string {
	if route.Kind == "git" && route.Owner != "" && route.Repo != "" {
		return fmt.Sprintf("%s/%s", route.Owner, route.Repo)
	}
	if route.User != "" && route.Host != "" {
		return fmt.Sprintf("%s@%s:%d", route.User, route.Host, route.Port)
	}
	return route.Target
}

func labRouteKeyLabel(route actions.SSHRouteDebug) string {
	return labFirstNonEmpty(route.KeyName, route.KeyRef, labShortFingerprint(route.Fingerprint), "unknown key")
}

func labRouteSummaryTitle(route actions.SSHRouteDebug, width int) string {
	targetWidth := max(18, (width*3)/5)
	keyWidth := max(14, width-targetWidth-3)
	target := theme.Success.Render(labTruncate(routeLabel(route), targetWidth))
	key := theme.BodyStrong.Render(labTruncate(labRouteKeyLabel(route), keyWidth))
	return target + theme.BodyMuted.Render("  ·  ") + key
}

func labRouteSummaryMeta(route actions.SSHRouteDebug) string {
	parts := []string{
		labRouteServiceLabel(route),
		labRouteAccessLabel(route),
		"last " + labRouteLastUsed(route),
		labRouteUseCount(route.SuccessCount),
	}
	return theme.BodyMuted.Render(strings.Join(parts, "  ·  "))
}

func labRouteServiceLabel(route actions.SSHRouteDebug) string {
	switch strings.ToLower(strings.TrimSpace(route.Host)) {
	case "github.com":
		return "GitHub"
	case "gitlab.com":
		return "GitLab"
	case "bitbucket.org":
		return "Bitbucket"
	}
	switch route.Kind {
	case "git":
		return labFirstNonEmpty(route.Host, "Git")
	case "ssh":
		return "SSH"
	default:
		return "Route"
	}
}

func labRouteAccessLabel(route actions.SSHRouteDebug) string {
	switch route.Kind {
	case "git":
		return labOperationLabel(route.Operation)
	case "ssh":
		return "SSH auth"
	default:
		return "learned"
	}
}

func labRouteLastUsed(route actions.SSHRouteDebug) string {
	if route.LastSuccessAt != nil {
		return labAgo(*route.LastSuccessAt)
	}
	return labAgo(route.Updated)
}

func labRouteUseCount(count int) string {
	if count == 1 {
		return "1 successful use"
	}
	return fmt.Sprintf("%d successful uses", count)
}

func labAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return labAgeSeconds(int64(time.Since(t).Seconds()))
}

func labAgeSeconds(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds ago", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dm ago", seconds/60)
	case seconds < 86400:
		return fmt.Sprintf("%dh ago", seconds/3600)
	default:
		return fmt.Sprintf("%dd ago", seconds/86400)
	}
}

func labOperationLabel(value string) string {
	switch value {
	case "read":
		return "read access"
	case "write":
		return "write access"
	case "ssh_auth":
		return "SSH authentication"
	case "":
		return "unknown operation"
	default:
		return strings.ReplaceAll(value, "_", " ")
	}
}

func labShortFingerprint(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "SHA256:") {
		value = strings.TrimPrefix(value, "SHA256:")
	}
	return "SHA256:" + labTruncate(value, 12)
}

func labTruncate(value string, width int) string {
	runes := []rune(strings.TrimSpace(value))
	if width <= 0 || len(runes) <= width {
		return string(runes)
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

func labFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
