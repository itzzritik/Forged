package core

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/picker"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const reportDedupeWindow = 30 * time.Second

type reported struct {
	fingerprint [sha256.Size]byte
	at          time.Time
}

type Reporter struct {
	deps Deps
	seen map[string]reported
}

func NewReporter(deps Deps) *Reporter {
	return &Reporter{deps: deps, seen: map[string]reported{}}
}

func (r *Reporter) Report(route, action string, err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	display := capitalize(ui.Sanitize(message))
	if r == nil ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, picker.ErrCanceled) ||
		errors.Is(err, sensitiveauth.ErrAuthenticationCanceled) ||
		actions.IsSensitiveAuthRequired(err) {
		return display
	}
	if route == "" {
		route = "unknown"
	}
	if action = strings.TrimSpace(action); action == "" {
		action = "unknown"
	}
	key := route + "\x00" + action
	now := time.Now()
	fingerprint := actions.DiagnosticErrorFingerprint(message)
	if prev, ok := r.seen[key]; ok && prev.fingerprint == fingerprint && now.Sub(prev.at) < reportDedupeWindow {
		return display
	}
	r.seen[key] = reported{fingerprint: fingerprint, at: now}
	r.deps.LogError(actions.DiagnosticErrorEvent{Route: route, Action: action, Version: r.deps.AppVersion, Message: message})
	return display
}

func capitalize(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}
