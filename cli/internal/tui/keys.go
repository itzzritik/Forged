package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/picker"
	commonscreen "github.com/itzzritik/forged/cli/internal/tui/screens/common"
	keyscreen "github.com/itzzritik/forged/cli/internal/tui/screens/keys"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

const (
	privateClipboardLifetime       = 45 * time.Second
	privateClipboardFastClearTries = 3
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
	name  string
	lease SensitiveClipboardLease
	err   error
}

type keyPrivateClipboardTickMsg struct {
	id  int
	now time.Time
}

type keyPrivateClipboardClearedMsg struct {
	id      int
	cleared bool
	err     error
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

type keyImportPreviewMsg struct {
	id     int
	result actions.ImportPreviewResult
	err    error
}

type keyExportFinishedMsg struct {
	id     int
	result actions.ExportResult
	err    error
}

type keyExportAuthorizedMsg struct {
	id    int
	token string
	err   error
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

type keyImportAutoReturnMsg struct {
	id int
}

type keyBrowserState struct {
	loading    bool
	loaded     bool
	refreshing bool
	err        string
	notice     string
	refreshErr string

	all      []actions.KeySummary
	rows     []actions.KeySummary
	selected int
	offset   int
	pageRows int

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

type privateClipboardState struct {
	id       int
	detailID int
	name     string
	deadline time.Time
	lease    SensitiveClipboardLease
	tries    int
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
	loading       bool
	importing     bool
	pickerOpening bool
	err           string
	status        string
	warning       string

	sourceIndex   int
	focus         int
	pathVisible   bool
	pathInput     textinput.Model
	step          keyImportStep
	previews      []actions.ImportPreview
	reviewCursor  int
	discovered    int
	duplicates    int
	result        actions.ImportResult
	failureCursor int
	failureOffset int
	success       *keyTransferSuccessState
}

type keyExportState struct {
	exporting     bool
	pickerOpening bool
	err           string
	status        string
	token         string

	pathVisible bool
	pathInput   textinput.Model
	success     *keyTransferSuccessState
}

type keyImportSource struct {
	ID          string
	Label       string
	NeedsPath   bool
	Placeholder string
}

type keyImportStep string

type keyTransferSuccessState struct {
	Title        string
	Message      string
	Detail       string
	autoReturnID int
}

const (
	keyImportStepSource           keyImportStep = "source"
	keyImportStepReview           keyImportStep = "review"
	keyImportStepResult           keyImportStep = "result"
	exportPlaintextWarning                      = "This export contains every private key in plaintext. Store it securely, avoid shared or cloud-synced folders, and delete it when finished."
	exportPlaintextSuccessWarning               = "Contains plaintext private keys. Store it securely and delete it when finished."

	keyBrowserSearchPrefixWidth = 3
	keyBrowserSearchGapWidth    = 4
	keyBrowserSearchCursorWidth = 1
	keyBrowserSearchMinInput    = 4
	maxImportFailureLogReasons  = 3
)

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
		return m.keyImport.loading || m.keyImport.importing || m.keyImport.pickerOpening
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
		return m.keyBrowser.loading || m.keyBrowser.loaded || m.keyBrowser.err != "" || strings.TrimSpace(m.keyBrowser.notice) != "" || strings.TrimSpace(m.keyBrowser.input.Value()) != ""
	case RouteKeysDetail:
		return !m.keyDetail.resolving && (m.keyDetail.loading || m.keyDetail.err != "" || strings.TrimSpace(m.keyDetail.key.Name) != "")
	case RouteKeysRename:
		return !m.keyRename.resolving && (m.keyRename.loading || m.keyRename.saving || m.keyRename.err != "" || strings.TrimSpace(m.keyRename.original) != "" || strings.TrimSpace(m.keyRename.input.Value()) != "")
	case RouteKeysDelete:
		return !m.keyDelete.resolving && (m.keyDelete.loading || m.keyDelete.deleting || m.keyDelete.err != "" || strings.TrimSpace(m.keyDelete.key.Name) != "")
	case RouteKeysGenerate:
		return strings.TrimSpace(m.keyGenerate.nameInput.Placeholder) != "" || m.keyGenerate.generating || strings.TrimSpace(m.keyGenerate.err) != "" || strings.TrimSpace(m.keyGenerate.status) != ""
	case RouteKeysImport:
		return len(keyImportSources) > 0 && (strings.TrimSpace(m.keyImport.err) != "" || strings.TrimSpace(m.keyImport.warning) != "" || strings.TrimSpace(m.keyImport.status) != "" || m.keyImport.loading || m.keyImport.importing || m.keyImport.pickerOpening || m.keyImport.pathVisible || strings.TrimSpace(m.keyImport.pathInput.Placeholder) != "" || len(m.keyImport.previews) > 0 || m.keyImport.step == keyImportStepResult || m.keyImport.success != nil)
	case RouteKeysExport:
		return strings.TrimSpace(m.keyExport.pathInput.Placeholder) != "" || strings.TrimSpace(m.keyExport.err) != "" || strings.TrimSpace(m.keyExport.status) != "" || m.keyExport.exporting || m.keyExport.pickerOpening || m.keyExport.pathVisible || m.keyExport.success != nil
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
			{Key: theme.Glyphs.UpDown, Label: "Move"},
			{Key: "Enter", Label: "View"},
			{Key: "E", Label: "Edit"},
			{Key: "D", Label: "Delete"},
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
		if m.keyRename.saving {
			return nil
		}
		if m.keyRename.loading {
			return []shell.FooterAction{
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		return []shell.FooterAction{
			{Key: "Enter", Label: "Save"},
			{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
		}
	case RouteKeysDelete:
		if m.keyDelete.deleting {
			return nil
		}
		if m.keyDelete.loading {
			return []shell.FooterAction{
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if !keyDeleteReviewValid(m.keyDelete.key) {
			if m.keyDelete.err != "" {
				return []shell.FooterAction{
					{Key: "Enter", Label: "Retry"},
					{Key: "Esc", Label: "Cancel"},
				}
			}
			return []shell.FooterAction{{Key: "Esc", Label: "Cancel"}}
		}
		return []shell.FooterAction{
			{Key: "Enter", Label: "Confirm Delete"},
			{Key: "Esc", Label: "Cancel"},
		}
	case RouteKeysGenerate:
		if m.keyGenerate.generating {
			return nil
		}
		return []shell.FooterAction{
			{Key: "Enter", Label: "Generate"},
			{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
		}
	case RouteKeysImport:
		if m.keyImport.importing {
			return nil
		}
		if m.keyImport.loading || m.keyImport.pickerOpening {
			return []shell.FooterAction{{Key: "Esc", Label: m.session.EscLabel(EscAuto)}}
		}
		if m.keyImport.success != nil {
			return []shell.FooterAction{{Key: "Esc", Label: "Dashboard"}}
		}
		if m.keyImport.step == keyImportStepResult {
			actions := make([]shell.FooterAction, 0, 2)
			if len(m.keyImport.result.Failures) > 1 {
				actions = append(actions, shell.FooterAction{Key: theme.Glyphs.UpDown, Label: "Move"})
			}
			if m.keyImportFailureHasMorePages() {
				actions = append(actions, shell.FooterAction{Key: theme.Glyphs.LeftRight, Label: "Reason"})
			}
			return append(actions, shell.FooterAction{Key: "Esc", Label: "Dashboard"})
		}
		if m.keyImport.step == keyImportStepReview {
			actions := []shell.FooterAction{
				{Key: theme.Glyphs.UpDown, Label: "Move"},
				{Key: "Space", Label: "Toggle"},
				{Key: "A", Label: m.keyImportBulkToggleLabel()},
			}
			if selected := m.keyImportSelectedCount(); selected > 0 {
				actions = append(actions, shell.FooterAction{Key: "Enter", Label: footerImportLabel(selected)})
			}
			actions = append(actions, shell.FooterAction{Key: "Esc", Label: m.session.EscLabel(EscAuto)})
			return actions
		}
		source := m.currentImportSource()
		if m.keyImport.pathVisible && m.keyImport.focus == 1 {
			return []shell.FooterAction{
				{Key: theme.Glyphs.UpDown, Label: "Source"},
				{Key: "Enter", Label: "Review"},
				{Key: "Tab", Label: "Choose file"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		enterLabel := "Review"
		if source.NeedsPath {
			enterLabel = "Choose File"
		}
		actions := []shell.FooterAction{
			{Key: theme.Glyphs.UpDown, Label: "Source"},
			{Key: "Enter", Label: enterLabel},
		}
		if m.keyImport.pathVisible {
			actions = append(actions, shell.FooterAction{Key: "Tab", Label: "Path"})
		}
		actions = append(actions, shell.FooterAction{Key: "Esc", Label: m.session.EscLabel(EscAuto)})
		return actions
	case RouteKeysExport:
		if m.keyExport.exporting {
			return nil
		}
		if m.keyExport.pickerOpening {
			return []shell.FooterAction{{Key: "Esc", Label: "Cancel"}}
		}
		if m.keyExport.success != nil {
			return []shell.FooterAction{{Key: "Enter", Label: "Dashboard"}}
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

func (m *model) renderKeyBody(contentWidth int, bodyHeight int) string {
	switch m.session.Current().ID {
	case RouteKeysBrowser:
		m.resizeKeyBrowserPage(bodyHeight)
		rows := m.keyBrowserVisibleRows()
		browserRows := make([]keyscreen.BrowserRow, 0, len(rows))
		for _, row := range rows {
			browserRows = append(browserRows, keyscreen.BrowserRow{
				Name:        row.Name,
				Type:        row.Type,
				Fingerprint: row.Fingerprint,
			})
		}
		return keyscreen.RenderBrowser(keyscreen.BrowserScreen{
			SearchView:   theme.AdaptTextInputPlaceholder(m.keyBrowser.input.View(), m.keyBrowser.input.Value()),
			SearchQuery:  m.keyBrowser.input.Value(),
			SearchActive: m.keyBrowser.searchActive,
			SearchNotice: m.keyBrowserNotice(),
			CountLabel:   m.keyBrowserCountLabel(),
			VisibleRows:  m.keyBrowserPageRows(),
			Rows:         browserRows,
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
		key := m.keyDetail.key
		if m.signingLoaded {
			key.GitSigning = m.keyMatchesCurrentSigning(key.PublicKey)
		}
		return keyscreen.RenderDetail(keyscreen.DetailScreen{
			Loading:     m.keyDetail.loading || m.keyDetail.resolving,
			Error:       m.keyDetail.err,
			Key:         key,
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
			FieldView: theme.AdaptTextInputPlaceholder(m.keyRename.input.View(), m.keyRename.input.Value()),
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
			Context: "Review this key before permanently deleting it from the vault",
			Key:     m.keyDelete.key,
			Warning: deleteWarning(m.keyDelete.key.Name),
			Status:  deleteStatus(m.keyDelete.deleting),
			Error:   m.keyDelete.err,
			Loading: m.keyDelete.loading,
		}, m.spinner.View(), contentWidth)
	case RouteKeysGenerate:
		return keyscreen.RenderGenerate(keyscreen.GenerateScreen{
			Context:    "Create a new SSH key and add it to this vault",
			NameView:   theme.AdaptTextInputPlaceholder(m.keyGenerate.nameInput.View(), m.keyGenerate.nameInput.Value()),
			Focused:    true,
			Status:     m.keyGenerate.status,
			Error:      m.keyGenerate.err,
			Generating: m.keyGenerate.generating,
		}, m.spinner.View(), contentWidth)
	case RouteKeysImport:
		if m.keyImport.success != nil {
			return commonscreen.RenderSuccess(commonscreen.SuccessScreen{
				Context: "Import keys from another source into this vault",
				Title:   m.keyImport.success.Title,
				Message: m.keyImport.success.Message,
				Detail:  m.keyImport.success.Detail,
			}, contentWidth)
		}
		if m.keyImport.step == keyImportStepResult {
			start, end := importReviewWindowBounds(len(m.keyImport.result.Failures), m.keyImport.failureCursor)
			items := make([]keyscreen.ImportReviewItem, 0, max(end-start, 0))
			for index := start; index < end; index++ {
				failure := m.keyImport.result.Failures[index]
				items = append(items, keyscreen.ImportReviewItem{
					Name:        failure.Name,
					Fingerprint: failure.Fingerprint,
					Active:      index == m.keyImport.failureCursor,
					Failed:      true,
				})
			}
			failure, _ := m.selectedKeyImportFailure()
			failurePage, _ := importFailureReasonPage(failure.Reason, m.keyImport.failureOffset, importReviewContentWidth(contentWidth))
			return keyscreen.RenderImportReview(keyscreen.ImportReviewScreen{
				Context:     "Review each key that could not be imported",
				SourceLabel: "Import failures",
				Count:       len(m.keyImport.result.Failures),
				Items:       items,
				HasAbove:    start > 0,
				HasBelow:    end < len(m.keyImport.result.Failures),
				Summary:     importFailureResultSummaryLines(m.keyImport.result),
				Guidance:    m.keyImportFailureGuidance(),
				Failure:     failurePage,
			}, m.spinner.View(), contentWidth, bodyHeight)
		}
		if m.keyImport.step == keyImportStepReview {
			start, end := importReviewWindowBounds(len(m.keyImport.previews), m.keyImport.reviewCursor)
			items := make([]keyscreen.ImportReviewItem, 0, max(end-start, 0))
			for index := start; index < end; index++ {
				preview := m.keyImport.previews[index]
				items = append(items, keyscreen.ImportReviewItem{
					Name:        preview.Key.Name,
					Fingerprint: preview.Fingerprint,
					Checked:     preview.Selected,
					Active:      index == m.keyImport.reviewCursor,
					Converted:   preview.Converted,
				})
			}
			return keyscreen.RenderImportReview(keyscreen.ImportReviewScreen{
				Context:     "Import keys from another source into this vault",
				SourceLabel: keyImportReviewSourceLabel(m.currentImportSource().ID),
				Count:       len(m.keyImport.previews),
				Items:       items,
				HasAbove:    start > 0,
				HasBelow:    end < len(m.keyImport.previews),
				Summary:     m.keyImportSummaryLines(),
				Guidance:    m.keyImportGuidanceLine(),
				Warning:     m.keyImportDuplicateWarning(),
				Error:       m.keyImport.err,
				Status:      m.keyImport.status,
				Busy:        m.keyImport.importing,
			}, m.spinner.View(), contentWidth, bodyHeight)
		}
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
			PathView:    theme.AdaptTextInputPlaceholder(m.keyImport.pathInput.View(), m.keyImport.pathInput.Value()),
			PathFocused: m.keyImport.focus == 1,
			PathVisible: m.keyImport.pathVisible,
			Status:      m.keyImport.status,
			Warning:     m.keyImport.warning,
			Error:       m.keyImport.err,
			Busy:        m.keyImport.loading || m.keyImport.importing || m.keyImport.pickerOpening,
		}, m.spinner.View(), contentWidth)
	case RouteKeysExport:
		if m.keyExport.success != nil {
			return commonscreen.RenderSuccess(commonscreen.SuccessScreen{
				Context: "Vault export",
				Title:   m.keyExport.success.Title,
				Message: m.keyExport.success.Message,
				Warning: m.keyExport.success.Detail,
			}, contentWidth)
		}
		return keyscreen.RenderExport(keyscreen.ExportScreen{
			Context:     "Choose where to save the export",
			Warning:     exportPlaintextWarning,
			PathView:    theme.AdaptTextInputPlaceholder(m.keyExport.pathInput.View(), m.keyExport.pathInput.Value()),
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
	m.ensureKeyBrowserInput()

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
	case "r", "R":
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
	case "e", "E":
		if key, ok := m.selectedKeyRow(); ok {
			m.session.Push(Route{ID: RouteKeysRename, Params: map[string]string{"old_name": key.Name, "source": "browser"}})
			return m, m.showCurrentRoute()
		}
	case "d", "D":
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
	case "c", "C":
		if strings.TrimSpace(m.keyDetail.key.PublicKey) == "" {
			return m, nil
		}
		m.keyDetail.status = ""
		m.keyDetail.statusErr = ""
		m.keyDetail.busy = true
		return m, tea.Batch(m.spinner.Tick, m.copyKeyText(m.keyDetail.key.PublicKey, "Public key copied"))
	case "f", "F":
		if strings.TrimSpace(m.keyDetail.key.Fingerprint) == "" {
			return m, nil
		}
		m.keyDetail.status = ""
		m.keyDetail.statusErr = ""
		m.keyDetail.busy = true
		return m, tea.Batch(m.spinner.Tick, m.copyKeyText(m.keyDetail.key.Fingerprint, "Fingerprint copied"))
	case "k", "K":
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
	if m.keyRename.saving {
		return m, nil
	}
	if m.keyRename.loading {
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
	if m.keyDelete.deleting {
		return m, nil
	}
	if m.keyDelete.loading {
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
		if !keyDeleteReviewValid(m.keyDelete.key) {
			m.keyDelete = keyDeleteState{loading: true, resolving: true}
			return m, tea.Batch(m.spinner.Tick, m.listKeys(m.nextKeyListID(), false))
		}
		m.keyDelete.err = ""
		m.keyDelete.deleting = true
		return m, tea.Batch(m.spinner.Tick, m.deleteKey(m.keyDelete.key.Name, m.keyDelete.key.Fingerprint))
	}
	return m, nil
}

func (m *model) updateKeyGenerate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keyGenerate.generating {
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
			m.keyGenerate.status = ""
			return m, nil
		}
		m.keyGenerate.err = ""
		m.keyGenerate.status = "Generating key"
		m.keyGenerate.generating = true
		return m, tea.Batch(m.spinner.Tick, m.generateKeyCmd(name, ""))
	default:
		return m, m.updateKeyGenerateInputs(msg)
	}
}

func (m *model) updateKeyImport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keyImport.importing {
		return m, nil
	}
	if m.keyImport.loading || m.keyImport.pickerOpening {
		if msg.String() == "esc" {
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		}
		return m, nil
	}
	if m.keyImport.success != nil {
		if msg.String() == "esc" {
			m.keyImport.success = nil
			return m, m.returnToDashboardRoute()
		}
		return m, nil
	}
	if m.keyImport.step == keyImportStepResult {
		switch msg.String() {
		case "esc":
			m.keyImport.result = actions.ImportResult{}
			m.keyImport.previews = nil
			m.keyImport.failureOffset = 0
			return m, m.returnToDashboardRoute()
		case "up", "k":
			m.moveKeyImportFailureCursor(-1)
		case "down", "j":
			m.moveKeyImportFailureCursor(1)
		case "left", "h":
			m.moveKeyImportFailurePage(false)
		case "right", "l":
			m.moveKeyImportFailurePage(true)
		}
		return m, nil
	}

	if m.keyImport.step == keyImportStepReview {
		switch msg.String() {
		case "esc":
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		case "up", "k":
			m.moveKeyImportReviewCursor(-1)
			return m, nil
		case "down", "j":
			m.moveKeyImportReviewCursor(1)
			return m, nil
		case " ":
			m.toggleCurrentImportReviewItem()
			return m, nil
		case "a", "A":
			m.toggleAllImportReviewItems()
			return m, nil
		case "enter":
			selected := m.keyImportSelectedCount()
			if selected == 0 {
				return m, nil
			}
			m.keyImport.err = ""
			m.keyImport.warning = ""
			m.keyImport.status = fmt.Sprintf("Importing %d keys", selected)
			m.keyImport.result = actions.ImportResult{}
			m.keyImport.failureOffset = 0
			m.keyImport.importing = true
			return m, tea.Batch(m.spinner.Tick, m.importSelectedPreviewsCmd(m.currentImportSource().ID, m.keyImport.discovered, m.keyImport.previews))
		default:
			return m, nil
		}
	}

	source := m.currentImportSource()
	if m.keyImport.pathVisible && m.keyImport.focus == 1 && len(msg.Runes) > 0 {
		return m, m.updateKeyImportPath(msg)
	}
	switch msg.String() {
	case "esc":
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "tab":
		if m.keyImport.pathVisible && m.keyImport.focus == 1 {
			m.keyImport.err = ""
			m.keyImport.warning = ""
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
				m.keyImport.warning = ""
				m.keyImport.status = "Choosing import file"
				m.keyImport.pickerOpening = true
				return m, tea.Batch(m.spinner.Tick, m.openImportPicker(source.ID))
			}
			m.keyImport.err = ""
			m.keyImport.warning = ""
			m.keyImport.status = importLoadingStatus(source.ID)
			m.keyImport.loading = true
			return m, tea.Batch(m.spinner.Tick, m.previewImportCmd(source.ID, ""))
		}
		file := ""
		if source.NeedsPath {
			file = strings.TrimSpace(m.keyImport.pathInput.Value())
			if file == "" {
				m.keyImport.err = "Enter a file path"
				m.keyImport.warning = ""
				return m, nil
			}
		}
		m.keyImport.err = ""
		m.keyImport.warning = ""
		m.keyImport.status = importLoadingStatus(source.ID)
		m.keyImport.loading = true
		return m, tea.Batch(m.spinner.Tick, m.previewImportCmd(source.ID, file))
	default:
		if m.keyImport.focus == 1 {
			return m, m.updateKeyImportPath(msg)
		}
	}
	return m, nil
}

func (m *model) updateKeyExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keyExport.exporting {
		return m, nil
	}
	if m.keyExport.pickerOpening {
		if msg.String() == "esc" {
			m.keyExportPickerID++
			m.keyExport.pickerOpening = false
			m.keyExport.token = ""
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		}
		return m, nil
	}
	if m.keyExport.success != nil {
		switch msg.String() {
		case "enter", "esc":
			m.keyExport.success = nil
			return m, m.returnToDashboardRoute()
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
		return m, tea.Batch(m.spinner.Tick, m.exportVault())
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
	m.keyGenerate.status = ""
	return cmd
}

func (m *model) updateKeyImportPath(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	m.keyImport.pathInput, cmd = m.keyImport.pathInput.Update(msg)
	m.keyImport.err = ""
	m.keyImport.warning = ""
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
		if m.keyBrowser.loaded {
			m.applyKeyBrowserRoute(route)
			m.keyBrowser.loading = false
			m.keyBrowser.refreshing = m.snapshot.LoggedIn
			m.keyBrowser.err = ""
			if m.snapshot.LoggedIn {
				return m.refreshKeyBrowser(false)
			}
			return nil
		}
		m.keyBrowser = keyBrowserState{
			loading: true,
			input:   newKeyInput("Search keys"),
			notice:  strings.TrimSpace(route.Params["notice"]),
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
			step:        keyImportStepSource,
			success:     nil,
		}
		m.keyImport.pathInput.CharLimit = 512
		m.keyImport.pathInput.Placeholder = m.currentImportSource().Placeholder
		m.resizeKeyInputs()
		return nil
	case RouteKeysExport:
		m.keyExport = keyExportState{
			pathInput: newKeyInput("Export path"),
			success:   nil,
		}
		m.keyExport.pathInput.CharLimit = 512
		m.keyExport.pathInput.SetValue(actions.DefaultExportPath())
		m.keyExport.pathVisible = false
		m.resizeKeyInputs()
		m.showPasswordScreenOnRoute(RouteKeysExport, passwordKeyExport, "", "", false)
		return m.passwordInput.Init()
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
	loggedIn := m.snapshot.LoggedIn
	return func() tea.Msg {
		if loggedIn {
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
	m.clipboardBusy = true
	copyText := m.deps.CopyText
	return func() tea.Msg {
		if strings.TrimSpace(value) == "" {
			return keyCopyFinishedMsg{err: fmt.Errorf("Nothing to copy")}
		}
		if err := copyText(value); err != nil {
			return keyCopyFinishedMsg{err: err}
		}
		return keyCopyFinishedMsg{status: status}
	}
}

func (m *model) copyPrivateKey(password []byte) tea.Cmd {
	m.clipboardBusy = true
	copySensitiveText := m.deps.CopySensitiveText
	name := strings.TrimSpace(m.keyDetail.key.Name)
	paths := config.DefaultPaths()
	passwordCopy := append([]byte(nil), password...)
	clear(password)
	return func() tea.Msg {
		defer clear(passwordCopy)
		detail, err := actions.ViewFullKey(paths, name, passwordCopy)
		if err != nil {
			return keyPrivateCopyFinishedMsg{err: err}
		}
		if strings.TrimSpace(detail.PrivateKey) == "" {
			return keyPrivateCopyFinishedMsg{err: fmt.Errorf("Private key is unavailable")}
		}
		lease, err := copySensitiveText(detail.PrivateKey)
		if err != nil {
			return keyPrivateCopyFinishedMsg{err: err}
		}
		return keyPrivateCopyFinishedMsg{name: name, lease: lease}
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

func (m *model) deleteKey(name string, fingerprint string) tea.Cmd {
	m.keyDeleteID++
	id := m.keyDeleteID
	paths := config.DefaultPaths()
	return func() tea.Msg {
		resolvedName, err := actions.DeleteKey(paths, name, fingerprint)
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

func (m *model) previewImportCmd(source, file string) tea.Cmd {
	m.keyImportPreviewID++
	id := m.keyImportPreviewID
	paths := config.DefaultPaths()
	return func() tea.Msg {
		result, err := actions.PreviewImportSource(paths, source, file)
		return keyImportPreviewMsg{id: id, result: result, err: err}
	}
}

func (m *model) importSelectedPreviewsCmd(source string, discovered int, previews []actions.ImportPreview) tea.Cmd {
	m.keyImportID++
	id := m.keyImportID
	paths := config.DefaultPaths()
	selected := append([]actions.ImportPreview(nil), previews...)
	return func() tea.Msg {
		result, err := actions.ImportSelectedPreviews(paths, source, discovered, selected)
		return keyImportFinishedMsg{id: id, result: result, err: err}
	}
}

func (m *model) exportVault() tea.Cmd {
	m.keyExportID++
	id := m.keyExportID
	paths := config.DefaultPaths()
	outPath := strings.TrimSpace(m.keyExport.pathInput.Value())
	token := strings.TrimSpace(m.keyExport.token)
	return func() tea.Msg {
		result, err := actions.ExportVaultWithToken(paths, outPath, token)
		return keyExportFinishedMsg{id: id, result: result, err: err}
	}
}

func (m *model) authorizeKeyExport(password []byte) tea.Cmd {
	m.keyExportID++
	id := m.keyExportID
	paths := config.DefaultPaths()
	passwordCopy := append([]byte(nil), password...)
	clear(password)
	return func() tea.Msg {
		defer clear(passwordCopy)
		token, err := actions.AuthorizeExport(paths, passwordCopy)
		return keyExportAuthorizedMsg{id: id, token: token, err: err}
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

	current := m.session.Current()
	if msg.preserve && current.ID != RouteKeysBrowser {
		if msg.err != nil {
			m.reportError("keys.list", msg.err)
			return m, nil
		}
		m.storeKeyCache(msg.keys)
		return m, nil
	}

	if msg.err != nil {
		errorText := m.reportError("keys.list", msg.err)
		m.keyBrowser.loading = false
		switch current.ID {
		case RouteKeysBrowser:
			m.keyBrowser.refreshing = false
			if msg.preserve && m.keyBrowser.loaded {
				m.keyBrowser.refreshErr = errorText
				return m, nil
			}
			m.keyBrowser.err = errorText
		case RouteKeysDetail:
			m.keyDetail.loading = false
			m.keyDetail.resolving = false
			m.keyDetail.err = errorText
		case RouteKeysRename:
			m.keyRename.loading = false
			m.keyRename.resolving = false
			m.keyRename.err = errorText
		case RouteKeysDelete:
			m.keyDelete.loading = false
			m.keyDelete.resolving = false
			m.keyDelete.err = errorText
		}
		return m, nil
	}

	m.storeKeyCache(msg.keys)
	switch current.ID {
	case RouteKeysBrowser:
		m.keyBrowser.loading = false
		m.keyBrowser.refreshing = false
		m.keyBrowser.err = ""
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
		errorText := m.reportError("keys.detail", msg.err)
		if strings.TrimSpace(m.keyDetail.key.Name) != "" {
			m.keyDetail.status = ""
			m.keyDetail.statusErr = errorText
			return m, nil
		}
		m.keyDetail.err = errorText
		return m, nil
	}
	m.keyDetail.key = msg.detail
	m.keyDetail.err = ""
	m.keyDetail.statusErr = ""
	return m, nil
}

func (m *model) handleKeyCopyFinishedMsg(msg keyCopyFinishedMsg) (tea.Model, tea.Cmd) {
	m.clipboardBusy = false
	m.keyDetail.busy = false
	if msg.err != nil {
		m.keyDetail.status = ""
		m.keyDetail.statusErr = m.reportError("keys.copy-public", msg.err)
		return m, nil
	}
	m.cancelPrivateClipboard()
	m.keyDetail.statusErr = ""
	m.keyDetail.status = msg.status
	return m, nil
}

func (m *model) handleKeyPrivateCopyFinishedMsg(msg keyPrivateCopyFinishedMsg) (tea.Model, tea.Cmd) {
	m.clipboardBusy = false
	if m.screen == screenPassword && m.passwordFlow == passwordKeyView {
		m.passwordBusy = false
		if msg.err != nil {
			m.passwordInput.SetError(m.reportError("keys.copy-private", msg.err))
			return m, nil
		}
		m.passwordOverlay = false
		m.passwordAuth = ""
		m.discardPasswordInput()
		m.screen = screenDashboard
		m.keyDetail.busy = false
		return m, m.startPrivateClipboard(msg.name, msg.lease)
	}

	m.keyDetail.busy = false
	if msg.err != nil {
		if actions.IsSensitiveAuthRequired(msg.err) {
			if m.isLockedAuthScreen() {
				return m, nil
			}
			m.showPasswordScreen(passwordStartupUnlock, "", "", true)
			m.passwordContext = "Please authenticate to continue using Forged."
			return m, m.passwordInput.Init()
		}
		m.keyDetail.status = ""
		m.keyDetail.statusErr = m.reportError("keys.copy-private", msg.err)
		return m, nil
	}
	return m, m.startPrivateClipboard(msg.name, msg.lease)
}

func (m *model) startPrivateClipboard(name string, lease SensitiveClipboardLease) tea.Cmd {
	m.privateClip.id++
	m.privateClip.detailID = m.keyDetailID
	m.privateClip.name = name
	m.privateClip.deadline = time.Now().Add(privateClipboardLifetime)
	m.privateClip.lease = lease
	m.privateClip.tries = 0
	m.setPrivateClipboardStatus(privateClipboardSecondsRemaining(m.privateClip.deadline, time.Now()))
	return privateClipboardTick(m.privateClip.id, m.privateClip.deadline)
}

func (m *model) handleKeyPrivateClipboardTickMsg(msg keyPrivateClipboardTickMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.privateClip.id || m.privateClip.lease == nil {
		return m, nil
	}
	remaining := privateClipboardSecondsRemaining(m.privateClip.deadline, msg.now)
	if remaining > 0 {
		m.setPrivateClipboardStatus(remaining)
		return m, privateClipboardTick(msg.id, m.privateClip.deadline)
	}
	if m.privateClipboardDetailActive() {
		m.keyDetail.status = "Clearing private key from clipboard"
		m.keyDetail.statusErr = ""
	}
	lease := m.privateClip.lease
	return m, func() tea.Msg {
		cleared, err := lease.ClearIfUnchanged()
		return keyPrivateClipboardClearedMsg{id: msg.id, cleared: cleared, err: err}
	}
}

func (m *model) handleKeyPrivateClipboardClearedMsg(msg keyPrivateClipboardClearedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.privateClip.id {
		return m, nil
	}
	activeDetail := m.privateClipboardDetailActive()
	if msg.err != nil {
		errorText := m.reportError("clipboard.clear-private", msg.err)
		m.privateClip.tries++
		if activeDetail {
			m.keyDetail.status = ""
			m.keyDetail.statusErr = "Couldn't clear private key from clipboard: " + errorText
		}
		delay := 5 * time.Second
		if m.privateClip.tries >= privateClipboardFastClearTries {
			delay = 30 * time.Second
		}
		return m, privateClipboardRetry(msg.id, delay)
	}
	m.privateClip.lease = nil
	if !activeDetail {
		return m, nil
	}
	m.keyDetail.statusErr = ""
	if msg.cleared {
		m.keyDetail.status = "Private key cleared from clipboard"
	} else {
		m.keyDetail.status = "Clipboard changed " + theme.Glyphs.Separator + " no clear needed"
	}
	return m, nil
}

func (m *model) cancelPrivateClipboard() {
	m.privateClip = privateClipboardState{id: m.privateClip.id + 1}
}

func (m *model) setPrivateClipboardStatus(remaining int) {
	if !m.privateClipboardDetailActive() {
		return
	}
	m.keyDetail.statusErr = ""
	m.keyDetail.status = fmt.Sprintf("Private key copied %s clears in %ds", theme.Glyphs.Separator, remaining)
}

func (m *model) privateClipboardDetailActive() bool {
	return m.privateClip.detailID == m.keyDetailID &&
		m.session.Current().ID == RouteKeysDetail &&
		strings.TrimSpace(m.keyDetail.key.Name) == m.privateClip.name
}

func privateClipboardTick(id int, deadline time.Time) tea.Cmd {
	delay := min(time.Second, max(time.Duration(0), time.Until(deadline)))
	return tea.Tick(delay, func(now time.Time) tea.Msg {
		return keyPrivateClipboardTickMsg{id: id, now: now}
	})
}

func privateClipboardRetry(id int, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(now time.Time) tea.Msg {
		return keyPrivateClipboardTickMsg{id: id, now: now}
	})
}

func privateClipboardSecondsRemaining(deadline time.Time, now time.Time) int {
	remaining := deadline.Sub(now)
	if remaining <= 0 {
		return 0
	}
	return int((remaining + time.Second - 1) / time.Second)
}

func (m *model) handleKeyRenameFinishedMsg(msg keyRenameFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyRenameID {
		return m, nil
	}
	current := m.session.Current().ID == RouteKeysRename && m.keyRename.saving
	visible := current && m.isKeyRoute()
	m.keyRename.saving = false
	if msg.err != nil {
		errorText := m.reportError("keys.rename", msg.err)
		if current {
			m.keyRename.err = errorText
		}
		return m, nil
	}
	m.renameCachedKey(msg.result.OldName, msg.result.NewName)
	if current {
		m.session.ReplaceCurrent(Route{
			ID: RouteKeysBrowser,
			Params: map[string]string{
				"query": msg.result.NewName,
			},
		})
	}
	if visible {
		return m, tea.Batch(m.showCurrentRoute(), m.invalidateSigningStatusCmd())
	}
	return m, m.invalidateSigningStatusCmd()
}

func (m *model) handleKeyDeleteFinishedMsg(msg keyDeleteFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyDeleteID {
		return m, nil
	}
	current := m.session.Current().ID == RouteKeysDelete && m.keyDelete.deleting
	visible := current && m.isKeyRoute()
	m.keyDelete.deleting = false
	if msg.err != nil {
		errorText := m.reportError("keys.delete", msg.err)
		if current {
			m.keyDelete.key = actions.KeySummary{}
			m.keyDelete.err = errorText
		}
		return m, nil
	}
	m.removeCachedKey(msg.name)
	if current {
		m.session.ReplaceCurrent(Route{ID: RouteKeysBrowser})
	}
	if visible {
		return m, tea.Batch(m.showCurrentRoute(), m.invalidateSigningStatusCmd())
	}
	return m, m.invalidateSigningStatusCmd()
}

func (m *model) handleKeyGenerateFinishedMsg(msg keyGenerateFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyGenerateID {
		return m, nil
	}
	current := m.session.Current().ID == RouteKeysGenerate && m.keyGenerate.generating
	visible := current && m.isKeyRoute()
	m.keyGenerate.generating = false
	if msg.err != nil {
		errorText := m.reportError("keys.generate", msg.err)
		if current {
			m.keyGenerate.status = ""
			m.keyGenerate.err = errorText
		}
		return m, nil
	}
	m.upsertCachedKey(actions.KeySummary{
		Name:        msg.result.Name,
		Type:        msg.result.Type,
		Fingerprint: msg.result.Fingerprint,
		Comment:     msg.result.Comment,
	})
	if current {
		m.session.ReplaceCurrent(Route{
			ID: RouteKeysDetail,
			Params: map[string]string{
				"name":   msg.result.Name,
				"source": "browser",
			},
		})
	}
	if visible {
		return m, tea.Batch(m.showCurrentRoute(), m.invalidateSigningStatusCmd())
	}
	return m, m.invalidateSigningStatusCmd()
}

func (m *model) handleKeyImportFinishedMsg(msg keyImportFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyImportID {
		return m, nil
	}
	m.keyImport.importing = false
	m.keyImport.status = ""
	if msg.err != nil {
		m.keyImport.err = m.reportError("keys.import", msg.err)
		m.keyImport.warning = ""
		return m, nil
	}
	for _, key := range msg.result.Keys {
		m.upsertCachedKey(key)
	}
	m.keyImport.result = msg.result
	if len(msg.result.Failures) > 0 {
		m.keyImport.step = keyImportStepResult
		m.keyImport.failureCursor = 0
		m.keyImport.failureOffset = 0
		m.keyImport.previews = nil
		m.keyImport.err = ""
		m.keyImport.warning = ""
		m.logImportFailures(msg.result)
		return m, m.invalidateSigningStatusCmd()
	}
	if msg.result.Imported == 0 {
		m.keyImport.err = "No keys were imported"
		m.keyImport.warning = ""
		return m, nil
	}
	m.keyImport.previews = nil
	m.keyImport.err = ""
	m.keyImport.warning = ""
	m.keyImport.success = m.newKeyTransferSuccessState(
		"Import complete",
		importSuccessMessage(msg.result),
		"Returning to dashboard...",
	)
	return m, tea.Batch(
		m.scheduleKeyImportAutoReturn(m.keyImport.success.autoReturnID),
		m.invalidateSigningStatusCmd(),
	)
}

func (m *model) handleKeyImportPreviewMsg(msg keyImportPreviewMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyImportPreviewID {
		return m, nil
	}
	m.keyImport.loading = false
	m.keyImport.result = actions.ImportResult{}
	m.keyImport.failureCursor = 0
	m.keyImport.failureOffset = 0
	source := m.currentImportSource()
	if msg.err != nil {
		m.keyImport.step = keyImportStepSource
		m.keyImport.previews = nil
		m.keyImport.discovered = 0
		m.keyImport.duplicates = 0
		m.keyImport.status = ""
		m.keyImport.warning = ""
		m.keyImport.err = m.reportError("keys.import-preview", msg.err)
		if source.NeedsPath {
			m.keyImport.pathVisible = true
			m.keyImport.focus = 1
			m.keyImport.pathInput.Focus()
			return m, textinput.Blink
		}
		return m, nil
	}
	if len(msg.result.Previews) == 0 {
		m.keyImport.step = keyImportStepSource
		m.keyImport.previews = nil
		m.keyImport.discovered = msg.result.Discovered
		m.keyImport.duplicates = msg.result.Duplicates
		m.keyImport.status = ""
		m.keyImport.warning = importEmptyResultMessage(source.ID)
		m.keyImport.err = ""
		if source.NeedsPath {
			m.keyImport.pathVisible = true
			m.keyImport.focus = 1
			m.keyImport.pathInput.Focus()
			return m, textinput.Blink
		}
		return m, nil
	}

	m.keyImport.err = ""
	m.keyImport.warning = ""
	m.keyImport.status = ""
	m.keyImport.step = keyImportStepReview
	m.keyImport.previews = append([]actions.ImportPreview(nil), msg.result.Previews...)
	m.keyImport.reviewCursor = 0
	m.keyImport.discovered = msg.result.Discovered
	m.keyImport.duplicates = msg.result.Duplicates
	return m, nil
}

func (m *model) handleKeyExportFinishedMsg(msg keyExportFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyExportID {
		return m, nil
	}
	if m.screen != screenDashboard || m.session.Current().ID != RouteKeysExport || !m.keyExport.exporting {
		return m, nil
	}

	m.keyExport.exporting = false
	m.keyExport.status = ""
	if msg.err != nil {
		if actions.IsSensitiveAuthRequired(msg.err) {
			m.keyExport.exporting = false
			m.keyExport.token = ""
			m.showPasswordScreen(passwordKeyExport, "", "", true)
			return m, m.passwordInput.Init()
		}
		m.keyExport.err = m.reportError("keys.export", msg.err)
		return m, nil
	}
	m.keyExport.err = ""
	m.keyExport.status = ""
	m.keyExport.success = m.newKeyTransferSuccessState(
		"Export saved",
		exportSuccessMessage(msg.result),
		exportPlaintextSuccessWarning,
	)
	return m, nil
}

func (m *model) handleKeyExportAuthorizedMsg(msg keyExportAuthorizedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyExportID ||
		m.screen != screenPassword ||
		m.passwordFlow != passwordKeyExport ||
		!m.passwordBusy ||
		m.session.Current().ID != RouteKeysExport {
		return m, nil
	}
	m.passwordBusy = false
	if msg.err != nil {
		m.passwordInput.SetError(m.reportError("keys.export-authorize", msg.err))
		return m, nil
	}
	m.passwordOverlay = false
	m.passwordAuth = ""
	m.discardPasswordInput()
	m.screen = screenDashboard
	m.keyExport.token = strings.TrimSpace(msg.token)
	m.keyExport.err = ""
	m.keyExport.status = "Choosing export file"
	m.keyExport.pickerOpening = true
	return m, tea.Batch(m.spinner.Tick, m.openExportPicker(strings.TrimSpace(m.keyExport.pathInput.Value())))
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
		m.keyImport.warning = ""
		m.keyImport.status = importLoadingStatus(source.ID)
		m.keyImport.loading = true
		return m, tea.Batch(m.spinner.Tick, m.previewImportCmd(source.ID, msg.path))
	}
	m.keyImport.pathVisible = true
	m.keyImport.focus = 1
	m.keyImport.pathInput.Focus()
	switch {
	case errors.Is(msg.err, picker.ErrUnavailable):
		m.reportError("picker.import", msg.err)
		m.keyImport.err = "File picker unavailable. Enter a file path instead"
	case errors.Is(msg.err, picker.ErrCanceled), msg.err == nil:
		m.keyImport.err = ""
	default:
		m.reportError("picker.import", msg.err)
		m.keyImport.err = "File picker failed. Enter a file path instead"
	}
	m.keyImport.warning = ""
	m.keyImport.status = ""
	return m, textinput.Blink
}

func (m *model) handleKeyExportPickerMsg(msg keyExportPickerMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.keyExportPickerID ||
		m.screen != screenDashboard ||
		m.session.Current().ID != RouteKeysExport ||
		!m.keyExport.pickerOpening {
		return m, nil
	}
	m.keyExport.pickerOpening = false
	if msg.err == nil && strings.TrimSpace(msg.path) != "" {
		m.keyExport.pathInput.SetValue(msg.path)
		m.keyExport.pathVisible = false
		m.keyExport.err = ""
		m.keyExport.status = "Exporting vault"
		m.keyExport.exporting = true
		return m, tea.Batch(m.spinner.Tick, m.exportVault())
	}
	m.keyExport.pathVisible = true
	m.keyExport.pathInput.Focus()
	switch {
	case errors.Is(msg.err, picker.ErrUnavailable):
		m.reportError("picker.export", msg.err)
		m.keyExport.err = "File picker unavailable. Enter an export path instead"
	case errors.Is(msg.err, picker.ErrCanceled), msg.err == nil:
		m.keyExport.err = ""
	default:
		m.reportError("picker.export", msg.err)
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
	m.keyBrowser.loaded = true
	m.keyBrowser.refreshing = false
	m.keyBrowser.err = ""
	m.keyBrowser.notice = strings.TrimSpace(notice)
	m.keyBrowser.refreshErr = ""
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

func (m *model) ensureKeyBrowserInput() {
	if m.keyBrowser.input.Cursor.BlinkSpeed != 0 {
		return
	}

	value := m.keyBrowser.input.Value()
	m.keyBrowser.input = newKeyInput("Search keys")
	if value != "" {
		m.keyBrowser.input.SetValue(value)
	}
	if m.keyBrowser.searchActive {
		m.keyBrowser.input.Focus()
	} else {
		m.keyBrowser.input.Blur()
	}
}

func (m *model) refreshKeyBrowserRows() {
	query := strings.TrimSpace(m.keyBrowser.input.Value())
	resolution := actions.ResolveKeyQuery(m.keyBrowser.all, query)
	m.keyBrowser.rows = resolution.Matches
	if query == "" {
		m.keyBrowser.rows = actions.ResolveKeyQuery(m.keyBrowser.all, "").Matches
	}
	m.resizeKeyBrowserSearchInput()

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
	m.keyBrowser.refreshErr = ""
	id := m.nextKeyListID()
	if sync {
		return tea.Batch(m.spinner.Tick, m.syncAndListKeys(id))
	}
	return tea.Batch(m.spinner.Tick, m.listKeys(id, true))
}

func (m *model) preloadKeyBrowser() tea.Cmd {
	if !m.snapshot.VaultExists || m.keyBrowser.loaded {
		return nil
	}
	return m.listLocalKeys(m.nextKeyListID(), true)
}

func (m *model) applyKeyBrowserRoute(route Route) {
	query := route.Params["query"]
	notice := route.Params["notice"]
	searchActive := route.Params["search"] == "true"
	if query == "" && notice == "" && !searchActive {
		m.ensureKeyBrowserInput()
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
	m.keyBrowser.loading = false
	m.keyBrowser.loaded = true
	m.keyBrowser.refreshErr = ""
	m.refreshKeyBrowserRows()
	m.selectKeyBrowserByName(preserveName)
}

func (m *model) keyBrowserNotice() string {
	if m.keyBrowser.refreshErr != "" {
		return m.keyBrowser.refreshErr
	}
	return m.keyBrowser.notice
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
	pageRows := m.keyBrowserPageRows()
	maxOffset := max(0, len(m.keyBrowser.rows)-pageRows)
	m.keyBrowser.offset = max(0, min(m.keyBrowser.offset, maxOffset))
	if m.keyBrowser.selected < m.keyBrowser.offset {
		m.keyBrowser.offset = m.keyBrowser.selected
	}
	if m.keyBrowser.selected >= m.keyBrowser.offset+pageRows {
		m.keyBrowser.offset = m.keyBrowser.selected - pageRows + 1
	}
}

func (m *model) keyBrowserVisibleRows() []actions.KeySummary {
	if len(m.keyBrowser.rows) == 0 {
		return nil
	}
	start := min(max(m.keyBrowser.offset, 0), len(m.keyBrowser.rows))
	end := min(len(m.keyBrowser.rows), start+m.keyBrowserPageRows())
	return m.keyBrowser.rows[start:end]
}

func (m *model) resizeKeyBrowserPage(bodyHeight int) {
	m.keyBrowser.pageRows = max(1, min(keyscreen.VisibleRows(), bodyHeight-5))
	m.ensureKeyBrowserVisible()
}

func (m *model) keyBrowserPageRows() int {
	if m.keyBrowser.pageRows <= 0 {
		return keyscreen.VisibleRows()
	}
	return m.keyBrowser.pageRows
}

func (m *model) keyBrowserCountLabel() string {
	return m.keyBrowserCountLabelForWidth(shell.BodyWidth(m.width))
}

func (m *model) keyBrowserCountLabelForWidth(width int) string {
	total := len(m.keyBrowser.all)
	if total == 0 {
		if m.keyBrowser.loading {
			return ""
		}
		return chooseKeyBrowserCountLabel(width, "0 keys", "0")
	}

	filtered := len(m.keyBrowser.rows)
	if strings.TrimSpace(m.keyBrowser.input.Value()) != "" {
		return chooseKeyBrowserCountLabel(
			width,
			fmt.Sprintf("%d of %d keys", filtered, total),
			fmt.Sprintf("%d/%d keys", filtered, total),
			fmt.Sprintf("%d/%d", filtered, total),
		)
	}
	if total == 1 {
		return chooseKeyBrowserCountLabel(width, "1 key", "1")
	}
	return chooseKeyBrowserCountLabel(width, fmt.Sprintf("%d keys", total), fmt.Sprintf("%d", total))
}

func (m *model) selectedKeyRow() (actions.KeySummary, bool) {
	if len(m.keyBrowser.rows) == 0 || m.keyBrowser.selected < 0 || m.keyBrowser.selected >= len(m.keyBrowser.rows) {
		return actions.KeySummary{}, false
	}
	return m.keyBrowser.rows[m.keyBrowser.selected], true
}

func (m *model) resizeKeyInputs() {
	m.resizeKeyBrowserSearchInput()
	m.keyRename.input.Width = max(18, min(shell.ClampBlockWidth(m.width, 44), 44))
	inputWidth := max(18, min(shell.ClampBlockWidth(m.width, 44), 44))
	m.keyGenerate.nameInput.Width = inputWidth
	m.keyImport.pathInput.Width = max(18, min(shell.ClampBlockWidth(m.width, 54), 54))
	m.keyExport.pathInput.Width = max(18, min(shell.ClampBlockWidth(m.width, 54), 54))
}

func (m *model) resizeKeyBrowserSearchInput() {
	rowWidth := shell.BodyWidth(m.width)
	searchWidth := max(1, rowWidth-keyBrowserSearchPrefixWidth-keyBrowserSearchCursorWidth)
	if countLabel := strings.TrimSpace(m.keyBrowserCountLabelForWidth(rowWidth)); countLabel != "" {
		searchWidth = max(1, rowWidth-lipgloss.Width(countLabel)-keyBrowserSearchPrefixWidth-keyBrowserSearchGapWidth-keyBrowserSearchCursorWidth)
	}
	m.keyBrowser.input.Width = searchWidth
}

func chooseKeyBrowserCountLabel(width int, variants ...string) string {
	maxCountWidth := max(0, width-keyBrowserSearchPrefixWidth-keyBrowserSearchGapWidth-keyBrowserSearchMinInput-keyBrowserSearchCursorWidth)
	for _, variant := range variants {
		if lipgloss.Width(variant) <= maxCountWidth {
			return variant
		}
	}
	return ""
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
	m.keyImport.step = keyImportStepSource
	m.keyImport.previews = nil
	m.keyImport.reviewCursor = 0
	m.keyImport.discovered = 0
	m.keyImport.duplicates = 0
	m.keyImport.err = ""
	m.keyImport.warning = ""
	m.keyImport.status = ""
	m.keyImport.success = nil
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

func exportSuccessMessage(result actions.ExportResult) string {
	if result.KeyCount == 1 {
		return fmt.Sprintf("1 key exported to %s", filepath.Base(result.Path))
	}
	return fmt.Sprintf("%d keys exported to %s", result.KeyCount, filepath.Base(result.Path))
}

func importSuccessMessage(result actions.ImportResult) string {
	if result.Imported == 1 {
		return "1 Key added to this vault"
	}
	return fmt.Sprintf("%d Keys added to this vault", result.Imported)
}

func importFailureResultSummaryLines(result actions.ImportResult) []string {
	lines := make([]string, 0, 2)
	if result.Imported > 0 {
		lines = append(lines, importSuccessMessage(result))
	} else {
		lines = append(lines, "No keys added to this vault")
	}
	if result.Skipped == 1 {
		lines = append(lines, "1 key failed to import")
	} else {
		lines = append(lines, fmt.Sprintf("%d keys failed to import", result.Skipped))
	}
	return lines
}

func (m *model) selectedKeyImportFailure() (actions.ImportFailure, bool) {
	failures := m.keyImport.result.Failures
	if len(failures) == 0 || m.keyImport.failureCursor < 0 || m.keyImport.failureCursor >= len(failures) {
		return actions.ImportFailure{}, false
	}
	return failures[m.keyImport.failureCursor], true
}

func importReviewContentWidth(width int) int {
	return max(28, min(width, theme.HeroMaxWidth+10))
}

func importFailureReasonPage(reason string, offset int, width int) (string, int) {
	const label = "Reason: "
	runes := []rune(strings.TrimSpace(reason))
	if len(runes) == 0 {
		return "", 0
	}
	if offset < 0 || offset >= len(runes) {
		offset = 0
	}

	available := max(1, width-lipgloss.Width(theme.Glyphs.Cross+" ")-lipgloss.Width(label))
	prefix := ""
	if offset > 0 {
		prefix = theme.Glyphs.Ellipsis + " "
	}
	used := lipgloss.Width(prefix)
	end := offset
	for end < len(runes) {
		runeWidth := lipgloss.Width(string(runes[end]))
		suffixWidth := 0
		if end+1 < len(runes) {
			suffixWidth = lipgloss.Width(" " + theme.Glyphs.Ellipsis)
		}
		if end > offset && used+runeWidth+suffixWidth > available {
			break
		}
		used += runeWidth
		end++
	}

	page := prefix + string(runes[offset:end])
	if end < len(runes) {
		page += " " + theme.Glyphs.Ellipsis
	}
	return label + page, end
}

func (m *model) keyImportFailurePageWidth() int {
	return importReviewContentWidth(shell.BodyWidth(m.width))
}

func (m *model) keyImportFailureHasMorePages() bool {
	failure, ok := m.selectedKeyImportFailure()
	if !ok {
		return false
	}
	_, end := importFailureReasonPage(failure.Reason, m.keyImport.failureOffset, m.keyImportFailurePageWidth())
	return m.keyImport.failureOffset > 0 || end < len([]rune(strings.TrimSpace(failure.Reason)))
}

func (m *model) keyImportFailureGuidance() string {
	guidance := "Use ↑/↓ to inspect each failed key."
	if m.keyImportFailureHasMorePages() {
		guidance += " Use ←/→ to read the selected reason."
	}
	return guidance + " Esc returns to dashboard."
}

func (m *model) moveKeyImportFailureCursor(delta int) {
	if len(m.keyImport.result.Failures) == 0 {
		m.keyImport.failureCursor = 0
		m.keyImport.failureOffset = 0
		return
	}
	next := m.keyImport.failureCursor + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.keyImport.result.Failures) {
		next = len(m.keyImport.result.Failures) - 1
	}
	m.keyImport.failureCursor = next
	m.keyImport.failureOffset = 0
}

func (m *model) moveKeyImportFailurePage(forward bool) {
	failure, ok := m.selectedKeyImportFailure()
	if !ok {
		return
	}
	width := m.keyImportFailurePageWidth()
	if forward {
		_, end := importFailureReasonPage(failure.Reason, m.keyImport.failureOffset, width)
		if end < len([]rune(strings.TrimSpace(failure.Reason))) {
			m.keyImport.failureOffset = end
		}
		return
	}
	if m.keyImport.failureOffset <= 0 {
		return
	}
	previous := 0
	for start := 0; start < m.keyImport.failureOffset; {
		_, end := importFailureReasonPage(failure.Reason, start, width)
		if end >= m.keyImport.failureOffset {
			previous = start
			break
		}
		previous = start
		start = end
	}
	m.keyImport.failureOffset = previous
}

func (m *model) logImportFailures(result actions.ImportResult) {
	limit := min(len(result.Failures), maxImportFailureLogReasons)
	reasons := make([]string, 0, limit+1)
	for _, failure := range result.Failures[:limit] {
		reasons = append(reasons, failure.Reason)
	}
	if result.Skipped > limit {
		reasons = append(reasons, fmt.Sprintf("%d additional failures", result.Skipped-limit))
	}
	m.deps.LogError(actions.DiagnosticErrorEvent{
		Route:   string(RouteKeysImport),
		Action:  "keys.import.partial",
		Version: m.deps.AppVersion,
		Message: fmt.Sprintf("%d import failures: %s", result.Skipped, strings.Join(reasons, "; ")),
	})
}

func (m *model) moveKeyImportReviewCursor(delta int) {
	if len(m.keyImport.previews) == 0 {
		m.keyImport.reviewCursor = 0
		return
	}
	next := m.keyImport.reviewCursor + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.keyImport.previews) {
		next = len(m.keyImport.previews) - 1
	}
	m.keyImport.reviewCursor = next
}

func (m *model) toggleCurrentImportReviewItem() {
	if len(m.keyImport.previews) == 0 || m.keyImport.reviewCursor < 0 || m.keyImport.reviewCursor >= len(m.keyImport.previews) {
		return
	}
	m.keyImport.previews[m.keyImport.reviewCursor].Selected = !m.keyImport.previews[m.keyImport.reviewCursor].Selected
	m.keyImport.err = ""
	m.keyImport.warning = ""
}

func (m *model) toggleAllImportReviewItems() {
	if len(m.keyImport.previews) == 0 {
		return
	}
	nextSelected := !m.allKeyImportReviewItemsSelected()
	for index := range m.keyImport.previews {
		m.keyImport.previews[index].Selected = nextSelected
	}
	m.keyImport.err = ""
	m.keyImport.warning = ""
}

func (m *model) allKeyImportReviewItemsSelected() bool {
	if len(m.keyImport.previews) == 0 {
		return false
	}
	for _, preview := range m.keyImport.previews {
		if !preview.Selected {
			return false
		}
	}
	return true
}

func (m *model) keyImportBulkToggleLabel() string {
	if m.allKeyImportReviewItemsSelected() {
		return "Unselect All"
	}
	return "Select All"
}

func (m *model) keyImportSelectedCount() int {
	count := 0
	for _, preview := range m.keyImport.previews {
		if preview.Selected {
			count++
		}
	}
	return count
}

func (m *model) keyImportSummaryLines() []string {
	upgradeCount := 0
	for _, preview := range m.keyImport.previews {
		if preview.Converted {
			upgradeCount++
		}
	}

	lines := make([]string, 0, 4)
	if upgradeCount > 0 {
		lines = append(lines, formatImportUpgradeSummary(upgradeCount))
	}
	lines = append(lines, formatImportReadySummary(m.keyImportSelectedCount()))
	return lines
}

func (m *model) keyImportGuidanceLine() string {
	return "Select keys you want to import."
}

func importReviewWindowBounds(total, cursor int) (int, int) {
	const window = 7
	if total <= window {
		return 0, total
	}
	start := cursor - window/2
	if start < 0 {
		start = 0
	}
	end := start + window
	if end > total {
		end = total
		start = end - window
	}
	return start, end
}

func importLoadingStatus(source string) string {
	if strings.TrimSpace(strings.ToLower(source)) == "ssh-dir" {
		return "Scanning ~/.ssh"
	}
	return "Reading data"
}

func importEmptyResultMessage(source string) string {
	if strings.TrimSpace(strings.ToLower(source)) == "ssh-dir" {
		return "No new SSH Keys found in ~/.ssh, Aborting"
	}
	return "No new SSH Key found in this file, Aborting"
}

func keyImportReviewSourceLabel(source string) string {
	switch strings.TrimSpace(strings.ToLower(source)) {
	case "1password":
		return "1Password import"
	case "bitwarden":
		return "Bitwarden import"
	case "forged":
		return "Forged export import"
	case "ssh-dir":
		return "SSH directory import"
	case "file":
		return "SSH key file import"
	default:
		return "Key import"
	}
}

func formatImportDuplicateWarning(count int) string {
	if count == 1 {
		return "1 duplicate key will not be imported"
	}
	return fmt.Sprintf("%d duplicate keys will not be imported", count)
}

func formatImportUpgradeSummary(count int) string {
	if count == 1 {
		return "1 key will be upgraded to OpenSSH"
	}
	return fmt.Sprintf("%d keys will be upgraded to OpenSSH", count)
}

func formatImportReadySummary(count int) string {
	if count == 1 {
		return "1 key ready to import"
	}
	return fmt.Sprintf("%d keys ready to import", count)
}

func (m *model) keyImportDuplicateWarning() string {
	if m.keyImport.duplicates <= 0 {
		return ""
	}
	return formatImportDuplicateWarning(m.keyImport.duplicates)
}

func (m *model) newKeyTransferSuccessState(title, message, detail string) *keyTransferSuccessState {
	m.keyTransferSuccessID++
	return &keyTransferSuccessState{
		Title:        title,
		Message:      message,
		Detail:       detail,
		autoReturnID: m.keyTransferSuccessID,
	}
}

func (m *model) scheduleKeyImportAutoReturn(id int) tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return keyImportAutoReturnMsg{id: id}
	})
}

func (m *model) returnToDashboardRoute() tea.Cmd {
	if m.session.Back() {
		return m.showCurrentRoute()
	}
	m.session.ReplaceCurrent(Route{ID: RouteDashboardHome})
	return m.showCurrentRoute()
}

func (m *model) handleKeyImportAutoReturnMsg(msg keyImportAutoReturnMsg) (tea.Model, tea.Cmd) {
	if m.keyImport.success == nil ||
		m.keyImport.success.autoReturnID != msg.id ||
		m.screen != screenDashboard ||
		m.session.Current().ID != RouteKeysImport {
		return m, nil
	}
	m.keyImport.success = nil
	return m, m.returnToDashboardRoute()
}

func footerImportLabel(selected int) string {
	if selected == 1 {
		return "Import 1 Key"
	}
	return fmt.Sprintf("Import %d Keys", selected)
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

func deleteWarning(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Deleting this key removes it from the vault and cannot be undone in Forged."
	}
	return fmt.Sprintf("Deleting %s removes this key from the vault and cannot be undone in Forged.", name)
}

func keyDeleteReviewValid(key actions.KeySummary) bool {
	return strings.TrimSpace(key.Name) != "" && strings.TrimSpace(key.Fingerprint) != ""
}

func deleteStatus(deleting bool) string {
	if deleting {
		return "Deleting key"
	}
	return ""
}
