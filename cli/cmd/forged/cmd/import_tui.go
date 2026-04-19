package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/commandui"
	"github.com/itzzritik/forged/cli/internal/importers"
	"github.com/spf13/cobra"
)

const importReviewWindowSize = 7

const importSpinnerInterval = 90 * time.Millisecond

var importSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type importStep string

const (
	importStepSource    importStep = "source-select"
	importStepFilePath  importStep = "file-path"
	importStepLoading   importStep = "loading"
	importStepReview    importStep = "review"
	importStepImporting importStep = "importing"
	importStepDone      importStep = "done"
)

type importSourceOption struct {
	ID        string
	Title     string
	Subtitle  string
	FileBased bool
}

type importPickerMsg struct {
	path string
	ok   bool
}

type importLoadedMsg struct {
	keys         []importers.ImportedKey
	sourceLabel  string
	emptyMessage string
	err          error
}

type importPreparedMsg struct {
	previews    []importPreview
	sourceLabel string
	err         error
}

type importFinishedMsg struct {
	result importExecutionResult
	err    error
}

type importSpinnerTickMsg struct{}

type importReviewItem struct {
	preview importPreview
	checked bool
}

type importModel struct {
	step         importStep
	width        int
	sourceCursor int
	reviewCursor int

	selected     importSourceOption
	sourceLabel  string
	filePath     string
	errorText    string
	loadingLabel string
	doneMessage  string
	spinnerFrame int
	returnToMenu bool

	fallbackStep importStep
	reviewBack   importStep
	review       []importReviewItem

	cancelled bool
}

func importSourceOptions() []importSourceOption {
	return []importSourceOption{
		{ID: "1password", Title: "1Password (.1pux, .csv)", Subtitle: "Import from 1Password export. .1pux is recommended.", FileBased: true},
		{ID: "bitwarden", Title: "Bitwarden (.json)", Subtitle: "Export from Bitwarden Settings > Export vault.", FileBased: true},
		{ID: "forged", Title: "Forged backup (.json)", Subtitle: "Import from a previous Forged export.", FileBased: true},
		{ID: "ssh-dir", Title: "SSH directory (~/.ssh/)", Subtitle: "Scan local SSH keys from your default SSH folder.", FileBased: false},
		{ID: "file", Title: "SSH key file", Subtitle: "Import a single private key file.", FileBased: true},
	}
}

func importSourceTitles() []string {
	options := importSourceOptions()
	titles := make([]string, len(options))
	for i, option := range options {
		titles[i] = option.Title
	}
	return titles
}

func newImportModel(from, file string, returnToMenu bool) *importModel {
	model := &importModel{step: importStepSource, filePath: file, returnToMenu: returnToMenu}
	if from == "" {
		return model
	}

	if option, index, ok := findImportSourceOption(from); ok {
		model.selected = option
		model.sourceCursor = index
	}

	return model
}

func (m *importModel) Init() tea.Cmd {
	if m.selected.ID == "" {
		return nil
	}
	if !m.selected.FileBased {
		return m.beginLoadKeys("", importStepSource)
	}
	if strings.TrimSpace(m.filePath) != "" {
		return m.beginLoadKeys(m.filePath, importStepFilePath)
	}
	return m.beginPicker()
}

