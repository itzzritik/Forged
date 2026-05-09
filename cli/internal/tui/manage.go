package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/tui/components"
	accountscreen "github.com/itzzritik/forged/cli/internal/tui/screens/account"
	commonscreen "github.com/itzzritik/forged/cli/internal/tui/screens/common"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type manageItemID string

const (
	manageItemProfile           manageItemID = "profile"
	manageItemSignIn            manageItemID = "sign-in"
	manageItemSync              manageItemID = "sync"
	manageItemMasterInterval    manageItemID = "master-password-interval"
	manageItemChangePassword    manageItemID = "change-password"
	manageItemLogout            manageItemID = "logout"
	manageListMinHeight                      = 6
	manageIntervalListMinHeight              = 3
)

type manageItem struct {
	ID      manageItemID
	Label   string
	Summary string
}

type manageState struct {
	selected               int
	masterIntervalSelected int
	changePasswordID       int
	autoReturnID           int
	syncBusy               bool
	logoutBusy             bool
	logoutArmed            bool
	settingItem            manageItemID
	settingErr             string
	success                *manageSuccessState
}

type manageSuccessState struct {
	Title        string
	Message      string
	Detail       string
	autoReturnID int
}

type manageSyncFinishedMsg struct {
	err error
}

type manageChangePasswordFinishedMsg struct {
	id     int
	result actions.ChangePasswordResult
	err    error
}

type manageLogoutFinishedMsg struct {
	err error
}

type manageAutoReturnMsg struct {
	id int
}

type manageSecuritySavedMsg struct {
	item  manageItemID
	state SecurityState
	err   error
}

func (m *model) isManageHomeRoute() bool {
	return m.screen == screenDashboard && m.snapshot.VaultExists && m.session.Current().ID == RouteVaultHome
}

func (m *model) isManageSuccessRoute() bool {
	return m.screen == screenDashboard &&
		m.snapshot.VaultExists &&
		m.session.Current().ID == RouteVaultChangePassword &&
		m.manage.success != nil
}

func (m *model) isManageProfileRoute() bool {
	return m.screen == screenDashboard && m.snapshot.VaultExists && m.session.Current().ID == RouteAccountStatus
}

func (m *model) isManageMasterIntervalRoute() bool {
	return m.screen == screenDashboard && m.snapshot.VaultExists && m.session.Current().ID == RouteVaultMasterPasswordInterval
}

func (m *model) manageSuccessTitle() string {
	return "Change Master Password"
}

func (m *model) manageItems() []manageItem {
	items := make([]manageItem, 0, 5)
	if m.snapshot.LoggedIn {
		items = append(items, manageItem{
			ID:      manageItemProfile,
			Label:   "Profile",
			Summary: "View your Forged profile and account settings",
		})
	} else {
		items = append(items, manageItem{
			ID:      manageItemSignIn,
			Label:   "Log In",
			Summary: "Log in to enable your Forged profile and synced vault features",
		})
	}

	syncLabel := "Sync Now"
	if !m.snapshot.LoggedIn {
		syncLabel = "Enable Sync"
	}
	items = append(items, manageItem{
		ID:      manageItemSync,
		Label:   syncLabel,
		Summary: m.manageSyncSummary(),
	})

	items = append(items,
		manageItem{
			ID:      manageItemMasterInterval,
			Label:   "Master Password Interval",
			Summary: m.masterPasswordIntervalSummary(),
		},
	)
	items = append(items, manageItem{
		ID:      manageItemChangePassword,
		Label:   "Change Master Password",
		Summary: "Change the master password protecting this vault",
	})

	if m.snapshot.LoggedIn {
		items = append(items, manageItem{
			ID:      manageItemLogout,
			Label:   "Log Out",
			Summary: "Log out of your Forged account on this machine",
		})
	}

	return items
}

func (m *model) manageDashboardPages() []dashboardPage {
	items := m.manageItems()
	pages := make([]dashboardPage, 0, len(items))
	for _, item := range items {
		pages = append(pages, dashboardPage{
			Label:   m.manageLabel(item),
			Summary: m.manageSummaryText(item),
		})
	}
	return pages
}

func (m *model) normalizeManageSelection(items []manageItem) {
	if len(items) == 0 {
		m.manage.selected = 0
		return
	}
	if m.manage.selected < 0 {
		m.manage.selected = 0
	}
	if m.manage.selected >= len(items) {
		m.manage.selected = len(items) - 1
	}
}

