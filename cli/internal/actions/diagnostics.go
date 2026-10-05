package actions

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
)

const (
	maxDiagnosticErrorRunes = 2048
	maxTUILogBytes          = 10 << 20
)

var (
	diagnosticBasicPattern        = regexp.MustCompile(`(?i)\bbasic\s+[a-z0-9._~+/=-]+`)
	diagnosticBearerPattern       = regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]+`)
	diagnosticEmailPattern        = regexp.MustCompile(`(?i)\b[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}\b`)
	diagnosticJSONSecretPattern   = regexp.MustCompile(`(?i)(["'](?:access[_-]?token|refresh[_-]?token|id[_-]?token|token|password|passphrase|secret|client[_-]?secret|api[_-]?key|private[_-]?key|authorization|auth[_-]?code|verification|verification[_-]?code|one[_-]?time[_-]?code|otp|cookie|set-cookie|session(?:[_-]?id)?)["']\s*[:=]\s*)(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|\[[^]]*\]|\{[^}]*\}|[^,}\s]+)`)
	diagnosticJWTPattern          = regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\b`)
	diagnosticLeadingContext      = regexp.MustCompile(`^[^:\r\n]{1,160}:\s+`)
	diagnosticOpaqueSecretPattern = regexp.MustCompile(`\b[a-zA-Z0-9_+/=-]{32,}\b`)
	diagnosticPathPattern         = regexp.MustCompile(`(?i)(?:(?:[a-z]:[\\/]|\\\\)[^\s"'<>]+|(?:[^\s"'<>:/\\]+[\\/])+[^\s"'<>]*|[\\/][^\s"'<>]+)`)
	diagnosticPEMPattern          = regexp.MustCompile(`(?s)-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----`)
	diagnosticQuotedDoublePattern = regexp.MustCompile(`"(?:\\.|[^"\\])*"`)
	diagnosticQuotedSinglePattern = regexp.MustCompile(`'(?:\\.|[^'\\])*'`)
	diagnosticSecretPattern       = regexp.MustCompile(`(?i)(^|[^a-z0-9])(access[_-]?token|refresh[_-]?token|id[_-]?token|token|password|passphrase|secret|client[_-]?secret|api[_-]?key|private[_-]?key|authorization|auth[_-]?code|verification|verification[_-]?code|one[_-]?time[_-]?code|otp|cookie|set-cookie|session(?:[_-]?id)?)(\s*[:=]\s*)(?:"[^"]*"|'[^']*'|(?:basic|bearer)\s+[^\s,;}]+|[^\s,;}]+)`)
	diagnosticURLPattern          = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s"'<>]+`)
)

type DiagnosticErrorEvent struct {
	Route   string
	Action  string
	Version string
	Message string
}

func AppendDiagnosticError(paths config.Paths, event DiagnosticErrorEvent) error {
	message := sanitizeDiagnosticError(event.Message)
	if message == "" {
		return nil
	}

	line := fmt.Sprintf(
		"time=%s level=ERROR msg=%q source=tui version=%q route=%q action=%q error=%q\n",
		time.Now().UTC().Format(time.RFC3339Nano),
		"tui action failed",
		diagnosticField(event.Version),
		diagnosticField(event.Route),
		diagnosticField(event.Action),
		message,
	)
	return appendTUILogLine(paths, line)
}

func DiagnosticErrorFingerprint(message string) [sha256.Size]byte {
	return sha256.Sum256([]byte(sanitizeDiagnosticError(message)))
}

func sanitizeDiagnosticError(message string) string {
	message = strings.ToValidUTF8(message, "�")
	message = diagnosticPEMPattern.ReplaceAllString(message, "[private key redacted]")
	message = diagnosticURLPattern.ReplaceAllStringFunc(message, redactDiagnosticURL)
	message = diagnosticJSONSecretPattern.ReplaceAllString(message, "$1[redacted]")
	message = diagnosticSecretPattern.ReplaceAllString(message, "$1$2$3[redacted]")
	message = diagnosticBasicPattern.ReplaceAllString(message, "Basic [redacted]")
	message = diagnosticBearerPattern.ReplaceAllString(message, "Bearer [redacted]")
	message = diagnosticJWTPattern.ReplaceAllString(message, "[token redacted]")
	message = diagnosticOpaqueSecretPattern.ReplaceAllString(message, "[value redacted]")
	message = diagnosticEmailPattern.ReplaceAllString(message, "[email redacted]")
	message = diagnosticPathPattern.ReplaceAllString(message, "[path redacted]")
	message = diagnosticQuotedDoublePattern.ReplaceAllString(message, `"[value redacted]"`)
	message = diagnosticQuotedSinglePattern.ReplaceAllString(message, `'[value redacted]'`)
	message = strings.Join(strings.Fields(message), " ")
	message = diagnosticLeadingContext.ReplaceAllString(message, "[context redacted]: ")
	runes := []rune(message)
	if len(runes) > maxDiagnosticErrorRunes {
		message = string(runes[:maxDiagnosticErrorRunes]) + "..."
	}
	return message
}

func appendTUILogLine(paths config.Paths, line string) error {
	if line == "" {
		return nil
	}
	if len(line) > maxTUILogBytes {
		return fmt.Errorf("TUI log entry exceeds size limit")
	}

	path := paths.TUILogFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}
	lockFile, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("opening TUI log lock: %w", err)
	}
	defer lockFile.Close()

	if err := platform.LockFileWait(lockFile); err != nil {
		return fmt.Errorf("locking TUI log: %w", err)
	}
	defer platform.UnlockFile(lockFile)

	// No O_APPEND: on Windows it drops the write access Truncate needs.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("opening TUI log: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("checking TUI log size: %w", err)
	}
	if info.Size()+int64(len(line)) > maxTUILogBytes {
		if err := file.Truncate(0); err != nil {
			return fmt.Errorf("resetting full TUI log: %w", err)
		}
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seeking TUI log: %w", err)
	}
	if _, err := file.WriteString(line); err != nil {
		return fmt.Errorf("writing TUI log: %w", err)
	}
	return nil
}

func redactDiagnosticURL(raw string) string {
	value := strings.TrimRight(raw, ".,);]")
	return "[url redacted]" + raw[len(value):]
}

func diagnosticField(value string) string {
	value = strings.Join(strings.Fields(strings.ToValidUTF8(value, "�")), " ")
	runes := []rune(value)
	if len(runes) > 128 {
		return string(runes[:128])
	}
	return value
}
