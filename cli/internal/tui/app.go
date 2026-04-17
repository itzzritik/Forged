package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui/components"
	accountscreen "github.com/itzzritik/forged/cli/internal/tui/screens/account"
	dashboardscreen "github.com/itzzritik/forged/cli/internal/tui/screens/dashboard"
	repairscreen "github.com/itzzritik/forged/cli/internal/tui/screens/repair"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type ExitAction string

const ExitNone ExitAction = ""

type Result struct {
	Action ExitAction
}

type Dependencies struct {
	Repair          func(readiness.RunOptions) (readiness.RunResult, error)
	CreateVault     func([]byte) error
	StartLogin      func(string) (actions.LoginSession, error)
	SaveCredentials func(actions.AccountCredentials) error
	CopyText        func(string) error
	OpenLink        func(string) error
	DefaultServer   string
	AppVersion      string
	CommitSigning   bool
}

type screenMode string

const (
	screenDashboard screenMode = "dashboard"
	screenLogin     screenMode = "login"
	screenPassword  screenMode = "password"
	screenRepair    screenMode = "repair"
)

type passwordFlow string

const (
	passwordCreate  passwordFlow = "create"
	passwordRestore passwordFlow = "restore"
	passwordRepair  passwordFlow = "repair"
)

type repairPurpose string

const (
	repairPurposeStartup   repairPurpose = "startup"
	repairPurposeSetup     repairPurpose = "setup"
	repairPurposeUnlock    repairPurpose = "unlock"
	repairPurposePostLogin repairPurpose = "post-login"
)

type notice struct {
	message string
	tone    dashboardscreen.Tone
}

type assessmentMsg struct {
	snapshot readiness.Snapshot
	err      error
}

type loginStartedMsg struct {
	id      int
	session actions.LoginSession
	err     error
}

type loginFinishedMsg struct {
	id       int
	creds    actions.AccountCredentials
	err      error
	canceled bool
}

type repairProgressMsg struct {
	id    int
	stage readiness.ProgressStage
}

type repairFinishedMsg struct {
	id     int
	result readiness.RunResult
	err    error
}

type copyFinishedMsg struct {
	err error
}

type openFinishedMsg struct {
	err error
}

type model struct {
	intent          Intent
	session         *Session
	repair          func(readiness.RunOptions) (readiness.RunResult, error)
	createVault     func([]byte) error
	startLogin      func(string) (actions.LoginSession, error)
	saveCredentials func(actions.AccountCredentials) error
	copyText        func(string) error
	openLink        func(string) error
	defaultServer   string
	appVersion      string
	commitSigning   bool

	spinner spinner.Model
	width   int
	result  Result

	screen   screenMode
	fatalErr error

	snapshot readiness.Snapshot
	summary  readiness.RepairSummary
	notice   notice

	onboardingCursor int
	accountEmail     string
	loginScreen      accountscreen.LoginScreen
	passwordInput    *components.PasswordInput
	passwordFlow     passwordFlow
	passwordTitle    string
	passwordContext  string
	passwordAuth     string
	repairScreen     repairscreen.TaskScreen

	loginID     int
	loginCancel context.CancelFunc

	repairID           int
	repairProgress     <-chan readiness.ProgressStage
	repairPurpose      repairPurpose
	repairUsedPassword bool
	repairAuthEmail    string
}

func Run(intent Intent, deps Dependencies) (Result, error) {
	switch {
	case deps.Repair == nil:
		return Result{}, fmt.Errorf("tui repair dependency is required")
	case deps.CreateVault == nil:
		return Result{}, fmt.Errorf("tui create-vault dependency is required")
	case deps.StartLogin == nil:
		return Result{}, fmt.Errorf("tui login dependency is required")
	case deps.SaveCredentials == nil:
		return Result{}, fmt.Errorf("tui save-credentials dependency is required")
	case deps.CopyText == nil:
		return Result{}, fmt.Errorf("tui copy-text dependency is required")
	case deps.OpenLink == nil:
		return Result{}, fmt.Errorf("tui open-link dependency is required")
	}

	final, err := tea.NewProgram(newModel(intent, deps, components.NewSpinner())).Run()
	if err != nil {
		return Result{}, err
	}

	rendered, ok := final.(*model)
	if !ok {
		return Result{}, fmt.Errorf("unexpected tui model type %T", final)
	}
	if rendered.fatalErr != nil {
		return Result{}, rendered.fatalErr
	}

	return rendered.result, nil
}

