package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/actions"
	commonscreen "github.com/itzzritik/forged/cli/internal/tui/screens/common"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

const (
	labRouteListHeight  = 8
	labWideLayoutWidth  = 104
	labInspectorMinSize = 38
)

type labState struct {
	routing     actions.SSHRoutingDebug
	loading     bool
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

func (m *model) isDevBuild() bool {
	version := strings.ToLower(strings.TrimSpace(m.appVersion))
	version = strings.TrimPrefix(version, "v")
	return version == "dev" || strings.HasPrefix(version, "dev-") || strings.HasSuffix(version, "-dev")
}

func (m *model) labDashboardPages() []dashboardPage {
	if !m.isDevBuild() {
		return nil
	}
	return []dashboardPage{
		{
			Label:   "SSH Routing",
			Summary: "Inspect learned route memory, public key hints, and active SSH attempts",
			Route:   RouteLabRouting,
		},
	}
}

func (m *model) isLabRoutingRoute() bool {
	return m.isDevBuild() &&
		m.screen == screenDashboard &&
		m.snapshot.VaultExists &&
		m.session.Current().ID == RouteLabRouting
}

func (m *model) startLabRoutingRoute() tea.Cmd {
	m.lab.loading = true
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
	m.lab.busy = false
	if msg.err != nil {
		m.lab.err = msg.err.Error()
		return m, nil
	}
	m.lab.routing = msg.routing
	m.lab.err = ""
	m.normalizeLabSelection()
	return m, nil
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
		return m, nil
	}
	m.lab.notice = "Route memory cleared"
	m.lab.loading = true
	m.lab.requestID++
	id := m.lab.requestID
	return m, tea.Batch(m.spinner.Tick, m.loadLabRoutingCmd(id))
}

func (m *model) updateLabRoutingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.lab.loading || m.lab.busy {
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
	case "r", "R":
		return m, m.startLabRoutingRoute()
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
	if m.lab.selected >= m.lab.offset+labRouteListHeight {
		m.lab.offset = m.lab.selected - labRouteListHeight + 1
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
	if m.lab.loading {
		return commonscreen.RenderFullPageLoader(commonscreen.FullPageLoaderScreen{
			Title:       "Loading SSH routing",
			Description: "Reading learned routes, public hints, and active attempts",
		}, m.spinner.View(), contentWidth)
	}

	width := max(48, contentWidth)
	sections := []string{
		m.renderLabRoutingSummary(width),
		"",
		m.renderLabRoutingConsole(width),
	}

	bottom := m.labStatusLine(width)
	if bottom != "" {
		return shell.DockBottom(strings.Join(sections, "\n")+"\n", bottom)
	}
	return strings.Join(sections, "\n")
}

func (m *model) renderLabRoutingSummary(width int) string {
	routing := m.lab.routing
	gitRoutes, sshRoutes := labRouteKindCounts(routing.Routes)
	staleHints := 0
	for _, hint := range routing.PublicHints {
		if hint.Stale {
			staleHints++
		}
	}

	items := []string{
		labMetric(len(routing.Routes), "learned", theme.ToneAccent),
		labMetric(gitRoutes, "git", theme.ToneNeutral),
		labMetric(sshRoutes, "ssh", theme.ToneNeutral),
		labMetric(len(routing.RuntimeAttempts), "active", labToneForCount(len(routing.RuntimeAttempts), theme.ToneWarning)),
		labMetric(len(routing.PublicHints), "hints", labHintTone(staleHints)),
	}
	if staleHints > 0 {
		items = append(items, labMetric(staleHints, "stale", theme.ToneDanger))
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(items, theme.BodyMuted.Render("  ·  ")))
}

func (m *model) renderLabRoutingConsole(width int) string {
	if width >= labWideLayoutWidth {
		gap := 4
		inspectorWidth := max(labInspectorMinSize, min(54, width/3))
		listWidth := max(52, width-inspectorWidth-gap)
		if listWidth+inspectorWidth+gap > width {
			listWidth = max(42, width-inspectorWidth-gap)
		}
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().Width(listWidth).Render(m.renderLabRouteList(listWidth)),
			strings.Repeat(" ", gap),
			lipgloss.NewStyle().Width(inspectorWidth).Render(m.renderLabInspector(inspectorWidth)),
		)
	}

	return strings.Join([]string{
		m.renderLabRouteList(width),
		"",
		m.renderLabInspector(width),
	}, "\n")
}

