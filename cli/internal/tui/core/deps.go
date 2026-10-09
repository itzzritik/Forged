package core

import (
	"context"
	"fmt"
	"reflect"

	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/readiness"
)

type ClipboardLease interface{ ClearIfUnchanged() (bool, error) }

type Deps struct {
	Repair                    func(readiness.RunOptions) (readiness.RunResult, error)
	CreateVault               func([]byte) error
	RestoreVault              func([]byte) error
	StartLogin                func(context.Context, string, func(actions.LoginProgress)) (actions.LoginSession, error)
	SaveCredentials           func(actions.AccountCredentials) error
	LoadCredentials           func() (actions.AccountCredentials, error)
	ClearCredentials          func() error
	TriggerSync               func() error
	LoadStatus                func() (actions.RuntimeStatus, error)
	LoadSecurityState         func() (actions.SecurityState, error)
	SetMasterPasswordInterval func(string) error
	HasLocalUnlockTrust       func() bool
	UnlockSensitiveLaunch     func(context.Context, []byte, bool) (actions.UnlockResult, error)
	EnableHeadlessUnlock      func() (bool, error)
	DisableHeadlessUnlock     func() error
	SkipHeadlessOffer         func() error
	ChangePassword            func([]byte, []byte) (actions.ChangePasswordResult, error)
	LoadSigningStatus         func() (actions.CommitSigningStatus, error)
	EnableSSHAgent            func() error
	DisableSSHAgent           func() error
	EnableCommitSigning       func(string) (actions.CommitSigningStatus, error)
	DisableCommitSigning      func() (actions.CommitSigningStatus, error)
	LoadSSHRoutingDebug       func() (actions.SSHRoutingDebug, error)
	ClearSSHRoute             func(string) error
	ClearAllSSHRoutes         func() error
	ListKeys                  func() ([]actions.KeySummary, error)
	ListLocalKeys             func() ([]actions.KeySummary, error)
	ViewKey                   func(string) (actions.KeyDetail, error)
	ViewFullKey               func(string, []byte) (actions.KeyDetail, error)
	GenerateKey               func(string) (actions.GenerateResult, error)
	RenameKey                 func(string, string) (actions.RenameResult, error)
	DeleteKey                 func(string, string) (string, error)
	PreviewImport             func(source, file string) (actions.ImportPreviewResult, error)
	PreviewImportText         func(string) (actions.ImportPreviewResult, error)
	ImportPreviews            func(string, int, []actions.ImportPreview) (actions.ImportResult, error)
	DefaultExportPath         func() string
	AuthorizeExport           func([]byte) (string, error)
	ExportVault               func(path, token string) (actions.ExportResult, error)
	ChooseFile                func() (string, error)
	ChooseSavePath            func(string) (string, error)
	CopyText                  func(string) error
	CopySensitiveText         func(string) (ClipboardLease, error)
	CloseClipboard            func() error
	OpenLink                  func(string) error
	LogError                  func(actions.DiagnosticErrorEvent)
	DefaultServer             string
	AppVersion                string
	TerminalClipboard         bool
	CanOpenLinks              bool
	CanPickFiles              bool
	Remote                    bool
}

func (d Deps) Validate() error {
	v := reflect.ValueOf(d)
	for i := 0; i < v.NumField(); i++ {
		if f := v.Field(i); f.Kind() == reflect.Func && f.IsNil() {
			return fmt.Errorf("TUI dependency %s is required", v.Type().Field(i).Name)
		}
	}
	return nil
}