func newModel(intent Intent, deps Dependencies, spin spinner.Model) *model {
	return &model{
		intent:          intent,
		session:         NewSession(intent),
		repair:          deps.Repair,
		createVault:     deps.CreateVault,
		startLogin:      deps.StartLogin,
		saveCredentials: deps.SaveCredentials,
		copyText:        deps.CopyText,
		openLink:        deps.OpenLink,
		defaultServer:   deps.DefaultServer,
		appVersion:      deps.AppVersion,
		commitSigning:   deps.CommitSigning,
		spinner:         spin,
	}
}

func (m *model) Init() tea.Cmd {
	switch m.intent.Entry {
	case RouteAccountLogin:
		m.screen = screenLogin
		m.loginScreen = accountscreen.LoginScreen{
			Title:   "Sign In to Sync Vault",
			Context: "Preparing secure browser approval.",
			Status:  "Checking local health",
			Waiting: true,
		}
		return tea.Batch(m.spinner.Tick, m.assessCurrentState())
	default:
		m.screen = screenRepair
		m.repairScreen = repairscreen.TaskScreen{
			Title:   "Checking local health",
			Context: "Reviewing the current state and applying safe fixes where needed.",
			Tasks:   m.newRepairTasks(""),
		}
		return tea.Batch(m.spinner.Tick, m.assessCurrentState())
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		if m.passwordInput != nil {
			m.passwordInput.SetWidth(max(18, shell.ClampBlockWidth(m.width, 40)-4))
		}
		return m, nil
	case spinner.TickMsg:
		if !m.usesSpinner() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case assessmentMsg:
		if msg.err != nil {
			if m.screen == screenLogin {
				m.loginScreen.Waiting = false
				m.loginScreen.Error = msg.err.Error()
				m.loginScreen.Status = ""
				return m, nil
			}
			m.repairScreen.Error = msg.err.Error()
			return m, nil
		}
		m.snapshot = msg.snapshot
		if m.screen == screenLogin {
			return m, m.startLoginFlow()
		}
		if !msg.snapshot.VaultExists {
			m.notice = notice{}
			m.summary = readiness.RepairSummary{}
			m.repairAuthEmail = ""
			m.repairUsedPassword = false
			m.screen = screenDashboard
			return m, nil
		}
		return m, m.startStartupRepair()
	case loginStartedMsg:
		if msg.id != m.loginID {
			return m, nil
		}
		if msg.err != nil {
			m.loginScreen.Waiting = false
			m.loginScreen.Error = msg.err.Error()
			m.loginScreen.Status = ""
			return m, nil
		}
		m.loginScreen = accountscreen.LoginScreen{
			Title:            "Sign In to Sync Vault",
			Context:          "Verify this code in your browser before approving.",
			Status:           "Waiting for browser approval",
			VerificationCode: msg.session.VerificationCode,
			URL:              msg.session.URL,
			Waiting:          true,
		}

		if m.loginCancel != nil {
			m.loginCancel()
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.loginCancel = cancel
		return m, m.waitForLogin(ctx, msg.id, msg.session)
	case loginFinishedMsg:
		if msg.id != m.loginID {
			return m, nil
		}
		if m.loginCancel != nil {
			m.loginCancel = nil
		}
		if msg.canceled {
			return m, nil
		}
		if msg.err != nil {
			m.loginScreen.Waiting = false
			m.loginScreen.Error = msg.err.Error()
			m.loginScreen.Status = ""
			return m, nil
		}

		m.snapshot.LoggedIn = true
		m.accountEmail = msg.creds.Email
		m.passwordAuth = msg.creds.Email
		if !m.snapshot.VaultExists {
			m.showPasswordScreen(passwordRestore, msg.creds.Email, "", true)
			return m, m.passwordInput.Init()
		}

		return m, m.startRepair(repairPurposePostLogin, nil, false, "Finishing account setup", "Linking the signed-in account to the local daemon and refreshing machine state.", msg.creds.Email)
	case repairProgressMsg:
		if msg.id != m.repairID {
			return m, nil
		}
		m.applyRepairProgress(msg.stage)
		return m, m.waitForRepairProgress(msg.id, m.repairProgress)
	case repairFinishedMsg:
		if msg.id != m.repairID {
			return m, nil
		}
		return m, m.handleRepairFinished(msg.result, msg.err)
	case copyFinishedMsg:
		if msg.err != nil {
			if m.screen == screenLogin {
				m.loginScreen.Error = msg.err.Error()
				return m, nil
			}
			m.notice = notice{message: msg.err.Error(), tone: dashboardscreen.ToneDanger}
			return m, nil
		}
		m.loginScreen.Copied = true
		return m, nil
	case openFinishedMsg:
		if msg.err != nil {
			if m.screen == screenLogin {
				m.loginScreen.Error = msg.err.Error()
				return m, nil
			}
			m.notice = notice{message: msg.err.Error(), tone: dashboardscreen.ToneDanger}
		}
		return m, nil
	case tea.KeyMsg:
		return m.updateKeys(msg)
	}

	if m.screen == screenPassword && m.passwordInput != nil {
		return m, m.passwordInput.Update(msg)
	}

	return m, nil
}

func (m *model) View() string {
	contentWidth := shell.ContentWidth(m.width)
	bodyWidth := shell.BodyWidth(m.width)
	header := m.renderHeader(contentWidth)
	body := m.renderBody(bodyWidth)
	if !m.isWelcomeState() {
		body = shell.IndentBlock(body, shell.ContentLeftInset)
	}
	footer := shell.RenderFooter(m.footerActions()...)
	return shell.Render(m.width, header, body, footer)
}

func (m *model) renderHeader(width int) string {
	data := shell.HeaderData{
		PageTitle:   m.headerPageTitle(),
		Breadcrumbs: m.headerBreadcrumbs(),
		PageNote:    m.headerPageNote(),
		Version:     m.appVersion,
		StatusItems: m.headerStatusItems(),
	}
	return shell.RenderHeader(width, data)
}

func (m *model) headerStatusItems() []shell.StatusItem {
	if m.shouldShowProductRail() {
		return []shell.StatusItem{
			{Label: "Encrypted key vault", Icon: "✦"},
			{Label: "Multi-device sync", Icon: "✦"},
			{Label: "SSH + commit signing", Icon: "✦"},
		}
	}

	sshReady := m.snapshot.ManagedConfigReady &&
		m.snapshot.SSHEnabled &&
		m.snapshot.IPCSocketReady &&
		m.snapshot.AgentSocketReady &&
		m.snapshot.IdentityAgentOwner.IsForged()
	cloudSyncActive := m.snapshot.LoggedIn

	items := []shell.StatusItem{
		{
			Label: sshAgentHeaderLabel(sshReady),
			Tone:  statusTone(sshReady, false),
		},
		{
			Label: "Commit Signing",
			Tone:  statusTone(m.commitSigning, false),
		},
		{
			Label: cloudSyncHeaderLabel(cloudSyncActive),
			Tone:  statusTone(cloudSyncActive, true),
		},
	}

	return items
}

func sshAgentHeaderLabel(healthy bool) string {
	if healthy {
		return "SSH Agent Healthy"
	}
	return "SSH Agent Unhealthy"
}

func cloudSyncHeaderLabel(active bool) string {
	if active {
		return "Cloud Sync Active"
	}
	return "Cloud Sync Inactive"
}

func statusTone(healthy bool, warnWhenFalse bool) shell.StatusTone {
	if healthy {
		return shell.StatusToneSuccess
	}
	if warnWhenFalse {
		return shell.StatusToneWarning
	}
	return shell.StatusToneDanger
}

func (m *model) headerPageTitle() string {
	if m.isWelcomeState() {
		return ""
	}

	switch m.screen {
	case screenLogin:
		if strings.TrimSpace(m.loginScreen.Title) != "" {
			return m.loginScreen.Title
		}
		return "Sign In to Sync Vault"
	case screenPassword:
		if strings.TrimSpace(m.passwordTitle) != "" {
			return m.passwordTitle
		}
		return "Vault"
	case screenRepair:
		if strings.TrimSpace(m.repairScreen.Title) != "" {
			return m.repairScreen.Title
		}
		return "Repair"
	default:
		return m.dashboardTitle()
	}
}

func (m *model) headerBreadcrumbs() []shell.Breadcrumb {
	if m.isWelcomeState() {
		return nil
	}

	switch m.screen {
	case screenLogin:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Account"},
			{Label: "Sign In", Current: true},
		}
	case screenPassword:
		label := "Unlock"
		if m.passwordFlow == passwordCreate {
			label = "Set Up"
		}
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Vault"},
			{Label: label, Current: true},
		}
	case screenRepair:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Repair", Current: true},
		}
	default:
		return nil
	}
}

