package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui/components"
	accountscreen "github.com/itzzritik/forged/cli/internal/tui/screens/account"
	commonscreen "github.com/itzzritik/forged/cli/internal/tui/screens/common"
	dashboardscreen "github.com/itzzritik/forged/cli/internal/tui/screens/dashboard"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type ExitAction string

const ExitNone ExitAction = ""

type Result struct {
	Action ExitAction
}

const (
	tuiIdleLockTimeout            = 4 * time.Minute
	runtimeStatusFailureThreshold = 2
)

type Dependencies struct {
	Repair                    func(readiness.RunOptions) (readiness.RunResult, error)
	CreateVault               func([]byte) error
	RestoreVault              func([]byte) error
	StartLogin                func(context.Context, string, func(actions.LoginProgress)) (actions.LoginSession, error)
	SaveCredentials           func(actions.AccountCredentials) error
	TriggerSync               func() error
	LockSensitive             func() error
	LoadStatus                func() (RuntimeStatus, error)
	LoadSecurityState         func() (SecurityState, error)
	SetMasterPasswordInterval func(string) error
	HasLocalUnlockTrust       func() bool
	UnlockSensitiveLaunch     func(context.Context, []byte) (actions.UnlockResult, error)
	ChangePassword            func([]byte, []byte) (actions.ChangePasswordResult, error)
	LoadSigningStatus         func() (actions.CommitSigningStatus, error)
	EnableSSHAgent            func() error
	DisableSSHAgent           func() error
	EnableCommitSigning       func(string) (actions.CommitSigningStatus, error)
	DisableCommitSigning      func() (actions.CommitSigningStatus, error)
	LoadSSHRoutingDebug       func() (actions.SSHRoutingDebug, error)
	ClearSSHRoute             func(string) error
	ClearAllSSHRoutes         func() error
	CopyText                  func(string) error
	CopySensitiveText         func(string) (SensitiveClipboardLease, error)
	CloseClipboard            func() error
	OpenLink                  func(string) error
	LogError                  func(actions.DiagnosticErrorEvent)
	DefaultServer             string
	AppVersion                string
}

func (d Dependencies) validate() error {
	required := []struct {
		name    string
		missing bool
	}{
		{name: "repair", missing: d.Repair == nil},
		{name: "create-vault", missing: d.CreateVault == nil},
		{name: "restore-vault", missing: d.RestoreVault == nil},
		{name: "log-in", missing: d.StartLogin == nil},
		{name: "save-credentials", missing: d.SaveCredentials == nil},
		{name: "trigger-sync", missing: d.TriggerSync == nil},
		{name: "lock-sensitive", missing: d.LockSensitive == nil},
		{name: "load-status", missing: d.LoadStatus == nil},
		{name: "load-security-state", missing: d.LoadSecurityState == nil},
		{name: "set-master-password-interval", missing: d.SetMasterPasswordInterval == nil},
		{name: "local-unlock-trust", missing: d.HasLocalUnlockTrust == nil},
		{name: "launch-unlock", missing: d.UnlockSensitiveLaunch == nil},
		{name: "change-password", missing: d.ChangePassword == nil},
		{name: "load-signing-status", missing: d.LoadSigningStatus == nil},
		{name: "enable-SSH-agent", missing: d.EnableSSHAgent == nil},
		{name: "disable-SSH-agent", missing: d.DisableSSHAgent == nil},
		{name: "enable-commit-signing", missing: d.EnableCommitSigning == nil},
		{name: "disable-commit-signing", missing: d.DisableCommitSigning == nil},
		{name: "SSH routing debug", missing: d.LoadSSHRoutingDebug == nil},
		{name: "SSH route clear", missing: d.ClearSSHRoute == nil},
		{name: "SSH routes clear-all", missing: d.ClearAllSSHRoutes == nil},
		{name: "copy-text", missing: d.CopyText == nil},
		{name: "sensitive-copy", missing: d.CopySensitiveText == nil},
		{name: "close-clipboard", missing: d.CloseClipboard == nil},
		{name: "open-link", missing: d.OpenLink == nil},
		{name: "log-error", missing: d.LogError == nil},
	}
	for _, dependency := range required {
		if dependency.missing {
			return fmt.Errorf("TUI %s dependency is required", dependency.name)
		}
	}
	return nil
}

type SensitiveClipboardLease interface {
	ClearIfUnchanged() (bool, error)
}

type screenMode string

const (
	screenDashboard screenMode = "dashboard"
	screenLogin     screenMode = "login"
	screenPassword  screenMode = "password"
)

type passwordFlow string

const (
	passwordCreate        passwordFlow = "create"
	passwordRestore       passwordFlow = "restore"
	passwordRepair        passwordFlow = "repair"
	passwordDoctorRepair  passwordFlow = "doctor-repair"
	passwordKeyView       passwordFlow = "key-view"
	passwordKeyExport     passwordFlow = "key-export"
	passwordStartupUnlock passwordFlow = "startup-unlock"
	passwordManageChange  passwordFlow = "manage-change"
)

type maintenanceTrigger string

const (
	maintenanceTriggerBoot      maintenanceTrigger = "boot"
	maintenanceTriggerSetup     maintenanceTrigger = "setup"
	maintenanceTriggerUnlock    maintenanceTrigger = "unlock"
	maintenanceTriggerPostLogin maintenanceTrigger = "post-login"
	maintenanceTriggerDoctor    maintenanceTrigger = "doctor"
)

type maintenancePolicy string

const (
	maintenancePolicyDefault maintenancePolicy = "default"
	maintenancePolicyDoctor  maintenancePolicy = "doctor"
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
	ctx     context.Context
	session actions.LoginSession
	err     error
}

type loginProgressMsg struct {
	id       int
	progress actions.LoginProgress
}

type loginApprovedMsg struct {
	id    int
	creds actions.AccountCredentials
}

type loginFinishedMsg struct {
	id       int
	creds    actions.AccountCredentials
	err      error
	canceled bool
}

type restoreFinishedMsg struct {
	id       int
	password *passwordBuffer
	err      error
}

type passwordBuffer struct {
	mu    sync.Mutex
	value []byte
}

func newPasswordBuffer(password []byte) *passwordBuffer {
	buffer := &passwordBuffer{value: append([]byte(nil), password...)}
	clear(password)
	return buffer
}

func (b *passwordBuffer) use(fn func([]byte) error) error {
	b.mu.Lock()
	if len(b.value) == 0 {
		b.mu.Unlock()
		return errors.New("password is unavailable")
	}
	working := append([]byte(nil), b.value...)
	b.mu.Unlock()
	defer clear(working)
	return fn(working)
}

func (b *passwordBuffer) take() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	value := b.value
	b.value = nil
	return value
}

func (b *passwordBuffer) clear() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	clear(b.value)
	b.value = nil
}

type maintenanceFinishedMsg struct {
	id        int
	result    readiness.RunResult
	err       error
	unlocked  bool
	unlockErr error
}

type runtimeStatusMsg struct {
	id     int
	status RuntimeStatus
	err    error
}

type idleLockMsg struct{}

type idleLockFinishedMsg struct {
	id  int
	err error
}

type snapshotRefreshMsg struct {
	id       int
	snapshot readiness.Snapshot
	err      error
}

type securityStateMsg struct {
	id    int
	state SecurityState
	err   error
}

type startupUnlockFinishedMsg struct {
	id     int
	result actions.UnlockResult
	err    error
}

type signingStatusMsg struct {
	id     int
	status actions.CommitSigningStatus
	err    error
}

type RuntimeStatus = actions.RuntimeStatus

type SecurityState = actions.SecurityState

const (
	securityCapabilityAvailable             = "available"
	securityCapabilityUnavailableByPlatform = "unavailable_by_platform"
	securityCapabilityUnavailableByEnv      = "unavailable_by_environment"
	securityCapabilityBroken                = "broken"
)

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
}

type systemHeaderState string

const (
	systemHeaderChecking  systemHeaderState = "checking"
	systemHeaderFixing    systemHeaderState = "fixing"
	systemHeaderHealthy   systemHeaderState = "healthy"
	systemHeaderUnhealthy systemHeaderState = "unhealthy"
)

type copyFinishedMsg struct {
	err error
}

type model struct {
	intent         Intent
	session        *Session
	deps           Dependencies
	lifetimeCtx    context.Context
	lifetimeCancel context.CancelFunc
	clipboardBusy  bool
	reportedErrors map[string]reportedError

	signingStatus        actions.CommitSigningStatus
	signingLoaded        bool
	signingStatusLoading bool
	signingStatusPending bool
	signingError         string
	signingLoadID        int

	spinner spinner.Model
	width   int
	height  int
	result  Result

	screen   screenMode
	fatalErr error

	snapshot readiness.Snapshot
	summary  readiness.RepairSummary
	notice   notice

	onboardingCursor     int
	dashboardTabIndex    int
	dashboardPageIndices []int
	accountEmail         string
	accountName          string
	loginScreen          accountscreen.LoginScreen
	passwordInput        *components.PasswordInput
	passwordFlow         passwordFlow
	passwordTitle        string
	passwordContext      string
	passwordAuth         string
	passwordBusy         bool
	passwordHideInput    bool
	passwordBusyMessage  string
	passwordOverlay      bool

	loginID         int
	loginProgress   <-chan actions.LoginProgress
	loginCancel     context.CancelFunc
	loginCommitting bool
	restoreID       int
	restorePassword *passwordBuffer

	maintenanceID           int
	maintenanceBusy         bool
	maintenanceTrigger      maintenanceTrigger
	maintenanceUsedPassword bool
	maintenanceAuthEmail    string

	bootAssessed             bool
	startupUnlockPending     bool
	startupUnlockNeedsRepair bool
	startupUnlockID          int
	startupUnlockCancel      context.CancelFunc
	systemHeader             systemHeaderState
	runtimeStatus            RuntimeStatus
	runtimeLoaded            bool
	runtimeStatusID          int
	runtimeStatusFailures    int
	runtimeUnavailable       bool
	snapshotRefreshID        int
	securityState            SecurityState
	securityLoaded           bool
	securityLoadID           int
	securityLoadErr          string
	doctorOffset             int
	doctorPageRows           int
	idleLockID               int
	idleLockDeadline         time.Time
	idleLockTimerArmed       bool
	idleLockInFlight         bool

	keyListID            int
	keyDetailID          int
	keyRenameID          int
	keyDeleteID          int
	keyGenerateID        int
	keyImportPreviewID   int
	keyImportID          int
	keyExportID          int
	keyImportPickerID    int
	keyExportPickerID    int
	keyTransferSuccessID int

	keyBrowser         keyBrowserState
	keyDetail          keyDetailState
	privateCopyID      int
	privateCopyPending bool
	privateClip        privateClipboardState
	keyRename          keyRenameState
	keyDelete          keyDeleteState
	keyGenerate        keyGenerateState
	keyImport          keyImportState
	keyExport          keyExportState
	manage             manageState
	agent              agentState
	lab                labState
}

