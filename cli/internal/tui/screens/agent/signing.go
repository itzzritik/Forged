package agent

import (
	"strings"

	"github.com/itzzritik/forged/cli/internal/actions"
	commonscreen "github.com/itzzritik/forged/cli/internal/tui/screens/common"
	keyscreen "github.com/itzzritik/forged/cli/internal/tui/screens/keys"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type SigningScreen struct {
	Loading     bool
	Busy        bool
	BusyMessage string
	Status      actions.CommitSigningStatus
	Browser     keyscreen.BrowserScreen
}

func RenderSigning(screen SigningScreen, spinner string, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth+10))
	if screen.Loading && len(screen.Browser.Rows) == 0 && strings.TrimSpace(screen.Browser.Error) == "" {
		return commonscreen.RenderFullPageLoader(commonscreen.FullPageLoaderScreen{
			Title:       "Opening commit signing",
			Description: "Reading current Git signing state and loading vault keys",
		}, spinner, contentWidth)
	}

	sections := []string{
		renderStatusCard(screen, spinner, contentWidth),
		"",
		keyscreen.RenderBrowser(screen.Browser, spinner, contentWidth),
	}
	return strings.Join(sections, "\n")
}

func renderStatusCard(screen SigningScreen, spinner string, width int) string {
	statusLine := ""
	details := []string{}

	switch screen.Status.Mode {
	case actions.CommitSigningForged:
		if keyName := strings.TrimSpace(screen.Status.KeyName); keyName != "" {
			statusLine = theme.Success.Render("✓ Signing with Forged: ") + theme.BodyStrong.Render(keyName)
		} else {
			statusLine = theme.Success.Render("✓ Signing with Forged")
		}
		if publicKey := strings.TrimSpace(screen.Status.PublicKey); publicKey != "" {
			details = append(details, theme.Body.Render(compactSigningValue(publicKey, max(28, width-10))))
		} else if fingerprint := strings.TrimSpace(screen.Status.Fingerprint); fingerprint != "" {
			details = append(details, theme.Body.Render(fingerprint))
		}
	case actions.CommitSigningExternal:
		statusLine = theme.Warning.Render("! External Signing")
		if publicKey := strings.TrimSpace(screen.Status.PublicKey); publicKey != "" {
			details = append(details, theme.Body.Render(compactSigningValue(publicKey, max(28, width-10))))
		}
	default:
		statusLine = theme.Warning.Render("! Commits on this machine are not being signed")
	}

	if screen.Busy {
		message := strings.TrimSpace(screen.BusyMessage)
		if message == "" {
			message = "Updating signing configuration"
		}
		statusLine = theme.BodyStrong.Render(spinner + " " + message)
		details = nil
	}

	lines := []string{statusLine}
	if len(details) > 0 {
		lines = append(lines, details...)
	}

	return strings.Join(lines, "\n")
}

func compactSigningValue(value string, maxRunes int) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	if len(fields) == 1 {
		return compactMiddle(fields[0], maxRunes)
	}

	bodyWidth := max(12, maxRunes-len(fields[0])-1)
	return fields[0] + " " + compactMiddle(fields[1], bodyWidth)
}

func compactMiddle(value string, maxRunes int) string {
	runes := []rune(value)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 12 {
		return string(runes[:maxRunes])
	}

	head := max(8, (maxRunes-1)/2)
	tail := max(6, maxRunes-head-1)
	if head+tail+1 > maxRunes {
		tail = max(1, maxRunes-head-1)
	}
	if head+tail >= len(runes) {
		return value
	}
	return string(runes[:head]) + "…" + string(runes[len(runes)-tail:])
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