func (m *model) headerPageNote() string {
	if m.screen != screenDashboard || !m.snapshot.VaultExists || m.isWelcomeState() {
		return ""
	}
	if m.snapshot.LoggedIn {
		if email := strings.TrimSpace(m.accountEmail); email != "" {
			return "Welcome back, " + email
		}
		return "Welcome back"
	}
	return "Your local vault is ready"
}

func (m *model) renderBody(contentWidth int) string {
	switch m.screen {
	case screenLogin:
		return accountscreen.Render(m.loginScreen, m.spinner.View(), contentWidth)
	case screenPassword:
		return m.renderPasswordBody(contentWidth)
	case screenRepair:
		return repairscreen.Render(m.repairScreen, m.spinner.View(), contentWidth)
	default:
		return dashboardscreen.Render(dashboardscreen.Screen{
			Title:   m.dashboardBodyTitle(),
			Context: m.dashboardLead(),
			Options: m.dashboardOptions(),
			Areas:   m.dashboardAreas(),
			Notice: dashboardscreen.Notice{
				Message: m.notice.message,
				Tone:    m.notice.tone,
			},
		}, contentWidth)
	}
}

func (m *model) renderPasswordBody(contentWidth int) string {
	sections := make([]string, 0, 5)

	if m.passwordAuth != "" {
		sections = append(sections,
			theme.Success.Render("✓ Authentication successful"),
			theme.Body.Render("Signed in as "+m.passwordAuth+"."),
			"",
		)
	}

	if strings.TrimSpace(m.passwordContext) != "" {
		sections = append(sections, theme.Body.Width(max(28, min(contentWidth, theme.HeroMaxWidth))).Render(m.passwordContext))
	}

	labels := []string{"Master password"}
	if m.passwordFlow == passwordCreate {
		labels = []string{"", ""}
	}
	sections = append(sections, "", m.passwordInput.View(labels...))
	return strings.Join(sections, "\n")
}