func (m *importModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case importPickerMsg:
		if msg.ok {
			m.filePath = msg.path
			return m, m.beginLoadKeys(msg.path, importStepFilePath)
		}
		m.step = importStepFilePath
		m.errorText = ""
		return m, nil
	case importLoadedMsg:
		if msg.err != nil {
			m.step = m.fallbackStep
			m.errorText = msg.err.Error()
			return m, nil
		}
		if len(msg.keys) == 0 {
			m.step = m.fallbackStep
			m.errorText = msg.emptyMessage
			return m, nil
		}
		return m, m.beginPreview(msg.keys, msg.sourceLabel)
	case importPreparedMsg:
		if msg.err != nil {
			m.step = m.fallbackStep
			m.errorText = msg.err.Error()
			return m, nil
		}
		if len(msg.previews) == 0 {
			m.step = m.fallbackStep
			m.errorText = "No SSH keys found."
			return m, nil
		}
		m.review = buildImportReviewItems(msg.previews)
		m.reviewCursor = 0
		m.sourceLabel = msg.sourceLabel
		m.reviewBack = m.fallbackStep
		m.step = importStepReview
		m.errorText = ""
		return m, nil
	case importFinishedMsg:
		if msg.err != nil {
			m.step = importStepReview
			m.errorText = msg.err.Error()
			return m, nil
		}
		m.step = importStepDone
		m.doneMessage = formatImportDoneMessage(msg.result)
		return m, nil
	case importSpinnerTickMsg:
		if m.step != importStepLoading && m.step != importStepImporting {
			return m, nil
		}
		m.spinnerFrame = (m.spinnerFrame + 1) % len(importSpinnerFrames)
		return m, tickImportSpinner()
	case tea.KeyMsg:
		switch m.step {
		case importStepSource:
			return m.updateSourceStep(msg)
		case importStepFilePath:
			return m.updateFilePathStep(msg)
		case importStepLoading, importStepImporting:
			switch msg.String() {
			case "ctrl+c", "esc", "q":
				m.cancelled = true
				return m, tea.Quit
			}
		case importStepReview:
			return m.updateReviewStep(msg)
		case importStepDone:
			switch msg.String() {
			case "enter", "ctrl+c", "esc", "q":
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m *importModel) updateSourceStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	options := importSourceOptions()
	switch msg.String() {
	case "ctrl+c", "esc", "q":
		m.cancelled = true
		return m, tea.Quit
	case "up", "k":
		if m.sourceCursor > 0 {
			m.sourceCursor--
		}
	case "down", "j":
		if m.sourceCursor < len(options)-1 {
			m.sourceCursor++
		}
	case "enter":
		m.selected = options[m.sourceCursor]
		m.errorText = ""
		m.filePath = ""
		if !m.selected.FileBased {
			return m, m.beginLoadKeys("", importStepSource)
		}
		return m, m.beginPicker()
	}
	return m, nil
}

func (m *importModel) updateFilePathStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancelled = true
		return m, tea.Quit
	case "esc":
		m.step = importStepSource
		m.errorText = ""
		m.filePath = ""
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.filePath)
		if value == "" {
			m.errorText = "A file path is required."
			return m, nil
		}
		return m, m.beginLoadKeys(value, importStepFilePath)
	case "backspace":
		m.filePath = trimTrailingRune(m.filePath)
		if strings.TrimSpace(m.filePath) != "" {
			m.errorText = ""
		}
	default:
		key := msg.String()
		if len([]rune(key)) == 1 {
			m.filePath += key
			m.errorText = ""
		}
	}
	return m, nil
}

func (m *importModel) updateReviewStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancelled = true
		return m, tea.Quit
	case "esc":
		m.cancelled = true
		return m, tea.Quit
	case "up", "k":
		if m.reviewCursor > 0 {
			m.reviewCursor--
		}
	case "down", "j":
		if m.reviewCursor < len(m.review)-1 {
			m.reviewCursor++
		}
	case " ":
		m.toggleCurrentReviewItem()
	case "enter":
		if len(m.selectedKeys()) == 0 {
			return m, nil
		}
		return m, m.beginImport()
	}
	return m, nil
}

func (m *importModel) View() string {
	var content string
	switch m.step {
	case importStepFilePath:
		content = m.renderFilePath()
	case importStepLoading, importStepImporting:
		content = m.renderLoading()
	case importStepReview:
		content = m.renderReview()
	case importStepDone:
		content = m.renderDone()
	default:
		content = m.renderSource()
	}
	return commandui.RenderContainer(m.width, content)
}

