package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/picker"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
)

const tuiErrorDedupeWindow = 30 * time.Second

type reportedError struct {
	fingerprint [32]byte
	loggedAt    time.Time
}

func (m *model) reportError(action string, err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, picker.ErrCanceled) ||
		errors.Is(err, sensitiveauth.ErrAuthenticationCanceled) ||
		actions.IsSensitiveAuthRequired(err) {
		return message
	}

	route := "unknown"
	if m.session != nil && m.session.Current().ID != "" {
		route = string(m.session.Current().ID)
	}
	action = strings.TrimSpace(action)
	if action == "" {
		action = "unknown"
	}
	key := route + "\x00" + action
	now := time.Now()
	fingerprint := actions.DiagnosticErrorFingerprint(message)
	if m.reportedErrors == nil {
		m.reportedErrors = make(map[string]reportedError)
	}
	if previous, ok := m.reportedErrors[key]; ok &&
		previous.fingerprint == fingerprint &&
		now.Sub(previous.loggedAt) < tuiErrorDedupeWindow {
		return message
	}
	m.reportedErrors[key] = reportedError{fingerprint: fingerprint, loggedAt: now}
	m.deps.LogError(actions.DiagnosticErrorEvent{
		Route:   route,
		Action:  action,
		Version: m.deps.AppVersion,
		Message: message,
	})
	return message
}

func (m *model) reportErrorText(action string, message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	return m.reportError(action, errors.New(message))
}