func (m *model) footerActions() []shell.FooterAction {
	switch m.screen {
	case screenLogin:
		actions := []shell.FooterAction{}
		if m.loginScreen.Error != "" {
			actions = append(actions, shell.FooterAction{Key: "Enter", Label: "Retry"})
		} else if m.loginScreen.URL != "" {
			actions = append(actions, shell.FooterAction{Key: "Enter", Label: "Open Link"})
			actions = append(actions, shell.FooterAction{Key: "C", Label: "Copy URL"})
		}
		actions = append(actions, shell.FooterAction{Key: "Esc", Label: m.session.EscLabel(EscCancel)})
		return actions
	case screenPassword:
		actions := []shell.FooterAction{
			{Key: "Enter", Label: "Continue"},
		}
		if m.passwordFlow == passwordCreate {
			actions = append(actions, shell.FooterAction{Key: "Tab", Label: "Next Field"})
		}
		actions = append(actions, shell.FooterAction{Key: "Esc", Label: m.session.EscLabel(EscAuto)})
		return actions
	case screenRepair:
		return nil
	default:
		if m.isWelcomeState() {
			return []shell.FooterAction{
				{Key: "↑/↓", Label: "Move"},
				{Key: "Enter", Label: "Select"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if areas := m.dashboardAreas(); len(areas) > 0 {
			actions := []shell.FooterAction{{Key: "↑/↓", Label: "Move"}}
			if dashboardscreen.AreaColumns(shell.BodyWidth(m.width), len(areas)) > 1 {
				actions = append(actions, shell.FooterAction{Key: "←/→", Label: "Move"})
			}
			if area := m.selectedDashboardArea(); area != nil && area.Label == "Account" && !m.snapshot.LoggedIn {
				actions = append(actions, shell.FooterAction{Key: "Enter", Label: "Sign In"})
			}
			actions = append(actions, shell.FooterAction{Key: "Esc", Label: m.session.EscLabel(EscAuto)})
			return actions
		}
		return []shell.FooterAction{{Key: "Esc", Label: m.session.EscLabel(EscAuto)}}
	}
}

func (m *model) updateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	}

	switch m.screen {
	case screenLogin:
		return m.updateLoginKeys(msg)
	case screenPassword:
		return m.updatePasswordKeys(msg)
	case screenRepair:
		return m, nil
	default:
		return m.updateDashboardKeys(msg)
	}
}

func (m *model) updateDashboardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if areas := m.dashboardAreas(); len(areas) > 0 {
		columns := dashboardscreen.AreaColumns(shell.BodyWidth(m.width), len(areas))
		switch msg.String() {
		case "esc":
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		case "up", "k":
			if m.onboardingCursor-columns >= 0 {
				m.onboardingCursor -= columns
			}
			return m, nil
		case "down", "j":
			if m.onboardingCursor+columns < len(areas) {
				m.onboardingCursor += columns
			}
			return m, nil
		case "left", "h":
			if columns > 1 && m.onboardingCursor%columns > 0 {
				m.onboardingCursor--
			}
			return m, nil
		case "right", "l":
			if columns > 1 && m.onboardingCursor%columns < columns-1 && m.onboardingCursor+1 < len(areas) {
				m.onboardingCursor++
			}
			return m, nil
		case "enter":
			if area := m.selectedDashboardArea(); area != nil && area.Label == "Account" && !m.snapshot.LoggedIn {
				if m.session.Current().ID != RouteAccountLogin {
					m.session.Push(Route{ID: RouteAccountLogin})
				}
				return m, m.startLoginFlow()
			}
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
		if len(m.dashboardOptions()) > 0 && m.onboardingCursor > 0 {
			m.onboardingCursor--
		}
		return m, nil
	case "down", "j":
		if len(m.dashboardOptions()) > 0 && m.onboardingCursor < len(m.dashboardOptions())-1 {
			m.onboardingCursor++
		}
		return m, nil
	case "left", "h":
		if m.isWelcomeState() && m.onboardingCursor > 0 {
			m.onboardingCursor--
		}
		return m, nil
	case "right", "l":
		if m.isWelcomeState() && m.onboardingCursor < len(m.dashboardOptions())-1 {
			m.onboardingCursor++
			return m, nil
		}
		return m, nil
	case "enter":
		if len(m.dashboardOptions()) == 0 {
			return m, nil
		}
		if m.onboardingCursor == 0 {
			if m.session.Current().ID != RouteAccountLogin {
				m.session.Push(Route{ID: RouteAccountLogin})
			}
			return m, m.startLoginFlow()
		}
		if m.onboardingCursor == 1 {
			if m.session.Current().ID != RouteVaultUnlock {
				m.session.Push(Route{ID: RouteVaultUnlock})
			}
			m.showPasswordScreen(passwordCreate, "", "", false)
			return m, m.passwordInput.Init()
		}
		return m, nil
	}
	return m, nil
}

func (m *model) updateLoginKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.loginCancel != nil {
			m.loginCancel()
			m.loginCancel = nil
		}
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "c":
		if m.loginScreen.URL == "" || m.loginScreen.Error != "" {
			return m, nil
		}
		return m, m.copyToClipboard(m.loginScreen.URL)
	case "enter":
		if m.loginScreen.Error != "" {
			return m, m.startLoginFlow()
		}
		if m.loginScreen.URL != "" {
			return m, m.openCurrentLoginURL()
		}
	}
	return m, nil
}