func (m *model) selectedManageItem() (manageItem, bool) {
	items := m.manageItems()
	if len(items) == 0 {
		return manageItem{}, false
	}
	m.normalizeManageSelection(items)
	return items[m.manage.selected], true
}

func (m *model) renderManageBody(contentWidth int) string {
	items := m.manageItems()
	if len(items) == 0 {
		return ""
	}
	m.normalizeManageSelection(items)

	listItems := make([]components.SelectionListItem, 0, len(items))
	for index, item := range items {
		listItems = append(listItems, components.SelectionListItem{
			Label:    m.manageLabel(item),
			Selected: index == m.manage.selected,
		})
	}

	top := components.RenderSelectionList(listItems, contentWidth, manageListMinHeight)
	bottom := ""

	if item, ok := m.selectedManageItem(); ok {
		if summary := strings.TrimSpace(m.manageSummaryText(item)); summary != "" {
			style := theme.BodyMuted
			if item.ID == manageItemLogout && m.manage.logoutArmed {
				style = theme.Warning
			}
			if item.ID == m.manage.settingItem && strings.TrimSpace(m.manage.settingErr) != "" {
				style = theme.Warning
			}
			bottom = style.Width(max(24, min(contentWidth, theme.HeroMaxWidth))).Render(summary)
		}
	}

	if bottom == "" {
		return top
	}
	return shell.DockBottom(top+"\n", bottom)
}

func (m *model) renderManageProfileBody(contentWidth int) string {
	return accountscreen.RenderProfile(accountscreen.ProfileScreen{
		Name:  m.accountDisplayName(),
		Email: strings.TrimSpace(m.accountEmail),
	}, contentWidth)
}

func (m *model) renderManageMasterIntervalBody(contentWidth int) string {
	descriptionWidth := max(28, min(contentWidth, theme.HeroMaxWidth))
	description := theme.Body.Width(descriptionWidth).Render(
		"Choose how often Forged asks for your master password again on this device.",
	)

	options := masterPasswordIntervalOptions()
	selected := m.manage.masterIntervalSelected
	if selected < 0 {
		selected = 0
	}
	if selected >= len(options) {
		selected = len(options) - 1
	}

	items := make([]components.SelectionListItem, 0, len(options))
	for index, option := range options {
		items = append(items, components.SelectionListItem{
			Label:    option.Label,
			Selected: index == selected,
		})
	}

	top := strings.Join([]string{
		description,
		"",
		components.RenderSelectionList(items, contentWidth, manageIntervalListMinHeight),
	}, "\n")

	summary := theme.BodyMuted.Width(descriptionWidth).Render(masterPasswordIntervalOptionSummary(options[selected].Value))
	if errText := strings.TrimSpace(m.manage.settingErr); errText != "" {
		summary = theme.Warning.Width(descriptionWidth).Render(errText)
	}
	return shell.DockBottom(top+"\n", summary)
}

func (m *model) renderManageSuccessBody(contentWidth int) string {
	if m.manage.success == nil {
		return ""
	}
	return commonscreen.RenderSuccess(commonscreen.SuccessScreen{
		Title:   m.manage.success.Title,
		Message: m.manage.success.Message,
		Detail:  m.manage.success.Detail,
	}, contentWidth)
}

func (m *model) updateManageKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.manage.logoutBusy {
		return m, nil
	}
	items := m.manageItems()
	m.normalizeManageSelection(items)

	switch msg.String() {
	case "esc":
		m.notice = notice{}
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "up", "k":
		m.notice = notice{}
		m.manage.logoutArmed = false
		m.manage.settingItem = ""
		m.manage.settingErr = ""
		if m.manage.selected > 0 {
			m.manage.selected--
		}
		return m, nil
	case "down", "j":
		m.notice = notice{}
		m.manage.logoutArmed = false
		m.manage.settingItem = ""
		m.manage.settingErr = ""
		if len(items) > 0 && m.manage.selected < len(items)-1 {
			m.manage.selected++
		}
		return m, nil
	case "enter":
		m.notice = notice{}
		item, ok := m.selectedManageItem()
		if !ok {
			return m, nil
		}
		return m.openManageItem(item)
	}

	return m, nil
}

