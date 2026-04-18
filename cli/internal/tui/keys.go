package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/picker"
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

type keyCopyFinishedMsg struct {
	status string
	err    error
}

type keyPrivateCopyFinishedMsg struct {
	status string
	err    error
}

type keyGenerateFinishedMsg struct {
	id     int
	result actions.GenerateResult
	err    error
}

type keyImportFinishedMsg struct {
	id     int
	result actions.ImportResult
	err    error
}

type keyExportFinishedMsg struct {
	id     int
	result actions.ExportResult
	err    error
}

type keyImportPickerMsg struct {
	id   int
	path string
	err  error
}

type keyExportPickerMsg struct {
	id   int
	path string
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
	busy      bool
	status    string
	statusErr string
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

type keyGenerateState struct {
	generating bool
	err        string
	status     string

	nameInput textinput.Model
}

type keyImportState struct {
	importing     bool
	pickerOpening bool
	err           string
	status        string

	sourceIndex int
	focus       int
	pathVisible bool
	pathInput   textinput.Model
}

type keyExportState struct {
	exporting     bool
	pickerOpening bool
	err           string
	status        string

	pathVisible bool
	pathInput   textinput.Model
}

type keyImportSource struct {
	ID          string
	Label       string
	NeedsPath   bool
	Placeholder string
}

var keyImportSources = []keyImportSource{
	{ID: "1password", Label: "1Password export", NeedsPath: true, Placeholder: "Path to 1Password export"},
	{ID: "bitwarden", Label: "Bitwarden export", NeedsPath: true, Placeholder: "Path to Bitwarden export"},
	{ID: "forged", Label: "Forged export", NeedsPath: true, Placeholder: "Path to Forged export"},
	{ID: "ssh-dir", Label: "SSH directory", NeedsPath: false, Placeholder: ""},
	{ID: "file", Label: "Key file", NeedsPath: true, Placeholder: "Path to private key file"},
}

func (m *model) isKeyRoute() bool {
	if m.screen != screenDashboard || !m.snapshot.VaultExists {
		return false
	}

	switch m.session.Current().ID {
	case RouteKeysBrowser, RouteKeysDetail, RouteKeysRename, RouteKeysDelete, RouteKeysGenerate, RouteKeysImport, RouteKeysExport:
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
		return m.keyDetail.loading || m.keyDetail.busy
	case RouteKeysRename:
		return m.keyRename.loading || m.keyRename.saving
	case RouteKeysDelete:
		return m.keyDelete.loading || m.keyDelete.deleting
	case RouteKeysGenerate:
		return m.keyGenerate.generating
	case RouteKeysImport:
		return m.keyImport.importing || m.keyImport.pickerOpening
	case RouteKeysExport:
		return m.keyExport.exporting || m.keyExport.pickerOpening
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
	case RouteKeysGenerate:
		return strings.TrimSpace(m.keyGenerate.nameInput.Placeholder) != "" || m.keyGenerate.generating || strings.TrimSpace(m.keyGenerate.err) != "" || strings.TrimSpace(m.keyGenerate.status) != ""
	case RouteKeysImport:
		return len(keyImportSources) > 0 && (strings.TrimSpace(m.keyImport.err) != "" || strings.TrimSpace(m.keyImport.status) != "" || m.keyImport.importing || m.keyImport.pickerOpening || m.keyImport.pathVisible || strings.TrimSpace(m.keyImport.pathInput.Placeholder) != "")
	case RouteKeysExport:
		return strings.TrimSpace(m.keyExport.pathInput.Placeholder) != "" || strings.TrimSpace(m.keyExport.err) != "" || strings.TrimSpace(m.keyExport.status) != "" || m.keyExport.exporting || m.keyExport.pickerOpening || m.keyExport.pathVisible
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
			return "View key"
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
	case RouteKeysGenerate:
		return "Generate key"
	case RouteKeysImport:
		return "Import keys"
	case RouteKeysExport:
		return "Export vault"
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
	case RouteKeysGenerate:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Key"},
			{Label: "Generate", Current: true},
		}
	case RouteKeysImport:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Key"},
			{Label: "Import", Current: true},
		}
	case RouteKeysExport:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Key"},
			{Label: "Export", Current: true},
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
			{Key: "G", Label: "Generate"},
			{Key: "I", Label: "Import"},
			{Key: "X", Label: "Export"},
			{Key: "R", Label: "Refresh"},
			{Key: "/", Label: "Search"},
			{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
		}
	case RouteKeysDetail:
		if m.keyDetail.loading {
			return []shell.FooterAction{
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if m.keyDetail.err != "" {
			return []shell.FooterAction{
				{Key: "Enter", Label: "Retry"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		return []shell.FooterAction{
			{Key: "C", Label: "Copy Public"},
			{Key: "K", Label: "Copy Private"},
			{Key: "F", Label: "Copy Fingerprint"},
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
	case RouteKeysGenerate:
		if m.keyGenerate.generating {
			return []shell.FooterAction{{Key: "Esc", Label: m.session.EscLabel(EscAuto)}}
		}
		return []shell.FooterAction{
			{Key: "Enter", Label: "Generate"},
			{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
		}
	case RouteKeysImport:
		if m.keyImport.importing || m.keyImport.pickerOpening {
			return []shell.FooterAction{{Key: "Esc", Label: m.session.EscLabel(EscAuto)}}
		}
		source := m.currentImportSource()
		if m.keyImport.pathVisible && m.keyImport.focus == 1 {
			return []shell.FooterAction{
				{Key: "↑/↓", Label: "Source"},
				{Key: "Enter", Label: "Import"},
				{Key: "Tab", Label: "Choose file"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		enterLabel := "Import"
		if source.NeedsPath {
			enterLabel = "Choose File"
		}
		actions := []shell.FooterAction{
			{Key: "↑/↓", Label: "Source"},
			{Key: "Enter", Label: enterLabel},
		}
		if m.keyImport.pathVisible {
			actions = append(actions, shell.FooterAction{Key: "Tab", Label: "Path"})
		}
		actions = append(actions, shell.FooterAction{Key: "Esc", Label: m.session.EscLabel(EscAuto)})
		return actions
	case RouteKeysExport:
		if m.keyExport.exporting || m.keyExport.pickerOpening {
			return []shell.FooterAction{{Key: "Esc", Label: m.session.EscLabel(EscAuto)}}
		}
		if !m.keyExport.pathVisible {
			return []shell.FooterAction{
				{Key: "Enter", Label: "Choose file"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		return []shell.FooterAction{
			{Key: "Enter", Label: "Export"},
			{Key: "Tab", Label: "Choose file"},
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
		return keyscreen.RenderDetail(keyscreen.DetailScreen{
			Loading:     m.keyDetail.loading || m.keyDetail.resolving,
			Error:       m.keyDetail.err,
			Key:         m.keyDetail.key,
			Status:      m.keyDetail.status,
			StatusError: m.keyDetail.statusErr,
			Busy:        m.keyDetail.busy,
		}, m.spinner.View(), contentWidth)
	case RouteKeysRename:
		if m.keyRename.resolving {
			return ""
		}
		return keyscreen.RenderRename(keyscreen.RenameScreen{
			Context:   renameContext(m.keyRename.original),
			FieldView: m.keyRename.input.View(),
			Focused:   true,
			Status:    renameStatus(m.keyRename.saving),
			Error:     m.keyRename.err,
			Loading:   m.keyRename.loading,
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
	case RouteKeysGenerate:
		return keyscreen.RenderGenerate(keyscreen.GenerateScreen{
			Context:    "Create a new SSH key and add it to this vault",
			NameView:   m.keyGenerate.nameInput.View(),
			Focused:    true,
			Status:     m.keyGenerate.status,
			Error:      m.keyGenerate.err,
			Generating: m.keyGenerate.generating,
		}, m.spinner.View(), contentWidth)
	case RouteKeysImport:
		options := make([]keyscreen.ImportSourceOption, 0, len(keyImportSources))
		for index, source := range keyImportSources {
			options = append(options, keyscreen.ImportSourceOption{
				Label:    source.Label,
				Selected: index == m.keyImport.sourceIndex,
			})
		}
		return keyscreen.RenderImport(keyscreen.ImportScreen{
			Context:     "Import keys from another source into this vault",
			Sources:     options,
			SourceFocus: m.keyImport.focus == 0,
			PathView:    m.keyImport.pathInput.View(),
			PathFocused: m.keyImport.focus == 1,
			PathVisible: m.keyImport.pathVisible,
			Status:      m.keyImport.status,
			Error:       m.keyImport.err,
			Busy:        m.keyImport.importing || m.keyImport.pickerOpening,
		}, m.spinner.View(), contentWidth)
	case RouteKeysExport:
		return keyscreen.RenderExport(keyscreen.ExportScreen{
			Context:     "Export this vault to a Forged JSON file",
			PathView:    m.keyExport.pathInput.View(),
			Focused:     m.keyExport.pathVisible,
			PathVisible: m.keyExport.pathVisible,
			Status:      m.keyExport.status,
			Error:       m.keyExport.err,
			Busy:        m.keyExport.exporting || m.keyExport.pickerOpening,
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
		return m.updateKeyDetail(msg)
	case RouteKeysRename:
		return m.updateKeyRename(msg)
	case RouteKeysDelete:
		return m.updateKeyDelete(msg)
	case RouteKeysGenerate:
		return m.updateKeyGenerate(msg)
	case RouteKeysImport:
		return m.updateKeyImport(msg)
	case RouteKeysExport:
		return m.updateKeyExport(msg)
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
	case "g":
		m.session.Push(Route{ID: RouteKeysGenerate})
		return m, m.showCurrentRoute()
	case "i":
		m.session.Push(Route{ID: RouteKeysImport})
		return m, m.showCurrentRoute()
	case "x":
		m.session.Push(Route{ID: RouteKeysExport})
		return m, m.showCurrentRoute()
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

func (m *model) updateKeyDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keyDetail.loading || m.keyDetail.busy {
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
		if m.keyDetail.err != "" {
			return m, m.startKeyRouteLoad()
		}
	case "c":
		if strings.TrimSpace(m.keyDetail.key.PublicKey) == "" {
			return m, nil
		}
		m.keyDetail.status = ""
		m.keyDetail.statusErr = ""
		m.keyDetail.busy = true
		return m, tea.Batch(m.spinner.Tick, m.copyKeyText(m.keyDetail.key.PublicKey, "Public key copied"))
	case "f":
		if strings.TrimSpace(m.keyDetail.key.Fingerprint) == "" {
			return m, nil
		}
		m.keyDetail.status = ""
		m.keyDetail.statusErr = ""
		m.keyDetail.busy = true
		return m, tea.Batch(m.spinner.Tick, m.copyKeyText(m.keyDetail.key.Fingerprint, "Fingerprint copied"))
	case "k":
		name := strings.TrimSpace(m.keyDetail.key.Name)
		if name == "" {
			return m, nil
		}
		m.keyDetail.status = ""
		m.keyDetail.statusErr = ""
		m.keyDetail.busy = true
		return m, tea.Batch(m.spinner.Tick, m.copyPrivateKey(nil))
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

func (m *model) updateKeyGenerate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keyGenerate.generating {
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
		name := strings.TrimSpace(m.keyGenerate.nameInput.Value())
		if name == "" {
			m.keyGenerate.err = "Enter a key name"
			return m, nil
		}
		m.keyGenerate.err = ""
		m.keyGenerate.status = "Generating key"
		m.keyGenerate.generating = true
		return m, tea.Batch(m.spinner.Tick, m.generateKeyCmd(name, ""))
	default:
		var cmd tea.Cmd
		m.keyGenerate.nameInput, cmd = m.keyGenerate.nameInput.Update(msg)
		m.keyGenerate.err = ""
		return m, cmd
	}
}

func (m *model) updateKeyImport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keyImport.importing || m.keyImport.pickerOpening {
		if msg.String() == "esc" {
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		}
		return m, nil
	}

	source := m.currentImportSource()
	switch msg.String() {
	case "esc":
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "tab":
		if m.keyImport.pathVisible && m.keyImport.focus == 1 {
			m.keyImport.err = ""
			m.keyImport.status = "Choosing import file"
			m.keyImport.pickerOpening = true
			return m, tea.Batch(m.spinner.Tick, m.openImportPicker(source.ID))
		}
		if m.keyImport.pathVisible {
			m.keyImport.focus = 1
			m.keyImport.pathInput.Focus()
			return m, textinput.Blink
		}
		return m, nil
	case "up", "k":
		if m.keyImport.focus == 0 {
			m.moveKeyImportSource(-1)
			return m, nil
		}
		m.keyImport.focus = 0
		m.keyImport.pathInput.Blur()
		return m, nil
	case "down", "j":
		if m.keyImport.focus == 0 {
			if m.keyImport.pathVisible {
				m.keyImport.focus = 1
				m.keyImport.pathInput.Focus()
				return m, textinput.Blink
			}
			m.moveKeyImportSource(1)
			return m, nil
		}
		m.keyImport.focus = 0
		m.keyImport.pathInput.Blur()
		return m, nil
	case "enter":
		if m.keyImport.focus == 0 {
			if source.NeedsPath {
				m.keyImport.err = ""
				m.keyImport.status = "Choosing import file"
				m.keyImport.pickerOpening = true
				return m, tea.Batch(m.spinner.Tick, m.openImportPicker(source.ID))
			}
			m.keyImport.err = ""
			m.keyImport.status = "Importing keys"
			m.keyImport.importing = true
			return m, tea.Batch(m.spinner.Tick, m.importKeysCmd(source.ID, ""))
		}
		file := ""
		if source.NeedsPath {
			file = strings.TrimSpace(m.keyImport.pathInput.Value())
			if file == "" {
				m.keyImport.err = "Enter a file path"
				return m, nil
			}
		}
		m.keyImport.err = ""
		m.keyImport.status = "Importing keys"
		m.keyImport.importing = true
		return m, tea.Batch(m.spinner.Tick, m.importKeysCmd(source.ID, file))
	default:
		if m.keyImport.focus == 1 {
			return m, m.updateKeyImportPath(msg)
		}
	}
	return m, nil
}

func (m *model) updateKeyExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keyExport.exporting || m.keyExport.pickerOpening {
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
	case "tab":
		if m.keyExport.pathVisible {
			m.keyExport.err = ""
			m.keyExport.status = "Choosing export file"
			m.keyExport.pickerOpening = true
			return m, tea.Batch(m.spinner.Tick, m.openExportPicker(strings.TrimSpace(m.keyExport.pathInput.Value())))
		}
		return m, nil
	case "enter":
		if !m.keyExport.pathVisible {
			m.keyExport.err = ""
			m.keyExport.status = "Choosing export file"
			m.keyExport.pickerOpening = true
			return m, tea.Batch(m.spinner.Tick, m.openExportPicker(strings.TrimSpace(m.keyExport.pathInput.Value())))
		}
		if strings.TrimSpace(m.keyExport.pathInput.Value()) == "" {
			m.keyExport.err = "Enter an export path"
			return m, nil
		}
		m.keyExport.err = ""
		m.keyExport.status = "Exporting vault"
		m.keyExport.exporting = true
		return m, tea.Batch(m.spinner.Tick, m.exportVault(nil))
	default:
		var cmd tea.Cmd
		m.keyExport.pathInput, cmd = m.keyExport.pathInput.Update(msg)
		m.keyExport.err = ""
		return m, cmd
	}
}

func (m *model) updateKeyGenerateInputs(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	m.keyGenerate.nameInput, cmd = m.keyGenerate.nameInput.Update(msg)
	m.keyGenerate.err = ""
	return cmd
}

func (m *model) updateKeyImportPath(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	m.keyImport.pathInput, cmd = m.keyImport.pathInput.Update(msg)
	m.keyImport.err = ""
	return cmd
}

func (m *model) startKeyRouteLoad() tea.Cmd {
	route := m.session.Current()
	switch route.ID {
	case RouteKeysBrowser:
		if m.keyBrowser.loading {
			if query := strings.TrimSpace(route.Params["query"]); query != "" {
				m.keyBrowser.input.SetValue(query)
			}
			m.keyBrowser.notice = strings.TrimSpace(route.Params["notice"])
			m.keyBrowser.searchActive = route.Params["search"] == "true"
			if m.keyBrowser.searchActive {
				m.keyBrowser.input.Focus()
				return textinput.Blink
			}
			m.keyBrowser.input.Blur()
			return nil
		}
		if len(m.keyBrowser.all) > 0 {
			m.applyKeyBrowserRoute(route)
			m.keyBrowser.loading = false
			m.keyBrowser.refreshing = m.snapshot.LoggedIn
			m.keyBrowser.err = ""
			if m.snapshot.LoggedIn {
				return m.refreshKeyBrowser(true)
			}
			return nil
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
			if key, ok := m.cachedKeyRow(name); ok {
				m.keyDetail = keyDetailState{
					key: detailFromSummary(key),
				}
				return m.refreshKeyDetail(name)
			}
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
	case RouteKeysGenerate:
		m.keyGenerate = keyGenerateState{
			nameInput: newKeyInput("Enter key name"),
		}
		if name := strings.TrimSpace(route.Params["name"]); name != "" {
			m.keyGenerate.nameInput.SetValue(name)
		}
		m.keyGenerate.nameInput.Focus()
		m.resizeKeyInputs()
		return textinput.Blink
	case RouteKeysImport:
		m.keyImport = keyImportState{
			sourceIndex: 0,
			focus:       0,
			pathVisible: false,
			pathInput:   newKeyInput("Enter import file path"),
		}
		m.keyImport.pathInput.CharLimit = 512
		m.keyImport.pathInput.Placeholder = m.currentImportSource().Placeholder
		m.resizeKeyInputs()
		return nil
	case RouteKeysExport:
		m.keyExport = keyExportState{
			pathInput: newKeyInput("Export path"),
		}
		m.keyExport.pathInput.CharLimit = 512
		m.keyExport.pathInput.SetValue(actions.DefaultExportPath())
		m.keyExport.pathVisible = false
		m.resizeKeyInputs()
		return nil
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

func (m *model) listLocalKeys(id int, preserve bool) tea.Cmd {
	paths := config.DefaultPaths()
	return func() tea.Msg {
		keys, err := actions.ListLocalKeys(paths)
		if err != nil {
			keys, err = actions.ListKeys(paths)
		}
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

func (m *model) refreshKeyDetail(name string) tea.Cmd {
	m.keyDetailID++
	id := m.keyDetailID
	paths := config.DefaultPaths()
	return func() tea.Msg {
		detail, err := actions.ViewKey(paths, name)
		return keyDetailMsg{id: id, detail: detail, err: err}
	}
}

func (m *model) copyKeyText(value string, status string) tea.Cmd {
	copyText := m.copyText
	return func() tea.Msg {
		if strings.TrimSpace(value) == "" {
			return keyCopyFinishedMsg{err: fmt.Errorf("nothing to copy")}
		}
		if err := copyText(value); err != nil {
			return keyCopyFinishedMsg{err: err}
		}
		return keyCopyFinishedMsg{status: status}
	}
}

func (m *model) copyPrivateKey(password []byte) tea.Cmd {
	copyText := m.copyText
	name := strings.TrimSpace(m.keyDetail.key.Name)
	paths := config.DefaultPaths()
	return func() tea.Msg {
		detail, err := actions.ViewFullKey(paths, name, password)
		if err != nil {
			return keyPrivateCopyFinishedMsg{err: err}
		}
		if strings.TrimSpace(detail.PrivateKey) == "" {
			return keyPrivateCopyFinishedMsg{err: fmt.Errorf("private key is unavailable")}
		}
		if err := copyText(detail.PrivateKey); err != nil {
			return keyPrivateCopyFinishedMsg{err: err}
		}
		return keyPrivateCopyFinishedMsg{status: "Private key copied"}
	}
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

func (m *model) generateKeyCmd(name, comment string) tea.Cmd {
	m.keyGenerateID++
	id := m.keyGenerateID
	paths := config.DefaultPaths()
	return func() tea.Msg {
		result, err := actions.GenerateKey(paths, name, comment)
		return keyGenerateFinishedMsg{id: id, result: result, err: err}
	}
}

func (m *model) importKeysCmd(source, file string) tea.Cmd {
	m.keyImportID++
	id := m.keyImportID
	paths := config.DefaultPaths()
	return func() tea.Msg {
		result, err := actions.ImportFromSource(paths, source, file)
		return keyImportFinishedMsg{id: id, result: result, err: err}
	}
}

func (m *model) exportVault(password []byte) tea.Cmd {
	m.keyExportID++
	id := m.keyExportID
	paths := config.DefaultPaths()
	outPath := strings.TrimSpace(m.keyExport.pathInput.Value())
	return func() tea.Msg {
		result, err := actions.ExportVault(paths, outPath, password)
		return keyExportFinishedMsg{id: id, result: result, err: err}
	}
}

func (m *model) openImportPicker(source string) tea.Cmd {
	m.keyImportPickerID++
	id := m.keyImportPickerID
	return func() tea.Msg {
		path, err := picker.ChooseFile()
		return keyImportPickerMsg{id: id, path: path, err: err}
	}
}

func (m *model) openExportPicker(defaultPath string) tea.Cmd {
	m.keyExportPickerID++
	id := m.keyExportPickerID
	defaultName := filepath.Base(strings.TrimSpace(defaultPath))
	if defaultName == "" || defaultName == "." || defaultName == string(filepath.Separator) {
		defaultName = filepath.Base(actions.DefaultExportPath())
	}
	return func() tea.Msg {
		path, err := picker.ChooseSavePath(defaultName)
		return keyExportPickerMsg{id: id, path: path, err: err}
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
	m.storeKeyCache(msg.keys)
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
			if !msg.preserve && m.snapshot.LoggedIn {
				return m, tea.Batch(textinput.Blink, m.refreshKeyBrowser(true))
			}
			return m, textinput.Blink
		}
		if !msg.preserve && m.snapshot.LoggedIn {
			return m, m.refreshKeyBrowser(true)
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
				original:  resolution.Exact.Name,
				input:     newKeyInput("Enter new key name"),
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
		if strings.TrimSpace(m.keyDetail.key.Name) != "" {
			m.keyDetail.status = ""
			m.keyDetail.statusErr = msg.err.Error()
			return m, nil
		}
		m.keyDetail.err = msg.err.Error()
		return m, nil
	}
	m.keyDetail.key = msg.detail
	m.keyDetail.err = ""
	m.keyDetail.statusErr = ""
	return m, nil
}

func (m *model) handleKeyCopyFinishedMsg(msg keyCopyFinishedMsg) (tea.Model, tea.Cmd) {
	m.keyDetail.busy = false
	if msg.err != nil {
		m.keyDetail.status = ""
		m.keyDetail.statusErr = msg.err.Error()
		return m, nil
	}
	m.keyDetail.statusErr = ""
	m.keyDetail.status = msg.status
	return m, nil
}

func (m *model) handleKeyPrivateCopyFinishedMsg(msg keyPrivateCopyFinishedMsg) (tea.Model, tea.Cmd) {
	if m.screen == screenPassword && m.passwordFlow == passwordKeyView {
		m.passwordBusy = false
		if msg.err != nil {
			m.passwordInput.SetError(msg.err.Error())
			return m, nil
		}
		m.passwordOverlay = false
		m.passwordAuth = ""
		m.screen = screenDashboard
		m.keyDetail.busy = false
		m.keyDetail.statusErr = ""
		m.keyDetail.status = msg.status
		return m, nil
	}

	m.keyDetail.busy = false
	if msg.err != nil {
		if actions.IsSensitiveAuthRequired(msg.err) {
			m.keyDetail.busy = false
			m.showPasswordScreen(passwordKeyView, "", "", true)
			return m, m.passwordInput.Init()
		}
		m.keyDetail.status = ""
		m.keyDetail.statusErr = msg.err.Error()
		return m, nil
	}
	m.keyDetail.statusErr = ""
	m.keyDetail.status = msg.status
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

func (m *model) handleKeyGenerateFinishedMsg(msg keyGenerateFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyGenerateID {
		return m, nil
	}
	m.keyGenerate.generating = false
	if msg.err != nil {
		m.keyGenerate.err = msg.err.Error()
		return m, nil
	}
	m.upsertCachedKey(actions.KeySummary{
		Name:        msg.result.Name,
		Type:        msg.result.Type,
		Fingerprint: msg.result.Fingerprint,
		Comment:     msg.result.Comment,
	})
	m.session.ReplaceCurrent(Route{
		ID: RouteKeysDetail,
		Params: map[string]string{
			"name":   msg.result.Name,
			"source": "browser",
		},
	})
	return m, m.showCurrentRoute()
}

func (m *model) handleKeyImportFinishedMsg(msg keyImportFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyImportID {
		return m, nil
	}
	m.keyImport.importing = false
	if msg.err != nil {
		m.keyImport.err = msg.err.Error()
		return m, nil
	}
	for _, key := range msg.result.Keys {
		m.upsertCachedKey(key)
	}
	notice := importResultNotice(msg.result)
	m.session.ReplaceCurrent(Route{
		ID: RouteKeysBrowser,
		Params: map[string]string{
			"notice": notice,
		},
	})
	return m, m.showCurrentRoute()
}

func (m *model) handleKeyExportFinishedMsg(msg keyExportFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyExportID {
		return m, nil
	}
	if m.screen == screenPassword && m.passwordFlow == passwordKeyExport {
		m.passwordBusy = false
		if msg.err != nil {
			m.passwordInput.SetError(msg.err.Error())
			return m, nil
		}
		m.passwordOverlay = false
		m.passwordAuth = ""
		m.screen = screenDashboard
		m.keyExport.exporting = false
		m.keyExport.err = ""
		m.keyExport.status = exportSuccessMessage(msg.result)
		return m, nil
	}

	m.keyExport.exporting = false
	if msg.err != nil {
		if actions.IsSensitiveAuthRequired(msg.err) {
			m.keyExport.exporting = false
			m.showPasswordScreen(passwordKeyExport, "", "", true)
			return m, m.passwordInput.Init()
		}
		m.keyExport.err = msg.err.Error()
		return m, nil
	}
	m.keyExport.err = ""
	m.keyExport.status = exportSuccessMessage(msg.result)
	return m, nil
}

func (m *model) handleKeyImportPickerMsg(msg keyImportPickerMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyImportPickerID {
		return m, nil
	}
	m.keyImport.pickerOpening = false
	source := m.currentImportSource()
	if msg.err == nil && strings.TrimSpace(msg.path) != "" {
		m.keyImport.pathInput.SetValue(msg.path)
		m.keyImport.pathVisible = false
		m.keyImport.err = ""
		m.keyImport.status = "Importing keys"
		m.keyImport.importing = true
		return m, tea.Batch(m.spinner.Tick, m.importKeysCmd(source.ID, msg.path))
	}
	m.keyImport.pathVisible = true
	m.keyImport.focus = 1
	m.keyImport.pathInput.Focus()
	switch {
	case errors.Is(msg.err, picker.ErrUnavailable):
		m.keyImport.err = "File picker unavailable. Enter a file path instead"
	case errors.Is(msg.err, picker.ErrCanceled), msg.err == nil:
		m.keyImport.err = ""
	default:
		m.keyImport.err = "File picker failed. Enter a file path instead"
	}
	m.keyImport.status = ""
	return m, textinput.Blink
}

func (m *model) handleKeyExportPickerMsg(msg keyExportPickerMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyExportPickerID {
		return m, nil
	}
	m.keyExport.pickerOpening = false
	if msg.err == nil && strings.TrimSpace(msg.path) != "" {
		m.keyExport.pathInput.SetValue(msg.path)
		m.keyExport.pathVisible = false
		m.keyExport.err = ""
		m.keyExport.status = "Exporting vault"
		m.keyExport.exporting = true
		return m, tea.Batch(m.spinner.Tick, m.exportVault(nil))
	}
	m.keyExport.pathVisible = true
	m.keyExport.pathInput.Focus()
	switch {
	case errors.Is(msg.err, picker.ErrUnavailable):
		m.keyExport.err = "File picker unavailable. Enter an export path instead"
	case errors.Is(msg.err, picker.ErrCanceled), msg.err == nil:
		m.keyExport.err = ""
	default:
		m.keyExport.err = "File picker failed. Enter an export path instead"
	}
	m.keyExport.status = ""
	return m, textinput.Blink
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

func (m *model) preloadKeyBrowser() tea.Cmd {
	if !m.snapshot.VaultExists || len(m.keyBrowser.all) > 0 {
		return nil
	}
	return m.listLocalKeys(m.nextKeyListID(), true)
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

func detailFromSummary(key actions.KeySummary) actions.KeyDetail {
	return actions.KeyDetail{
		ResolvedName: key.Name,
		Name:         key.Name,
		Type:         key.Type,
		Fingerprint:  key.Fingerprint,
		Comment:      key.Comment,
	}
}

func (m *model) storeKeyCache(keys []actions.KeySummary) {
	preserveName := ""
	if key, ok := m.selectedKeyRow(); ok {
		preserveName = key.Name
	}
	m.keyBrowser.all = keys
	m.refreshKeyBrowserRows()
	m.selectKeyBrowserByName(preserveName)
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
	inputWidth := max(18, min(shell.ClampBlockWidth(m.width, 44), 44))
	m.keyGenerate.nameInput.Width = inputWidth
	m.keyImport.pathInput.Width = max(18, min(shell.ClampBlockWidth(m.width, 54), 54))
	m.keyExport.pathInput.Width = max(18, min(shell.ClampBlockWidth(m.width, 54), 54))
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

func (m *model) moveKeyImportSource(delta int) {
	next := m.keyImport.sourceIndex + delta
	if next < 0 {
		next = len(keyImportSources) - 1
	}
	if next >= len(keyImportSources) {
		next = 0
	}
	m.keyImport.sourceIndex = next
	source := m.currentImportSource()
	m.keyImport.pathInput.Placeholder = source.Placeholder
	if !source.NeedsPath {
		m.keyImport.pathVisible = false
		m.keyImport.focus = 0
		m.keyImport.pathInput.Blur()
		m.keyImport.pathInput.SetValue("")
	}
}

func (m *model) currentImportSource() keyImportSource {
	if len(keyImportSources) == 0 {
		return keyImportSource{}
	}
	if m.keyImport.sourceIndex < 0 {
		m.keyImport.sourceIndex = 0
	}
	if m.keyImport.sourceIndex >= len(keyImportSources) {
		m.keyImport.sourceIndex = len(keyImportSources) - 1
	}
	return keyImportSources[m.keyImport.sourceIndex]
}

func (m *model) upsertCachedKey(summary actions.KeySummary) {
	found := false
	for index := range m.keyBrowser.all {
		if m.keyBrowser.all[index].Name == summary.Name {
			m.keyBrowser.all[index] = summary
			found = true
			break
		}
	}
	if !found {
		m.keyBrowser.all = append(m.keyBrowser.all, summary)
	}
	slices.SortFunc(m.keyBrowser.all, func(a, b actions.KeySummary) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	m.refreshKeyBrowserRows()
	m.selectKeyBrowserByName(summary.Name)
}

func importResultNotice(result actions.ImportResult) string {
	label := actions.ImportSourceLabel(result.Source)
	switch {
	case result.Imported == 0 && result.Discovered == 0:
		return "No keys were found to import"
	case result.Imported == 0:
		return fmt.Sprintf("No keys were imported from %s", label)
	case result.Skipped > 0:
		return fmt.Sprintf("Imported %d keys from %s, skipped %d", result.Imported, label, result.Skipped)
	default:
		return fmt.Sprintf("Imported %d keys from %s", result.Imported, label)
	}
}

func exportSuccessMessage(result actions.ExportResult) string {
	return fmt.Sprintf("Exported %d keys to %s", result.KeyCount, filepath.Base(result.Path))
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
