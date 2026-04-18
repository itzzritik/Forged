package tui

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui/components"
	accountscreen "github.com/itzzritik/forged/cli/internal/tui/screens/account"
	commonscreen "github.com/itzzritik/forged/cli/internal/tui/screens/common"
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
	passwordKeyView passwordFlow = "key-view"
	passwordKeyExport passwordFlow = "key-export"
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

type dashboardSectionAction struct {
	Label       string
	Description string
}

type dashboardPage struct {
	Label   string
	Summary string
	Route   RouteID
}

type dashboardTab struct {
	Label string
	Pages []dashboardPage
}

type dashboardSection struct {
	Title   string
	Context string
	Actions []dashboardSectionAction
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
	dashboardTabIndex int
	dashboardPageIndices []int
	accountEmail     string
	loginScreen      accountscreen.LoginScreen
	passwordInput    *components.PasswordInput
	passwordFlow     passwordFlow
	passwordTitle    string
	passwordContext  string
	passwordAuth     string
	passwordBusy     bool
	passwordOverlay  bool
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

	keyListID   int
	keyDetailID int
	keyRenameID int
	keyDeleteID int
	keyGenerateID int
	keyImportID int
	keyExportID int
	keyImportPickerID int
	keyExportPickerID int

	keyBrowser keyBrowserState
	keyDetail  keyDetailState
	keyRename  keyRenameState
	keyDelete  keyDeleteState
	keyGenerate keyGenerateState
	keyImport   keyImportState
	keyExport   keyExportState
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
	model := &model{
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
	model.initializePendingRouteState()
	return model
}

func (m *model) initializePendingRouteState() {
	switch m.intent.Entry {
	case RouteKeysDetail:
		m.keyDetail.resolving = true
	case RouteKeysRename:
		m.keyRename.resolving = true
	case RouteKeysDelete:
		m.keyDelete.resolving = true
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
		cmds := []tea.Cmd{m.spinner.Tick, m.assessCurrentState()}
		if m.intent.Entry == RouteKeysBrowser {
			m.keyBrowser = keyBrowserState{
				loading: true,
				input:   newKeyInput("Search keys"),
			}
			route := m.session.Current()
			if query := strings.TrimSpace(route.Params["query"]); query != "" {
				m.keyBrowser.input.SetValue(query)
			}
			if route.Params["search"] == "true" {
				m.keyBrowser.searchActive = true
				m.keyBrowser.input.Focus()
				cmds = append(cmds, textinput.Blink)
			}
			cmds = append(cmds, m.listLocalKeys(m.nextKeyListID(), false))
		}
		return tea.Batch(cmds...)
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		if m.passwordInput != nil {
			m.passwordInput.SetWidth(max(18, shell.ClampBlockWidth(m.width, 40)-4))
		}
		m.resizeKeyInputs()
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
		return m, tea.Batch(
			m.startStartupRepair(),
			m.preloadKeyBrowser(),
		)
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
		wasUsingSpinner := m.usesSpinner()
		if msg.err == nil {
			m.runtimeStatus = msg.status
			m.runtimeLoaded = true
		} else if m.snapshot.LoggedIn && m.systemHeader == systemHeaderHealthy {
			m.runtimeStatus = RuntimeStatus{Error: msg.err.Error()}
			m.runtimeLoaded = true
		}
		if m.screen == screenDashboard && m.snapshot.VaultExists {
			cmds := []tea.Cmd{m.pollRuntimeStatus(time.Second)}
			if !wasUsingSpinner && m.usesSpinner() {
				cmds = append([]tea.Cmd{m.spinner.Tick}, cmds...)
			}
			return m, tea.Batch(cmds...)
		}
		return m, nil
	case keyListMsg:
		return m.handleKeyListMsg(msg)
	case keyDetailMsg:
		return m.handleKeyDetailMsg(msg)
	case keyRenameFinishedMsg:
		return m.handleKeyRenameFinishedMsg(msg)
	case keyDeleteFinishedMsg:
		return m.handleKeyDeleteFinishedMsg(msg)
	case keyCopyFinishedMsg:
		return m.handleKeyCopyFinishedMsg(msg)
	case keyPrivateCopyFinishedMsg:
		return m.handleKeyPrivateCopyFinishedMsg(msg)
	case keyGenerateFinishedMsg:
		return m.handleKeyGenerateFinishedMsg(msg)
	case keyImportFinishedMsg:
		return m.handleKeyImportFinishedMsg(msg)
	case keyExportFinishedMsg:
		return m.handleKeyExportFinishedMsg(msg)
	case keyImportPickerMsg:
		return m.handleKeyImportPickerMsg(msg)
	case keyExportPickerMsg:
		return m.handleKeyExportPickerMsg(msg)
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
	tightFooter := m.isKeyRoute() && m.session.Current().ID == RouteKeysBrowser
	tightBody := (m.isKeyRoute() && m.session.Current().ID == RouteKeysBrowser) || m.isTabbedDashboardRoot()
	return shell.Render(m.width, header, body, footer, tightFooter, tightBody)
}

func (m *model) isTabbedDashboardRoot() bool {
	return m.bootAssessed &&
		m.screen == screenDashboard &&
		m.snapshot.VaultExists &&
		!m.isWelcomeState() &&
		!m.isKeyRoute() &&
		m.currentDashboardSection() == nil &&
		m.session.Current().ID == RouteDashboardHome &&
		len(m.dashboardTabs()) > 0
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
		return []shell.StatusItem{
			{Label: "Checking health", Icon: m.spinner.View()},
			m.commitSigningHeaderItem(),
			{Label: "Loading vault", Icon: m.spinner.View()},
		}
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
		return shell.StatusItem{Label: "Restoring Health", Icon: m.spinner.View()}
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
	if !m.bootAssessed {
		if title := m.pendingDashboardRouteTitle(); title != "" {
			return title
		}
		return ""
	}
	if m.isWelcomeState() {
		return ""
	}
	if m.screen == screenDashboard && m.snapshot.VaultExists {
		if m.isKeyRoute() {
			return m.keyHeaderTitle()
		}
		if section := m.currentDashboardSection(); section != nil {
			return section.Title
		}
	}

	switch m.screen {
	case screenLogin:
		if strings.TrimSpace(m.loginScreen.Title) != "" {
			return m.loginScreen.Title
		}
		return "Sign In to Sync Vault"
	case screenPassword:
		switch m.passwordFlow {
		case passwordKeyView:
			return "Unlock private key"
		case passwordKeyExport:
			return "Export vault"
		}
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
	if !m.bootAssessed {
		return m.pendingDashboardRouteBreadcrumbs()
	}

	if m.isWelcomeState() {
		return nil
	}

	if m.screen == screenDashboard && m.snapshot.VaultExists {
		if m.isKeyRoute() {
			return m.keyBreadcrumbs()
		}
		if section := m.currentDashboardSection(); section != nil {
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: section.Title, Current: true},
			}
		}
	}

	switch m.screen {
	case screenLogin:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Account"},
			{Label: "Sign In", Current: true},
		}
	case screenPassword:
		switch m.passwordFlow {
		case passwordKeyView:
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Key"},
				{Label: "View", Current: true},
			}
		case passwordKeyExport:
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Key"},
				{Label: "Export", Current: true},
			}
		}
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
	if m.screen != screenDashboard || !m.snapshot.VaultExists || m.isWelcomeState() || m.currentDashboardSection() != nil || m.isKeyRoute() {
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
			return m.renderPendingBody(contentWidth)
		}
		if m.isKeyRoute() {
			if m.session.Current().ID == RouteKeysBrowser && m.keyBrowser.loading && len(m.keyBrowser.all) == 0 {
				return commonscreen.RenderFullPageLoader(commonscreen.FullPageLoaderScreen{
					Title:       "Loading keys",
					Description: "Reading keys from this vault",
				}, m.spinner.View(), contentWidth)
			}
			if !m.keyRouteLoaded() {
				if m.session.Current().ID == RouteKeysDetail && m.keyDetail.resolving {
					return m.renderKeyBody(contentWidth)
				}
				return ""
			}
			return m.renderKeyBody(contentWidth)
		}
		if section := m.currentDashboardSection(); section != nil {
			return m.renderDashboardSection(contentWidth, *section)
		}
		tabs, pages, summary := m.dashboardRootScreen()
		return dashboardscreen.Render(dashboardscreen.Screen{
			Title:   m.dashboardBodyTitle(),
			Context: m.dashboardLead(),
			Options: m.dashboardOptions(),
			Tabs:    tabs,
			Pages:   pages,
			Summary: summary,
			Notice: dashboardscreen.Notice{
				Message: m.notice.message,
				Tone:    m.notice.tone,
			},
		}, contentWidth)
	}
}