func (m *model) updatePasswordKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.session.Back() {
			m.passwordAuth = ""
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "enter":
		password, err := m.passwordInput.Submit()
		if err != nil {
			m.passwordInput.SetError(err.Error())
			return m, nil
		}
		switch m.passwordFlow {
		case passwordCreate:
			return m, m.startRepair(repairPurposeSetup, password, true, "Setting up Forged", "Creating the local vault and preparing background services for this device.", "")
		case passwordRestore:
			return m, m.startRepair(repairPurposeUnlock, password, false, "Unlocking your vault", "Decrypting the linked vault and finishing device setup.", m.passwordAuth)
		default:
			return m, m.startRepair(repairPurposeUnlock, password, false, "Unlocking Forged", "Verifying the vault and repairing the background service.", "")
		}
	default:
		return m, m.passwordInput.Update(msg)
	}
}

func (m *model) dashboardTitle() string {
	return "Dashboard"
}

func (m *model) dashboardBodyTitle() string {
	if m.isWelcomeState() {
		return "Welcome to Forged"
	}
	return m.dashboardTitle()
}

func (m *model) dashboardLead() string {
	if m.isWelcomeState() {
		return "Restore your synced vault or start fresh on this device"
	}
	return ""
}

func (m *model) dashboardOptions() []dashboardscreen.Option {
	if m.snapshot.VaultExists {
		return nil
	}
	return []dashboardscreen.Option{
		{
			Label:       "Log in to Forged",
			Description: "Create and sync encrypted keys across all machines",
			Primary:     true,
			Selected:    m.onboardingCursor == 0,
		},
		{
			Label:       "Create local vault",
			Description: "Create encrypted keys that stay private to this machine",
			Selected:    m.onboardingCursor == 1,
		},
	}
}

