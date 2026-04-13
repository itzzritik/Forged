//go:build darwin

package sensitiveauth

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type darwinNativeProvider struct {
	mu         sync.Mutex
	scriptPath string
}

func NewNativeProvider() NativeProvider {
	return &darwinNativeProvider{}
}

func (p *darwinNativeProvider) Name() string { return "local-authentication" }

func (p *darwinNativeProvider) Authorize(ctx context.Context, action Action) error {
	swiftPath, err := exec.LookPath("swift")
	if err != nil {
		return ErrNativeUnavailable
	}

	scriptPath, err := p.ensureScript()
	if err != nil {
		return ErrNativeUnavailable
	}

	moduleCache := filepath.Join(os.TempDir(), "forged-swift-clang-module-cache")
	tmpDir := filepath.Join(os.TempDir(), "forged-swift-tmp")
	if err := os.MkdirAll(moduleCache, 0o700); err != nil {
		return ErrNativeUnavailable
	}
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return ErrNativeUnavailable
	}

	cmd := exec.CommandContext(ctx, swiftPath, scriptPath, action.NativeReason())
	cmd.Env = append(os.Environ(),
		"CLANG_MODULE_CACHE_PATH="+moduleCache,
		"TMPDIR="+tmpDir,
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return ErrNativeUnavailable
	}

	message := string(output)
	switch exitErr.ExitCode() {
	case 2:
		return ErrNativeUnavailable
	case 3:
		return ErrAuthenticationCanceled
	default:
		if strings.Contains(message, "Couldn’t communicate with a helper application.") ||
			strings.Contains(message, "Couldn't communicate with a helper application.") {
			return ErrNativeUnavailable
		}
		return ErrAuthenticationFailed
	}
}

func (p *darwinNativeProvider) ensureScript() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.scriptPath != "" {
		return p.scriptPath, nil
	}

	dir := filepath.Join(os.TempDir(), "forged-sensitiveauth")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	scriptPath := filepath.Join(dir, "native_auth.swift")
	if err := os.WriteFile(scriptPath, []byte(darwinNativeAuthScript), 0o600); err != nil {
		return "", err
	}

	p.scriptPath = scriptPath
	return scriptPath, nil
}

const darwinNativeAuthScript = `import Foundation
import LocalAuthentication
import Dispatch

let reason = CommandLine.arguments.dropFirst().joined(separator: " ")
let context = LAContext()
var policyError: NSError?
let policy: LAPolicy = .deviceOwnerAuthentication

guard context.canEvaluatePolicy(policy, error: &policyError) else {
    if let policyError {
        fputs(policyError.localizedDescription + "\n", stderr)
    }
    Foundation.exit(2)
}

let semaphore = DispatchSemaphore(value: 0)
var exitCode: Int32 = 1

context.evaluatePolicy(policy, localizedReason: reason.isEmpty ? "Authenticate to continue" : reason) { success, evalError in
    defer { semaphore.signal() }
    if success {
        exitCode = 0
        return
    }

    if let laError = evalError as? LAError {
        switch laError.code {
        case .userCancel, .appCancel, .systemCancel:
            exitCode = 3
        case .biometryNotAvailable, .biometryNotEnrolled, .biometryLockout, .passcodeNotSet, .notInteractive:
            exitCode = 2
        default:
            exitCode = 1
        }
        fputs(laError.localizedDescription + "\n", stderr)
        return
    }

    if let evalError {
        fputs(evalError.localizedDescription + "\n", stderr)
    }
    exitCode = 1
}

semaphore.wait()
Foundation.exit(exitCode)
`
