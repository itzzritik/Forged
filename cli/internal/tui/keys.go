package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	keyscreen "github.com/itzzritik/forged/cli/internal/tui/screens/keys"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type keyListMsg struct {
	id       int
	keys     []actions.KeySummary
	err      error
	preserve bool
}

type keyDetailMsg struct {
	id     int
	detail actions.KeyDetail
	err    error
}

type keyRenameFinishedMsg struct {
	id     int
	result actions.RenameResult
	err    error
}

type keyDeleteFinishedMsg struct {
	id   int
	name string
	err  error
}

type keyBrowserState struct {
	loading    bool
	refreshing bool
	err        string
	notice     string

	all      []actions.KeySummary
	rows     []actions.KeySummary
	selected int
	offset   int

	searchActive bool
	input        textinput.Model
}

type keyDetailState struct {
	loading   bool
	resolving bool
	err       string
	key       actions.KeyDetail
}

type keyRenameState struct {
	loading   bool
	saving    bool
	resolving bool
	err       string

	original string
	input    textinput.Model
}

type keyDeleteState struct {
	loading   bool
	deleting  bool
	resolving bool
	err       string
	key       actions.KeySummary
}

func (m *model) isKeyRoute() bool {
	if m.screen != screenDashboard || !m.snapshot.VaultExists {
		return false
	}

	switch m.session.Current().ID {
	case RouteKeysBrowser, RouteKeysDetail, RouteKeysRename, RouteKeysDelete:
		return true
	default:
		return false
	}
}

func (m *model) keyUsesSpinner() bool {
	if !m.isKeyRoute() {
		return false
	}

	current := m.session.Current().ID
	switch current {
	case RouteKeysBrowser:
		return m.keyBrowser.loading
	case RouteKeysDetail:
		return m.keyDetail.loading
	case RouteKeysRename:
		return m.keyRename.loading || m.keyRename.saving
	case RouteKeysDelete:
		return m.keyDelete.loading || m.keyDelete.deleting
	default:
		return false
	}
}

func (m *model) keyRouteLoaded() bool {
	if !m.isKeyRoute() {
		return false
	}

	switch m.session.Current().ID {
	case RouteKeysBrowser:
		return m.keyBrowser.loading || m.keyBrowser.err != "" || len(m.keyBrowser.all) > 0 || strings.TrimSpace(m.keyBrowser.notice) != "" || strings.TrimSpace(m.keyBrowser.input.Value()) != ""
	case RouteKeysDetail:
		return !m.keyDetail.resolving && (m.keyDetail.loading || m.keyDetail.err != "" || strings.TrimSpace(m.keyDetail.key.Name) != "")
	case RouteKeysRename:
		return !m.keyRename.resolving && (m.keyRename.loading || m.keyRename.saving || m.keyRename.err != "" || strings.TrimSpace(m.keyRename.original) != "" || strings.TrimSpace(m.keyRename.input.Value()) != "")
	case RouteKeysDelete:
		return !m.keyDelete.resolving && (m.keyDelete.loading || m.keyDelete.deleting || m.keyDelete.err != "" || strings.TrimSpace(m.keyDelete.key.Name) != "")
	default:
		return false
	}
}

func (m *model) keyHeaderTitle() string {
	switch m.session.Current().ID {
	case RouteKeysBrowser:
		return "View keys"
	case RouteKeysDetail:
		if m.keyDetail.resolving {
			return ""
		}
		if name := strings.TrimSpace(m.keyDetail.key.Name); name != "" {
			return name
		}
		return "Key details"
	case RouteKeysRename:
		if m.keyRename.resolving {
			return ""
		}
		return "Rename key"
	case RouteKeysDelete:
		if m.keyDelete.resolving {
			return ""
		}
		return "Delete key"
	default:
		return ""
	}
}