func (m *model) dashboardAreas() []dashboardscreen.Area {
	if !m.snapshot.VaultExists {
		return nil
	}

	areas := []dashboardscreen.Area{
		{
			Label:       "Key",
			Summary:     "Generate, import, and inspect keys",
			Description: "Browse encrypted keys and prepare generate, import, export, rename, and delete flows from one place.",
		},
		{
			Label:       "Vault",
			Summary:     "Lock, unlock, and protect this machine",
			Description: "Control the local vault, change its password, and manage how this machine protects encrypted key material.",
		},
		{
			Label:       "Agent",
			Summary:     "Own SSH routing and signing",
			Description: "Manage SSH agent ownership, signing integration, and the runtime behavior behind developer workflows.",
		},
		{
			Label:       "Account",
			Summary:     "Profile, access, and linked features",
			Description: "Review your Forged account, sign in for multi-device sync, and manage access to linked features.",
		},
		{
			Label:       "Sync",
			Summary:     "Track machine-to-cloud state",
			Description: "Review sync state, refresh linked data, and keep this machine aligned with your Forged account.",
		},
		{
			Label:       "Doctor",
			Summary:     "Inspect and repair health",
			Description: "Audit sockets, service state, SSH routing, and other runtime issues when this machine needs attention.",
		},
	}

	if m.snapshot.LoggedIn {
		areas[3].Summary = "Profile, session, and linked features"
		if email := strings.TrimSpace(m.accountEmail); email != "" {
			areas[3].Description = "Review the account for " + email + ", manage linked access, and control synced Forged features."
		}
		areas[4].Summary = "Track and refresh sync state"
		areas[4].Description = "Review sync status, refresh linked data, and keep this machine aligned with your signed-in Forged account."
	} else {
		areas[4].Summary = "Available after account sign-in"
		areas[4].Description = "Sign in first to enable multi-device sync, linked backup, and account-aware machine state."
	}

	for index := range areas {
		areas[index].Selected = index == m.onboardingCursor
	}
	return areas
}

func (m *model) selectedDashboardArea() *dashboardscreen.Area {
	areas := m.dashboardAreas()
	if len(areas) == 0 {
		return nil
	}
	if m.onboardingCursor < 0 {
		m.onboardingCursor = 0
	}
	if m.onboardingCursor >= len(areas) {
		m.onboardingCursor = len(areas) - 1
	}
	return &areas[m.onboardingCursor]
}

func (m *model) usesSpinner() bool {
	switch m.screen {
	case screenRepair:
		return m.repairScreen.Error == ""
	case screenLogin:
		return m.loginScreen.Waiting
	default:
		return false
	}
}

func (m *model) assessCurrentState() tea.Cmd {
	repair := m.repair
	return func() tea.Msg {
		result, err := repair(readiness.RunOptions{Mode: readiness.ModeAssessOnly})
		return assessmentMsg{snapshot: result.Snapshot, err: err}
	}
}

func (m *model) startLoginFlow() tea.Cmd {
	m.screen = screenLogin
	m.notice = notice{}
	m.loginScreen = accountscreen.LoginScreen{
		Title:   "Sign In to Sync Vault",
		Context: "Preparing secure browser approval.",
		Status:  "Opening approval link",
		Waiting: true,
	}
	m.loginID++
	id := m.loginID
	startLogin := m.startLogin
	server := m.serverURL()
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			session, err := startLogin(server)
			return loginStartedMsg{id: id, session: session, err: err}
		},
	)
}

func (m *model) waitForLogin(ctx context.Context, id int, session actions.LoginSession) tea.Cmd {
	save := m.saveCredentials
	return func() tea.Msg {
		creds, err := session.Wait(ctx)
		if errors.Is(err, context.Canceled) {
			return loginFinishedMsg{id: id, canceled: true}
		}
		if err == nil {
			err = save(creds)
		}
		return loginFinishedMsg{id: id, creds: creds, err: err}
	}
}

func (m *model) startRepair(purpose repairPurpose, password []byte, createVaultFirst bool, title string, contextLine string, authEmail string) tea.Cmd {
	if m.session.Current().ID != RouteRepairTask {
		m.session.Push(Route{ID: RouteRepairTask})
	}
	m.screen = screenRepair
	m.passwordAuth = authEmail
	m.repairPurpose = purpose
	m.repairUsedPassword = len(password) > 0
	m.repairAuthEmail = authEmail
	m.repairScreen = repairscreen.TaskScreen{
		Title:      title,
		Context:    contextLine,
		Tasks:      m.newRepairTasks(authEmail),
		StatusRows: m.repairStatusRows(),
	}

	progressCh := make(chan readiness.ProgressStage, 16)
	m.repairProgress = progressCh
	m.repairID++
	id := m.repairID
	repairFn := m.repair
	createVault := m.createVault
	passwordCopy := append([]byte(nil), password...)
	progress := func(stage readiness.ProgressStage) {
		select {
		case progressCh <- stage:
		default:
		}
	}

	return tea.Batch(
		m.waitForRepairProgress(id, progressCh),
		func() tea.Msg {
			if createVaultFirst {
				progress(readiness.ProgressVault)
				if err := createVault(passwordCopy); err != nil {
					close(progressCh)
					return repairFinishedMsg{id: id, err: err}
				}
			}

			opts := readiness.RunOptions{
				Mode: readiness.ModeInteractiveLauncher,
				Progress: func(stage readiness.ProgressStage) {
					if !(createVaultFirst && stage == readiness.ProgressVault) {
						progress(stage)
					}
				},
			}
			if len(passwordCopy) > 0 {
				opts.PromptPassword = func(string) ([]byte, error) {
					return append([]byte(nil), passwordCopy...), nil
				}
			}

			result, err := repairFn(opts)
			close(progressCh)
			return repairFinishedMsg{id: id, result: result, err: err}
		},
	)
}