func (m *model) updateManageMasterIntervalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	options := masterPasswordIntervalOptions()
	if len(options) == 0 {
		return m, nil
	}
	if m.manage.masterIntervalSelected < 0 {
		m.manage.masterIntervalSelected = 0
	}
	if m.manage.masterIntervalSelected >= len(options) {
		m.manage.masterIntervalSelected = len(options) - 1
	}

	switch msg.String() {
	case "esc":
		m.manage.settingItem = ""
		m.manage.settingErr = ""
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "up", "k":
		m.manage.settingItem = ""
		m.manage.settingErr = ""
		if m.manage.masterIntervalSelected > 0 {
			m.manage.masterIntervalSelected--
		}
		return m, nil
	case "down", "j":
		m.manage.settingItem = ""
		m.manage.settingErr = ""
		if m.manage.masterIntervalSelected < len(options)-1 {
			m.manage.masterIntervalSelected++
		}
		return m, nil
	case "enter":
		m.manage.settingItem = ""
		m.manage.settingErr = ""
		selected := options[m.manage.masterIntervalSelected]
		return m, m.saveManageSecuritySettingCmd(manageItemMasterInterval, selected.Value)
	}

	return m, nil
}

func (m *model) openManageItem(item manageItem) (tea.Model, tea.Cmd) {
	if m.manage.syncBusy || m.manage.logoutBusy {
		return m, nil
	}

	switch item.ID {
	case manageItemProfile:
		m.manage.logoutArmed = false
		m.loadStoredAccountIdentity()
		if m.session.Current().ID != RouteAccountStatus {
			m.session.Push(Route{ID: RouteAccountStatus})
		}
		return m, m.showCurrentRoute()
	case manageItemSignIn:
		m.manage.logoutArmed = false
		if m.session.Current().ID != RouteAccountLogin {
			m.session.Push(Route{ID: RouteAccountLogin})
		}
		return m, m.startLoginFlow()
	case manageItemSync:
		m.manage.logoutArmed = false
		if !m.snapshot.LoggedIn {
			if m.session.Current().ID != RouteSyncHome {
				m.session.Push(Route{ID: RouteSyncHome})
			}
			return m, m.showCurrentRoute()
		}
		return m, m.runManageSync()
	case manageItemMasterInterval:
		m.manage.logoutArmed = false
		m.manage.settingItem = ""
		m.manage.settingErr = ""
		m.manage.masterIntervalSelected = m.currentMasterPasswordIntervalIndex()
		if m.session.Current().ID != RouteVaultMasterPasswordInterval {
			m.session.Push(Route{ID: RouteVaultMasterPasswordInterval})
		}
		return m, m.showCurrentRoute()
	case manageItemChangePassword:
		m.manage.logoutArmed = false
		if m.session.Current().ID != RouteVaultChangePassword {
			m.session.Push(Route{ID: RouteVaultChangePassword})
		}
		return m, m.showCurrentRoute()
	case manageItemLogout:
		if !m.snapshot.LoggedIn {
			m.manage.logoutArmed = false
			return m, nil
		}
		if !m.manage.logoutArmed {
			m.manage.logoutArmed = true
			return m, nil
		}
		m.manage.logoutArmed = false
		m.manage.logoutBusy = true
		m.manage.settingItem = ""
		m.manage.settingErr = ""
		return m, tea.Batch(
			m.spinner.Tick,
			m.runManageLogout(),
		)
	default:
		return m, nil
	}
}

func (m *model) runManageSync() tea.Cmd {
	triggerSync := m.triggerSync
	m.manage.syncBusy = true
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			return manageSyncFinishedMsg{err: triggerSync()}
		},
	)
}

func (m *model) runManageLogout() tea.Cmd {
	paths := config.DefaultPaths()
	return func() tea.Msg {
		return manageLogoutFinishedMsg{err: actions.ClearCredentials(paths)}
	}
}

func (m *model) changePasswordCmd(id int, currentPassword []byte, newPassword []byte) tea.Cmd {
	changePassword := m.changePassword
	currentCopy := append([]byte(nil), currentPassword...)
	newCopy := append([]byte(nil), newPassword...)
	return func() tea.Msg {
		result, err := changePassword(currentCopy, newCopy)
		return manageChangePasswordFinishedMsg{id: id, result: result, err: err}
	}
}

func (m *model) submitManageChangePassword() tea.Cmd {
	currentPassword, newPassword, err := m.passwordInput.SubmitChangePassword()
	if err != nil {
		m.passwordInput.SetError(err.Error())
		return nil
	}

	m.passwordBusy = true
	m.passwordHideInput = false
	m.passwordBusyMessage = ""
	m.passwordInput.SetInfo("Changing master password")
	m.manage.changePasswordID++
	return tea.Batch(
		m.spinner.Tick,
		m.changePasswordCmd(m.manage.changePasswordID, currentPassword, newPassword),
	)
}

