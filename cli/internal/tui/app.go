package tui

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

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
	RestoreVault    func([]byte) error
	StartLogin      func(string, func(actions.LoginProgress)) (actions.LoginSession, error)
	SaveCredentials func(actions.AccountCredentials) error
	LoadStatus      func() (RuntimeStatus, error)
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

type setupVariant string

const (
	setupVariantNone    setupVariant = ""
	setupVariantLocal   setupVariant = "local"
	setupVariantRestore setupVariant = "restore"
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

type loginProgressMsg struct {
	id       int
	progress actions.LoginProgress
}

type loginFinishedMsg struct {
	id       int
	creds    actions.AccountCredentials
	err      error
	canceled bool
}

type restoreFinishedMsg struct {
	id       int
	password []byte
	err      error
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

type setupTaskDoneMsg struct {
	sequence int
	index    int
}

type setupFinalizeMsg struct {
	sequence int
}

type runtimeStatusMsg struct {
	status RuntimeStatus
	err    error
}

type RuntimeStatus struct {
	Syncing bool
	Dirty   bool
	Linked  bool
	Error   string
}

type systemHeaderState string

const (
	systemHeaderChecking  systemHeaderState = "checking"
	systemHeaderFixing    systemHeaderState = "fixing"
	systemHeaderHealthy   systemHeaderState = "healthy"
	systemHeaderUnhealthy systemHeaderState = "unhealthy"
)

type pendingSetupResult struct {
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
	restoreVault    func([]byte) error
	startLogin      func(string, func(actions.LoginProgress)) (actions.LoginSession, error)
	saveCredentials func(actions.AccountCredentials) error
	loadStatus      func() (RuntimeStatus, error)
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
	passwordBusy     bool
	repairScreen     repairscreen.TaskScreen

	loginID       int
	loginProgress <-chan actions.LoginProgress
	loginCancel   context.CancelFunc
	restoreID     int

	repairID           int
	repairProgress     <-chan readiness.ProgressStage
	repairPurpose      repairPurpose
	repairUsedPassword bool
	repairAuthEmail    string
	setupVariant       setupVariant

	bootAssessed    bool
	systemHeader    systemHeaderState
	runtimeStatus   RuntimeStatus
	runtimeLoaded   bool
	setupStageIndex int
	setupSequenceID int
	setupPending    *pendingSetupResult
	setupFinalizing bool
	random          *rand.Rand
}

func Run(intent Intent, deps Dependencies) (Result, error) {
	switch {
	case deps.Repair == nil:
		return Result{}, fmt.Errorf("tui repair dependency is required")
	case deps.CreateVault == nil:
		return Result{}, fmt.Errorf("tui create-vault dependency is required")
	case deps.RestoreVault == nil:
		return Result{}, fmt.Errorf("tui restore-vault dependency is required")
	case deps.StartLogin == nil:
		return Result{}, fmt.Errorf("tui login dependency is required")
	case deps.SaveCredentials == nil:
		return Result{}, fmt.Errorf("tui save-credentials dependency is required")
	case deps.LoadStatus == nil:
		return Result{}, fmt.Errorf("tui load-status dependency is required")
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
		restoreVault:    deps.RestoreVault,
		startLogin:      deps.StartLogin,
		saveCredentials: deps.SaveCredentials,
		loadStatus:      deps.LoadStatus,
		copyText:        deps.CopyText,
		openLink:        deps.OpenLink,
		defaultServer:   deps.DefaultServer,
		appVersion:      deps.AppVersion,
		commitSigning:   deps.CommitSigning,
		spinner:         spin,
		random:          rand.New(rand.NewSource(time.Now().UnixNano())),
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
		m.screen = screenDashboard
		m.bootAssessed = false
		m.systemHeader = systemHeaderChecking
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
		m.bootAssessed = true
		if msg.err != nil {
			if m.screen == screenLogin {
				m.loginScreen.Waiting = false
				m.loginScreen.Error = msg.err.Error()
				m.loginScreen.Status = ""
				return m, nil
			}
			m.systemHeader = systemHeaderUnhealthy
			m.notice = notice{message: msg.err.Error(), tone: dashboardscreen.ToneDanger}
			return m, nil
		}
		m.snapshot = msg.snapshot
		m.runtimeLoaded = false
		if m.screen == screenLogin {
			return m, m.startLoginFlow()
		}
		if !msg.snapshot.VaultExists {
			m.notice = notice{}
			m.summary = readiness.RepairSummary{}
			m.repairAuthEmail = ""
			m.repairUsedPassword = false
			m.setupVariant = setupVariantNone
			m.screen = screenDashboard
			m.systemHeader = systemHeaderHealthy
			return m, nil
		}
		m.screen = screenDashboard
		m.systemHeader = systemHeaderChecking
		return m, m.startStartupRepair()
	case loginStartedMsg:
		if msg.id != m.loginID || m.screen != screenLogin {
			return m, nil
		}
		m.loginProgress = nil
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
	case loginProgressMsg:
		if msg.id != m.loginID || m.screen != screenLogin {
			return m, nil
		}
		if strings.TrimSpace(msg.progress.Status) != "" {
			m.loginScreen.Waiting = true
			m.loginScreen.Status = msg.progress.Status
			m.loginScreen.Error = ""
		}
		return m, m.waitForLoginStartProgress(msg.id, m.loginProgress)
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
	case restoreFinishedMsg:
		if msg.id != m.restoreID {
			return m, nil
		}
		m.passwordBusy = false
		if msg.err != nil {
			switch {
			case errors.Is(msg.err, readiness.ErrInvalidRestorePassword):
				m.passwordInput.SetError("Couldn't decrypt vault, incorrect password")
			case errors.Is(msg.err, readiness.ErrNoRemoteLinkedVault):
				m.passwordInput.SetError("No linked vault was found for this account.")
			default:
				m.passwordInput.SetError(msg.err.Error())
			}
			return m, nil
		}
		return m, m.startRepair(repairPurposeUnlock, msg.password, false, "Setting up Forged", "Restoring your vault and preparing secure access on this machine.", m.passwordAuth)
	case repairProgressMsg:
		if msg.id != m.repairID {
			return m, nil
		}
		if m.isSetupSequence() {
			return m, nil
		}
		m.applyRepairProgress(msg.stage)
		return m, m.waitForRepairProgress(msg.id, m.repairProgress)
	case repairFinishedMsg:
		if msg.id != m.repairID {
			return m, nil
		}
		if m.isSetupSequence() {
			if msg.err != nil {
				m.setupSequenceID++
				return m, m.handleRepairFinished(msg.result, msg.err)
			}
			m.setupPending = &pendingSetupResult{result: msg.result, err: msg.err}
			m.setupFinalizing = true
			m.completeSetupTasks()
			return m, m.finalizeSetupAfter(time.Second)
		}
		return m, m.handleRepairFinished(msg.result, msg.err)
	case setupTaskDoneMsg:
		if m.screen != screenRepair {
			return m, nil
		}
		if m.setupFinalizing || !m.isSetupSequence() || msg.sequence != m.setupSequenceID {
			return m, nil
		}
		if msg.index < 0 || msg.index >= len(m.repairScreen.Tasks) {
			return m, nil
		}
		if msg.index >= len(m.repairScreen.Tasks)-1 {
			return m, nil
		}
		if m.repairScreen.Tasks[msg.index].State == repairscreen.TaskActive {
			m.repairScreen.Tasks[msg.index].State = repairscreen.TaskDone
			m.advanceSetupStatus()
		}
		return m, nil
	case setupFinalizeMsg:
		if m.screen != screenRepair {
			return m, nil
		}
		if msg.sequence != m.setupSequenceID {
			return m, nil
		}
		if !m.setupFinalizing || m.setupPending == nil {
			return m, nil
		}
		pending := m.setupPending
		m.setupPending = nil
		m.setupFinalizing = false
		return m, m.handleRepairFinished(pending.result, pending.err)
	case runtimeStatusMsg:
		if msg.err == nil {
			m.runtimeStatus = msg.status
			m.runtimeLoaded = true
		} else if m.snapshot.LoggedIn && m.systemHeader == systemHeaderHealthy {
			m.runtimeStatus = RuntimeStatus{Error: msg.err.Error()}
			m.runtimeLoaded = true
		}
		if m.screen == screenDashboard && m.snapshot.VaultExists {
			return m, m.pollRuntimeStatus(time.Second)
		}
		return m, nil
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
	if !m.bootAssessed {
		return nil
	}
	if m.shouldShowProductRail() {
		return []shell.StatusItem{
			{Label: "Encrypted key vault", Icon: "✦"},
			{Label: "Multi-device sync", Icon: "✦"},
			{Label: "SSH + commit signing", Icon: "✦"},
		}
	}

	items := []shell.StatusItem{
		m.systemHeaderItem(),
		m.commitSigningHeaderItem(),
		m.vaultSyncHeaderItem(),
	}

	return items
}

func (m *model) systemHeaderItem() shell.StatusItem {
	switch m.systemHeader {
	case systemHeaderChecking:
		return shell.StatusItem{Label: "Checking health", Icon: m.spinner.View()}
	case systemHeaderFixing:
		return shell.StatusItem{Label: "Fixing issues", Icon: m.spinner.View()}
	case systemHeaderHealthy:
		return shell.StatusItem{Label: "System healthy", Tone: shell.StatusToneSuccess}
	default:
		return shell.StatusItem{Label: "System unhealthy", Tone: shell.StatusToneDanger}
	}
}

func (m *model) commitSigningHeaderItem() shell.StatusItem {
	if m.commitSigning {
		return shell.StatusItem{Label: "Commit signing", Tone: shell.StatusToneSuccess}
	}
	return shell.StatusItem{Label: "Commit not signing", Tone: shell.StatusToneWarning}
}

func (m *model) vaultSyncHeaderItem() shell.StatusItem {
	if !m.snapshot.LoggedIn {
		return shell.StatusItem{Label: "Local vault healthy", Tone: shell.StatusToneSuccess}
	}
	if m.runtimeStatus.Syncing || m.systemHeader == systemHeaderChecking || m.systemHeader == systemHeaderFixing {
		return shell.StatusItem{Label: "Vault syncing", Icon: m.spinner.View()}
	}
	if m.runtimeLoaded && (m.runtimeStatus.Dirty || strings.TrimSpace(m.runtimeStatus.Error) != "") {
		return shell.StatusItem{Label: "Sync issue", Tone: shell.StatusToneDanger}
	}
	return shell.StatusItem{Label: "Vault up to date", Tone: shell.StatusToneSuccess}
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
		label := "Health"
		if m.isSetupSequence() {
			label = "Setup"
		}
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: label, Current: true},
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
		if !m.bootAssessed {
			return ""
		}
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
			theme.Success.Render("✓")+" "+theme.BodyMuted.Render(" Signed in as")+" "+theme.Body.Render(m.passwordAuth),
			"",
		)
	}

	if strings.TrimSpace(m.passwordContext) != "" {
		sections = append(sections, theme.Body.Width(max(28, min(contentWidth, theme.HeroMaxWidth))).Render(m.passwordContext))
	}

	labels := []string{""}
	if m.passwordFlow == passwordCreate {
		labels = []string{"", ""}
	}
	sections = append(sections, "", m.passwordInput.View(m.spinner.View(), labels...))
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
		if m.passwordBusy {
			return nil
		}
		enterLabel := "Continue"
		if m.passwordFlow == passwordCreate && m.passwordInput != nil && m.passwordInput.FocusIndex() == 0 {
			enterLabel = "Next"
		}
		actions := []shell.FooterAction{{Key: "Enter", Label: enterLabel}}
		actions = append(actions, shell.FooterAction{Key: "Esc", Label: m.session.EscLabel(EscAuto)})
		return actions
	case screenRepair:
		if m.repairScreen.Error != "" {
			return []shell.FooterAction{
				{Key: "Enter", Label: "Retry"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		return nil
	default:
		if !m.bootAssessed {
			return []shell.FooterAction{{Key: "Esc", Label: m.session.EscLabel(EscAuto)}}
		}
		if m.isWelcomeState() {
			return []shell.FooterAction{
				{Key: "↑/↓", Label: "Move"},
				{Key: "Enter", Label: "Select"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if areas := m.dashboardAreas(); len(areas) > 0 {
			actions := []shell.FooterAction{{Key: "↕/↔", Label: "Move"}}
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
		if m.repairScreen.Error == "" {
			return m, nil
		}
		switch msg.String() {
		case "esc":
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		case "enter":
			if m.isSetupSequence() && m.snapshot.VaultExists {
				return m, m.restartAfterVaultReady()
			}
			return m, m.startStartupRepair()
		}
		return m, nil
	default:
		return m.updateDashboardKeys(msg)
	}
}

func (m *model) updateDashboardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.bootAssessed {
		switch msg.String() {
		case "esc":
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		}
		return m, nil
	}

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
		m.loginID++
		m.loginProgress = nil
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
	if m.passwordBusy {
		return m, nil
	}

	switch msg.String() {
	case "esc":
		if m.session.Back() {
			m.passwordAuth = ""
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "enter":
		if m.passwordFlow == passwordCreate && m.passwordInput != nil && m.passwordInput.FieldCount() > 1 && m.passwordInput.FocusIndex() == 0 {
			m.passwordInput.MoveNext()
			return m, nil
		}
		password, err := m.passwordInput.Submit()
		if err != nil {
			m.passwordInput.SetError(err.Error())
			return m, nil
		}
		switch m.passwordFlow {
		case passwordCreate:
			return m, m.startRepair(repairPurposeSetup, password, true, "Setting up Forged", "Creating the local vault and preparing background services for this machine.", "")
		case passwordRestore:
			m.passwordBusy = true
			m.passwordInput.SetInfo("Decrypting vault")
			m.restoreID++
			return m, tea.Batch(m.spinner.Tick, m.restoreLinkedVault(m.restoreID, password))
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
			Summary:     "Browse, create, import, and export keys",
			Description: "Browse encrypted keys and prepare generate, import, export, rename, and delete flows from one place.",
		},
		{
			Label:       "Vault",
			Summary:     "Lock, unlock, and protect this machine",
			Description: "Control the local vault, change its password, and manage how this machine protects encrypted key material.",
		},
		{
			Label:       "Agent",
			Summary:     "Control SSH routing and signing",
			Description: "Manage SSH agent ownership, signing integration, and the runtime behavior behind developer workflows.",
		},
		{
			Label:       "Account",
			Summary:     "Profile, access, and linked features",
			Description: "Review your Forged account, sign in for multi-device sync, and manage access to linked features.",
		},
		{
			Label:       "Sync",
			Summary:     "Refresh and review vault sync",
			Description: "Review sync state, refresh linked data, and keep this machine aligned with your Forged account.",
		},
		{
			Label:       "Doctor",
			Summary:     "Inspect health and fix issues",
			Description: "Audit sockets, service state, SSH routing, and other runtime issues when this machine needs attention.",
		},
	}

	if m.snapshot.LoggedIn {
		areas[3].Summary = "Profile, session, and linked features"
		if email := strings.TrimSpace(m.accountEmail); email != "" {
			areas[3].Description = "Review the account for " + email + ", manage linked access, and control synced Forged features."
		}
		areas[4].Summary = "Refresh and review sync state"
		areas[4].Description = "Review sync status, refresh linked data, and keep this machine aligned with your signed-in Forged account."
	} else {
		areas[4].Summary = "Unlocks after account sign-in"
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
	case screenPassword:
		return m.passwordBusy
	case screenDashboard:
		return m.systemHeader == systemHeaderChecking || m.systemHeader == systemHeaderFixing || m.runtimeStatus.Syncing
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
	progressCh := make(chan actions.LoginProgress, 8)
	m.loginProgress = progressCh
	return tea.Batch(
		m.spinner.Tick,
		m.waitForLoginStartProgress(id, progressCh),
		func() tea.Msg {
			session, err := startLogin(server, func(progress actions.LoginProgress) {
				select {
				case progressCh <- progress:
				default:
				}
			})
			close(progressCh)
			return loginStartedMsg{id: id, session: session, err: err}
		},
	)
}

func (m *model) waitForLoginStartProgress(id int, ch <-chan actions.LoginProgress) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		progress, ok := <-ch
		if !ok {
			return nil
		}
		return loginProgressMsg{id: id, progress: progress}
	}
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

func (m *model) restoreLinkedVault(id int, password []byte) tea.Cmd {
	restore := m.restoreVault
	passwordCopy := append([]byte(nil), password...)
	return func() tea.Msg {
		err := restore(passwordCopy)
		return restoreFinishedMsg{id: id, password: passwordCopy, err: err}
	}
}

func (m *model) startRepair(purpose repairPurpose, password []byte, createVaultFirst bool, title string, contextLine string, authEmail string) tea.Cmd {
	backgroundStartup := purpose == repairPurposeStartup && !m.isSetupSequence()

	if !backgroundStartup && m.session.Current().ID != RouteRepairTask {
		m.session.Push(Route{ID: RouteRepairTask})
	}
	if backgroundStartup {
		m.screen = screenDashboard
		m.notice = notice{}
		m.systemHeader = systemHeaderFixing
	} else {
		m.screen = screenRepair
	}
	m.passwordAuth = authEmail
	m.repairPurpose = purpose
	m.repairUsedPassword = len(password) > 0
	m.repairAuthEmail = authEmail
	m.setupVariant = m.setupVariantForRepair(purpose, createVaultFirst, authEmail)
	m.setupPending = nil
	m.setupStageIndex = 0
	if !backgroundStartup {
		m.repairScreen = repairscreen.TaskScreen{
			Kind:       m.repairScreenKind(),
			Title:      title,
			Context:    contextLine,
			Tasks:      m.newRepairTasks(authEmail),
			StatusRows: m.repairStatusRows(),
		}
		if m.isSetupSequence() {
			m.initializeSetupSequence()
		}
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
		m.spinner.Tick,
		m.waitForRepairProgress(id, progressCh),
		m.startSetupSequence(),
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
	m.systemHeader = m.systemHeaderForSnapshot(result.Snapshot)
	if m.repairAuthEmail != "" {
		m.accountEmail = m.repairAuthEmail
	}
	m.finishRepairTasks(result)

	if err != nil {
		m.systemHeader = systemHeaderUnhealthy
		switch {
		case m.repairPurpose == repairPurposeSetup:
			if result.Snapshot.VaultExists {
				m.markActiveRepairTaskFailed()
				m.repairScreen.Error = err.Error()
				return nil
			}
			m.showPasswordScreen(passwordCreate, "", err.Error(), false)
			return m.passwordInput.Init()
		case m.repairUsedPassword:
			m.markActiveRepairTaskFailed()
			m.repairScreen.Error = err.Error()
			return nil
		default:
			m.showDashboardNotice(err.Error(), dashboardscreen.ToneDanger)
			if m.snapshot.VaultExists {
				return m.pollRuntimeStatus(time.Second)
			}
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
		m.setupVariant = setupVariantNone
		m.screen = screenDashboard
		if m.snapshot.VaultExists {
			return m.pollRuntimeStatus(0)
		}
		return nil
	}
}

func (m *model) systemHeaderForSnapshot(snapshot readiness.Snapshot) systemHeaderState {
	switch snapshot.State {
	case readiness.StateReady, readiness.StateReadyEmpty:
		return systemHeaderHealthy
	default:
		return systemHeaderUnhealthy
	}
}

func (m *model) startStartupRepair() tea.Cmd {
	m.systemHeader = systemHeaderFixing
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
	m.screen = screenDashboard
	m.bootAssessed = false
	m.systemHeader = systemHeaderChecking
	m.setupVariant = setupVariantNone
	m.repairPurpose = repairPurposeStartup
	m.repairUsedPassword = false
	m.repairAuthEmail = ""
	m.setupPending = nil
	m.runtimeLoaded = false
	return m.assessCurrentState()
}

func (m *model) showPasswordScreen(flow passwordFlow, authEmail string, errorText string, reuseCurrentRoute bool) {
	if !reuseCurrentRoute && m.session.Current().ID != RouteVaultUnlock {
		m.session.Push(Route{ID: RouteVaultUnlock})
	}

	m.screen = screenPassword
	m.passwordFlow = flow
	m.passwordAuth = authEmail
	m.passwordBusy = false
	switch flow {
	case passwordCreate:
		m.passwordTitle = "Create local vault"
		m.passwordContext = "Set an encryption password for your vault. Save it securely. If you lose it, your keys are lost."
		m.passwordInput = components.NewCreatePasswordInput()
	case passwordRestore:
		m.passwordTitle = "Unlock your vault"
		m.passwordContext = "Master password is required to decrypt this vault and unlock its keys"
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
	return m.bootAssessed && m.screen == screenDashboard && len(m.dashboardOptions()) > 0
}

func (m *model) shouldShowProductRail() bool {
	return !m.snapshot.VaultExists
}

func (m *model) isSetupSequence() bool {
	return m.setupVariant != setupVariantNone
}

func (m *model) repairScreenKind() repairscreen.ScreenKind {
	if m.isSetupSequence() {
		return repairscreen.ScreenKindSetup
	}
	return repairscreen.ScreenKindRepair
}

func (m *model) setupVariantForRepair(purpose repairPurpose, createVaultFirst bool, authEmail string) setupVariant {
	switch purpose {
	case repairPurposeSetup:
		if createVaultFirst {
			return setupVariantLocal
		}
	case repairPurposeUnlock:
		if authEmail != "" || m.passwordFlow == passwordRestore || (!m.snapshot.VaultExists && m.snapshot.LoggedIn) {
			return setupVariantRestore
		}
	case repairPurposeStartup:
		return m.setupVariant
	}
	return setupVariantNone
}

func (m *model) setupContextLine() string {
	switch m.setupVariant {
	case setupVariantRestore:
		return "Restoring your vault and preparing secure access on this machine."
	default:
		return "Creating the local vault and preparing background services for this machine."
	}
}

func (m *model) serverURL() string {
	if server := strings.TrimSpace(m.intent.Param("server")); server != "" {
		return server
	}
	return m.defaultServer
}

func (m *model) pollRuntimeStatus(delay time.Duration) tea.Cmd {
	if m.loadStatus == nil || !m.snapshot.VaultExists {
		return nil
	}
	if delay <= 0 {
		delay = 50 * time.Millisecond
	}
	return tea.Tick(delay, func(time.Time) tea.Msg {
		status, err := m.loadStatus()
		return runtimeStatusMsg{status: status, err: err}
	})
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
	if m.isSetupSequence() {
		switch m.setupVariant {
		case setupVariantRestore:
			return []repairscreen.Task{
				{Label: "Account", State: repairscreen.TaskPending},
				{Label: "Vault", State: repairscreen.TaskPending},
				{Label: "SSH", State: repairscreen.TaskPending},
				{Label: "Agent", State: repairscreen.TaskPending},
				{Label: "Service", State: repairscreen.TaskPending},
			}
		default:
			return []repairscreen.Task{
				{Label: "Password", State: repairscreen.TaskPending},
				{Label: "Vault", State: repairscreen.TaskPending},
				{Label: "SSH", State: repairscreen.TaskPending},
				{Label: "Agent", State: repairscreen.TaskPending},
				{Label: "Service", State: repairscreen.TaskPending},
			}
		}
	}

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

func (m *model) initializeSetupSequence() {
	m.setupStageIndex = 0
	m.setupSequenceID++
	m.setupFinalizing = false
	m.setupPending = nil
	for index := range m.repairScreen.Tasks {
		m.repairScreen.Tasks[index].State = repairscreen.TaskActive
	}
	m.setSetupStatusForStage()
}

func (m *model) startSetupSequence() tea.Cmd {
	if !m.isSetupSequence() || len(m.repairScreen.Tasks) == 0 {
		return nil
	}
	sequence := m.setupSequenceID
	lastIndex := len(m.repairScreen.Tasks) - 1
	cmds := make([]tea.Cmd, 0, lastIndex)
	for index := 0; index < lastIndex; index++ {
		cmds = append(cmds, m.completeSetupTaskAfter(sequence, index, m.setupStageDelay()))
	}
	return tea.Batch(cmds...)
}

func (m *model) completeSetupTaskAfter(sequence int, index int, delay time.Duration) tea.Cmd {
	if delay <= 0 {
		delay = 3 * time.Second
	}
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return setupTaskDoneMsg{sequence: sequence, index: index}
	})
}

func (m *model) setupStageDelay() time.Duration {
	if m.random == nil {
		return 5 * time.Second
	}
	return time.Duration(3+m.random.Intn(6)) * time.Second
}

func (m *model) finalizeSetupAfter(delay time.Duration) tea.Cmd {
	if delay <= 0 {
		delay = time.Second
	}
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return setupFinalizeMsg{sequence: m.setupSequenceID}
	})
}

func (m *model) completeSetupTasks() {
	for index := range m.repairScreen.Tasks {
		m.repairScreen.Tasks[index].State = repairscreen.TaskDone
	}
	m.setupStageIndex = len(m.repairScreen.Tasks) - 1
	m.setSetupStatusForStage()
}

func (m *model) advanceSetupStatus() {
	if len(m.repairScreen.Tasks) == 0 {
		return
	}
	lastIndex := len(m.repairScreen.Tasks) - 1
	if m.setupStageIndex < lastIndex {
		m.setupStageIndex++
	}
	m.setSetupStatusForStage()
}

func (m *model) setSetupStatusForStage() {
	if len(m.repairScreen.Tasks) == 0 {
		m.repairScreen.SetupStatus = ""
		return
	}
	lastIndex := len(m.repairScreen.Tasks) - 1
	stageIndex := m.setupStageIndex
	if stageIndex < 0 {
		stageIndex = 0
	}
	if stageIndex > lastIndex {
		stageIndex = lastIndex
	}
	status := repairscreen.SetupStatusLabel(m.repairScreen.Tasks[stageIndex].Label)
	if strings.TrimSpace(status) == "" {
		status = "Preparing secure access"
	}
	m.repairScreen.SetupStatus = status
}

func (m *model) finishRepairTasks(result readiness.RunResult) {
	if m.isSetupSequence() {
		for index := range m.repairScreen.Tasks {
			task := &m.repairScreen.Tasks[index]
			if result.Snapshot.Service.Running {
				task.State = repairscreen.TaskDone
				continue
			}
			if task.Label == "Service" {
				task.State = repairscreen.TaskActive
			} else {
				task.State = repairscreen.TaskDone
			}
		}
		return
	}
	for index := range m.repairScreen.Tasks {
		task := &m.repairScreen.Tasks[index]
		switch task.Label {
		case "Password":
			task.State = repairscreen.TaskDone
		case "Account":
			if result.Snapshot.LoggedIn || m.repairAuthEmail != "" {
				task.State = repairscreen.TaskDone
			}
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
		}
	}
}

func (m *model) markActiveRepairTaskFailed() {
	for index := range m.repairScreen.Tasks {
		if m.repairScreen.Tasks[index].State == repairscreen.TaskActive {
			m.repairScreen.Tasks[index].State = repairscreen.TaskFailed
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