func Run(intent Intent, deps Dependencies) (Result, error) {
	if err := deps.validate(); err != nil {
		return Result{}, err
	}

	initial := newModel(intent, deps, components.NewSpinner())
	final, err := tea.NewProgram(initial, tea.WithAltScreen()).Run()
	initial.stopLifetime()
	initial.cancelLoginFlow()
	initial.invalidateStartupUnlock()
	initial.clearRestorePassword()
	initial.discardPasswordInput()
	closeErr := deps.CloseClipboard()
	initial.reportError("tui.run", err)
	initial.reportError("clipboard.close", closeErr)
	if err != nil {
		return Result{}, errors.Join(err, closeErr)
	}
	if closeErr != nil {
		return Result{}, closeErr
	}

	rendered, ok := final.(*model)
	if !ok {
		err := fmt.Errorf("Unexpected TUI model type %T", final)
		initial.reportError("tui.result", err)
		return Result{}, err
	}
	if rendered.fatalErr != nil {
		rendered.reportError("tui.fatal", rendered.fatalErr)
		return Result{}, rendered.fatalErr
	}

	return rendered.result, nil
}

func newModel(intent Intent, deps Dependencies, spin spinner.Model) *model {
	lifetimeCtx, lifetimeCancel := context.WithCancel(context.Background())
	model := &model{
		intent:         intent,
		session:        NewSession(intent),
		deps:           deps,
		lifetimeCtx:    lifetimeCtx,
		lifetimeCancel: lifetimeCancel,
		spinner:        spin,
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
			Title:   "Log In to Sync Vault",
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
		}
		return tea.Batch(cmds...)
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.passwordInput != nil {
			m.passwordInput.SetWidth(max(18, shell.ClampBlockWidth(m.width, 40)-4))
		}
		m.resizeKeyInputs()
		m.resizeAgentInputs()
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
		m.startupUnlockPending = false
		m.startupUnlockNeedsRepair = false
		if msg.err != nil {
			errorText := m.reportError("startup.assess", msg.err)
			if m.screen == screenLogin {
				m.loginScreen.Waiting = false
				m.loginScreen.Error = errorText
				m.loginScreen.Status = ""
				return m, nil
			}
			m.systemHeader = systemHeaderUnhealthy
			m.notice = notice{message: errorText, tone: dashboardscreen.ToneDanger}
			return m, nil
		}
		m.snapshot = msg.snapshot
		m.runtimeLoaded = false
		if runtimePathError := strings.TrimSpace(msg.snapshot.RuntimePathError); runtimePathError != "" {
			m.enterRuntimePathRecovery(runtimePathError)
			return m, nil
		}
		if m.screen == screenLogin {
			return m, m.startLoginFlow()
		}
		if !msg.snapshot.VaultExists {
			m.notice = notice{}
			m.summary = readiness.RepairSummary{}
			m.maintenanceAuthEmail = ""
			m.maintenanceUsedPassword = false
			m.discardPasswordInput()
			m.screen = screenDashboard
			if m.isDoctorOverviewRoute() {
				m.systemHeader = m.systemHeaderForSnapshot(msg.snapshot)
				return m, tea.Batch(m.loadSecurityStateCmd(), m.invalidateSigningStatusCmd())
			}
			m.systemHeader = systemHeaderHealthy
			return m, nil
		}
		m.discardPasswordInput()
		m.screen = screenDashboard
		m.systemHeader = systemHeaderChecking
		needsRepair := msg.snapshot.State != readiness.StateReady &&
			msg.snapshot.State != readiness.StateReadyEmpty
		if needsRepair {
			m.startupUnlockPending = true
			m.startupUnlockNeedsRepair = false
			return m, m.startStartupRepair()
		}
		if msg.snapshot.IPCSocketReady {
			m.startupUnlockPending = true
			m.startupUnlockNeedsRepair = false
			return m, m.startStartupUnlockFlow()
		}
		m.startupUnlockPending = true
		return m, m.startStartupRepair()
	case loginStartedMsg:
		if msg.id != m.loginID || m.screen != screenLogin {
			return m, nil
		}
		m.loginProgress = nil
		if msg.err != nil {
			m.cancelLoginFlow()
			errorText := m.reportError("login.start", msg.err)
			m.loginScreen.Waiting = false
			m.loginScreen.Error = errorText
			m.loginScreen.Status = ""
			return m, nil
		}
		m.loginScreen = accountscreen.LoginScreen{
			Title:            "Log In to Sync Vault",
			Context:          "Verify this code in your browser before approving.",
			Status:           "Waiting for browser approval",
			VerificationCode: msg.session.VerificationCode,
			URL:              msg.session.URL,
			Waiting:          true,
		}
		if err := m.openCurrentLoginURL(); err != nil {
			m.loginScreen.Status = "Open the approval link with Enter or copy it with C."
		}

		return m, m.waitForLogin(msg.ctx, msg.id, msg.session)
	case startupUnlockFinishedMsg:
		return m, m.handleStartupUnlockFinishedMsg(msg)
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
	case loginApprovedMsg:
		if msg.id != m.loginID || m.screen != screenLogin {
			return m, nil
		}
		m.cancelLoginFlow()
		m.loginCommitting = true
		m.loginScreen.Waiting = true
		m.loginScreen.Status = "Saving account securely"
		m.loginScreen.Error = ""
		return m, m.commitLogin(msg.id, msg.creds)
	case loginFinishedMsg:
		if msg.id != m.loginID {
			return m, nil
		}
		m.loginCommitting = false
		m.cancelLoginFlow()
		if msg.canceled {
			return m, nil
		}
		if msg.err != nil {
			errorText := m.reportError("login.finish", msg.err)
			m.loginScreen.Waiting = false
			m.loginScreen.Error = errorText
			m.loginScreen.Status = ""
			return m, nil
		}
		if m.session.Current().ID == RouteAccountLogin {
			if m.session.CanGoBack() {
				m.session.Back()
			} else {
				m.session.ReplaceCurrent(Route{ID: RouteDashboardHome})
			}
		}

		m.snapshot.LoggedIn = true
		m.accountEmail = msg.creds.Email
		m.accountName = msg.creds.Name
		m.passwordAuth = msg.creds.Email
		m.discardPasswordInput()
		m.screen = screenDashboard
		m.loginScreen.Waiting = true
		m.loginScreen.Status = "Finishing account setup"
		m.loginScreen.Error = ""
		if !m.snapshot.VaultExists {
			m.showPasswordScreen(passwordRestore, msg.creds.Email, "", false)
			return m, m.passwordInput.Init()
		}

		return m, m.startMaintenance(maintenanceTriggerPostLogin, nil, false, "Finishing account setup", msg.creds.Email)
	case restoreFinishedMsg:
		if msg.id != m.restoreID {
			msg.password.clear()
			return m, nil
		}
		if m.restorePassword == msg.password {
			m.restorePassword = nil
		}
		m.passwordBusy = false
		if msg.err != nil {
			errorText := m.reportError("vault.restore", msg.err)
			msg.password.clear()
			switch {
			case errors.Is(msg.err, readiness.ErrInvalidRestorePassword):
				m.passwordInput.SetError("Couldn't decrypt vault, incorrect password")
			case errors.Is(msg.err, readiness.ErrNoRemoteLinkedVault):
				m.passwordInput.SetError("No linked vault was found for this account.")
			case errors.Is(msg.err, readiness.ErrRestoreTargetExists):
				m.passwordInput.SetError("A local vault was created while restoring. It was kept; reopen Forged.")
			default:
				m.passwordInput.SetError(errorText)
			}
			return m, nil
		}
		password := msg.password.take()
		cmd := m.startMaintenance(maintenanceTriggerUnlock, password, false, "Setting up Forged", m.passwordAuth)
		clear(password)
		return m, cmd
	case maintenanceFinishedMsg:
		if msg.id != m.maintenanceID {
			return m, nil
		}
		return m, m.handleMaintenanceFinished(msg.result, msg.err, msg.unlocked, msg.unlockErr)
	case snapshotRefreshMsg:
		return m.handleSnapshotRefreshMsg(msg)
	case securityStateMsg:
		if msg.id != m.securityLoadID {
			return m, nil
		}
		m.securityLoaded = true
		if msg.err != nil {
			m.securityLoadErr = m.reportError("security.load", msg.err)
		} else {
			m.securityState = msg.state
			m.securityLoadErr = ""
			if m.isManageMasterIntervalRoute() {
				m.manage.masterIntervalSelected = m.currentMasterPasswordIntervalIndex()
			}
		}
		return m, nil
	case runtimeStatusMsg:
		if msg.id != m.runtimeStatusID {
			return m, nil
		}
		wasUsingSpinner := m.usesSpinner()
		wasUnlocked := m.runtimeStatus.SensitiveKnown && m.runtimeStatus.Unlocked
		wasSyncPending := m.runtimeSyncPending()
		hadRuntimeStatus := m.runtimeLoaded
		lastSuccessfulPullAt := m.runtimeStatus.LastSuccessfulPullAt
		lastSuccessfulPushAt := m.runtimeStatus.LastSuccessfulPushAt
		refreshHealth := false
		if msg.err == nil {
			refreshHealth = m.runtimeUnavailable
			m.runtimeStatusFailures = 0
			m.runtimeUnavailable = false
			if !msg.status.SensitiveReported {
				msg.status.Unlocked = m.runtimeStatus.Unlocked
				msg.status.SensitiveKnown = m.runtimeStatus.SensitiveKnown
			}
			m.runtimeStatus = msg.status
			m.runtimeLoaded = true
			m.reportErrorText("sync.status", msg.status.Error)
		} else {
			m.reportError("runtime.status", msg.err)
			m.runtimeStatusFailures++
			if m.runtimeStatusFailures >= runtimeStatusFailureThreshold {
				refreshHealth = !m.runtimeUnavailable
				m.runtimeUnavailable = true
			}
			m.runtimeStatus.Syncing = false
		}
		var healthCmd tea.Cmd
		if refreshHealth {
			if m.systemHeader != systemHeaderFixing {
				m.systemHeader = systemHeaderChecking
			}
			healthCmd = m.refreshSnapshotCmd()
		}
		lossCmd := m.handleSensitiveSessionLoss(wasUnlocked)
		if m.snapshot.VaultExists {
			cmds := []tea.Cmd{m.pollRuntimeStatus(time.Second)}
			if lossCmd != nil {
				cmds = append(cmds, lossCmd)
			}
			if healthCmd != nil {
				cmds = append(cmds, healthCmd)
			}
			syncCompleted := msg.err == nil && hadRuntimeStatus && (m.runtimeStatus.LastSuccessfulPullAt.After(lastSuccessfulPullAt) ||
				m.runtimeStatus.LastSuccessfulPushAt.After(lastSuccessfulPushAt))
			if msg.err == nil && (syncCompleted || wasSyncPending && !m.runtimeSyncPending()) {
				cmds = append(cmds, m.invalidateSigningStatusCmd())
			}
			if !wasUsingSpinner && m.usesSpinner() {
				cmds = append([]tea.Cmd{m.spinner.Tick}, cmds...)
			}
			return m, tea.Batch(cmds...)
		}
		if lossCmd != nil && healthCmd != nil {
			return m, tea.Batch(lossCmd, healthCmd)
		}
		if lossCmd != nil {
			return m, lossCmd
		}
		return m, healthCmd
	case idleLockMsg:
		m.idleLockTimerArmed = false
		if !m.shouldTrackIdleLock() || m.idleLockDeadline.IsZero() {
			m.idleLockDeadline = time.Time{}
			return m, nil
		}
		if time.Until(m.idleLockDeadline) > 0 {
			return m, m.armIdleLockCmd()
		}
		if m.idleLockInFlight {
			return m, nil
		}
		m.idleLockInFlight = true
		m.idleLockID++
		m.cancelPrivateKeyCopy()
		return m, tea.Batch(m.clearPrivateClipboardForLock(), m.lockSensitiveCmd(m.idleLockID))
	case idleLockFinishedMsg:
		if msg.id != m.idleLockID {
			return m, nil
		}
		m.idleLockInFlight = false
		if msg.err != nil {
			errorText := m.reportError("vault.idle-lock", msg.err)
			if m.screen == screenDashboard {
				m.notice = notice{message: errorText, tone: dashboardscreen.ToneDanger}
			}
			return m, m.resetIdleLockCmd()
		}
		m.idleLockDeadline = time.Time{}
		m.runtimeStatusID++
		wasUnlocked := m.runtimeStatus.SensitiveKnown && m.runtimeStatus.Unlocked
		m.runtimeStatus.Unlocked = false
		m.runtimeStatus.SensitiveKnown = true
		if cmd := m.handleSensitiveSessionLoss(wasUnlocked); cmd != nil {
			return m, cmd
		}
		return m, nil
	case signingStatusMsg:
		return m.handleSigningStatusMsg(msg)
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
	case keyPrivateClipboardTickMsg:
		return m.handleKeyPrivateClipboardTickMsg(msg)
	case keyPrivateClipboardClearedMsg:
		return m.handleKeyPrivateClipboardClearedMsg(msg)
	case keyStalePrivateClipboardClearedMsg:
		return m.handleKeyStalePrivateClipboardClearedMsg(msg)
	case keyStalePrivateClipboardRetryMsg:
		return m, clearStalePrivateClipboard(msg.lease, msg.tries)
	case keyGenerateFinishedMsg:
		return m.handleKeyGenerateFinishedMsg(msg)
	case keyImportPreviewMsg:
		return m.handleKeyImportPreviewMsg(msg)
	case keyImportFinishedMsg:
		return m.handleKeyImportFinishedMsg(msg)
	case keyExportFinishedMsg:
		return m.handleKeyExportFinishedMsg(msg)
	case keyExportAuthorizedMsg:
		return m.handleKeyExportAuthorizedMsg(msg)
	case keyImportPickerMsg:
		return m.handleKeyImportPickerMsg(msg)
	case keyExportPickerMsg:
		return m.handleKeyExportPickerMsg(msg)
	case keyImportAutoReturnMsg:
		return m.handleKeyImportAutoReturnMsg(msg)
	case manageSyncFinishedMsg:
		return m.handleManageSyncFinishedMsg(msg)
	case manageChangePasswordFinishedMsg:
		return m.handleManageChangePasswordFinishedMsg(msg)
	case manageLogoutFinishedMsg:
		return m.handleManageLogoutFinishedMsg(msg)
	case manageSecuritySavedMsg:
		return m.handleManageSecuritySavedMsg(msg)
	case manageAutoReturnMsg:
		return m.handleManageAutoReturnMsg(msg)
	case agentSSHFinishedMsg:
		return m.handleAgentSSHFinishedMsg(msg)
	case agentSigningKeysMsg:
		return m.handleAgentSigningKeysMsg(msg)
	case agentSigningFinishedMsg:
		return m.handleAgentSigningFinishedMsg(msg)
	case labRoutingLoadedMsg:
		return m.handleLabRoutingLoadedMsg(msg)
	case labRoutingClearedMsg:
		return m.handleLabRoutingClearedMsg(msg)
	case labRoutingPollMsg:
		return m.handleLabRoutingPollMsg(msg)
	case doctorReportCopiedMsg:
		return m.handleDoctorReportCopiedMsg(msg)
	case copyFinishedMsg:
		m.clipboardBusy = false
		if msg.err != nil {
			errorText := m.reportError("clipboard.copy", msg.err)
			if m.screen == screenLogin {
				m.loginScreen.Error = errorText
				return m, nil
			}
			m.notice = notice{message: errorText, tone: dashboardscreen.ToneDanger}
			return m, nil
		}
		m.cancelPrivateClipboard()
		m.loginScreen.Copied = true
		return m, nil
	case tea.KeyMsg:
		nextModel, cmd := m.updateKeys(msg)
		next, ok := nextModel.(*model)
		if !ok {
			return nextModel, cmd
		}
		idleCmd := next.resetIdleLockCmd()
		switch {
		case cmd != nil && idleCmd != nil:
			return next, tea.Batch(cmd, idleCmd)
		case idleCmd != nil:
			return next, idleCmd
		default:
			return next, cmd
		}
	}

	if m.screen == screenPassword && m.passwordInput != nil {
		return m, m.passwordInput.Update(msg)
	}

	return m, nil
}