func (m *importModel) renderSource() string {
	options := importSourceOptions()
	lines := []string{
		commandui.TitleStyle.Render("Import Keys"),
		"",
		commandui.MutedStyle.Render("Choose a source to continue."),
		"",
	}

	for i, option := range options {
		title := "  " + option.Title
		if i == m.sourceCursor {
			title = commandui.SelectedItemStyle.Render("› " + option.Title)
		}
		lines = append(lines, title)
		lines = append(lines, "  "+commandui.MutedStyle.Render(option.Subtitle))
		lines = append(lines, "")
	}

	if m.errorText != "" {
		lines = append(lines, commandui.ErrorStyle.Render(m.errorText), "")
	}
	lines = append(lines,
		commandui.RenderFooter(
			commandui.FooterAction("↑/↓", "Move"),
			commandui.FooterAction("Enter", "Select"),
			commandui.FooterAction("Esc", m.escapeActionTitle()),
		),
	)
	return strings.Join(lines, "\n")
}

func (m *importModel) renderFilePath() string {
	lines := []string{
		commandui.TitleStyle.Render("Import Keys"),
		"",
		commandui.MutedStyle.Render("Enter the file path to continue."),
		"",
		renderTextPromptValue(m.filePath, "File path"),
	}
	if m.errorText != "" {
		lines = append(lines, commandui.ErrorStyle.Render(m.errorText))
	}
	lines = append(lines, "",
		commandui.RenderFooter(
			commandui.FooterAction("Enter", "Continue"),
			commandui.FooterAction("Esc", "Back"),
		),
	)
	return strings.Join(lines, "\n")
}

func (m *importModel) renderLoading() string {
	return strings.Join([]string{
		commandui.TitleStyle.Render("Import Keys"),
		"",
		renderImportLoadingLine(m.spinnerFrame, m.loadingLabel),
		"",
		commandui.RenderFooter(
			commandui.FooterAction("Esc", m.escapeActionTitle()),
		),
	}, "\n")
}

func (m *importModel) renderReview() string {
	lines := []string{
		commandui.TitleStyle.Render("Import Keys"),
		"",
		commandui.MutedStyle.Render(m.sourceLabel),
		commandui.MutedStyle.Render(fmt.Sprintf("%d keys found", len(m.review))),
		"",
	}

	start, end := importReviewWindowBounds(len(m.review), m.reviewCursor)
	if start > 0 {
		lines = append(lines, commandui.MutedStyle.Render("..."), "")
	}
	for i := start; i < end; i++ {
		lines = append(lines, renderImportReviewRow(m.review[i], i == m.reviewCursor))
		lines = append(lines, "")
	}
	if end < len(m.review) {
		lines = append(lines, commandui.MutedStyle.Render("..."), "")
	}

	lines = append(lines, commandui.MutedStyle.Render("Summary"))
	for _, line := range buildImportSummaryLines(reviewPreviews(m.review), collapsedDuplicateCount(m.review)) {
		lines = append(lines, commandui.MutedStyle.Render(line))
	}
	if guidance := m.reviewGuidanceLine(); guidance != "" {
		lines = append(lines, "", commandui.MutedStyle.Render(guidance))
	}

	if m.errorText != "" {
		lines = append(lines, "", commandui.ErrorStyle.Render(m.errorText))
	}

	if footer := renderImportFooter(m.reviewFooterItems()); footer != "" {
		lines = append(lines, "", footer)
	}
	return strings.Join(lines, "\n")
}

func (m *importModel) renderDone() string {
	return strings.Join([]string{
		commandui.TitleStyle.Render("Import Keys"),
		"",
		commandui.SuccessStyle.Render(m.doneMessage),
		"",
		commandui.RenderFooter(
			commandui.FooterAction("Enter", "Close"),
			commandui.FooterAction("Esc", "Close"),
		),
	}, "\n")
}

func (m *importModel) beginPicker() tea.Cmd {
	m.loadingLabel = "Opening file picker"
	m.step = importStepLoading
	m.spinnerFrame = 0
	return tea.Batch(
		tickImportSpinner(),
		func() tea.Msg {
			path, ok := chooseFileWithPicker()
			return importPickerMsg{path: path, ok: ok}
		},
	)
}

