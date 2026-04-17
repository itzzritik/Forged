package readiness

import "errors"

type repairState struct {
	result   RunResult
	password []byte
}

func (e *Engine) Run(opts RunOptions) (RunResult, error) {
	snapshot, err := e.Assess()
	if err != nil {
		return RunResult{}, err
	}
	if opts.Mode == ModeAssessOnly {
		return RunResult{
			Snapshot: snapshot,
			Next:     NextActionNone,
		}, nil
	}
	return e.repair(snapshot, opts)
}

func (e *Engine) repair(current Snapshot, opts RunOptions) (RunResult, error) {
	state := &repairState{
		result: RunResult{
			Snapshot: current,
			Next:     NextActionNone,
		},
	}

	e.emitProgress(opts, ProgressConfig)
	if err := e.ensureConfigStage(state); err != nil {
		return state.result, err
	}
	e.emitProgress(opts, ProgressSSH)
	if err := e.ensureSSHStage(state); err != nil {
		return state.result, err
	}
	e.emitProgress(opts, ProgressVault)
	if err := e.ensureVaultAndCredentialsStage(state, opts); err != nil {
		return state.result, err
	}
	if state.result.Next != NextActionNone {
		state.result.Snapshot.State = classifyState(state.result.Snapshot)
		return state.result, nil
	}
	e.emitProgress(opts, ProgressService)
	if err := e.ensureServiceStage(state, opts); err != nil {
		return state.result, err
	}
	e.emitProgress(opts, ProgressSockets)
	if err := e.ensureSocketStage(state); err != nil {
		return state.result, err
	}

	state.result.Snapshot.State = classifyState(state.result.Snapshot)
	return state.result, nil
}

func (e *Engine) emitProgress(opts RunOptions, stage ProgressStage) {
	if opts.Progress == nil {
		return
	}
	opts.Progress(stage)
}

func (e *Engine) ensureConfigStage(state *repairState) error {
	if state.result.Snapshot.ConfigExists {
		return nil
	}
	if err := e.ensureConfigFile(e.Paths); err != nil {
		e.markFailed(&state.result.Summary, "config")
		return nil
	}
	if err := e.refreshSnapshot(state); err != nil {
		return err
	}
	if state.result.Snapshot.ConfigExists {
		e.markFixed(&state.result.Summary, "config")
		return nil
	}
	e.markFailed(&state.result.Summary, "config")
	return nil
}

func (e *Engine) ensureSSHStage(state *repairState) error {
	if state.result.Snapshot.SSHEnabled &&
		state.result.Snapshot.ManagedConfigReady &&
		state.result.Snapshot.IdentityAgentOwner.IsForged() {
		return nil
	}

	if err := e.enableSSHConfig(e.Paths); err != nil {
		e.markFailed(&state.result.Summary, "ssh")
		return nil
	}
	if err := e.refreshSnapshot(state); err != nil {
		return err
	}
	if state.result.Snapshot.SSHEnabled &&
		state.result.Snapshot.ManagedConfigReady &&
		state.result.Snapshot.IdentityAgentOwner.IsForged() {
		e.markFixed(&state.result.Summary, "ssh")
		return nil
	}
	e.markFailed(&state.result.Summary, "ssh")
	return nil
}

func (e *Engine) ensureVaultAndCredentialsStage(state *repairState, opts RunOptions) error {
	if state.result.Snapshot.VaultExists {
		return nil
	}
	if state.result.Snapshot.LoggedIn {
		return e.restoreLinkedVault(state, opts)
	}
	state.result.Next = NextActionNeedsInteractiveSetup
	return nil
}