func (m *model) View() string {
	contentWidth := shell.ContentWidth(m.width)
	bodyWidth := shell.BodyWidth(m.width)
	if m.isCenteredStartupUnlockScreen() {
		bodyWidth = contentWidth
	}
	header := m.renderHeader(contentWidth)
	footer := shell.RenderFooter(contentWidth, m.footerActions()...)
	tightFooter := (m.isKeyRoute() && m.session.Current().ID == RouteKeysBrowser) || m.isAgentSigningRoute()
	tightBody := (m.isKeyRoute() && m.session.Current().ID == RouteKeysBrowser) || m.isTabbedDashboardRoot()
	bodyHeight := shell.BodyHeight(m.width, m.height, header, footer, tightFooter, tightBody)
	body := m.renderBody(bodyWidth, bodyHeight)
	if !m.isWelcomeState() && !m.isCenteredStartupUnlockScreen() {
		body = shell.IndentBlock(body, shell.ContentLeftInset)
	}
	return shell.Render(m.width, m.height, header, body, footer, tightFooter, tightBody)
}

func (m *model) isTabbedDashboardRoot() bool {
	return m.bootAssessed &&
		m.screen == screenDashboard &&
		(m.snapshot.VaultExists || strings.TrimSpace(m.snapshot.RuntimePathError) != "") &&
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
		Version:     m.deps.AppVersion,
		StatusItems: m.headerStatusItems(),
	}
	return shell.RenderHeader(width, data)
}

func (m *model) isLockedAuthScreen() bool {
	if m.screen != screenPassword {
		return false
	}
	return m.passwordFlow == passwordStartupUnlock
}

func (m *model) isCenteredStartupUnlockScreen() bool {
	return m.screen == screenPassword && m.passwordFlow == passwordStartupUnlock
}

func (m *model) canRetryStartupSystemAuth() bool {
	return m.screen == screenPassword &&
		m.passwordFlow == passwordStartupUnlock &&
		!m.passwordBusy &&
		!m.passwordHideInput &&
		m.passwordInput != nil &&
		m.passwordInput.IsEmpty() &&
		m.deps.HasLocalUnlockTrust()
}

func (m *model) productRailItems() []shell.StatusItem {
	return []shell.StatusItem{
		{Label: "Encrypted key vault", Icon: theme.Glyphs.Sparkle},
		{Label: "Multi-device sync", Icon: theme.Glyphs.Sparkle},
		{Label: "SSH + commit signing", Icon: theme.Glyphs.Sparkle},
	}
}

func (m *model) headerStatusItems() []shell.StatusItem {
	if m.isLockedAuthScreen() {
		return m.productRailItems()
	}
	if !m.bootAssessed {
		return []shell.StatusItem{
			{Label: "Checking health", Icon: m.spinner.View()},
			m.commitSigningHeaderItem(),
			{Label: "Loading vault", Icon: m.spinner.View()},
		}
	}
	if m.shouldShowProductRail() {
		return m.productRailItems()
	}

	items := []shell.StatusItem{
		m.systemHeaderItem(),
		m.commitSigningHeaderItem(),
		m.vaultSyncHeaderItem(),
	}

	return items
}

func (m *model) systemHeaderItem() shell.StatusItem {
	if m.systemHeader == systemHeaderHealthy && m.snapshot.AgentDisabled {
		return shell.StatusItem{Label: "Agent disabled", Tone: shell.StatusToneWarning}
	}

	switch m.systemHeader {
	case systemHeaderChecking:
		return shell.StatusItem{Label: "Checking health", Icon: m.spinner.View()}
	case systemHeaderFixing:
		return shell.StatusItem{Label: "Restoring health", Icon: m.spinner.View()}
	case systemHeaderHealthy:
		return shell.StatusItem{Label: "System healthy", Tone: shell.StatusToneSuccess}
	default:
		return shell.StatusItem{Label: "System unhealthy", Tone: shell.StatusToneDanger}
	}
}

func (m *model) commitSigningHeaderItem() shell.StatusItem {
	if strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
		return shell.StatusItem{Label: "Signing unavailable", Tone: shell.StatusToneDanger}
	}
	if !m.signingLoaded {
		return shell.StatusItem{Label: "Checking signing", Icon: m.spinner.View()}
	}
	if strings.TrimSpace(m.signingError) != "" {
		return shell.StatusItem{Label: "Signing issue", Tone: shell.StatusToneDanger}
	}
	switch m.signingStatus.Mode {
	case actions.CommitSigningForged:
		return shell.StatusItem{Label: "Commit signing", Tone: shell.StatusToneSuccess}
	case actions.CommitSigningExternal:
		return shell.StatusItem{Label: "External signing", Tone: shell.StatusToneWarning}
	default:
		return shell.StatusItem{Label: "Commit not signing", Tone: shell.StatusToneWarning}
	}
}