func (m *importModel) beginLoadKeys(filePath string, fallback importStep) tea.Cmd {
	m.errorText = ""
	m.filePath = filePath
	m.fallbackStep = fallback
	m.loadingLabel = m.readingMessage()
	m.step = importStepLoading
	m.spinnerFrame = 0

	from := m.selected.ID
	emptyMessage := m.emptyResultMessage()
	return tea.Batch(
		tickImportSpinner(),
		func() tea.Msg {
			keys, sourceLabel, err := loadImportedKeys(from, filePath)
			if err != nil {
				return importLoadedMsg{err: err}
			}
			return importLoadedMsg{
				keys:         keys,
				sourceLabel:  sourceLabel,
				emptyMessage: emptyMessage,
			}
		},
	)
}

func (m *importModel) beginPreview(keys []importers.ImportedKey, sourceLabel string) tea.Cmd {
	m.errorText = ""
	m.loadingLabel = "Checking existing keys"
	m.step = importStepLoading
	m.spinnerFrame = 0
	return tea.Batch(
		tickImportSpinner(),
		func() tea.Msg {
			previews, err := buildImportPreview(keys)
			if err != nil {
				return importPreparedMsg{err: err}
			}
			return importPreparedMsg{previews: previews, sourceLabel: sourceLabel}
		},
	)
}

func (m *importModel) beginImport() tea.Cmd {
	selected := m.selectedKeys()
	m.errorText = ""
	m.loadingLabel = fmt.Sprintf("Importing %d keys", len(selected))
	m.step = importStepImporting
	m.spinnerFrame = 0
	return tea.Batch(
		tickImportSpinner(),
		func() tea.Msg {
			result, err := executeImport(selected)
			return importFinishedMsg{result: result, err: err}
		},
	)
}

func (m *importModel) readingMessage() string {
	if m.selected.ID == "ssh-dir" {
		return "Scanning ~/.ssh"
	}
	return "Reading data"
}

func (m *importModel) emptyResultMessage() string {
	if m.selected.ID == "ssh-dir" {
		return "No SSH keys found in ~/.ssh."
	}
	return "No SSH keys found in this file."
}

func (m *importModel) toggleCurrentReviewItem() {
	if len(m.review) == 0 || m.reviewCursor < 0 || m.reviewCursor >= len(m.review) {
		return
	}
	m.review[m.reviewCursor].checked = !m.review[m.reviewCursor].checked
	m.errorText = ""
}

func (m *importModel) toggleAllReviewItems() {
	if len(m.review) == 0 {
		return
	}
	if m.allReviewItemsAreDuplicates() {
		return
	}
	if !m.hasVaultDuplicates() {
		nextChecked := !m.allReviewItemsChecked()
		for i := range m.review {
			m.review[i].checked = nextChecked
		}
		m.errorText = ""
		return
	}

	everyChecked := m.allReviewItemsChecked()
	anyDuplicateSelected := m.anyDuplicateSelected()

	switch {
	case everyChecked:
		for i := range m.review {
			m.review[i].checked = false
		}
	case anyDuplicateSelected:
		for i := range m.review {
			m.review[i].checked = true
		}
	default:
		for i := range m.review {
			m.review[i].checked = !m.review[i].preview.alreadyInVault
		}
	}
	m.errorText = ""
}

func (m *importModel) bulkToggleLabel() string {
	if !m.hasVaultDuplicates() {
		if m.allReviewItemsChecked() {
			return "Deselect all"
		}
		return "Select all"
	}
	if m.allReviewItemsChecked() {
		return "Deselect all"
	}
	if m.anyDuplicateSelected() {
		return "Select all"
	}
	return "Select all unique"
}

func (m *importModel) importButtonLabel() string {
	selected := len(m.selectedKeys())
	total := len(m.review)
	switch {
	case selected == 0:
		return "Import Keys"
	case selected == total && total == 1:
		return "Import Key"
	case selected == total:
		return "Import All Keys"
	case selected == 1:
		return "Import Key"
	default:
		return fmt.Sprintf("Import %d Keys", selected)
	}
}

