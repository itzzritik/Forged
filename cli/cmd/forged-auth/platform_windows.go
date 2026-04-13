//go:build windows

package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"os/exec"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
)

func providerName() string { return "windows-hello" }

func authorize(ctx context.Context, action sensitiveauth.Action) string {
	shell, err := windowsPowerShellPath()
	if err != nil {
		return "unavailable"
	}

	cmd := exec.CommandContext(
		ctx,
		shell,
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-EncodedCommand",
		encodePowerShellCommand(windowsHelloScript(action.NativeReason())),
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return "ok"
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return "unavailable"
	}

	switch exitErr.ExitCode() {
	case 2:
		return "unavailable"
	case 3:
		return "canceled"
	default:
		if strings.Contains(strings.ToLower(string(output)), "notsupportedexception") {
			return "unavailable"
		}
		return "failed"
	}
}

func startLockLoop(onLock func()) {
	if onLock == nil {
		return
	}
	go watchWindowsLocks(onLock)
}

func windowsPowerShellPath() (string, error) {
	for _, candidate := range []string{"powershell.exe", "pwsh.exe"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", exec.ErrNotFound
}

func encodePowerShellCommand(script string) string {
	encoded := utf16.Encode([]rune(script))
	bytes := make([]byte, len(encoded)*2)
	for i, r := range encoded {
		bytes[i*2] = byte(r)
		bytes[i*2+1] = byte(r >> 8)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func powershellQuote(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func windowsHelloScript(reason string) string {
	return `
$ErrorActionPreference = 'Stop'
try {
  Add-Type -ReferencedAssemblies @('System.Runtime.WindowsRuntime', "$env:SystemRoot\System32\WinMetadata\Windows.winmd") -TypeDefinition @'
using System;
using Windows.Security.Credentials.UI;

public static class ForgedWindowsHello {
    public static int Run(string message) {
        try {
            var availability = UserConsentVerifier.CheckAvailabilityAsync().AsTask().GetAwaiter().GetResult();
            if (availability != UserConsentVerifierAvailability.Available) {
                return 2;
            }

            var result = UserConsentVerifier.RequestVerificationAsync(message).AsTask().GetAwaiter().GetResult();
            switch (result) {
                case UserConsentVerificationResult.Verified:
                    return 0;
                case UserConsentVerificationResult.Canceled:
                    return 3;
                case UserConsentVerificationResult.DeviceNotPresent:
                case UserConsentVerificationResult.NotConfiguredForUser:
                case UserConsentVerificationResult.DisabledByPolicy:
                    return 2;
                default:
                    return 1;
            }
        } catch {
            return 1;
        }
    }
}
'@
} catch {
  exit 2
}

exit [ForgedWindowsHello]::Run('` + powershellQuote(reason) + `')
`
}

func watchWindowsLocks(onLock func()) {
	shell, err := windowsPowerShellPath()
	if err != nil {
		return
	}

	script := `
$source = 'ForgedSessionLock'
Register-WmiEvent -Class Win32_SessionChangeEvent -SourceIdentifier $source | Out-Null
try {
  while ($true) {
    $event = Wait-Event -SourceIdentifier $source
    if ($null -eq $event) { continue }
    try {
      if ($event.SourceEventArgs.NewEvent.Reason -eq 7) {
        Write-Output 'LOCK'
      }
    } finally {
      Remove-Event -EventIdentifier $event.EventIdentifier -ErrorAction SilentlyContinue | Out-Null
    }
  }
} finally {
  Get-EventSubscriber -SourceIdentifier $source -ErrorAction SilentlyContinue | Unregister-Event -Force -ErrorAction SilentlyContinue
}
`

	for {
		cmd := exec.Command(
			shell,
			"-NoProfile",
			"-NonInteractive",
			"-ExecutionPolicy",
			"Bypass",
			"-Command",
			script,
		)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		if err := cmd.Start(); err != nil {
			time.Sleep(time.Second)
			continue
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == "LOCK" {
				onLock()
			}
		}

		_ = cmd.Wait()
		time.Sleep(time.Second)
	}
}