func (m *model) vaultSyncHeaderItem() shell.StatusItem {
	if strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
		return shell.StatusItem{Label: "Daemon/IPC unavailable", Tone: shell.StatusToneDanger}
	}
	switch m.systemHeader {
	case systemHeaderChecking:
		return shell.StatusItem{Label: "Checking vault", Icon: m.spinner.View()}
	case systemHeaderFixing:
		return shell.StatusItem{Label: "Repairing vault", Icon: m.spinner.View()}
	}
	if !m.snapshot.VaultExists {
		return shell.StatusItem{Label: "Vault missing", Tone: shell.StatusToneDanger}
	}
	if m.runtimeUnavailable {
		if !m.snapshot.LoggedIn {
			return shell.StatusItem{Label: "Local vault unavailable", Tone: shell.StatusToneDanger}
		}
		return shell.StatusItem{Label: "Vault unavailable", Tone: shell.StatusToneDanger}
	}
	if m.runtimeLoaded && strings.TrimSpace(m.runtimeStatus.Error) != "" {
		return shell.StatusItem{Label: "Sync issue", Tone: shell.StatusToneDanger}
	}
	if !m.snapshot.LoggedIn {
		return shell.StatusItem{Label: "Local vault healthy", Tone: shell.StatusToneSuccess}
	}
	if m.runtimeSyncPending() {
		return shell.StatusItem{Label: "Vault syncing", Icon: m.spinner.View()}
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
	if m.screen == screenDashboard && (m.snapshot.VaultExists || m.isDoctorOverviewRoute() || strings.TrimSpace(m.snapshot.RuntimePathError) != "") {
		if m.isKeyRoute() {
			return m.keyHeaderTitle()
		}
		if m.isManageHomeRoute() {
			return "Manage"
		}
		if m.isManageProfileRoute() {
			return "Profile"
		}
		if m.isManageMasterIntervalRoute() {
			return "Master Password Interval"
		}
		if m.isManageSuccessRoute() {
			return m.manageSuccessTitle()
		}
		if m.isAgentHomeRoute() {
			return "Agent"
		}
		if m.isAgentSigningRoute() {
			return "Commit Signing"
		}
		if m.isLabRoutingRoute() {
			return "SSH Routing"
		}
		if m.isDoctorOverviewRoute() {
			return "Doctor"
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
		return "Log In to Sync Vault"
	case screenPassword:
		switch m.passwordFlow {
		case passwordKeyView:
			return "Unlock private key"
		case passwordKeyExport:
			return "Export vault"
		case passwordStartupUnlock:
			return "Unlock Forged"
		}
		if strings.TrimSpace(m.passwordTitle) != "" {
			return m.passwordTitle
		}
		return "Vault"
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

	if m.screen == screenDashboard && (m.snapshot.VaultExists || m.isDoctorOverviewRoute() || strings.TrimSpace(m.snapshot.RuntimePathError) != "") {
		if m.isKeyRoute() {
			return m.keyBreadcrumbs()
		}
		if m.isManageHomeRoute() {
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Manage", Current: true},
			}
		}
		if m.isManageProfileRoute() {
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Manage"},
				{Label: "Profile", Current: true},
			}
		}
		if m.isManageMasterIntervalRoute() {
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Manage"},
				{Label: "Master Password Interval", Current: true},
			}
		}
		if m.isManageSuccessRoute() {
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Manage"},
				{Label: m.manageSuccessTitle(), Current: true},
			}
		}
		if m.isAgentHomeRoute() {
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Agent", Current: true},
			}
		}
		if m.isAgentSigningRoute() {
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Agent"},
				{Label: "Commit Signing", Current: true},
			}
		}
		if m.isLabRoutingRoute() {
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Agent"},
				{Label: "SSH Routing", Current: true},
			}
		}
		if m.isDoctorOverviewRoute() {
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Doctor", Current: true},
			}
		}
		if m.session.Current().ID == RouteSyncHome {
			label := "Sync"
			if !m.snapshot.LoggedIn {
				label = "Enable Sync"
			}
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Manage"},
				{Label: label, Current: true},
			}
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
			{Label: "Log In", Current: true},
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
		case passwordStartupUnlock:
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Unlock", Current: true},
			}
		case passwordManageChange:
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Manage"},
				{Label: "Change Master Password", Current: true},
			}
		case passwordDoctorRepair:
			return []shell.Breadcrumb{
				{Label: "Home"},
				{Label: "Doctor"},
				{Label: "Fix Issues", Current: true},
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
	default:
		return nil
	}
}