func (m *importModel) selectedKeys() []importers.ImportedKey {
	keys := make([]importers.ImportedKey, 0, len(m.review))
	for _, item := range m.review {
		if item.checked {
			keys = append(keys, item.preview.key)
		}
	}
	return keys
}

func (m *importModel) hasVaultDuplicates() bool {
	for _, item := range m.review {
		if item.preview.alreadyInVault {
			return true
		}
	}
	return false
}

func (m *importModel) allReviewItemsChecked() bool {
	if len(m.review) == 0 {
		return false
	}
	for _, item := range m.review {
		if !item.checked {
			return false
		}
	}
	return true
}

func (m *importModel) anyDuplicateSelected() bool {
	for _, item := range m.review {
		if item.preview.alreadyInVault && item.checked {
			return true
		}
	}
	return false
}

func (m *importModel) allReviewItemsAreDuplicates() bool {
	if len(m.review) == 0 {
		return false
	}
	for _, item := range m.review {
		if !item.preview.alreadyInVault {
			return false
		}
	}
	return true
}

func (m *importModel) reviewGuidanceLine() string {
	if m.allReviewItemsAreDuplicates() && len(m.selectedKeys()) == 0 {
		return "No new keys found. Press [Esc] to " + m.escapeActionLabel() + "."
	}
	return "Select keys you want to import."
}

func (m *importModel) reviewFooterItems() []string {
	selected := len(m.selectedKeys())
	items := []string{
		commandui.FooterAction("↑/↓", "Move"),
		commandui.FooterAction("Space", "Toggle"),
	}
	if selected > 0 {
		items = append(items, commandui.FooterAction("Enter", footerImportLabel(selected)))
	}
	items = append(items, commandui.FooterAction("Esc", m.escapeActionTitle()))
	return items
}

func runImportTUI(cmd *cobra.Command, from, file string, returnToMenu bool) error {
	final, err := tea.NewProgram(newImportModel(from, file, returnToMenu)).Run()
	if err != nil {
		return err
	}

	model, ok := final.(*importModel)
	if !ok || model.cancelled {
		return nil
	}

	return nil
}

func (m *importModel) escapeActionLabel() string {
	if m.returnToMenu {
		return "back"
	}
	return "exit"
}

func (m *importModel) escapeActionTitle() string {
	if m.returnToMenu {
		return "Back"
	}
	return "Exit"
}

func buildImportReviewItems(previews []importPreview) []importReviewItem {
	items := make([]importReviewItem, 0, len(previews))
	for _, preview := range previews {
		items = append(items, importReviewItem{
			preview: preview,
			checked: preview.selected,
		})
	}
	return items
}

func buildImportSummaryLines(previews []importPreview, collapsedDuplicates int) []string {
	duplicateCount := 0
	upgradeCount := 0
	for _, preview := range previews {
		if preview.alreadyInVault {
			duplicateCount++
		}
		if preview.converted {
			upgradeCount++
		}
	}

	var lines []string
	if duplicateCount > 0 {
		lines = append(lines, formatImportDuplicateSummary(duplicateCount))
	}
	if upgradeCount > 0 {
		lines = append(lines, formatImportUpgradeSummary(upgradeCount))
	}
	if duplicateCount > 0 {
		lines = append(lines, "Duplicates start unselected")
	}
	if collapsedDuplicates > 0 {
		lines = append(lines, formatImportMergedRowsSummary(collapsedDuplicates))
	}
	if len(lines) == 0 {
		lines = append(lines, formatImportReadySummary(len(previews)))
	}
	return lines
}

func renderImportReviewRow(item importReviewItem, active bool) string {
	prefix := " "
	if active {
		prefix = commandui.SelectedItemStyle.Render("›")
	}

	firstLine := fmt.Sprintf("%s %s %s", prefix, renderImportCheckbox(item.checked), item.preview.key.Name)

	lines := []string{
		firstLine,
		"    " + renderImportMetadataLine(item),
	}
	if item.preview.collapsedDuplicates > 0 {
		lines = append(lines, "    "+commandui.MutedStyle.Render(formatImportMergedRowsSummary(item.preview.collapsedDuplicates)))
	}
	return strings.Join(lines, "\n")
}