func (m *model) newManageSuccessState(title string, message string, detail string) *manageSuccessState {
	m.manage.autoReturnID++
	return &manageSuccessState{
		Title:        title,
		Message:      message,
		Detail:       detail,
		autoReturnID: m.manage.autoReturnID,
	}
}

func (m *model) scheduleManageAutoReturn(id int) tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return manageAutoReturnMsg{id: id}
	})
}

func (m *model) returnFromManageFlow() tea.Cmd {
	m.screen = screenDashboard
	m.passwordAuth = ""
	m.passwordBusy = false
	m.passwordHideInput = false
	m.passwordBusyMessage = ""

	if m.session.Back() {
		return m.showCurrentRoute()
	}

	m.session.ReplaceCurrent(Route{ID: RouteVaultHome})
	return m.showCurrentRoute()
}

func (m *model) loadStoredAccountIdentity() {
	creds, err := actions.LoadCredentials(config.DefaultPaths())
	if err != nil {
		return
	}
	m.accountEmail = strings.TrimSpace(creds.Email)
	m.accountName = strings.TrimSpace(creds.Name)
}

func (m *model) accountDisplayName() string {
	if name := strings.TrimSpace(m.accountName); name != "" {
		return name
	}
	if email := strings.TrimSpace(m.accountEmail); email != "" {
		return fallbackAccountNameFromEmail(email)
	}
	return ""
}

func (m *model) manageSummaryText(item manageItem) string {
	if item.ID == manageItemLogout && m.manage.logoutBusy {
		return "Logging out of your Forged account on this machine"
	}
	if item.ID == manageItemLogout && m.manage.logoutArmed {
		return "Are you sure you want to log out? Press Enter again to continue"
	}
	if item.ID == m.manage.settingItem && strings.TrimSpace(m.manage.settingErr) != "" {
		return m.manage.settingErr
	}
	return item.Summary
}

func (m *model) manageLabel(item manageItem) string {
	if item.ID == manageItemLogout && m.manage.logoutBusy {
		return m.spinner.View() + " Logging out"
	}
	if item.ID == manageItemLogout && m.manage.logoutArmed {
		return "Are you sure?"
	}
	return item.Label
}

func fallbackAccountNameFromEmail(email string) string {
	local := strings.TrimSpace(email)
	if local == "" {
		return ""
	}
	if at := strings.Index(local, "@"); at > 0 {
		local = local[:at]
	}
	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	local = strings.ReplaceAll(local, "-", " ")
	words := strings.Fields(local)
	if len(words) == 0 {
		return ""
	}
	for index, word := range words {
		runes := []rune(strings.ToLower(word))
		if len(runes) == 0 {
			continue
		}
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		words[index] = string(runes)
	}
	return strings.Join(words, " ")
}

func (m *model) handleManageSyncFinishedMsg(msg manageSyncFinishedMsg) (tea.Model, tea.Cmd) {
	m.manage.syncBusy = false
	return m, m.pollRuntimeStatus(0)
}

func (m *model) handleManageLogoutFinishedMsg(msg manageLogoutFinishedMsg) (tea.Model, tea.Cmd) {
	m.manage.logoutBusy = false
	if msg.err != nil {
		m.manage.settingItem = manageItemLogout
		m.manage.settingErr = msg.err.Error()
		return m, nil
	}

	m.snapshot.LoggedIn = false
	m.accountName = ""
	m.accountEmail = ""
	m.manage.syncBusy = false
	m.manage.logoutArmed = false
	m.manage.settingItem = ""
	m.manage.settingErr = ""
	m.runtimeStatus.Linked = false
	m.runtimeStatus.Syncing = false
	m.runtimeStatus.Error = ""
	m.runtimeStatus.LastSuccessfulPullAt = time.Time{}
	m.runtimeStatus.LastSuccessfulPushAt = time.Time{}
	return m, tea.Batch(m.refreshSnapshotCmd(), m.pollRuntimeStatus(0))
}

func (m *model) handleManageChangePasswordFinishedMsg(msg manageChangePasswordFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.manage.changePasswordID || m.passwordFlow != passwordManageChange {
		return m, nil
	}

	m.passwordBusy = false
	m.passwordBusyMessage = ""
	m.passwordHideInput = false
	if msg.err != nil {
		m.passwordInput.SetError(msg.err.Error())
		return m, nil
	}

	detail := strings.TrimSpace(msg.result.Detail)
	if detail != "" {
		detail += " Returning to Manage..."
	} else {
		detail = "Returning to Manage..."
	}

	m.screen = screenDashboard
	m.passwordAuth = ""
	m.manage.success = m.newManageSuccessState(
		"Password Updated",
		"Master password changed successfully.",
		detail,
	)
	return m, m.scheduleManageAutoReturn(m.manage.success.autoReturnID)
}