func labMetric(value int, label string, tone theme.Tone) string {
	numberStyle := theme.BodyStrong
	switch tone {
	case theme.ToneAccent:
		numberStyle = theme.Kicker
	case theme.ToneSuccess:
		numberStyle = theme.Success
	case theme.ToneWarning:
		numberStyle = theme.Warning
	case theme.ToneDanger:
		numberStyle = theme.Danger
	}
	return numberStyle.Render(fmt.Sprintf("%d", value)) + " " + theme.BodyMuted.Render(label)
}

func labToneForCount(value int, nonZero theme.Tone) theme.Tone {
	if value > 0 {
		return nonZero
	}
	return theme.ToneNeutral
}

func labHintTone(stale int) theme.Tone {
	if stale > 0 {
		return theme.ToneDanger
	}
	return theme.ToneSuccess
}

func labRouteKindCounts(routes []actions.SSHRouteDebug) (gitRoutes int, sshRoutes int) {
	for _, route := range routes {
		switch route.Kind {
		case "git":
			gitRoutes++
		case "ssh":
			sshRoutes++
		}
	}
	return gitRoutes, sshRoutes
}

func (m *model) renderLabRouteList(width int) string {
	routes := m.lab.routing.Routes
	title := shell.JoinRow(
		width,
		theme.SectionTitle.Render("Routes"),
		theme.BodyMuted.Render(m.labRouteRangeLabel()),
	)
	if len(routes) == 0 {
		return title + "\n" + theme.BodyMuted.Render("No learned routes yet.")
	}
	m.normalizeLabSelection()

	start := m.lab.offset
	end := min(len(routes), start+labRouteListHeight)
	innerWidth := max(36, width-2)
	ageWidth := 8
	proofWidth := min(12, max(8, innerWidth/7))
	keyWidth := min(24, max(16, innerWidth/4))
	targetWidth := max(12, innerWidth-keyWidth-proofWidth-ageWidth-8)

	lines := []string{
		title,
		theme.RowLabel.Render(
			"  " +
				labPadRight("TARGET", targetWidth+2) +
				labPadRight("KEY", keyWidth+2) +
				labPadRight("PROOF", proofWidth+2) +
				"LAST",
		),
	}
	for index := start; index < end; index++ {
		route := routes[index]
		lines = append(lines, labRouteRow(route, index == m.lab.selected, targetWidth, keyWidth, proofWidth, ageWidth))
	}
	for len(lines) < labRouteListHeight+2 {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m *model) labRouteRangeLabel() string {
	total := len(m.lab.routing.Routes)
	if total == 0 {
		return "0 routes"
	}
	start := min(total, m.lab.offset+1)
	end := min(total, m.lab.offset+labRouteListHeight)
	return fmt.Sprintf("%d-%d of %d", start, end, total)
}

func labRouteRow(route actions.SSHRouteDebug, selected bool, targetWidth, keyWidth, proofWidth, ageWidth int) string {
	prefix := theme.BodyMuted.Render("  ")
	targetStyle := theme.BodyStrong
	keyStyle := theme.Body
	proofStyle := theme.BodyMuted
	ageStyle := theme.BodyMuted
	if selected {
		prefix = theme.Bullet.Render("▸ ")
		targetStyle = theme.Kicker
		keyStyle = theme.BodyStrong
		proofStyle = theme.BodyStrong
	}

	target := targetStyle.Render(labPadRight(labTruncate(routeLabel(route), targetWidth), targetWidth+2))
	key := keyStyle.Render(labPadRight(labTruncate(labRouteKeyLabel(route), keyWidth), keyWidth+2))
	proof := proofStyle.Render(labPadRight(labTruncate(labProofLabel(route.ProvenBy), proofWidth), proofWidth+2))
	last := ageStyle.Render(labTruncate(labRouteLastUsed(route), ageWidth))
	return prefix + target + key + proof + last
}

func (m *model) renderLabInspector(width int) string {
	route, ok := m.selectedLabRoute()
	if !ok {
		return theme.SectionTitle.Render("Inspector") + "\n" + theme.BodyMuted.Render("Select a route to inspect.")
	}

	routeTitle := shell.JoinRow(
		width,
		theme.SectionTitle.Render("Inspector"),
		theme.Chip(labRouteTypeLabel(route), theme.ToneAccent),
	)
	lines := []string{
		routeTitle,
		theme.BodyStrong.Width(width).Render(routeLabel(route)),
		theme.BodyMuted.Width(width).Render(labRouteSubtitle(route)),
		"",
		labInspectorRow("Key", labRouteKeyLabel(route), width),
		labInspectorRow("Ref", labFirstNonEmpty(route.KeyRef, "not linked"), width),
		labInspectorRow("Proof", labProofSentence(route.ProvenBy), width),
		labInspectorRow("Usage", labUsageLabel(route), width),
		labInspectorRow("Scope", labScopeLabel(route), width),
	}
	if fingerprint := labShortFingerprint(route.Fingerprint); fingerprint != "" {
		lines = append(lines, labInspectorRow("Fingerprint", fingerprint, width))
	}
	if len(route.Attempts) > 0 {
		lines = append(lines, labInspectorRow("Attempts", labAttemptsLabel(route.Attempts), width))
	}
	lines = append(lines, "", m.renderLabSignals(width))
	return strings.Join(lines, "\n")
}

func (m *model) renderLabSignals(width int) string {
	attempts := m.lab.routing.RuntimeAttempts
	hints := m.lab.routing.PublicHints
	staleHints := 0
	for _, hint := range hints {
		if hint.Stale {
			staleHints++
		}
	}

	active := "none"
	if len(attempts) > 0 {
		active = fmt.Sprintf("%d snippet", len(attempts))
		if len(attempts) != 1 {
			active += "s"
		}
		active += " · newest " + labAgeSeconds(attempts[0].AgeSeconds)
	}
	hintLabel := fmt.Sprintf("%d current", len(hints))
	if staleHints > 0 {
		hintLabel = fmt.Sprintf("%d current · %d stale", len(hints), staleHints)
	}

	lines := []string{
		theme.SectionTitle.Render("Signals"),
		labInspectorRow("Active", active, width),
		labInspectorRow("Key hints", hintLabel, width),
	}
	if len(hints) > 0 {
		lines = append(lines, labInspectorRow("Newest hint", labNewestHintLabel(hints), width))
	}
	return strings.Join(lines, "\n")
}

func (m *model) labStatusLine(width int) string {
	if m.lab.busy {
		return theme.BodyStrong.Width(width).Render(m.spinner.View() + " Clearing route memory")
	}
	if m.lab.err != "" {
		return theme.Danger.Width(width).Render("✕ " + m.lab.err)
	}
	if m.lab.notice != "" {
		return theme.Warning.Width(width).Render(m.lab.notice)
	}
	return theme.BodyMuted.Width(width).Render("Route memory is stored in the encrypted vault. Runtime snippets are temporary.")
}

func (m *model) labFooterActions() []shell.FooterAction {
	if m.lab.loading || m.lab.busy {
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
		{Key: "R", Label: "Refresh"},
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

func labRouteSubtitle(route actions.SSHRouteDebug) string {
	if route.Kind == "git" {
		return labFirstNonEmpty(route.Host, "git host") + " · " + labOperationLabel(route.Operation)
	}
	if route.Kind == "ssh" {
		return labFirstNonEmpty(route.Host, "ssh host") + " · " + labFirstNonEmpty(route.User, "unknown user")
	}
	return labFirstNonEmpty(route.Target, "unknown route")
}

func labRouteKeyLabel(route actions.SSHRouteDebug) string {
	return labFirstNonEmpty(route.KeyName, route.KeyRef, labShortFingerprint(route.Fingerprint), "unknown key")
}

func labRouteLastUsed(route actions.SSHRouteDebug) string {
	if route.LastSuccessAt != nil {
		return labAgo(*route.LastSuccessAt)
	}
	return labAgo(route.Updated)
}

func labRouteTypeLabel(route actions.SSHRouteDebug) string {
	switch route.Kind {
	case "git":
		return "Git repo"
	case "ssh":
		return "SSH host"
	default:
		return "Route"
	}
}

func labProofLabel(value string) string {
	switch value {
	case "provider_probe":
		return "Verified"
	case "ssh_auth":
		return "SSH Auth"
	case "":
		return "Learned"
	default:
		return strings.ReplaceAll(value, "_", " ")
	}
}

func labProofSentence(value string) string {
	switch value {
	case "provider_probe":
		return "Verified by provider access probe"
	case "ssh_auth":
		return "Learned from successful SSH authentication"
	case "":
		return "Learned route with no proof label"
	default:
		return labProofLabel(value)
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

func labUsageLabel(route actions.SSHRouteDebug) string {
	uses := fmt.Sprintf("%d successful uses", route.SuccessCount)
	if route.SuccessCount == 1 {
		uses = "1 successful use"
	}
	return uses + " · last " + labRouteLastUsed(route)
}

func labScopeLabel(route actions.SSHRouteDebug) string {
	switch route.Kind {
	case "git":
		return "repo-level route · " + labOperationLabel(route.Operation)
	case "ssh":
		return "host-level route · " + labFirstNonEmpty(route.User, "unknown user")
	default:
		return "learned route"
	}
}

func labAttemptsLabel(attempts []actions.SSHRouteDebugAttempt) string {
	if len(attempts) == 0 {
		return "none"
	}
	parts := make([]string, 0, min(3, len(attempts)))
	for index, attempt := range attempts {
		if index >= 3 {
			break
		}
		parts = append(parts, labFirstNonEmpty(attempt.KeyName, attempt.KeyRef, labShortFingerprint(attempt.Fingerprint), "unknown")+" "+labAgo(attempt.AttemptedAt))
	}
	if more := len(attempts) - len(parts); more > 0 {
		parts = append(parts, fmt.Sprintf("+%d more", more))
	}
	return strings.Join(parts, ", ")
}

func labNewestHintLabel(hints []actions.SSHRoutePublicHint) string {
	if len(hints) == 0 {
		return "none"
	}
	newest := hints[0]
	for _, hint := range hints[1:] {
		if hint.Updated.After(newest.Updated) {
			newest = hint
		}
	}
	label := labFirstNonEmpty(newest.KeyName, newest.Ref, labShortFingerprint(newest.Fingerprint), "unknown")
	if newest.Stale {
		label += " · stale"
	}
	return label + " · " + labAgo(newest.Updated)
}

func labInspectorRow(label, value string, width int) string {
	labelWidth := 11
	valueWidth := max(10, width-labelWidth-2)
	return labPadRight(theme.RowLabel.Render(labTruncate(label, labelWidth)), labelWidth+2) +
		theme.Body.Render(labTruncate(strings.TrimSpace(value), valueWidth))
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

func labPadRight(value string, width int) string {
	if width <= 0 {
		return value
	}
	visible := lipgloss.Width(value)
	if visible >= width {
		return value
	}
	return value + strings.Repeat(" ", width-visible)
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