func renderImportCheckbox(checked bool) string {
	if checked {
		return "■"
	}
	return "□"
}

func renderImportMetadataLine(item importReviewItem) string {
	parts := []string{commandui.MutedStyle.Render(truncateImportFingerprint(item.preview.fingerprint))}
	if badges := renderImportBadges(item); badges != "" {
		parts = append(parts, badges)
	}
	return strings.Join(parts, commandui.MutedStyle.Render(" | "))
}

func renderImportBadges(item importReviewItem) string {
	var badges []string
	if item.preview.alreadyInVault {
		badges = append(badges, commandui.WarnStyle.Render("Duplicate"))
	}
	if item.preview.converted {
		badges = append(badges, commandui.AccentStyle.Render("Upgrade"))
	}
	return strings.Join(badges, commandui.MutedStyle.Render(" | "))
}

func importReviewWindowBounds(total, cursor int) (int, int) {
	if total <= importReviewWindowSize {
		return 0, total
	}
	start := cursor - importReviewWindowSize/2
	if start < 0 {
		start = 0
	}
	end := start + importReviewWindowSize
	if end > total {
		end = total
		start = end - importReviewWindowSize
	}
	return start, end
}

func reviewPreviews(items []importReviewItem) []importPreview {
	previews := make([]importPreview, 0, len(items))
	for _, item := range items {
		previews = append(previews, item.preview)
	}
	return previews
}

func collapsedDuplicateCount(items []importReviewItem) int {
	total := 0
	for _, item := range items {
		total += item.preview.collapsedDuplicates
	}
	return total
}

func formatImportDoneMessage(result importExecutionResult) string {
	switch {
	case result.Imported == 1 && result.Skipped == 0:
		return "Imported 1 key."
	case result.Skipped == 0:
		return fmt.Sprintf("Imported %d keys.", result.Imported)
	default:
		return fmt.Sprintf("Imported %d keys. Skipped %d.", result.Imported, result.Skipped)
	}
}

func renderImportLoadingLine(frame int, label string) string {
	if len(importSpinnerFrames) == 0 {
		return label
	}
	if frame < 0 {
		frame = 0
	}
	return commandui.AccentStyle.Render(importSpinnerFrames[frame%len(importSpinnerFrames)]) + " " + label
}

func renderImportFooter(items []string) string {
	return commandui.RenderFooter(items...)
}

func footerImportLabel(selected int) string {
	if selected == 1 {
		return "Import 1 Key"
	}
	return fmt.Sprintf("Import %d Keys", selected)
}

func tickImportSpinner() tea.Cmd {
	return tea.Tick(importSpinnerInterval, func(time.Time) tea.Msg {
		return importSpinnerTickMsg{}
	})
}

func pluralizeImportSummary(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func truncateImportFingerprint(value string) string {
	if len(value) <= 20 {
		return value
	}
	return value[:13] + "..." + value[len(value)-4:]
}

func formatImportDuplicateSummary(count int) string {
	if count == 1 {
		return "1 key is already in your vault"
	}
	return fmt.Sprintf("%d keys are already in your vault", count)
}

func formatImportUpgradeSummary(count int) string {
	if count == 1 {
		return "1 key will be upgraded to OpenSSH"
	}
	return fmt.Sprintf("%d keys will be upgraded to OpenSSH", count)
}

func formatImportMergedRowsSummary(count int) string {
	if count == 1 {
		return "1 repeated row was merged"
	}
	return fmt.Sprintf("%d repeated rows were merged", count)
}

func formatImportReadySummary(count int) string {
	if count == 1 {
		return "1 key ready to import"
	}
	return fmt.Sprintf("%d keys ready to import", count)
}

func findImportSourceOption(id string) (importSourceOption, int, bool) {
	options := importSourceOptions()
	for i, option := range options {
		if option.ID == id {
			return option, i, true
		}
	}
	return importSourceOption{}, 0, false
}