func (m *model) headerPageNote() string {
	if m.screen != screenDashboard ||
		!m.snapshot.VaultExists ||
		m.isWelcomeState() ||
		m.currentDashboardSection() != nil ||
		m.isKeyRoute() ||
		m.session.Current().ID != RouteDashboardHome {
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

func (m *model) renderBody(contentWidth int, bodyHeight int) string {
	switch m.screen {
	case screenLogin:
		return accountscreen.Render(m.loginScreen, m.spinner.View(), contentWidth)
	case screenPassword:
		return m.renderPasswordBody(contentWidth)
	default:
		if !m.bootAssessed {
			return m.renderPendingBody(contentWidth, bodyHeight)
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
					return m.renderKeyBody(contentWidth, bodyHeight)
				}
				return ""
			}
			return m.renderKeyBody(contentWidth, bodyHeight)
		}
		if m.isManageHomeRoute() {
			return m.renderManageBody(contentWidth)
		}
		if m.isManageProfileRoute() {
			return m.renderManageProfileBody(contentWidth)
		}
		if m.isManageMasterIntervalRoute() {
			return m.renderManageMasterIntervalBody(contentWidth)
		}
		if m.isManageSuccessRoute() {
			return m.renderManageSuccessBody(contentWidth)
		}
		if m.isAgentHomeRoute() {
			return m.renderAgentBody(contentWidth)
		}
		if m.isAgentSigningRoute() {
			return m.renderAgentSigningBody(contentWidth, bodyHeight)
		}
		if m.isLabRoutingRoute() {
			return m.renderLabRoutingBody(contentWidth, bodyHeight)
		}
		if m.isDoctorDashboardTab() {
			return m.renderDoctorDashboardBody(contentWidth, bodyHeight)
		}
		if m.isDoctorOverviewRoute() {
			return m.renderDoctorBody(contentWidth, bodyHeight)
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

func (m *model) renderPendingBody(contentWidth int, bodyHeight int) string {
	switch m.session.Current().ID {
	case RouteKeysDetail:
		return m.renderKeyBody(contentWidth, bodyHeight)
	case RouteKeysBrowser:
		return m.renderKeyBody(contentWidth, bodyHeight)
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
	textWidth := max(28, min(contentWidth, theme.HeroMaxWidth))

	if m.passwordAuth != "" {
		sections = append(sections,
			theme.Success.Render(theme.Glyphs.Check)+" "+theme.BodyMuted.Render(" Logged in as")+" "+theme.Body.Render(m.passwordAuth),
			"",
		)
	}

	if context := strings.TrimSpace(m.passwordContext); context != "" {
		style := theme.Body
		if m.passwordFlow == passwordKeyExport {
			style = theme.Warning
			context = "! " + context
		}
		sections = append(sections, style.Width(textWidth).Render(context))
	}

	if m.passwordHideInput && !m.passwordBusy {
		block := strings.Join([]string{
			theme.Body.Width(textWidth).Align(lipgloss.Center).Render(strings.TrimSpace(m.passwordContext)),
			"",
			theme.BodyMuted.Width(textWidth).Align(lipgloss.Center).Render("Press Enter to authenticate."),
		}, "\n")
		return shell.CenterInFixedBody(contentWidth, block)
	}

	if m.passwordHideInput && m.passwordBusy {
		busyMessage := strings.TrimSpace(m.passwordBusyMessage)
		if busyMessage == "" {
			busyMessage = "Working"
		}
		block := strings.Join([]string{
			theme.Body.Width(textWidth).Align(lipgloss.Center).Render(strings.TrimSpace(m.passwordContext)),
			"",
			theme.BodyStrong.Width(textWidth).Align(lipgloss.Center).Render(m.spinner.View() + " " + busyMessage),
		}, "\n")
		return shell.CenterInFixedBody(contentWidth, block)
	}

	labels := []string{""}
	if m.passwordFlow == passwordCreate {
		labels = []string{"", ""}
	}
	if m.passwordFlow == passwordManageChange {
		labels = []string{"", "", ""}
	}
	sections = append(sections, "", m.passwordInput.View(m.spinner.View(), labels...))
	return strings.Join(sections, "\n")
}

func (m *model) renderDashboardSection(contentWidth int, section dashboardSection) string {
	return theme.Body.Width(max(28, min(contentWidth, theme.HeroMaxWidth))).Render(strings.TrimSpace(section.Context))
}

func (m *model) footerActions() []shell.FooterAction {
	switch m.screen {
	case screenLogin:
		if m.loginCommitting {
			return nil
		}
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
			if m.passwordFlow == passwordStartupUnlock && m.passwordHideInput {
				return []shell.FooterAction{{Key: "Esc", Label: "Use Master Password"}}
			}
			return nil
		}
		enterLabel := "Continue"
		if m.passwordFlow == passwordStartupUnlock && m.passwordHideInput {
			enterLabel = "System Auth"
		}
		if m.passwordInput != nil && m.passwordInput.FieldCount() > 1 && m.passwordInput.FocusIndex() < m.passwordInput.FieldCount()-1 {
			enterLabel = "Next"
		}
		actions := []shell.FooterAction{{Key: "Enter", Label: enterLabel}}
		if m.canRetryStartupSystemAuth() {
			actions = append(actions, shell.FooterAction{Key: "Ctrl+A", Label: "System Auth"})
		}
		actions = append(actions, shell.FooterAction{Key: "Esc", Label: m.session.EscLabel(EscAuto)})
		return actions
	default:
		if !m.bootAssessed {
			return []shell.FooterAction{{Key: "Esc", Label: m.session.EscLabel(EscAuto)}}
		}
		if m.isKeyRoute() {
			return m.keyFooterActions()
		}
		if m.isManageHomeRoute() {
			if m.manage.logoutBusy {
				return nil
			}
			return []shell.FooterAction{
				{Key: theme.Glyphs.UpDown, Label: "Move"},
				{Key: "Enter", Label: "Open"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if m.isManageProfileRoute() {
			return []shell.FooterAction{
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if m.isManageMasterIntervalRoute() {
			if m.manage.settingBusy {
				return nil
			}
			if !m.securityLoaded {
				return []shell.FooterAction{{Key: "Esc", Label: m.session.EscLabel(EscAuto)}}
			}
			if m.securityLoadErr != "" {
				return []shell.FooterAction{
					{Key: "Enter", Label: "Retry"},
					{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
				}
			}
			return []shell.FooterAction{
				{Key: theme.Glyphs.UpDown, Label: "Move"},
				{Key: "Enter", Label: "Apply"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if m.isManageSuccessRoute() {
			return []shell.FooterAction{
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if m.isAgentHomeRoute() {
			if strings.TrimSpace(m.snapshot.RuntimePathError) != "" && m.snapshot.AgentDisabled {
				return []shell.FooterAction{{Key: "Esc", Label: m.session.EscLabel(EscAuto)}}
			}
			return []shell.FooterAction{
				{Key: theme.Glyphs.UpDown, Label: "Move"},
				{Key: "Enter", Label: "Open"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if m.isAgentSigningRoute() {
			if m.agent.signing.busy {
				return nil
			}
			if m.agent.signing.loading || !m.signingLoaded {
				return []shell.FooterAction{{Key: "Esc", Label: m.session.EscLabel(EscAuto)}}
			}
			if m.agent.signing.disableArmed {
				return []shell.FooterAction{
					{Key: "Enter", Label: "Confirm Disable"},
					{Key: "Esc", Label: "Cancel"},
				}
			}
			if m.agent.signing.err != "" {
				return []shell.FooterAction{
					{Key: "Enter", Label: "Retry"},
					{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
				}
			}
			if m.agent.signing.searchActive {
				return []shell.FooterAction{
					{Key: "Enter", Label: "Done"},
					{Key: "Esc", Label: m.session.EscLabel(EscCancel)},
				}
			}
			actions := []shell.FooterAction{
				{Key: theme.Glyphs.UpDown, Label: "Move"},
			}
			if _, ok := m.selectedAgentSigningKey(); ok && !m.selectedAgentSigningKeyApplied() {
				actions = append(actions, shell.FooterAction{Key: "Enter", Label: "Use For Signing"})
			}
			if m.signingStatus.Enabled() {
				actions = append(actions, shell.FooterAction{Key: "D", Label: "Disable Signing"})
			}
			if m.signingError != "" {
				actions = append(actions, shell.FooterAction{Key: "R", Label: "Retry Status"})
			}
			actions = append(actions, shell.FooterAction{Key: "/", Label: "Search"})
			actions = append(actions, shell.FooterAction{Key: "Esc", Label: m.session.EscLabel(EscAuto)})
			return actions
		}
		if m.isLabRoutingRoute() {
			return m.labFooterActions()
		}
		if m.isDoctorDashboardTab() {
			return m.doctorFooterActions(true)
		}
		if m.isDoctorOverviewRoute() {
			return m.doctorFooterActions(false)
		}
		if m.isWelcomeState() {
			return []shell.FooterAction{
				{Key: theme.Glyphs.UpDown, Label: "Move"},
				{Key: "Enter", Label: "Select"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if m.currentDashboardSection() != nil {
			if m.session.Current().ID == RouteSyncHome {
				if !m.snapshot.LoggedIn {
					return []shell.FooterAction{
						{Key: "Enter", Label: "Log In"},
						{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
					}
				}
				return []shell.FooterAction{
					{Key: "Enter", Label: "Sync Now"},
					{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
				}
			}
			return []shell.FooterAction{
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		if tabs := m.dashboardTabs(); len(tabs) > 0 {
			if strings.TrimSpace(m.snapshot.RuntimePathError) != "" &&
				m.snapshot.AgentDisabled &&
				tabs[m.dashboardTabIndex].Label == "Agent" {
				return []shell.FooterAction{
					{Key: theme.Glyphs.LeftRight, Label: "Tabs"},
					{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
				}
			}
			return []shell.FooterAction{
				{Key: theme.Glyphs.LeftRight, Label: "Tabs"},
				{Key: theme.Glyphs.UpDown, Label: "Pages"},
				{Key: "Enter", Label: "Open"},
				{Key: "Esc", Label: m.session.EscLabel(EscAuto)},
			}
		}
		return []shell.FooterAction{{Key: "Esc", Label: m.session.EscLabel(EscAuto)}}
	}
}

func (m *model) updateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.passwordBusy && m.passwordFlow == passwordManageChange {
			if m.passwordInput != nil {
				m.passwordInput.SetInfo("Changing master password " + theme.Glyphs.Empty + " quit is unavailable until this finishes")
			}
			return m, nil
		}
		if m.passwordInput != nil {
			m.passwordInput.Clear()
		}
		m.invalidateStartupUnlock()
		m.stopLifetime()
		return m, tea.Quit
	}
	if m.idleLockInFlight {
		return m, nil
	}
	if m.clipboardBusy {
		return m, nil
	}

	switch m.screen {
	case screenLogin:
		return m.updateLoginKeys(msg)
	case screenPassword:
		return m.updatePasswordKeys(msg)
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

	if m.isManageSuccessRoute() {
		switch msg.String() {
		case "esc":
			m.manage.success = nil
			return m, m.returnFromManageFlow()
		}
		return m, nil
	}

	if m.isManageHomeRoute() {
		return m.updateManageKeys(msg)
	}

	if m.isManageProfileRoute() {
		switch msg.String() {
		case "esc":
			if m.session.Back() {
				return m, m.showCurrentRoute()
			}
			return m, tea.Quit
		}
		return m, nil
	}

	if m.isManageMasterIntervalRoute() {
		return m.updateManageMasterIntervalKeys(msg)
	}

	if m.isAgentHomeRoute() {
		return m.updateAgentKeys(msg)
	}

	if m.isAgentSigningRoute() {
		return m.updateAgentSigningKeys(msg)
	}

	if m.isLabRoutingRoute() {
		return m.updateLabRoutingKeys(msg)
	}

	if m.isDoctorDashboardTab() {
		return m.updateDoctorDashboardKeys(msg)
	}

	if m.isDoctorOverviewRoute() {
		return m.updateDoctorKeys(msg)
	}

	if m.currentDashboardSection() != nil {
		switch msg.String() {
		case "enter":
			if m.session.Current().ID == RouteSyncHome {
				if !m.snapshot.LoggedIn {
					if m.session.Current().ID != RouteAccountLogin {
						m.session.Push(Route{ID: RouteAccountLogin})
					}
					return m, m.startLoginFlow()
				}
				return m, m.runManageSync()
			}
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
			if tabs[m.dashboardTabIndex].Label == "Manage" {
				m.notice = notice{}
				m.manage.logoutArmed = false
				m.manage.selected = m.dashboardPageIndices[m.dashboardTabIndex]
			}
			if tabs[m.dashboardTabIndex].Label == "Agent" {
				m.agent.statusErr = ""
				m.agent.selected = m.dashboardPageIndices[m.dashboardTabIndex]
			}
			return m, nil
		case "down", "j":
			pages := tabs[m.dashboardTabIndex].Pages
			pageIndex := m.dashboardPageIndices[m.dashboardTabIndex]
			if len(pages) > 0 && pageIndex < len(pages)-1 {
				m.dashboardPageIndices[m.dashboardTabIndex]++
			}
			if tabs[m.dashboardTabIndex].Label == "Manage" {
				m.notice = notice{}
				m.manage.logoutArmed = false
				m.manage.selected = m.dashboardPageIndices[m.dashboardTabIndex]
			}
			if tabs[m.dashboardTabIndex].Label == "Agent" {
				m.agent.statusErr = ""
				m.agent.selected = m.dashboardPageIndices[m.dashboardTabIndex]
			}
			return m, nil
		case "left", "h":
			return m, m.switchDashboardTab(-1, tabs)
		case "right", "l":
			return m, m.switchDashboardTab(1, tabs)
		case "enter":
			if tabs[m.dashboardTabIndex].Label == "Manage" {
				items := m.manageItems()
				if len(items) == 0 {
					return m, nil
				}
				m.notice = notice{}
				m.manage.selected = m.dashboardPageIndices[m.dashboardTabIndex]
				if m.manage.selected < 0 {
					m.manage.selected = 0
				}
				if m.manage.selected >= len(items) {
					m.manage.selected = len(items) - 1
				}
				return m.openManageItem(items[m.manage.selected])
			}
			if tabs[m.dashboardTabIndex].Label == "Agent" {
				items := m.agentItems()
				if len(items) == 0 {
					return m, nil
				}
				m.notice = notice{}
				m.agent.statusErr = ""
				m.agent.selected = m.dashboardPageIndices[m.dashboardTabIndex]
				if m.agent.selected < 0 {
					m.agent.selected = 0
				}
				if m.agent.selected >= len(items) {
					m.agent.selected = len(items) - 1
				}
				return m.openAgentItem(items[m.agent.selected])
			}
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
	if m.loginCommitting {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.loginID++
		m.loginProgress = nil
		m.cancelLoginFlow()
		if m.session.Back() {
			return m, m.showCurrentRoute()
		}
		return m, tea.Quit
	case "c", "C":
		if m.loginScreen.URL == "" || m.loginScreen.Error != "" {
			return m, nil
		}
		return m, m.copyToClipboard(m.loginScreen.URL)
	case "enter":
		if m.loginScreen.Error != "" {
			return m, m.startLoginFlow()
		}
		if m.loginScreen.URL != "" {
			if err := m.openCurrentLoginURL(); err != nil {
				m.loginScreen.Error = m.reportError("browser.open", err)
				m.loginScreen.Status = ""
			}
		}
	}
	return m, nil
}

func (m *model) updatePasswordKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.passwordBusy {
		if msg.String() == "esc" && m.passwordFlow == passwordStartupUnlock && m.passwordHideInput {
			return m, m.useStartupMasterPassword()
		}
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
			m.discardPasswordInput()
			m.screen = screenDashboard
			return m, nil
		}
		if m.session.Back() {
			restartRuntimePoll := m.passwordFlow == passwordDoctorRepair
			m.discardPasswordInput()
			m.passwordAuth = ""
			cmd := m.showCurrentRoute()
			if restartRuntimePoll {
				return m, tea.Batch(cmd, m.pollRuntimeStatus(0))
			}
			return m, cmd
		}
		if m.passwordInput != nil {
			m.passwordInput.Clear()
		}
		return m, tea.Quit
	case "ctrl+a":
		if m.canRetryStartupSystemAuth() {
			return m, m.startStartupUnlockFlow()
		}
		return m, m.passwordInput.Update(msg)
	case "enter":
		if m.passwordFlow == passwordStartupUnlock && m.passwordHideInput && !m.passwordBusy {
			return m, m.startStartupUnlockFlow()
		}
		if m.passwordInput != nil && m.passwordInput.FieldCount() > 1 && m.passwordInput.FocusIndex() < m.passwordInput.FieldCount()-1 {
			m.passwordInput.MoveNext()
			return m, nil
		}
		if m.passwordFlow == passwordManageChange {
			return m, m.submitManageChangePassword()
		}
		password, err := m.passwordInput.Submit()
		switch m.passwordFlow {
		case passwordCreate:
			if err != nil {
				m.passwordInput.SetError(err.Error())
				return m, nil
			}
			return m, m.startMaintenance(maintenanceTriggerSetup, password, true, "Setting up Forged", "")
		case passwordRestore:
			if err != nil {
				m.passwordInput.SetError(err.Error())
				return m, nil
			}
			m.passwordBusy = true
			m.passwordInput.SetInfo("Decrypting vault")
			m.restoreID++
			return m, tea.Batch(m.spinner.Tick, m.restoreLinkedVault(m.restoreID, password))
		case passwordKeyView:
			if err != nil {
				m.passwordInput.SetError(err.Error())
				return m, nil
			}
			m.passwordBusy = true
			m.passwordInput.SetInfo("Decrypting vault")
			return m, tea.Batch(m.spinner.Tick, m.copyPrivateKey(password))
		case passwordKeyExport:
			if err != nil {
				m.passwordInput.SetError(err.Error())
				return m, nil
			}
			m.passwordBusy = true
			m.passwordInput.SetInfo("Verifying password")
			return m, tea.Batch(m.spinner.Tick, m.authorizeKeyExport(password))
		case passwordStartupUnlock:
			if err != nil {
				m.passwordInput.SetError(err.Error())
				return m, nil
			}
			return m, m.submitStartupUnlock(password)
		case passwordDoctorRepair:
			if err != nil {
				m.passwordInput.SetError(err.Error())
				return m, nil
			}
			return m, m.startDoctorRepair(password)
		default:
			if err != nil {
				m.passwordInput.SetError(err.Error())
				return m, nil
			}
			return m, m.startMaintenance(maintenanceTriggerUnlock, password, false, "Unlocking Forged", "")
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
	if strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
		return []dashboardTab{
			{Label: "Agent", Pages: m.agentDashboardPages()},
			{Label: "Doctor"},
		}
	}
	if !m.snapshot.VaultExists {
		return nil
	}

	tabs := []dashboardTab{
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
			Label: "Agent",
			Pages: m.agentDashboardPages(),
		},
		{
			Label: "Manage",
			Pages: m.manageDashboardPages(),
		},
		{
			Label: "Doctor",
			Pages: nil,
		},
	}
	return tabs
}

func (m *model) enterRuntimePathRecovery(runtimePathError string) {
	m.cancelLoginFlow()
	m.discardPasswordInput()
	route := RouteDashboardHome
	if m.intent.Entry == RouteDoctorOverview || m.session.Current().ID == RouteDoctorOverview {
		route = RouteDoctorOverview
	}
	m.session.Reset(Route{ID: route})
	m.dashboardTabIndex = 0
	m.dashboardPageIndices = nil
	m.agent.selected = 0
	m.runtimeLoaded = false
	m.screen = screenDashboard
	m.systemHeader = systemHeaderUnhealthy
	m.notice = notice{message: runtimePathError, tone: dashboardscreen.ToneDanger}
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

func (m *model) switchDashboardTab(delta int, tabs []dashboardTab) tea.Cmd {
	if len(tabs) == 0 || delta == 0 {
		return nil
	}

	next := m.dashboardTabIndex + delta
	if next < 0 {
		next = 0
	}
	if next >= len(tabs) {
		next = len(tabs) - 1
	}
	if next == m.dashboardTabIndex {
		return nil
	}

	m.manage.logoutArmed = false
	m.dashboardTabIndex = next
	switch tabs[m.dashboardTabIndex].Label {
	case "Manage":
		return m.loadSecurityStateCmd()
	case "Agent":
		if strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
			return nil
		}
		return tea.Batch(m.refreshSnapshotCmd(), m.loadSigningStatusCmd())
	case "Doctor":
		if strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
			return nil
		}
		return tea.Batch(m.refreshSnapshotCmd(), m.loadSecurityStateCmd())
	default:
		return nil
	}
}

func (m *model) currentDashboardSection() *dashboardSection {
	if m.screen != screenDashboard || !m.snapshot.VaultExists {
		return nil
	}

	switch m.session.Current().ID {
	case RouteSyncHome:
		if !m.snapshot.LoggedIn {
			return &dashboardSection{
				Title: "Enable Sync",
				Context: strings.Join([]string{
					"Sync your encrypted vault across devices with your Forged account.",
					"Your keys stay encrypted and available on every machine you trust.",
				}, "\n"),
			}
		}
		return &dashboardSection{
			Title:   "Sync",
			Context: m.manageSyncSummary(),
		}
	default:
		return nil
	}
}

func (m *model) dashboardOptions() []dashboardscreen.Option {
	if m.snapshot.VaultExists || strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
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

func (m *model) usesSpinner() bool {
	switch m.screen {
	case screenLogin:
		return m.loginScreen.Waiting
	case screenPassword:
		return m.passwordBusy
	case screenDashboard:
		return m.keyUsesSpinner() || m.agentUsesSpinner() || m.lab.loading || m.lab.busy || m.manage.syncBusy || m.manage.logoutBusy || m.manage.settingBusy || (m.isManageMasterIntervalRoute() && !m.securityLoaded) || m.systemHeader == systemHeaderChecking || m.systemHeader == systemHeaderFixing || m.runtimeSyncPending()
	default:
		return false
	}
}

func (m *model) runtimeSyncPending() bool {
	if m.runtimeStatus.Syncing {
		return true
	}
	return m.runtimeLoaded && m.runtimeStatus.Dirty && strings.TrimSpace(m.runtimeStatus.Error) == ""
}

func (m *model) shouldTrackIdleLock() bool {
	if !m.bootAssessed || !m.snapshot.VaultExists {
		return false
	}
	if !m.runtimeStatus.SensitiveKnown || !m.runtimeStatus.Unlocked {
		return false
	}
	if m.screen == screenLogin {
		return false
	}
	return !m.isLockedAuthScreen()
}

func (m *model) resetIdleLockCmd() tea.Cmd {
	if m.idleLockInFlight {
		return nil
	}
	if !m.shouldTrackIdleLock() {
		m.idleLockDeadline = time.Time{}
		return nil
	}
	m.idleLockDeadline = time.Now().Add(tuiIdleLockTimeout)
	return m.armIdleLockCmd()
}

func (m *model) armIdleLockCmd() tea.Cmd {
	if m.idleLockTimerArmed || m.idleLockInFlight || m.idleLockDeadline.IsZero() {
		return nil
	}
	delay := time.Until(m.idleLockDeadline)
	if delay < 0 {
		delay = 0
	}
	m.idleLockTimerArmed = true
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return idleLockMsg{}
	})
}

func (m *model) assessCurrentState() tea.Cmd {
	repair := m.deps.Repair
	return func() tea.Msg {
		result, err := repair(readiness.RunOptions{Mode: readiness.ModeAssessOnly})
		return assessmentMsg{snapshot: result.Snapshot, err: err}
	}
}

func (m *model) startLoginFlow() tea.Cmd {
	m.cancelLoginFlow()
	m.discardPasswordInput()
	m.screen = screenLogin
	m.notice = notice{}
	m.loginCommitting = false
	m.loginScreen = accountscreen.LoginScreen{
		Title:   "Log In to Sync Vault",
		Context: "Preparing secure browser approval.",
		Status:  "Opening approval link",
		Waiting: true,
	}
	m.loginID++
	id := m.loginID
	ctx, cancel := context.WithCancel(m.lifetimeCtx)
	m.loginCancel = cancel
	startLogin := m.deps.StartLogin
	server := m.serverURL()
	progressCh := make(chan actions.LoginProgress, 8)
	m.loginProgress = progressCh
	return tea.Batch(
		m.spinner.Tick,
		m.waitForLoginStartProgress(id, progressCh),
		func() tea.Msg {
			session, err := startLogin(ctx, server, func(progress actions.LoginProgress) {
				select {
				case progressCh <- progress:
				default:
				}
			})
			close(progressCh)
			return loginStartedMsg{id: id, ctx: ctx, session: session, err: err}
		},
	)
}

func (m *model) cancelLoginFlow() {
	if m.loginCancel == nil {
		return
	}
	m.loginCancel()
	m.loginCancel = nil
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
	return func() tea.Msg {
		creds, err := session.Wait(ctx)
		if errors.Is(err, context.Canceled) {
			return loginFinishedMsg{id: id, canceled: true}
		}
		if err != nil {
			return loginFinishedMsg{id: id, err: err}
		}
		return loginApprovedMsg{id: id, creds: creds}
	}
}

func (m *model) commitLogin(id int, creds actions.AccountCredentials) tea.Cmd {
	save := m.deps.SaveCredentials
	return func() tea.Msg {
		return loginFinishedMsg{id: id, creds: creds, err: save(creds)}
	}
}

func (m *model) restoreLinkedVault(id int, password []byte) tea.Cmd {
	restore := m.deps.RestoreVault
	m.clearRestorePassword()
	passwordBuffer := newPasswordBuffer(password)
	m.restorePassword = passwordBuffer
	return func() tea.Msg {
		err := passwordBuffer.use(restore)
		if err != nil {
			passwordBuffer.clear()
		}
		return restoreFinishedMsg{id: id, password: passwordBuffer, err: err}
	}
}

func (m *model) startMaintenance(trigger maintenanceTrigger, password []byte, createVaultFirst bool, title string, authEmail string) tea.Cmd {
	m.notice = notice{}
	m.runtimeStatusID++
	m.snapshotRefreshID++
	m.runtimeStatusFailures = 0
	m.runtimeUnavailable = false
	m.systemHeader = systemHeaderFixing
	m.passwordAuth = authEmail
	m.maintenanceTrigger = trigger
	m.maintenanceUsedPassword = len(password) > 0
	m.maintenanceAuthEmail = authEmail
	m.maintenanceBusy = true
	if m.screen == screenPassword {
		m.passwordBusy = true
		m.passwordHideInput = false
		m.passwordBusyMessage = ""
		if m.passwordInput != nil {
			m.passwordInput.SetInfo(title)
		}
	}

	m.maintenanceID++
	id := m.maintenanceID
	repairFn := m.deps.Repair
	createVault := m.deps.CreateVault
	unlock := m.deps.UnlockSensitiveLaunch
	lifetimeCtx := m.lifetimeCtx
	passwordCopy := append([]byte(nil), password...)
	clear(password)

	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			defer func() {
				for i := range passwordCopy {
					passwordCopy[i] = 0
				}
			}()

			if createVaultFirst {
				if err := createVault(passwordCopy); err != nil {
					return maintenanceFinishedMsg{id: id, err: err}
				}
			}

			opts := readiness.RunOptions{Mode: m.maintenanceModeForTrigger(trigger)}
			if len(passwordCopy) > 0 {
				opts.PromptPassword = func(string) ([]byte, error) {
					return append([]byte(nil), passwordCopy...), nil
				}
			}

			result, err := repairFn(opts)
			unlocked := false
			var unlockErr error
			if err == nil &&
				len(passwordCopy) > 0 &&
				result.Snapshot.VaultExists &&
				result.Next != readiness.NextActionNeedsPassword &&
				(trigger == maintenanceTriggerSetup || trigger == maintenanceTriggerUnlock) {
				unlockResult, err := unlock(lifetimeCtx, passwordCopy)
				switch {
				case err != nil:
					unlockErr = err
				case unlockResult.PasswordRequired:
					unlockErr = errors.New("startup authentication still required")
				default:
					unlocked = true
				}
			}
			return maintenanceFinishedMsg{id: id, result: result, err: err, unlocked: unlocked, unlockErr: unlockErr}
		},
	)
}

func (m *model) handleMaintenanceFinished(result readiness.RunResult, err error, unlocked bool, unlockErr error) tea.Cmd {
	action := "maintenance." + string(m.maintenanceTrigger)
	errorText := m.reportError(action, err)
	m.reportError(action+".unlock", unlockErr)
	m.snapshot = result.Snapshot
	m.summary = result.Summary
	m.systemHeader = m.systemHeaderForSnapshot(result.Snapshot)
	m.maintenanceBusy = false
	if unlocked {
		m.runtimeStatus.Unlocked = true
		m.runtimeStatus.SensitiveKnown = true
	}
	if m.screen == screenPassword {
		m.passwordBusy = false
		m.passwordBusyMessage = ""
	}
	if m.maintenanceAuthEmail != "" {
		m.accountEmail = m.maintenanceAuthEmail
	}
	if result.Snapshot.LoggedIn {
		m.loadStoredAccountIdentity()
	} else {
		m.accountName = ""
		m.accountEmail = ""
	}
	if err != nil {
		m.systemHeader = systemHeaderUnhealthy
		switch {
		case m.screen == screenPassword && m.maintenanceTrigger == maintenanceTriggerSetup:
			m.passwordInput.SetError(errorText)
			return m.passwordInput.Init()
		case m.screen == screenPassword && m.maintenanceUsedPassword:
			m.passwordInput.SetError(errorText)
			return nil
		default:
			m.showDashboardNotice(errorText, dashboardscreen.ToneDanger)
			if m.snapshot.VaultExists {
				return m.pollRuntimeStatus(time.Second)
			}
			return nil
		}
	}

	switch result.Next {
	case readiness.NextActionNeedsPassword:
		errorText := ""
		if m.maintenanceUsedPassword {
			errorText = "That password did not unlock this device."
		}
		if m.maintenanceTrigger == maintenanceTriggerDoctor {
			m.showPasswordScreenOnRoute(RouteVaultUnlock, passwordDoctorRepair, m.maintenanceAuthEmail, errorText, false)
			return m.passwordInput.Init()
		}
		m.showPasswordScreen(m.passwordFlowForSnapshot(result.Snapshot), m.maintenanceAuthEmail, errorText, m.maintenanceAuthEmail != "")
		return m.passwordInput.Init()
	case readiness.NextActionNeedsInteractiveSetup:
		if result.Snapshot.LoggedIn {
			m.showDashboardNotice("No synced vault was found for this account. Start a new vault on this device.", dashboardscreen.ToneWarning)
		} else if m.maintenanceTrigger == maintenanceTriggerDoctor {
			m.showDashboardNotice("No repairs were made. Create or restore a vault to continue.", dashboardscreen.ToneWarning)
		} else {
			m.showDashboardNotice(m.summaryMessage(), dashboardscreen.ToneSuccess)
		}
		m.popWizardRoutes()
		m.discardPasswordInput()
		m.screen = screenDashboard
		if m.maintenanceTrigger == maintenanceTriggerDoctor {
			return m.loadSecurityStateCmd()
		}
		return nil
	default:
		if (m.maintenanceTrigger == maintenanceTriggerSetup || m.maintenanceTrigger == maintenanceTriggerUnlock) && result.Snapshot.VaultExists {
			m.popWizardRoutes()
			if unlocked {
				m.startupUnlockPending = false
				m.startupUnlockNeedsRepair = false
				m.maintenanceUsedPassword = false
				m.maintenanceAuthEmail = ""
				return m.finishVaultBoot()
			}
			if unlockErr != nil {
				m.runtimeStatus.Unlocked = false
				m.runtimeStatus.SensitiveKnown = false
			}
			return m.restartAfterVaultReady()
		}
		if m.maintenanceTrigger == maintenanceTriggerBoot && result.Snapshot.VaultExists && m.startupUnlockPending && !m.startupUnlockNeedsRepair {
			return m.startStartupUnlockFlow()
		}
		m.popWizardRoutes()
		return m.finishVaultBoot()
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

func (m *model) unlockSensitiveLaunchCmd(password []byte) tea.Cmd {
	m.invalidateStartupUnlock()
	id := m.startupUnlockID
	unlock := m.deps.UnlockSensitiveLaunch
	ctx, cancel := context.WithCancel(m.lifetimeCtx)
	m.startupUnlockCancel = cancel
	passwordCopy := append([]byte(nil), password...)
	clear(password)
	return func() tea.Msg {
		defer cancel()
		defer clear(passwordCopy)
		result, err := unlock(ctx, passwordCopy)
		return startupUnlockFinishedMsg{id: id, result: result, err: err}
	}
}

func (m *model) startStartupUnlockFlow() tea.Cmd {
	m.showPasswordScreen(passwordStartupUnlock, "", "", true)
	m.passwordContext = "Approve the System Auth prompt to open Forged."
	m.passwordBusy = true
	m.passwordHideInput = true
	m.passwordBusyMessage = "Waiting for System Auth"
	return tea.Batch(m.spinner.Tick, m.unlockSensitiveLaunchCmd(nil))
}

func (m *model) useStartupMasterPassword() tea.Cmd {
	m.invalidateStartupUnlock()
	m.passwordBusy = false
	m.passwordHideInput = false
	m.passwordBusyMessage = ""
	m.passwordContext = "Enter your master password to open Forged."
	m.passwordInput.ClearStatus()
	return m.passwordInput.Init()
}

func (m *model) handleSensitiveSessionLoss(wasUnlocked bool) tea.Cmd {
	if m.runtimeStatus.SensitiveKnown && m.runtimeStatus.Unlocked {
		m.privateClip.lockClear = false
	}
	locked := m.runtimeStatus.SensitiveKnown && !m.runtimeStatus.Unlocked
	var clipboardCmd tea.Cmd
	if locked && (wasUnlocked || m.privateCopyPending || m.privateClip.lease != nil) {
		m.cancelPrivateKeyCopy()
		clipboardCmd = m.clearPrivateClipboardForLock()
	}
	if !wasUnlocked || !m.snapshot.VaultExists {
		return clipboardCmd
	}
	if m.screen == screenPassword && m.passwordFlow == passwordStartupUnlock {
		return clipboardCmd
	}
	if m.runtimeStatus.Unlocked || !m.runtimeStatus.SensitiveKnown {
		return clipboardCmd
	}
	if !m.deps.HasLocalUnlockTrust() {
		m.showPasswordScreen(passwordStartupUnlock, "", "", true)
		m.passwordContext = "Enter your master password to continue using Forged."
		return tea.Batch(clipboardCmd, m.passwordInput.Init())
	}
	m.showPasswordScreen(passwordStartupUnlock, "", "", true)
	m.passwordContext = "Please authenticate to continue using Forged."
	m.passwordHideInput = true
	m.passwordBusy = false
	m.passwordBusyMessage = ""
	m.passwordInput.ClearStatus()
	return clipboardCmd
}

func (m *model) submitStartupUnlock(password []byte) tea.Cmd {
	m.passwordBusy = true
	m.passwordHideInput = false
	m.passwordBusyMessage = ""
	m.passwordInput.SetInfo("Unlocking Forged")
	return tea.Batch(m.spinner.Tick, m.unlockSensitiveLaunchCmd(password))
}

func (m *model) finishVaultBoot() tea.Cmd {
	m.notice = notice{}
	m.discardPasswordInput()
	m.screen = screenDashboard
	if m.systemHeader == systemHeaderChecking {
		m.systemHeader = m.systemHeaderForSnapshot(m.snapshot)
	}
	m.passwordFlow = ""
	m.passwordTitle = ""
	m.passwordContext = ""
	m.passwordAuth = ""
	m.passwordBusy = false
	m.passwordHideInput = false
	m.passwordBusyMessage = ""
	m.passwordOverlay = false
	if !m.snapshot.VaultExists {
		return nil
	}
	cmds := []tea.Cmd{
		m.pollRuntimeStatus(0),
		m.loadSecurityStateCmd(),
		m.preloadKeyBrowser(),
		m.loadSigningStatusCmd(),
		m.resetIdleLockCmd(),
	}
	if route := m.session.Current().ID; route != "" && route != RouteDashboardHome {
		cmds = append([]tea.Cmd{m.showCurrentRoute()}, cmds...)
	}
	return tea.Batch(cmds...)
}

func (m *model) handleStartupUnlockFinishedMsg(msg startupUnlockFinishedMsg) tea.Cmd {
	if msg.id != m.startupUnlockID {
		return nil
	}
	m.cancelStartupUnlock()
	if m.screen != screenPassword ||
		m.passwordFlow != passwordStartupUnlock ||
		!m.passwordBusy {
		return nil
	}

	m.passwordBusy = false
	m.passwordBusyMessage = ""
	if msg.err != nil {
		m.passwordHideInput = false
		m.passwordContext = "Enter your master password to open Forged."
		m.passwordInput.SetError(m.reportError("vault.unlock", msg.err))
		return nil
	}

	if msg.result.PasswordRequired {
		m.passwordHideInput = false
		prompt := strings.TrimSpace(msg.result.Prompt)
		if prompt == "" {
			prompt = "Authentication unavailable. Enter your master password to open Forged."
		}
		m.passwordContext = prompt
		m.passwordInput.ClearStatus()
		return m.passwordInput.Init()
	}

	m.runtimeStatus.Unlocked = true
	m.runtimeStatus.SensitiveKnown = true
	m.passwordHideInput = false
	m.notice = notice{}
	pending := m.startupUnlockPending
	needsRepair := m.startupUnlockNeedsRepair
	m.startupUnlockPending = false
	m.startupUnlockNeedsRepair = false

	if pending && needsRepair {
		return tea.Batch(
			m.startStartupRepair(),
			m.preloadKeyBrowser(),
		)
	}

	return m.finishVaultBoot()
}

func (m *model) cancelStartupUnlock() {
	if m.startupUnlockCancel == nil {
		return
	}
	m.startupUnlockCancel()
	m.startupUnlockCancel = nil
}

func (m *model) invalidateStartupUnlock() {
	m.startupUnlockID++
	m.cancelStartupUnlock()
}

func (m *model) stopLifetime() {
	if m.lifetimeCancel == nil {
		return
	}
	m.lifetimeCancel()
	m.lifetimeCancel = nil
}

func (m *model) startStartupRepair() tea.Cmd {
	m.systemHeader = systemHeaderFixing
	return m.startMaintenance(
		maintenanceTriggerBoot,
		nil,
		false,
		"Checking local health",
		"",
	)
}

func (m *model) maintenancePolicyForTrigger(trigger maintenanceTrigger) maintenancePolicy {
	if trigger == maintenanceTriggerDoctor {
		return maintenancePolicyDoctor
	}
	return maintenancePolicyDefault
}

func (m *model) maintenanceModeForTrigger(trigger maintenanceTrigger) readiness.Mode {
	if m.maintenancePolicyForTrigger(trigger) == maintenancePolicyDoctor {
		return readiness.ModeInteractiveDoctor
	}
	return readiness.ModeInteractiveLauncher
}

func (m *model) restartAfterVaultReady() tea.Cmd {
	m.notice = notice{}
	m.onboardingCursor = 0
	m.discardPasswordInput()
	m.screen = screenDashboard
	if m.session.Current().ID == RouteAccountLogin {
		m.session.ReplaceCurrent(Route{ID: RouteDashboardHome})
	}
	m.bootAssessed = false
	m.systemHeader = systemHeaderChecking
	m.maintenanceTrigger = maintenanceTriggerBoot
	m.maintenanceUsedPassword = false
	m.maintenanceAuthEmail = ""
	m.runtimeLoaded = false
	m.signingLoaded = false
	m.signingStatusLoading = false
	m.signingStatusPending = false
	m.signingError = ""
	m.signingLoadID++
	return tea.Batch(
		m.spinner.Tick,
		m.assessCurrentState(),
	)
}

func (m *model) showPasswordScreen(flow passwordFlow, authEmail string, errorText string, reuseCurrentRoute bool) {
	m.showPasswordScreenOnRoute(RouteVaultUnlock, flow, authEmail, errorText, reuseCurrentRoute)
}

func (m *model) showPasswordScreenOnRoute(route RouteID, flow passwordFlow, authEmail string, errorText string, reuseCurrentRoute bool) {
	m.discardPasswordInput()
	if !reuseCurrentRoute && m.session.Current().ID != route {
		m.session.Push(Route{ID: route})
	}

	m.screen = screenPassword
	m.passwordFlow = flow
	m.passwordAuth = authEmail
	m.passwordBusy = false
	m.passwordHideInput = false
	m.passwordBusyMessage = ""
	m.passwordOverlay = reuseCurrentRoute && (flow == passwordKeyView || flow == passwordKeyExport)
	switch flow {
	case passwordCreate:
		m.passwordTitle = "Create local vault"
		m.passwordContext = "Set an encryption password for your vault. Save it securely. If you lose it, your keys are lost."
		m.passwordInput = components.NewCreatePasswordInput()
	case passwordRestore:
		m.passwordTitle = "Master Password"
		m.passwordContext = "Enter your master password to restore and unlock your synced vault."
		m.passwordInput = components.NewUnlockPasswordInput()
	case passwordKeyView:
		m.passwordTitle = "Unlock private key"
		m.passwordContext = "Master password is required to decrypt this vault and copy its private key"
		m.passwordInput = components.NewUnlockPasswordInput()
	case passwordKeyExport:
		m.passwordTitle = "Export vault"
		m.passwordContext = exportPlaintextWarning + " Enter your master password to continue."
		m.passwordInput = components.NewUnlockPasswordInput()
	case passwordStartupUnlock:
		m.passwordTitle = "Unlock Forged"
		m.passwordContext = "Please authenticate to open Forged."
		m.passwordInput = components.NewUnlockPasswordInput()
	case passwordManageChange:
		m.passwordTitle = "Change Master Password"
		m.passwordContext = "Enter your current password, then choose a new master password for this vault."
		m.passwordInput = components.NewChangePasswordInput()
	case passwordDoctorRepair:
		m.passwordTitle = "Fix Issues"
		m.passwordContext = "Enter your master password to repair this machine and restore secure access."
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

func (m *model) discardPasswordInput() {
	if m.passwordInput == nil {
		return
	}
	m.passwordInput.Clear()
	m.passwordInput = nil
}

func (m *model) clearRestorePassword() {
	if m.restorePassword == nil {
		return
	}
	m.restorePassword.clear()
	m.restorePassword = nil
}

func (m *model) showCurrentRoute() tea.Cmd {
	m.notice = notice{}
	m.discardPasswordInput()
	m.screen = screenDashboard
	switch m.session.Current().ID {
	case RouteAccountLogin:
		return m.startLoginFlow()
	case RouteKeysBrowser, RouteKeysDetail, RouteKeysRename, RouteKeysDelete, RouteKeysGenerate, RouteKeysImport, RouteKeysExport:
		return m.startKeyRouteLoad()
	case RouteVaultChangePassword:
		m.showPasswordScreenOnRoute(RouteVaultChangePassword, passwordManageChange, "", "", false)
		return m.passwordInput.Init()
	case RouteAgentHome:
		if strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
			return nil
		}
		return tea.Batch(
			m.refreshSnapshotCmd(),
			m.loadSigningStatusCmd(),
		)
	case RouteAgentSigning:
		if strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
			return nil
		}
		return m.startAgentSigningRoute()
	case RouteAgentRouting:
		if !platform.SSHRoutingSupported() {
			m.session.Reset(Route{ID: RouteAgentHome})
			m.agent.selected = 0
			m.agent.statusErr = "SSH routing is unavailable on this platform"
			m.notice = notice{message: m.agent.statusErr, tone: dashboardscreen.ToneWarning}
			return nil
		}
		if strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
			return nil
		}
		return m.startLabRoutingRoute()
	case RouteVaultHome, RouteAccountStatus, RouteSyncHome, RouteDoctorOverview:
		cmds := []tea.Cmd{}
		if m.session.Current().ID == RouteVaultHome {
			cmds = append(cmds, m.loadSecurityStateCmd())
		}
		if m.session.Current().ID == RouteAccountStatus {
			m.loadStoredAccountIdentity()
		}
		if m.session.Current().ID == RouteDoctorOverview {
			if strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
				return nil
			}
			cmds = append(cmds, m.refreshSnapshotCmd(), m.loadSecurityStateCmd())
		}
		return tea.Batch(cmds...)
	case RouteVaultMasterPasswordInterval:
		m.manage.masterIntervalSelected = m.currentMasterPasswordIntervalIndex()
		return tea.Batch(m.spinner.Tick, m.loadSecurityStateCmd())
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
		return "Manage"
	case RouteVaultChangePassword:
		return "Change Master Password"
	case RouteVaultMasterPasswordInterval:
		return "Master Password Interval"
	case RouteAgentHome:
		return "Agent"
	case RouteAgentSigning:
		return "Commit Signing"
	case RouteAgentRouting:
		return "SSH Routing"
	case RouteAccountStatus:
		return "Profile"
	case RouteSyncHome:
		if !m.snapshot.LoggedIn {
			return "Enable Sync"
		}
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
			{Label: "Manage", Current: true},
		}
	case RouteVaultChangePassword:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Manage"},
			{Label: "Change Master Password", Current: true},
		}
	case RouteVaultMasterPasswordInterval:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Manage"},
			{Label: "Master Password Interval", Current: true},
		}
	case RouteAgentHome:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Agent", Current: true},
		}
	case RouteAgentSigning:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Agent"},
			{Label: "Commit Signing", Current: true},
		}
	case RouteAgentRouting:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Agent"},
			{Label: "SSH Routing", Current: true},
		}
	case RouteAccountStatus:
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Manage"},
			{Label: "Profile", Current: true},
		}
	case RouteSyncHome:
		label := "Sync"
		if !m.snapshot.LoggedIn {
			label = "Enable Sync"
		}
		return []shell.Breadcrumb{
			{Label: "Home"},
			{Label: "Manage"},
			{Label: label, Current: true},
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
	return m.bootAssessed &&
		m.screen == screenDashboard &&
		strings.TrimSpace(m.snapshot.RuntimePathError) == "" &&
		m.session.Current().ID != RouteDoctorOverview &&
		len(m.dashboardOptions()) > 0
}

func (m *model) shouldShowProductRail() bool {
	return !m.snapshot.VaultExists &&
		strings.TrimSpace(m.snapshot.RuntimePathError) == "" &&
		!m.isDoctorOverviewRoute()
}

func (m *model) serverURL() string {
	if server := strings.TrimSpace(m.intent.Param("server")); server != "" {
		return server
	}
	return m.deps.DefaultServer
}

func (m *model) pollRuntimeStatus(delay time.Duration) tea.Cmd {
	if !m.snapshot.VaultExists || strings.TrimSpace(m.snapshot.RuntimePathError) != "" {
		return nil
	}
	m.runtimeStatusID++
	id := m.runtimeStatusID
	if delay <= 0 {
		delay = 50 * time.Millisecond
	}
	loadStatus := m.deps.LoadStatus
	return tea.Tick(delay, func(time.Time) tea.Msg {
		status, err := loadStatus()
		return runtimeStatusMsg{id: id, status: status, err: err}
	})
}

func (m *model) loadSecurityStateCmd() tea.Cmd {
	m.securityLoaded = false
	m.securityLoadErr = ""
	m.securityLoadID++
	id := m.securityLoadID
	loadSecurityState := m.deps.LoadSecurityState
	return func() tea.Msg {
		state, err := loadSecurityState()
		return securityStateMsg{id: id, state: state, err: err}
	}
}

func (m *model) lockSensitiveCmd(id int) tea.Cmd {
	lockSensitive := m.deps.LockSensitive
	return func() tea.Msg {
		return idleLockFinishedMsg{id: id, err: lockSensitive()}
	}
}

func (m *model) copyToClipboard(value string) tea.Cmd {
	m.clipboardBusy = true
	copyText := m.deps.CopyText
	return func() tea.Msg {
		return copyFinishedMsg{err: copyText(value)}
	}
}

func (m *model) openCurrentLoginURL() error {
	url := strings.TrimSpace(m.loginScreen.URL)
	if url == "" {
		return nil
	}
	return m.deps.OpenLink(url)
}

func (m *model) passwordFlowForSnapshot(snapshot readiness.Snapshot) passwordFlow {
	if !snapshot.VaultExists && (snapshot.LoggedIn || m.maintenanceAuthEmail != "") {
		return passwordRestore
	}
	return passwordRepair
}

func (m *model) popWizardRoutes() {
	for m.session.Current().ID == RouteVaultUnlock {
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