func (m *model) handleManageAutoReturnMsg(msg manageAutoReturnMsg) (tea.Model, tea.Cmd) {
	if m.manage.success == nil || m.manage.success.autoReturnID != msg.id || m.session.Current().ID != RouteVaultChangePassword {
		return m, nil
	}
	m.manage.success = nil
	return m, m.returnFromManageFlow()
}

func (m *model) manageSyncSummary() string {
	if !m.snapshot.LoggedIn {
		return "Keep your encrypted vault in sync across the devices you trust"
	}
	if !m.runtimeLoaded {
		return "Loading sync state"
	}
	if !m.runtimeStatus.Linked {
		return "Sync is not linked on this machine yet"
	}
	if m.manage.syncBusy || m.runtimeStatus.Syncing {
		return m.spinner.View() + " Syncing vault"
	}
	if errText := strings.TrimSpace(m.runtimeStatus.Error); errText != "" {
		return errText
	}
	if syncedAt := latestSyncTime(m.runtimeStatus); !syncedAt.IsZero() {
		return "Last synced " + syncedAt.In(time.Local).Format("02 Jan 2006, 3:04 PM MST")
	}
	return "Not yet synced on this machine"
}

func latestSyncTime(status RuntimeStatus) time.Time {
	switch {
	case status.LastSuccessfulPushAt.After(status.LastSuccessfulPullAt):
		return status.LastSuccessfulPushAt
	default:
		return status.LastSuccessfulPullAt
	}
}

func (m *model) masterPasswordIntervalSummary() string {
	if !m.securityLoaded {
		return "Loading security settings"
	}
	return fmt.Sprintf("Ask for your master password again every %s on this device.", formatMasterPasswordInterval(m.currentMasterPasswordInterval()))
}

func (m *model) currentMasterPasswordInterval() string {
	return config.NormalizeMasterPasswordInterval(m.securityState.MasterPasswordInterval)
}

func formatMasterPasswordInterval(value string) string {
	switch value {
	case config.MasterPasswordInterval15Days:
		return "15 days"
	case config.MasterPasswordInterval30Days:
		return "1 month"
	default:
		return "7 days"
	}
}

type masterPasswordIntervalOption struct {
	Value string
	Label string
}

func masterPasswordIntervalOptions() []masterPasswordIntervalOption {
	return []masterPasswordIntervalOption{
		{Value: config.MasterPasswordInterval7Days, Label: "7 days"},
		{Value: config.MasterPasswordInterval15Days, Label: "15 days"},
		{Value: config.MasterPasswordInterval30Days, Label: "1 month"},
	}
}

func masterPasswordIntervalOptionSummary(value string) string {
	return "Current setting: " + formatMasterPasswordInterval(value)
}

func (m *model) currentMasterPasswordIntervalIndex() int {
	current := m.currentMasterPasswordInterval()
	options := masterPasswordIntervalOptions()
	for index, option := range options {
		if option.Value == current {
			return index
		}
	}
	return 0
}

func (m *model) saveManageSecuritySettingCmd(item manageItemID, value string) tea.Cmd {
	setInterval := m.setMasterPasswordInterval
	loadSecurity := m.loadSecurityState
	return func() tea.Msg {
		var err error
		switch item {
		case manageItemMasterInterval:
			err = setInterval(value)
		default:
			return manageSecuritySavedMsg{item: item, err: fmt.Errorf("Unknown security setting")}
		}
		if err != nil {
			return manageSecuritySavedMsg{item: item, err: err}
		}
		state, err := loadSecurity()
		return manageSecuritySavedMsg{item: item, state: state, err: err}
	}
}

func (m *model) handleManageSecuritySavedMsg(msg manageSecuritySavedMsg) (tea.Model, tea.Cmd) {
	m.manage.settingItem = msg.item
	if msg.err != nil {
		m.manage.settingErr = msg.err.Error()
		return m, nil
	}
	m.manage.settingErr = ""
	m.securityState = msg.state
	m.securityLoaded = true
	if msg.item == manageItemMasterInterval {
		m.manage.masterIntervalSelected = m.currentMasterPasswordIntervalIndex()
		if m.session.Current().ID == RouteVaultMasterPasswordInterval && m.session.Back() {
			return m, m.showCurrentRoute()
		}
	}
	return m, nil
}