func (e *Engine) restoreLinkedVault(state *repairState, opts RunOptions) error {
	plan, err := prepareLinkedRestore(e.Paths)
	switch {
	case errors.Is(err, errNoRemoteLinkedVault):
		state.result.Next = NextActionNeedsInteractiveSetup
		return nil
	case err != nil:
		return err
	}

	if opts.PromptPassword == nil {
		state.result.Next = NextActionNeedsPassword
		return nil
	}

	password, err := opts.PromptPassword("Enter your Forged master password to restore your linked vault on this device:")
	if err != nil || len(password) == 0 {
		state.result.Next = NextActionNeedsPassword
		return nil
	}

	if err := applyLinkedRestore(e.Paths, plan, password); err != nil {
		if errors.Is(err, errInvalidRestorePassword) {
			state.result.Next = NextActionNeedsPassword
			return nil
		}
		return err
	}

	state.password = append([]byte(nil), password...)
	e.markFixed(&state.result.Summary, "vault")
	return e.refreshSnapshot(state)
}

func (e *Engine) ensureServiceStage(state *repairState, opts RunOptions) error {
	if !serviceNeedsRepair(state.result.Snapshot) {
		return nil
	}

	password, err := e.servicePasswordForRepair(state, opts)
	if err != nil {
		return err
	}
	if len(password) == 0 {
		state.result.Next = NextActionNeedsPassword
		e.markFailed(&state.result.Summary, "service")
		return nil
	}

	if err := e.ensureServiceWithPassword(password); err != nil {
		if updated, waitErr := e.waitForServiceReady(); waitErr == nil {
			state.result.Snapshot = updated
			if serviceHealthy(updated) {
				e.markFixed(&state.result.Summary, "service")
				return nil
			}
		}

		e.pauseForServiceRetry()
		if retryErr := e.ensureServiceWithPassword(password); retryErr != nil {
			if updated, waitErr := e.waitForServiceReady(); waitErr == nil {
				state.result.Snapshot = updated
				if serviceHealthy(updated) {
					e.markFixed(&state.result.Summary, "service")
					return nil
				}
			}
			return retryErr
		}
	}
	updated, err := e.waitForServiceReady()
	if err != nil {
		return err
	}
	state.result.Snapshot = updated
	if serviceHealthy(updated) {
		e.markFixed(&state.result.Summary, "service")
		return nil
	}
	e.markFailed(&state.result.Summary, "service")
	return nil
}

func (e *Engine) ensureSocketStage(state *repairState) error {
	if state.result.Snapshot.IPCSocketReady && state.result.Snapshot.AgentSocketReady {
		return nil
	}
	if !state.result.Snapshot.Service.Running {
		return nil
	}

	updated, err := e.waitForServiceReady()
	if err != nil {
		return err
	}
	state.result.Snapshot = updated
	if updated.IPCSocketReady && updated.AgentSocketReady {
		e.markFixed(&state.result.Summary, "service")
		return nil
	}
	e.markFailed(&state.result.Summary, "service")
	return nil
}

func (e *Engine) servicePasswordForRepair(state *repairState, opts RunOptions) ([]byte, error) {
	if ok, err := passwordUnlocksVault(e.Paths, state.password); err != nil {
		return nil, err
	} else if ok {
		return append([]byte(nil), state.password...), nil
	}

	if installedPassword, err := e.installedServicePassword(); err == nil && installedPassword != "" {
		candidate := []byte(installedPassword)
		if ok, verifyErr := passwordUnlocksVault(e.Paths, candidate); verifyErr != nil {
			return nil, verifyErr
		} else if ok {
			state.password = append([]byte(nil), candidate...)
			return append([]byte(nil), candidate...), nil
		}
	}

	if opts.PromptPassword == nil {
		return nil, nil
	}

	password, err := opts.PromptPassword("Enter your Forged master password to repair the background service:")
	if err != nil || len(password) == 0 {
		return nil, nil
	}
	ok, err := passwordUnlocksVault(e.Paths, password)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	state.password = append([]byte(nil), password...)
	return append([]byte(nil), password...), nil
}

func (e *Engine) refreshSnapshot(state *repairState) error {
	updated, err := e.Assess()
	if err != nil {
		return err
	}
	state.result.Snapshot = updated
	return nil
}