func (m *model) keyBreadcrumbs() []shell.Breadcrumb {
	switch m.session.Current().ID {
	case RouteKeysBrowser:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Key", Current: true},
		}
	case RouteKeysDetail:
		if m.keyDetail.resolving {
			return nil
		}
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Key"},
			{Label: "View", Current: true},
		}
	case RouteKeysRename:
		if m.keyRename.resolving {
			return nil
		}
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Key"},
			{Label: "Rename", Current: true},
		}
	case RouteKeysDelete:
		if m.keyDelete.resolving {
			return nil
		}
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Key"},
			{Label: "Delete", Current: true},
		}
	default:
		return nil
	}
}

func (m *model) keyFooterActions() []shell.FooterAction {
	switch m.session.Current().ID {
	case RouteKeysBrowser:
		if m.keyBrowser.err != "" {
			return []shell.FooterAction{
				{Key: "Enter", Label: "Retry"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if m.keyBrowser.searchActive {
			return []shell.FooterAction{
				{Key: "Enter", Label: "Done"},
				{Key: "Esc", Label: m.session.EscLabel(EscCancel)},
			}
		}
		return []shell.FooterAction{
			{Key: "↑/↓", Label: "Move"},
			{Key: "Enter", Label: "View"},
			{Key: "E", Label: "Edit"},
			{Key: "D", Label: "Delete"},
			{Key: "R", Label: "Refresh"},
			{Key: "/", Label: "Search"},
			{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
		}
	case RouteKeysDetail:
		if m.keyDetail.err != "" {
			return []shell.FooterAction{
				{Key: "Enter", Label: "Retry"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		return []shell.FooterAction{
			{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
		}
	case RouteKeysRename:
		if m.keyRename.loading || m.keyRename.saving {
			return []shell.FooterAction{
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		return []shell.FooterAction{
			{Key: "Enter", Label: "Save"},
			{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
		}
	case RouteKeysDelete:
		if m.keyDelete.loading || m.keyDelete.deleting {
			return []shell.FooterAction{
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		return []shell.FooterAction{
			{Key: "Enter", Label: "Delete"},
			{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
		}
	default:
		return nil
	}
}

func (m *model) renderKeyBody(contentWidth int) string {
	switch m.session.Current().ID {
	case RouteKeysBrowser:
		return keyscreen.RenderBrowser(keyscreen.BrowserScreen{
			SearchView:   m.keyBrowser.input.View(),
			SearchActive: m.keyBrowser.searchActive,
			SearchNotice: m.keyBrowser.notice,
			Rows:         m.keyBrowserVisibleRows(),
			SelectedIndex: func() int {
				if m.keyBrowser.selected < m.keyBrowser.offset {
					return 0
				}
				return m.keyBrowser.selected - m.keyBrowser.offset
			}(),
			Loading: m.keyBrowser.loading,
			Error:   m.keyBrowser.err,
		}, m.spinner.View(), contentWidth)
	case RouteKeysDetail:
		if m.keyDetail.resolving {
			return ""
		}
		return keyscreen.RenderDetail(keyscreen.DetailScreen{
			Loading: m.keyDetail.loading,
			Error:   m.keyDetail.err,
			Key:     m.keyDetail.key,
		}, m.spinner.View(), contentWidth)
	case RouteKeysRename:
		if m.keyRename.resolving {
			return ""
		}
		return keyscreen.RenderRename(keyscreen.RenameScreen{
			Context:  renameContext(m.keyRename.original),
			FieldView: m.keyRename.input.View(),
			Focused: true,
			Status:  renameStatus(m.keyRename.saving),
			Error:   m.keyRename.err,
			Loading: m.keyRename.loading,
		}, m.spinner.View(), contentWidth)
	case RouteKeysDelete:
		if m.keyDelete.resolving {
			return ""
		}
		return keyscreen.RenderDelete(keyscreen.DeleteScreen{
			Context: deleteContext(m.keyDelete.key.Name),
			Key:     m.keyDelete.key,
			Status:  deleteStatus(m.keyDelete.deleting),
			Error:   m.keyDelete.err,
			Loading: m.keyDelete.loading,
		}, m.spinner.View(), contentWidth)
	default:
		return ""
	}
}

func (m *model) updateKeyKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.session.Current().ID {
	case RouteKeysBrowser:
		return m.updateKeyBrowser(msg)
	case RouteKeysDetail:
		switch msg.String() {
		case "esc":
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		case "enter":
			if m.keyDetail.err != "" {
				return m, m.startKeyRouteLoad()
			}
		}
		return m, nil
	case RouteKeysRename:
		return m.updateKeyRename(msg)
	case RouteKeysDelete:
		return m.updateKeyDelete(msg)
	default:
		return m, nil
	}
}

func (m *model) updateKeyBrowser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keyBrowser.loading {
		if msg.String() == "esc" {
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		}
		return m, nil
	}
	if m.keyBrowser.err != "" {
		switch msg.String() {
		case "esc":
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		case "enter":
			return m, m.startKeyRouteLoad()
		}
		return m, nil
	}

	if m.keyBrowser.searchActive {
		switch msg.String() {
		case "esc":
			m.keyBrowser.searchActive = false
			m.keyBrowser.input.Blur()
			m.keyBrowser.input.SetValue("")
			m.refreshKeyBrowserRows()
			return m, nil
		case "enter":
			m.keyBrowser.searchActive = false
			m.keyBrowser.input.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.keyBrowser.input, cmd = m.keyBrowser.input.Update(msg)
		m.refreshKeyBrowserRows()
		return m, cmd
	}

	switch msg.String() {
	case "esc":
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "/":
		m.keyBrowser.searchActive = true
		m.keyBrowser.input.Focus()
		return m, textinput.Blink
	case "r":
		if m.keyBrowser.loading || m.keyBrowser.refreshing {
			return m, nil
		}
		return m, m.refreshKeyBrowser(true)
	case "up", "k":
		m.moveKeyBrowserSelection(-1)
		return m, nil
	case "down", "j":
		m.moveKeyBrowserSelection(1)
		return m, nil
	case "enter":
		if key, ok := m.selectedKeyRow(); ok {
			m.session.Push(Route{ID: RouteKeysDetail, Params: map[string]string{"name": key.Name, "source": "browser"}})
			return m, m.showCurrentRoute()
		}
	case "e":
		if key, ok := m.selectedKeyRow(); ok {
			m.session.Push(Route{ID: RouteKeysRename, Params: map[string]string{"old_name": key.Name, "source": "browser"}})
			return m, m.showCurrentRoute()
		}
	case "d":
		if key, ok := m.selectedKeyRow(); ok {
			m.session.Push(Route{ID: RouteKeysDelete, Params: map[string]string{"name": key.Name, "source": "browser"}})
			return m, m.showCurrentRoute()
		}
	}
	return m, nil
}

func (m *model) updateKeyRename(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keyRename.loading || m.keyRename.saving {
		if msg.String() == "esc" {
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "enter":
		newName := strings.TrimSpace(m.keyRename.input.Value())
		if newName == "" {
			m.keyRename.err = "Enter a new key name"
			return m, nil
		}
		if newName == m.keyRename.original {
			m.keyRename.err = "Enter a different key name"
			return m, nil
		}
		m.keyRename.err = ""
		m.keyRename.saving = true
		return m, tea.Batch(m.spinner.Tick, m.renameKey(m.keyRename.original, newName))
	}

	var cmd tea.Cmd
	m.keyRename.input, cmd = m.keyRename.input.Update(msg)
	m.keyRename.err = ""
	return m, cmd
}

func (m *model) updateKeyDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keyDelete.loading || m.keyDelete.deleting {
		if msg.String() == "esc" {
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "enter":
		m.keyDelete.err = ""
		m.keyDelete.deleting = true
		return m, tea.Batch(m.spinner.Tick, m.deleteKey(m.keyDelete.key.Name))
	}
	return m, nil
}

func (m *model) startKeyRouteLoad() tea.Cmd {
	route := m.session.Current()
	switch route.ID {
	case RouteKeysBrowser:
		if len(m.keyBrowser.all) > 0 {
			m.applyKeyBrowserRoute(route)
			m.keyBrowser.loading = false
			m.keyBrowser.refreshing = true
			m.keyBrowser.err = ""
			return m.listKeys(m.nextKeyListID(), true)
		}
		m.keyBrowser = keyBrowserState{
			loading: true,
			input:   newKeyInput("Search keys"),
		}
		if query := strings.TrimSpace(route.Params["query"]); query != "" {
			m.keyBrowser.input.SetValue(query)
		}
		if route.Params["search"] == "true" {
			m.keyBrowser.searchActive = true
			m.keyBrowser.input.Focus()
		}
	case RouteKeysDetail:
		if route.Params["source"] == "browser" {
			name := strings.TrimSpace(route.Params["name"])
			if name != "" {
				m.keyDetail = keyDetailState{loading: true}
				return tea.Batch(m.spinner.Tick, m.loadKeyDetail(name))
			}
		}
		m.keyDetail = keyDetailState{loading: true, resolving: true}
	case RouteKeysRename:
		if route.Params["source"] == "browser" {
			name := strings.TrimSpace(route.Params["old_name"])
			if key, ok := m.cachedKeyRow(name); ok {
				m.keyRename = keyRenameState{
					original: key.Name,
					input:    newKeyInput("Enter new key name"),
				}
				m.keyRename.input.SetValue(key.Name)
				m.keyRename.input.Focus()
				m.resizeKeyInputs()
				return textinput.Blink
			}
		}
		m.keyRename = keyRenameState{loading: true, resolving: true}
	case RouteKeysDelete:
		if route.Params["source"] == "browser" {
			name := strings.TrimSpace(route.Params["name"])
			if key, ok := m.cachedKeyRow(name); ok {
				m.keyDelete = keyDeleteState{key: key}
				return nil
			}
		}
		m.keyDelete = keyDeleteState{loading: true, resolving: true}
	default:
		return nil
	}

	m.resizeKeyInputs()
	return tea.Batch(m.spinner.Tick, m.listKeys(m.nextKeyListID(), false))
}

func (m *model) listKeys(id int, preserve bool) tea.Cmd {
	paths := config.DefaultPaths()
	return func() tea.Msg {
		keys, err := actions.ListKeys(paths)
		return keyListMsg{id: id, keys: keys, err: err, preserve: preserve}
	}
}

func (m *model) syncAndListKeys(id int) tea.Cmd {
	paths := config.DefaultPaths()
	return func() tea.Msg {
		if m.snapshot.LoggedIn {
			if err := actions.TriggerSync(paths); err != nil {
				return keyListMsg{id: id, err: err, preserve: true}
			}
		}
		keys, err := actions.ListKeys(paths)
		return keyListMsg{id: id, keys: keys, err: err, preserve: true}
	}
}

func (m *model) loadKeyDetail(name string) tea.Cmd {
	m.keyDetail = keyDetailState{loading: true}
	m.keyDetailID++
	id := m.keyDetailID
	paths := config.DefaultPaths()
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		detail, err := actions.ViewKey(paths, name)
		return keyDetailMsg{id: id, detail: detail, err: err}
	})
}

func (m *model) renameKey(oldName, newName string) tea.Cmd {
	m.keyRenameID++
	id := m.keyRenameID
	paths := config.DefaultPaths()
	return func() tea.Msg {
		result, err := actions.RenameKey(paths, oldName, newName)
		return keyRenameFinishedMsg{id: id, result: result, err: err}
	}
}

func (m *model) deleteKey(name string) tea.Cmd {
	m.keyDeleteID++
	id := m.keyDeleteID
	paths := config.DefaultPaths()
	return func() tea.Msg {
		resolvedName, err := actions.DeleteKey(paths, name)
		return keyDeleteFinishedMsg{id: id, name: resolvedName, err: err}
	}
}

func (m *model) handleKeyListMsg(msg keyListMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyListID {
		return m, nil
	}

	if msg.err != nil {
		switch m.session.Current().ID {
		case RouteKeysBrowser:
			m.keyBrowser.loading = false
			m.keyBrowser.refreshing = false
			if msg.preserve && len(m.keyBrowser.all) > 0 {
				m.keyBrowser.notice = msg.err.Error()
				return m, nil
			}
			m.keyBrowser.err = msg.err.Error()
		case RouteKeysDetail:
			m.keyDetail.loading = false
			m.keyDetail.resolving = false
			m.keyDetail.err = msg.err.Error()
		case RouteKeysRename:
			m.keyRename.loading = false
			m.keyRename.resolving = false
			m.keyRename.err = msg.err.Error()
		case RouteKeysDelete:
			m.keyDelete.loading = false
			m.keyDelete.resolving = false
			m.keyDelete.err = msg.err.Error()
		}
		return m, nil
	}

	current := m.session.Current()
	switch current.ID {
	case RouteKeysBrowser:
		preserveName := ""
		if key, ok := m.selectedKeyRow(); ok {
			preserveName = key.Name
		}
		m.keyBrowser.loading = false
		m.keyBrowser.refreshing = false
		m.keyBrowser.err = ""
		if len(m.keyBrowser.all) == 0 {
			searchActive := current.Params["search"] == "true"
			m.prepareKeyBrowser(msg.keys, current.Params["query"], current.Params["notice"], searchActive)
		} else {
			query := m.keyBrowser.input.Value()
			notice := m.keyBrowser.notice
			searchActive := m.keyBrowser.searchActive
			m.prepareKeyBrowser(msg.keys, query, notice, searchActive)
			m.selectKeyBrowserByName(preserveName)
		}
		if m.keyBrowser.searchActive {
			return m, textinput.Blink
		}
		return m, nil
	case RouteKeysDetail:
		query := current.Params["name"]
		resolution := actions.ResolveKeyQuery(msg.keys, query)
		if resolution.Exact != nil {
			m.keyDetail.resolving = false
			return m, m.loadKeyDetail(resolution.Exact.Name)
		}
		return m.fallbackKeyBrowser(msg.keys, query, fallbackNotice(RouteKeysDetail, query, len(resolution.Matches)))
	case RouteKeysRename:
		query := current.Params["old_name"]
		resolution := actions.ResolveKeyQuery(msg.keys, query)
		if resolution.Exact != nil {
			m.keyRename = keyRenameState{
				resolving: false,
				original: resolution.Exact.Name,
				input:    newKeyInput("Enter new key name"),
			}
			m.keyRename.input.SetValue(strings.TrimSpace(current.Params["new_name"]))
			if strings.TrimSpace(current.Params["new_name"]) == "" {
				m.keyRename.input.SetValue(resolution.Exact.Name)
			}
			m.keyRename.input.Focus()
			m.resizeKeyInputs()
			return m, textinput.Blink
		}
		return m.fallbackKeyBrowser(msg.keys, query, fallbackNotice(RouteKeysRename, query, len(resolution.Matches)))
	case RouteKeysDelete:
		query := current.Params["name"]
		resolution := actions.ResolveKeyQuery(msg.keys, query)
		if resolution.Exact != nil {
			m.keyDelete = keyDeleteState{key: *resolution.Exact}
			return m, nil
		}
		return m.fallbackKeyBrowser(msg.keys, query, fallbackNotice(RouteKeysDelete, query, len(resolution.Matches)))
	default:
		return m, nil
	}
}

func (m *model) handleKeyDetailMsg(msg keyDetailMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyDetailID {
		return m, nil
	}
	m.keyDetail.loading = false
	if msg.err != nil {
		m.keyDetail.err = msg.err.Error()
		return m, nil
	}
	m.keyDetail.key = msg.detail
	m.keyDetail.err = ""
	return m, nil
}

func (m *model) handleKeyRenameFinishedMsg(msg keyRenameFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyRenameID {
		return m, nil
	}
	m.keyRename.saving = false
	if msg.err != nil {
		m.keyRename.err = msg.err.Error()
		return m, nil
	}
	m.renameCachedKey(msg.result.OldName, msg.result.NewName)

	m.session.ReplaceCurrent(Route{
		ID: RouteKeysBrowser,
		Params: map[string]string{
			"query":  msg.result.NewName,
			"notice": fmt.Sprintf("Renamed %s to %s", msg.result.OldName, msg.result.NewName),
		},
	})
	return m, m.showCurrentRoute()
}

func (m *model) handleKeyDeleteFinishedMsg(msg keyDeleteFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyDeleteID {
		return m, nil
	}
	m.keyDelete.deleting = false
	if msg.err != nil {
		m.keyDelete.err = msg.err.Error()
		return m, nil
	}
	m.removeCachedKey(msg.name)

	m.session.ReplaceCurrent(Route{
		ID: RouteKeysBrowser,
		Params: map[string]string{
			"notice": fmt.Sprintf("Deleted %s", msg.name),
		},
	})
	return m, m.showCurrentRoute()
}

func (m *model) fallbackKeyBrowser(keys []actions.KeySummary, query string, notice string) (tea.Model, tea.Cmd) {
	searchActive := len(actions.ResolveKeyQuery(keys, query).Matches) == 0
	m.session.ReplaceCurrent(Route{
		ID: RouteKeysBrowser,
		Params: map[string]string{
			"query":  query,
			"notice": notice,
			"search": fmt.Sprintf("%t", searchActive),
		},
	})
	m.prepareKeyBrowser(keys, query, notice, searchActive)
	if m.keyBrowser.searchActive {
		return m, textinput.Blink
	}
	return m, nil
}

func (m *model) prepareKeyBrowser(keys []actions.KeySummary, query string, notice string, searchActive bool) {
	m.keyBrowser.loading = false
	m.keyBrowser.refreshing = false
	m.keyBrowser.err = ""
	m.keyBrowser.notice = strings.TrimSpace(notice)
	m.keyBrowser.all = keys
	m.keyBrowser.input = newKeyInput("Search keys")
	m.keyBrowser.input.SetValue(query)
	m.keyBrowser.searchActive = searchActive
	if searchActive {
		m.keyBrowser.input.Focus()
	} else {
		m.keyBrowser.input.Blur()
	}
	m.resizeKeyInputs()
	m.refreshKeyBrowserRows()
}

func (m *model) refreshKeyBrowserRows() {
	query := strings.TrimSpace(m.keyBrowser.input.Value())
	resolution := actions.ResolveKeyQuery(m.keyBrowser.all, query)
	m.keyBrowser.rows = resolution.Matches
	if query == "" {
		m.keyBrowser.rows = actions.ResolveKeyQuery(m.keyBrowser.all, "").Matches
	}

	if len(m.keyBrowser.rows) == 0 {
		m.keyBrowser.selected = 0
		m.keyBrowser.offset = 0
		return
	}
	if m.keyBrowser.selected >= len(m.keyBrowser.rows) {
		m.keyBrowser.selected = len(m.keyBrowser.rows) - 1
	}
	if m.keyBrowser.selected < 0 {
		m.keyBrowser.selected = 0
	}
	m.ensureKeyBrowserVisible()
}

func (m *model) moveKeyBrowserSelection(delta int) {
	if len(m.keyBrowser.rows) == 0 {
		return
	}
	next := m.keyBrowser.selected + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.keyBrowser.rows) {
		next = len(m.keyBrowser.rows) - 1
	}
	m.keyBrowser.selected = next
	m.ensureKeyBrowserVisible()
}

func (m *model) nextKeyListID() int {
	m.keyListID++
	return m.keyListID
}

func (m *model) refreshKeyBrowser(sync bool) tea.Cmd {
	m.keyBrowser.refreshing = true
	m.keyBrowser.err = ""
	id := m.nextKeyListID()
	if sync {
		return tea.Batch(m.spinner.Tick, m.syncAndListKeys(id))
	}
	return tea.Batch(m.spinner.Tick, m.listKeys(id, true))
}

func (m *model) applyKeyBrowserRoute(route Route) {
	query := route.Params["query"]
	notice := route.Params["notice"]
	searchActive := route.Params["search"] == "true"
	if query == "" && notice == "" && !searchActive {
		m.resizeKeyInputs()
		return
	}
	m.prepareKeyBrowser(m.keyBrowser.all, query, notice, searchActive)
}

func (m *model) cachedKeyRow(name string) (actions.KeySummary, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return actions.KeySummary{}, false
	}
	for _, key := range m.keyBrowser.all {
		if key.Name == name {
			return key, true
		}
	}
	return actions.KeySummary{}, false
}

func (m *model) selectKeyBrowserByName(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	for index, key := range m.keyBrowser.rows {
		if key.Name == name {
			m.keyBrowser.selected = index
			m.ensureKeyBrowserVisible()
			return
		}
	}
}

func (m *model) renameCachedKey(oldName, newName string) {
	for index := range m.keyBrowser.all {
		if m.keyBrowser.all[index].Name == oldName {
			m.keyBrowser.all[index].Name = newName
			break
		}
	}
	m.refreshKeyBrowserRows()
	m.selectKeyBrowserByName(newName)
}

func (m *model) removeCachedKey(name string) {
	m.keyBrowser.all = slices.DeleteFunc(m.keyBrowser.all, func(key actions.KeySummary) bool {
		return key.Name == name
	})
	m.refreshKeyBrowserRows()
}

func (m *model) ensureKeyBrowserVisible() {
	if m.keyBrowser.selected < m.keyBrowser.offset {
		m.keyBrowser.offset = m.keyBrowser.selected
	}
	if m.keyBrowser.selected >= m.keyBrowser.offset+keyscreen.VisibleRows() {
		m.keyBrowser.offset = m.keyBrowser.selected - keyscreen.VisibleRows() + 1
	}
	if m.keyBrowser.offset < 0 {
		m.keyBrowser.offset = 0
	}
}

func (m *model) keyBrowserVisibleRows() []actions.KeySummary {
	if len(m.keyBrowser.rows) == 0 {
		return nil
	}
	start := min(max(m.keyBrowser.offset, 0), len(m.keyBrowser.rows))
	end := min(len(m.keyBrowser.rows), start+keyscreen.VisibleRows())
	return m.keyBrowser.rows[start:end]
}

func (m *model) selectedKeyRow() (actions.KeySummary, bool) {
	if len(m.keyBrowser.rows) == 0 || m.keyBrowser.selected < 0 || m.keyBrowser.selected >= len(m.keyBrowser.rows) {
		return actions.KeySummary{}, false
	}
	return m.keyBrowser.rows[m.keyBrowser.selected], true
}

func (m *model) resizeKeyInputs() {
	searchWidth := max(18, shell.BodyWidth(m.width)-3)
	m.keyBrowser.input.Width = searchWidth
	m.keyRename.input.Width = max(18, min(shell.ClampBlockWidth(m.width, 44), 44))
}

func newKeyInput(placeholder string) textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 128
	input.Width = 32
	input.Cursor.Style = theme.FooterKey
	input.TextStyle = theme.FieldValue
	input.PlaceholderStyle = theme.BodyMuted
	input.Placeholder = placeholder
	return input
}

func fallbackNotice(route RouteID, query string, matches int) string {
	if strings.TrimSpace(query) == "" {
		return ""
	}
	switch route {
	case RouteKeysRename:
		if matches == 0 {
			return fmt.Sprintf("No exact match for %q. Refine your search, then press E to rename", query)
		}
		return fmt.Sprintf("No exact match for %q. Select a key, then press E to rename it", query)
	case RouteKeysDelete:
		if matches == 0 {
			return fmt.Sprintf("No exact match for %q. Refine your search, then press D to delete", query)
		}
		return fmt.Sprintf("No exact match for %q. Select a key, then press D to delete it", query)
	default:
		if matches == 0 {
			return fmt.Sprintf("No exact match for %q. Refine your search to find a key", query)
		}
		return fmt.Sprintf("No exact match for %q. Showing closest results", query)
	}
}

func renameContext(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Choose a new name for this key"
	}
	return fmt.Sprintf("Choose a new name for %s", name)
}

func renameStatus(saving bool) string {
	if saving {
		return "Saving key name"
	}
	return ""
}

func deleteContext(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Delete this key from the vault"
	}
	return fmt.Sprintf("Delete %s from this vault", name)
}

func deleteStatus(deleting bool) string {
	if deleting {
		return "Deleting key"
	}
	return ""
}