func (m *model) renderPendingBody(contentWidth int) string {
	switch m.session.Current().ID {
	case RouteKeysDetail:
		return m.renderKeyBody(contentWidth)
	case RouteKeysBrowser:
		return m.renderKeyBody(contentWidth)
	case RouteKeysRename:
		return commonscreen.RenderFullPageLoader(commonscreen.FullPageLoaderScreen{
			Title:       "Loading key",
			Description: "Preparing the selected key for rename",
		}, m.spinner.View(), contentWidth)
	case RouteKeysDelete:
		return commonscreen.RenderFullPageLoader(commonscreen.FullPageLoaderScreen{
			Title:       "Loading key",
			Description: "Preparing the selected key for deletion",
		}, m.spinner.View(), contentWidth)
	case RouteKeysGenerate:
		return commonscreen.RenderFullPageLoader(commonscreen.FullPageLoaderScreen{
			Title:       "Opening generate key",
			Description: "Preparing the key creation flow",
		}, m.spinner.View(), contentWidth)
	case RouteKeysImport:
		return commonscreen.RenderFullPageLoader(commonscreen.FullPageLoaderScreen{
			Title:       "Opening import keys",
			Description: "Preparing the key import flow",
		}, m.spinner.View(), contentWidth)
	case RouteKeysExport:
		return commonscreen.RenderFullPageLoader(commonscreen.FullPageLoaderScreen{
			Title:       "Opening export vault",
			Description: "Preparing the vault export flow",
		}, m.spinner.View(), contentWidth)
	default:
		return commonscreen.RenderFullPageLoader(commonscreen.FullPageLoaderScreen{
			Title:       "Opening Forged",
			Description: "Checking local health and preparing this machine",
		}, m.spinner.View(), contentWidth)
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

func (m *model) renderDashboardSection(contentWidth int, section dashboardSection) string {
	sections := make([]string, 0, len(section.Actions)*4+2)

	if strings.TrimSpace(section.Context) != "" {
		sections = append(sections, theme.Body.Width(max(28, min(contentWidth, theme.HeroMaxWidth))).Render(section.Context))
	}

	if len(section.Actions) > 0 {
		if len(sections) > 0 {
			sections = append(sections, "")
		}
		for index, action := range section.Actions {
			sections = append(sections, theme.Bullet.Render("•")+" "+theme.BodyStrong.Render(action.Label))
			if strings.TrimSpace(action.Description) != "" {
				desc := theme.BodyMuted.Width(max(24, min(contentWidth-2, theme.HeroMaxWidth))).Render(action.Description)
				sections = append(sections, shell.IndentBlock(desc, 2))
			}
			if index < len(section.Actions)-1 {
				sections = append(sections, "")
			}
		}
	}

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
		if m.isKeyRoute() {
			return m.keyFooterActions()
		}
		if m.isWelcomeState() {
			return []shell.FooterAction{
				{Key: "↑/↓", Label: "Move"},
				{Key: "Enter", Label: "Select"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if m.currentDashboardSection() != nil {
			return []shell.FooterAction{
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if tabs := m.dashboardTabs(); len(tabs) > 0 {
			return []shell.FooterAction{
				{Key: "←/→", Label: "Tabs"},
				{Key: "↑/↓", Label: "Pages"},
				{Key: "Enter", Label: "Open"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
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

	if m.isKeyRoute() {
		return m.updateKeyKeys(msg)
	}

	if m.currentDashboardSection() != nil {
		switch msg.String() {
		case "esc":
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		}
		return m, nil
	}

	if tabs := m.dashboardTabs(); len(tabs) > 0 {
		m.normalizeDashboardSelection(tabs)
		switch msg.String() {
		case "esc":
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		case "up", "k":
			pageIndex := m.dashboardPageIndices[m.dashboardTabIndex]
			if pageIndex > 0 {
				m.dashboardPageIndices[m.dashboardTabIndex]--
			}
			return m, nil
		case "down", "j":
			pages := tabs[m.dashboardTabIndex].Pages
			pageIndex := m.dashboardPageIndices[m.dashboardTabIndex]
			if len(pages) > 0 && pageIndex < len(pages)-1 {
				m.dashboardPageIndices[m.dashboardTabIndex]++
			}
			return m, nil
		case "left", "h":
			if m.dashboardTabIndex > 0 {
				m.dashboardTabIndex--
			}
			return m, nil
		case "right", "l":
			if m.dashboardTabIndex < len(tabs)-1 {
				m.dashboardTabIndex++
			}
			return m, nil
		case "enter":
			if page := m.selectedDashboardPage(); page != nil {
				if page.Route != "" && m.session.Current().ID != page.Route {
					m.session.Push(Route{ID: page.Route})
				}
				return m, m.showCurrentRoute()
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
		if m.passwordOverlay {
			if m.passwordFlow == passwordKeyView {
				m.keyDetail.busy = false
			}
			if m.passwordFlow == passwordKeyExport {
				m.keyExport.exporting = false
			}
			m.passwordOverlay = false
			m.passwordBusy = false
			m.passwordAuth = ""
			m.screen = screenDashboard
			return m, nil
		}
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
		case passwordKeyView:
			m.passwordBusy = true
			m.passwordInput.SetInfo("Decrypting vault")
			return m, tea.Batch(m.spinner.Tick, m.copyPrivateKey(password))
		case passwordKeyExport:
			m.passwordBusy = true
			m.passwordInput.SetInfo("Exporting vault")
			return m, tea.Batch(m.spinner.Tick, m.exportVault(password))
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

func (m *model) dashboardTabs() []dashboardTab {
	if !m.snapshot.VaultExists {
		return nil
	}

	accountPages := []dashboardPage{
		{
			Label:   "Log in",
			Summary: "Log in to Forged and sync keys across all devices",
			Route:   RouteAccountLogin,
		},
	}
	syncPages := []dashboardPage{
		{
			Label:   "Log in",
			Summary: "Connect this machine to enable vault sync",
			Route:   RouteAccountLogin,
		},
	}

	if m.snapshot.LoggedIn {
		accountPages = []dashboardPage{
			{
				Label:   "Profile",
				Summary: "View account details and profile settings",
				Route:   RouteAccountStatus,
			},
			{
				Label:   "Log out",
				Summary: "Log out of Forged while keeping your local vault available",
				Route:   RouteAccountStatus,
			},
		}
		syncPages = []dashboardPage{
			{
				Label:   "Status",
				Summary: "Check sync health and recent activity",
				Route:   RouteSyncHome,
			},
			{
				Label:   "Sync now",
				Summary: "Run a fresh sync for this vault",
				Route:   RouteSyncHome,
			},
		}
	}

	return []dashboardTab{
		{
			Label: "Key",
			Pages: []dashboardPage{
				{Label: "View", Summary: "Browse keys, open details, and manage them", Route: RouteKeysBrowser},
				{Label: "Generate", Summary: "Create a new key in this vault", Route: RouteKeysGenerate},
				{Label: "Import", Summary: "Bring existing keys into this vault", Route: RouteKeysImport},
				{Label: "Export", Summary: "Export keys from this vault", Route: RouteKeysExport},
			},
		},
		{
			Label: "Vault",
			Pages: []dashboardPage{
				{Label: "Unlock", Summary: "Unlock this vault with your master password", Route: RouteVaultHome},
				{Label: "Lock", Summary: "Lock the vault until your password is entered again", Route: RouteVaultHome},
				{Label: "Change password", Summary: "Change the master password that protects this vault", Route: RouteVaultHome},
			},
		},
		{
			Label: "Agent",
			Pages: []dashboardPage{
				{Label: "Enable SSH agent", Summary: "Set Forged as your default SSH agent", Route: RouteAgentHome},
				{Label: "Disable SSH agent", Summary: "Turn off Forged SSH routing on this machine", Route: RouteAgentHome},
				{Label: "Commit signing", Summary: "Sign Git commits using keys in your Forged vault", Route: RouteAgentHome},
			},
		},
		{
			Label: "Account",
			Pages: accountPages,
		},
		{
			Label: "Sync",
			Pages: syncPages,
		},
		{
			Label: "Doctor",
			Pages: []dashboardPage{
				{Label: "Overview", Summary: "Review system health and current checks", Route: RouteDoctorOverview},
				{Label: "Fix issues", Summary: "Repair runtime issues on this machine", Route: RouteDoctorOverview},
			},
		},
	}
}

func (m *model) normalizeDashboardSelection(tabs []dashboardTab) {
	if len(tabs) == 0 {
		m.dashboardTabIndex = 0
		m.dashboardPageIndices = nil
		return
	}

	if m.dashboardTabIndex < 0 {
		m.dashboardTabIndex = 0
	}
	if m.dashboardTabIndex >= len(tabs) {
		m.dashboardTabIndex = len(tabs) - 1
	}

	if len(m.dashboardPageIndices) < len(tabs) {
		m.dashboardPageIndices = append(m.dashboardPageIndices, make([]int, len(tabs)-len(m.dashboardPageIndices))...)
	}
	if len(m.dashboardPageIndices) > len(tabs) {
		m.dashboardPageIndices = m.dashboardPageIndices[:len(tabs)]
	}

	for index, tab := range tabs {
		if len(tab.Pages) == 0 {
			m.dashboardPageIndices[index] = 0
			continue
		}
		if m.dashboardPageIndices[index] < 0 {
			m.dashboardPageIndices[index] = 0
		}
		if m.dashboardPageIndices[index] >= len(tab.Pages) {
			m.dashboardPageIndices[index] = len(tab.Pages) - 1
		}
	}
}

func (m *model) selectedDashboardPage() *dashboardPage {
	tabs := m.dashboardTabs()
	if len(tabs) == 0 {
		return nil
	}
	m.normalizeDashboardSelection(tabs)
	pages := tabs[m.dashboardTabIndex].Pages
	if len(pages) == 0 {
		return nil
	}
	page := pages[m.dashboardPageIndices[m.dashboardTabIndex]]
	return &page
}

func (m *model) dashboardRootScreen() ([]dashboardscreen.Tab, []dashboardscreen.Page, string) {
	tabs := m.dashboardTabs()
	if len(tabs) == 0 {
		return nil, nil, ""
	}
	m.normalizeDashboardSelection(tabs)

	tabItems := make([]dashboardscreen.Tab, 0, len(tabs))
	for index, tab := range tabs {
		tabItems = append(tabItems, dashboardscreen.Tab{
			Label:    tab.Label,
			Selected: index == m.dashboardTabIndex,
		})
	}

	pages := tabs[m.dashboardTabIndex].Pages
	pageItems := make([]dashboardscreen.Page, 0, len(pages))
	for index, page := range pages {
		pageItems = append(pageItems, dashboardscreen.Page{
			Label:    page.Label,
			Selected: index == m.dashboardPageIndices[m.dashboardTabIndex],
		})
	}

	summary := ""
	if page := m.selectedDashboardPage(); page != nil {
		summary = page.Summary
	}
	return tabItems, pageItems, summary
}

func (m *model) dashboardAreaRoute(label string) RouteID {
	switch label {
	case "Key":
		return RouteKeysBrowser
	case "Vault":
		return RouteVaultHome
	case "Agent":
		return RouteAgentHome
	case "Account":
		if !m.snapshot.LoggedIn {
			return RouteAccountLogin
		}
		return RouteAccountStatus
	case "Sync":
		return RouteSyncHome
	case "Doctor":
		return RouteDoctorOverview
	default:
		return ""
	}
}

func (m *model) currentDashboardSection() *dashboardSection {
	if m.screen != screenDashboard || !m.snapshot.VaultExists {
		return nil
	}

	switch m.session.Current().ID {
	case RouteVaultHome:
		return &dashboardSection{
			Title:   "Vault",
			Context: "Control how this machine unlocks, protects, and changes the local encrypted vault.",
			Actions: []dashboardSectionAction{
				{Label: "Unlock vault", Description: "Open the vault and enable sensitive access"},
				{Label: "Lock vault", Description: "Seal sensitive access until the master password is entered again"},
				{Label: "Change password", Description: "Rotate the password used to encrypt this vault"},
			},
		}
	case RouteAgentHome:
		return &dashboardSection{
			Title:   "Agent",
			Context: "Manage SSH routing, agent ownership, and commit-signing behavior across developer workflows.",
			Actions: []dashboardSectionAction{
				{Label: "Enable SSH agent", Description: "Route OpenSSH through Forged on this machine"},
				{Label: "Disable SSH agent", Description: "Stop routing SSH requests through Forged"},
				{Label: "Commit signing", Description: "Review and control how Forged signs Git commits"},
			},
		}
	case RouteAccountStatus:
		context := "Review your Forged account and manage the features linked to this machine."
		if email := strings.TrimSpace(m.accountEmail); email != "" {
			context = "Signed in as " + email + " and ready to manage linked account access on this machine."
		}
		actions := []dashboardSectionAction{
			{Label: "Profile", Description: "Review signed-in account identity and linked access"},
		}
		if m.snapshot.LoggedIn {
			actions = append(actions, dashboardSectionAction{Label: "Log out", Description: "Remove linked account access from this machine"})
		}
		return &dashboardSection{
			Title:   "Account",
			Context: context,
			Actions: actions,
		}
	case RouteSyncHome:
		context := "Keep this machine aligned with your linked Forged vault and review the current sync state."
		actions := []dashboardSectionAction{
			{Label: "Sync status", Description: "Review the current linked sync state"},
			{Label: "Sync now", Description: "Trigger a fresh sync when account features are enabled"},
		}
		if !m.snapshot.LoggedIn {
			context = "Sign in to enable multi-device vault sync and linked recovery features."
			actions = []dashboardSectionAction{
				{Label: "Sign in", Description: "Connect this machine to your Forged account"},
			}
		}
		return &dashboardSection{
			Title:   "Sync",
			Context: context,
			Actions: actions,
		}
	case RouteDoctorOverview:
		return &dashboardSection{
			Title:   "Doctor",
			Context: "Inspect runtime health, check the local service, and surface issues that need attention on this machine.",
			Actions: []dashboardSectionAction{
				{Label: "Health overview", Description: "Review the current machine health contract"},
				{Label: "Fix issues", Description: "Run the guided repair flow when the machine needs attention"},
			},
		}
	default:
		return nil
	}
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
			Label:   "Key",
			Summary: "Browse, create, import, and export keys",
		},
		{
			Label:   "Vault",
			Summary: "Lock, unlock, and protect this machine",
		},
		{
			Label:   "Agent",
			Summary: "Control SSH routing and signing",
		},
		{
			Label:   "Account",
			Summary: "Profile, access, and linked features",
		},
		{
			Label:   "Sync",
			Summary: "Refresh and review vault sync",
		},
		{
			Label:   "Doctor",
			Summary: "Inspect health and fix issues",
		},
	}

	if m.snapshot.KeyCount == 0 {
		areas[0].Summary = "Browse, create, import, and export keys"
	} else if m.snapshot.KeyCount == 1 {
		areas[0].Summary = "1 key ready to view, export, or manage"
	} else {
		areas[0].Summary = fmt.Sprintf("%d keys ready to view, export, or manage", m.snapshot.KeyCount)
	}

	if m.snapshot.LoggedIn {
		areas[1].Summary = "Lock, unlock, and protect your synced vault"
	}

	if m.snapshot.LoggedIn {
		areas[3].Summary = "Profile, session, and linked features"
		areas[4].Summary = "Refresh and review vault sync"
	} else {
		areas[4].Summary = "Review sync once account access is enabled"
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
		return m.keyUsesSpinner() || m.systemHeader == systemHeaderChecking || m.systemHeader == systemHeaderFixing || m.runtimeStatus.Syncing
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
				if route := m.session.Current().ID; route != "" && route != RouteDashboardHome {
					return tea.Batch(
						m.showCurrentRoute(),
						m.pollRuntimeStatus(0),
					)
				}
				return tea.Batch(
					m.pollRuntimeStatus(0),
					m.preloadKeyBrowser(),
				)
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
	if m.session.Current().ID == RouteAccountLogin {
		m.session.ReplaceCurrent(Route{ID: RouteDashboardHome})
	}
	m.bootAssessed = false
	m.systemHeader = systemHeaderChecking
	m.setupVariant = setupVariantNone
	m.repairPurpose = repairPurposeStartup
	m.repairUsedPassword = false
	m.repairAuthEmail = ""
	m.setupPending = nil
	m.runtimeLoaded = false
	return tea.Batch(
		m.spinner.Tick,
		m.assessCurrentState(),
	)
}

func (m *model) showPasswordScreen(flow passwordFlow, authEmail string, errorText string, reuseCurrentRoute bool) {
	if !reuseCurrentRoute && m.session.Current().ID != RouteVaultUnlock {
		m.session.Push(Route{ID: RouteVaultUnlock})
	}

	m.screen = screenPassword
	m.passwordFlow = flow
	m.passwordAuth = authEmail
	m.passwordBusy = false
	m.passwordOverlay = reuseCurrentRoute && (flow == passwordKeyView || flow == passwordKeyExport)
	switch flow {
	case passwordCreate:
		m.passwordTitle = "Create local vault"
		m.passwordContext = "Set an encryption password for your vault. Save it securely. If you lose it, your keys are lost."
		m.passwordInput = components.NewCreatePasswordInput()
	case passwordRestore:
		m.passwordTitle = "Unlock your vault"
		m.passwordContext = "Master password is required to decrypt this vault and unlock its keys"
		m.passwordInput = components.NewUnlockPasswordInput()
	case passwordKeyView:
		m.passwordTitle = "Unlock private key"
		m.passwordContext = "Master password is required to decrypt this vault and copy its private key"
		m.passwordInput = components.NewUnlockPasswordInput()
	case passwordKeyExport:
		m.passwordTitle = "Export vault"
		m.passwordContext = "Master password is required to export this vault and its private keys"
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
	case RouteKeysBrowser, RouteKeysDetail, RouteKeysRename, RouteKeysDelete, RouteKeysGenerate, RouteKeysImport, RouteKeysExport:
		return m.startKeyRouteLoad()
	case RouteVaultHome, RouteAgentHome, RouteAccountStatus, RouteSyncHome, RouteDoctorOverview:
		return nil
	default:
		return nil
	}
}

func (m *model) pendingDashboardRouteTitle() string {
	if m.screen != screenDashboard {
		return ""
	}

	switch m.session.Current().ID {
	case RouteKeysBrowser:
		return "View keys"
	case RouteKeysDetail:
		return "View key"
	case RouteKeysRename:
		return "Rename key"
	case RouteKeysDelete:
		return "Delete key"
	case RouteKeysGenerate:
		return "Generate key"
	case RouteKeysImport:
		return "Import keys"
	case RouteKeysExport:
		return "Export vault"
	case RouteVaultHome:
		return "Vault"
	case RouteAgentHome:
		return "Agent"
	case RouteAccountStatus:
		return "Account"
	case RouteSyncHome:
		return "Sync"
	case RouteDoctorOverview:
		return "Doctor"
	default:
		return ""
	}
}

func (m *model) pendingDashboardRouteBreadcrumbs() []shell.Breadcrumb {
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
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Key"},
			{Label: "Rename", Current: true},
		}
	case RouteKeysDelete:
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
	case RouteVaultHome:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Vault", Current: true},
		}
	case RouteAgentHome:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Agent", Current: true},
		}
	case RouteAccountStatus:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Account", Current: true},
		}
	case RouteSyncHome:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Sync", Current: true},
		}
	case RouteDoctorOverview:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Doctor", Current: true},
		}
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