func (m *model) waitForRepairProgress(id int, ch <-chan readiness.ProgressStage) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		stage, ok := <-ch
		if !ok {
			return nil
		}
		return repairProgressMsg{id: id, stage: stage}
	}
}

func (m *model) handleRepairFinished(result readiness.RunResult, err error) tea.Cmd {
	m.snapshot = result.Snapshot
	m.summary = result.Summary
	if m.repairAuthEmail != "" {
		m.accountEmail = m.repairAuthEmail
	}
	m.finishRepairTasks(result)

	if err != nil {
		switch {
		case m.repairUsedPassword:
			m.showPasswordScreen(m.passwordFlowForSnapshot(result.Snapshot), m.repairAuthEmail, err.Error(), m.repairAuthEmail != "")
			return m.passwordInput.Init()
		case m.repairPurpose == repairPurposeSetup:
			m.showPasswordScreen(passwordCreate, "", err.Error(), false)
			return m.passwordInput.Init()
		default:
			m.showDashboardNotice(err.Error(), dashboardscreen.ToneDanger)
			return nil
		}
	}

	switch result.Next {
	case readiness.NextActionNeedsPassword:
		errorText := ""
		if m.repairUsedPassword {
			errorText = "That password did not unlock this device."
		}
		m.showPasswordScreen(m.passwordFlowForSnapshot(result.Snapshot), m.repairAuthEmail, errorText, m.repairAuthEmail != "")
		return m.passwordInput.Init()
	case readiness.NextActionNeedsInteractiveSetup:
		if result.Snapshot.LoggedIn {
			m.showDashboardNotice("No synced vault was found for this account. Start a new vault on this device.", dashboardscreen.ToneWarning)
		} else {
			m.showDashboardNotice(m.summaryMessage(), dashboardscreen.ToneSuccess)
		}
		m.popWizardRoutes()
		m.screen = screenDashboard
		return nil
	default:
		if (m.repairPurpose == repairPurposeSetup || m.repairPurpose == repairPurposeUnlock) && result.Snapshot.VaultExists {
			m.popWizardRoutes()
			return m.restartAfterVaultReady()
		}
		m.popWizardRoutes()
		m.notice = notice{}
		m.screen = screenDashboard
		return nil
	}
}

func (m *model) startStartupRepair() tea.Cmd {
	return m.startRepair(
		repairPurposeStartup,
		nil,
		false,
		"Checking local health",
		"Reviewing the current state and applying safe fixes where needed.",
		"",
	)
}

func (m *model) restartAfterVaultReady() tea.Cmd {
	m.notice = notice{}
	m.onboardingCursor = 0
	m.screen = screenRepair
	m.repairPurpose = repairPurposeStartup
	m.repairUsedPassword = false
	m.repairAuthEmail = ""
	m.repairScreen = repairscreen.TaskScreen{
		Title:   "Checking local health",
		Context: "Reviewing the current state and applying safe fixes where needed.",
		Tasks:   m.newRepairTasks(""),
	}
	return m.assessCurrentState()
}

func (m *model) showPasswordScreen(flow passwordFlow, authEmail string, errorText string, reuseCurrentRoute bool) {
	if !reuseCurrentRoute && m.session.Current().ID != RouteVaultUnlock {
		m.session.Push(Route{ID: RouteVaultUnlock})
	}

	m.screen = screenPassword
	m.passwordFlow = flow
	m.passwordAuth = authEmail
	switch flow {
	case passwordCreate:
		m.passwordTitle = "Create local vault"
		m.passwordContext = "Set an encryption password for your vault. Save it securely. If you lose it, your keys are lost."
		m.passwordInput = components.NewCreatePasswordInput()
	case passwordRestore:
		m.passwordTitle = "Unlock your vault"
		m.passwordContext = "Enter your master password to decrypt the keys already linked to this Forged account on this device."
		m.passwordInput = components.NewUnlockPasswordInput()
	default:
		m.passwordTitle = "Unlock Forged"
		m.passwordContext = "Enter your master password to verify the local vault and finish repairing the background service."
		m.passwordInput = components.NewUnlockPasswordInput()
	}
	m.passwordInput.SetWidth(max(18, shell.ClampBlockWidth(m.width, 40)-4))
	if errorText != "" {
		m.passwordInput.SetError(errorText)
	}
}

