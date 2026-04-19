package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/components"
	commonscreen "github.com/itzzritik/forged/cli/internal/tui/screens/common"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type manageItemID string

const (
	manageItemProfile        manageItemID = "profile"
	manageItemSignIn         manageItemID = "sign-in"
	manageItemVaultLock      manageItemID = "vault-lock"
	manageItemVaultUnlock    manageItemID = "vault-unlock"
	manageItemChangePassword manageItemID = "change-password"
	manageItemLogout         manageItemID = "logout"
	manageListMinHeight                   = 6
)

type manageItem struct {
	ID      manageItemID
	Label   string
	Summary string
}

type manageState struct {
	selected         int
	changePasswordID int
	autoReturnID     int
	success          *manageSuccessState
}

type manageSuccessState struct {
	Title        string
	Message      string
	Detail       string
	autoReturnID int
}

type manageLockFinishedMsg struct {
	err error
}

type manageUnlockFinishedMsg struct {
	result actions.UnlockResult
	err    error
}

type manageChangePasswordFinishedMsg struct {
	id     int
	result actions.ChangePasswordResult
	err    error
}

type manageAutoReturnMsg struct {
	id int
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

func (m *model) manageSuccessTitle() string {
	return "Change Master Password"
}

func (m *model) manageItems() []manageItem {
	items := []manageItem{
		{
			ID:      manageItemSignIn,
			Label:   "Sign In",
			Summary: "Sign in to enable your Forged profile and synced vault features",
		},
		{
			ID:      manageItemVaultUnlock,
			Label:   "Vault Unlock",
			Summary: "Unlock this vault with Touch ID or your master password",
		},
		{
			ID:      manageItemChangePassword,
			Label:   "Change Master Password",
			Summary: "Change the master password protecting this vault",
		},
	}

	if m.snapshot.LoggedIn {
		items[0] = manageItem{
			ID:      manageItemProfile,
			Label:   "Profile",
			Summary: "View your Forged profile and account settings",
		}
		items = append(items, manageItem{
			ID:      manageItemLogout,
			Label:   "Logout",
			Summary: "Log out of your Forged account on this machine",
		})
	}

	if m.runtimeStatus.Unlocked {
		items[1] = manageItem{
			ID:      manageItemVaultLock,
			Label:   "Vault Lock",
			Summary: "Lock this vault until your password is entered again",
		}
	}

	return items
}

func (m *model) manageDashboardPages() []dashboardPage {
	items := m.manageItems()
	pages := make([]dashboardPage, 0, len(items))
	for _, item := range items {
		pages = append(pages, dashboardPage{
			Label:   item.Label,
			Summary: item.Summary,
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
			Label:    item.Label,
			Selected: index == m.manage.selected,
		})
	}

	sections := make([]string, 0, 3)
	sections = append(sections, components.RenderSelectionList(listItems, contentWidth, manageListMinHeight))

	if item, ok := m.selectedManageItem(); ok && strings.TrimSpace(item.Summary) != "" {
		sections = append(sections, "", theme.BodyMuted.Width(max(24, min(contentWidth, theme.HeroMaxWidth))).Render(item.Summary))
	}

	return strings.Join(sections, "\n")
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
		if m.manage.selected > 0 {
			m.manage.selected--
		}
		return m, nil
	case "down", "j":
		m.notice = notice{}
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

func (m *model) openManageItem(item manageItem) (tea.Model, tea.Cmd) {
	switch item.ID {
	case manageItemVaultLock:
		return m, m.runManageLock()
	case manageItemVaultUnlock:
		if m.session.Current().ID != RouteVaultUnlock {
			m.session.Push(Route{ID: RouteVaultUnlock})
		}
		return m, m.showCurrentRoute()
	case manageItemChangePassword:
		if m.session.Current().ID != RouteVaultChangePassword {
			m.session.Push(Route{ID: RouteVaultChangePassword})
		}
		return m, m.showCurrentRoute()
	default:
		return m, nil
	}
}

func (m *model) runManageLock() tea.Cmd {
	lock := m.lockSensitive
	return func() tea.Msg {
		return manageLockFinishedMsg{err: lock()}
	}
}

func (m *model) unlockSensitiveCmd(password []byte) tea.Cmd {
	unlock := m.unlockSensitive
	passwordCopy := append([]byte(nil), password...)
	return func() tea.Msg {
		result, err := unlock(passwordCopy)
		return manageUnlockFinishedMsg{result: result, err: err}
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

func (m *model) startManageUnlockFlow() tea.Cmd {
	m.showPasswordScreenOnRoute(RouteVaultUnlock, passwordManageUnlock, "", "", false)
	m.passwordBusy = true
	m.passwordHideInput = true
	m.passwordBusyMessage = "Waiting for Touch ID"
	return tea.Batch(m.spinner.Tick, m.unlockSensitiveCmd(nil))
}

func (m *model) submitManageUnlock(password []byte) tea.Cmd {
	m.passwordBusy = true
	m.passwordHideInput = false
	m.passwordBusyMessage = ""
	m.passwordInput.SetInfo("Unlocking vault")
	return tea.Batch(m.spinner.Tick, m.unlockSensitiveCmd(password))
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

func (m *model) handleManageLockFinishedMsg(msg manageLockFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, nil
	}

	m.runtimeStatus.Unlocked = false
	m.runtimeStatus.SensitiveKnown = true
	return m, m.pollRuntimeStatus(0)
}

func (m *model) handleManageUnlockFinishedMsg(msg manageUnlockFinishedMsg) (tea.Model, tea.Cmd) {
	if m.passwordFlow != passwordManageUnlock {
		return m, nil
	}

	m.passwordBusy = false
	m.passwordBusyMessage = ""
	if msg.err != nil {
		m.passwordHideInput = false
		m.passwordInput.SetError(msg.err.Error())
		return m, nil
	}

	if msg.result.PasswordRequired {
		m.passwordHideInput = false
		m.passwordContext = "Touch ID unavailable. Enter your master password to unlock this vault."
		m.passwordInput.ClearStatus()
		return m, m.passwordInput.Init()
	}

	m.runtimeStatus.Unlocked = true
	m.runtimeStatus.SensitiveKnown = true
	m.passwordHideInput = false
	return m, tea.Batch(
		m.returnFromManageFlow(),
		m.pollRuntimeStatus(0),
	)
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