func (m *model) showCurrentRoute() tea.Cmd {
	m.notice = notice{}
	m.screen = screenDashboard
	switch m.session.Current().ID {
	case RouteAccountLogin:
		return m.startLoginFlow()
	default:
		return nil
	}
}

func (m *model) showDashboardNotice(message string, tone dashboardscreen.Tone) {
	m.notice = notice{message: strings.TrimSpace(message), tone: tone}
	if m.snapshot.VaultExists {
		m.onboardingCursor = 0
	}
}

func (m *model) isWelcomeState() bool {
	return m.screen == screenDashboard && len(m.dashboardOptions()) > 0
}

func (m *model) shouldShowProductRail() bool {
	return !m.snapshot.VaultExists
}

func (m *model) serverURL() string {
	if server := strings.TrimSpace(m.intent.Param("server")); server != "" {
		return server
	}
	return m.defaultServer
}

func (m *model) copyToClipboard(value string) tea.Cmd {
	copyText := m.copyText
	return func() tea.Msg {
		return copyFinishedMsg{err: copyText(value)}
	}
}

func (m *model) openCurrentLoginURL() tea.Cmd {
	url := strings.TrimSpace(m.loginScreen.URL)
	openLink := m.openLink
	return func() tea.Msg {
		if url == "" {
			return openFinishedMsg{}
		}
		return openFinishedMsg{err: openLink(url)}
	}
}

func (m *model) newRepairTasks(authEmail string) []repairscreen.Task {
	accountState := repairscreen.TaskPending
	if authEmail != "" || m.snapshot.LoggedIn {
		accountState = repairscreen.TaskDone
	}

	return []repairscreen.Task{
		{Label: "Vault", State: repairscreen.TaskPending},
		{Label: "Service", State: repairscreen.TaskPending},
		{Label: "SSH", State: repairscreen.TaskPending},
		{Label: "Agent", State: repairscreen.TaskPending},
		{Label: "Account", State: accountState},
	}
}

func (m *model) applyRepairProgress(stage readiness.ProgressStage) {
	target := ""
	switch stage {
	case readiness.ProgressVault:
		target = "Vault"
	case readiness.ProgressService:
		target = "Service"
	case readiness.ProgressSSH:
		target = "SSH"
	case readiness.ProgressSockets:
		target = "Agent"
	}
	if target == "" {
		return
	}

	for index := range m.repairScreen.Tasks {
		task := &m.repairScreen.Tasks[index]
		if task.State == repairscreen.TaskActive {
			task.State = repairscreen.TaskDone
		}
		if task.Label == target {
			task.State = repairscreen.TaskActive
		}
	}
}

func (m *model) finishRepairTasks(result readiness.RunResult) {
	for index := range m.repairScreen.Tasks {
		task := &m.repairScreen.Tasks[index]
		switch task.Label {
		case "Vault":
			if result.Snapshot.VaultExists {
				task.State = repairscreen.TaskDone
			}
		case "Service":
			if result.Snapshot.Service.Running {
				task.State = repairscreen.TaskDone
			}
		case "SSH":
			if result.Snapshot.SSHEnabled && result.Snapshot.ManagedConfigReady {
				task.State = repairscreen.TaskDone
			}
		case "Agent":
			if result.Snapshot.IPCSocketReady && result.Snapshot.AgentSocketReady {
				task.State = repairscreen.TaskDone
			}
		case "Account":
			if result.Snapshot.LoggedIn || m.repairAuthEmail != "" {
				task.State = repairscreen.TaskDone
			}
		}
	}
}

func (m *model) repairStatusRows() []repairscreen.StatusRow {
	rows := NewState(m.snapshot).SummaryRows()
	out := make([]repairscreen.StatusRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, repairscreen.StatusRow{Label: row.Label, Value: row.Value})
	}
	return out
}

func (m *model) passwordFlowForSnapshot(snapshot readiness.Snapshot) passwordFlow {
	if !snapshot.VaultExists && (snapshot.LoggedIn || m.repairAuthEmail != "") {
		return passwordRestore
	}
	return passwordRepair
}

func (m *model) popWizardRoutes() {
	for m.session.Current().ID == RouteRepairTask || m.session.Current().ID == RouteVaultUnlock {
		if !m.session.Back() {
			break
		}
	}
}

func (m *model) summaryMessage() string {
	if len(m.summary.Fixed) == 0 {
		return "All systems operational."
	}

	items := append([]string(nil), m.summary.Fixed...)
	for index, item := range items {
		items[index] = strings.ToUpper(item[:1]) + item[1:]
	}
	return "Updated " + strings.Join(items, ", ") + "."
}
