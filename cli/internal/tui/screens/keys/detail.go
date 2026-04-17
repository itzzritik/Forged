package keys

import (
	"fmt"
	"strings"

	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type DetailScreen struct {
	Loading bool
	Error   string
	Key     actions.KeyDetail
}

func RenderDetail(screen DetailScreen, spinner string, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth+10))
	if screen.Loading {
		return theme.BodyStrong.Render(theme.Spinner.Render(spinner) + " Loading key details")
	}
	if msg := strings.TrimSpace(screen.Error); msg != "" {
		return theme.Danger.Render("✕ " + msg)
	}

	lines := []string{
		renderDetailRow("Name", screen.Key.Name),
		renderDetailRow("Type", strings.ToUpper(screen.Key.Type)),
		renderDetailRow("Fingerprint", screen.Key.Fingerprint),
	}
	if comment := strings.TrimSpace(screen.Key.Comment); comment != "" {
		lines = append(lines, renderDetailRow("Comment", comment))
	}
	lines = append(lines, renderDetailRow("Private key", "••••••••••••••••"))

	sections := []string{strings.Join(lines, "\n")}

	publicKey := strings.TrimSpace(screen.Key.PublicKey)
	if publicKey != "" {
		sections = append(sections, "", theme.SectionTitle.Render("Public key"), theme.Body.Width(contentWidth).Render(publicKey))
	}

	meta := buildMetadata(screen.Key)
	if len(meta) > 0 {
		sections = append(sections, "", theme.SectionTitle.Render("Metadata"), strings.Join(meta, "\n"))
	}

	return strings.Join(sections, "\n")
}

func renderDetailRow(label, value string) string {
	return padRight(theme.RowLabel.Render(strings.ToUpper(label)), 15) + theme.BodyStrong.Render(value)
}

func buildMetadata(key actions.KeyDetail) []string {
	lines := make([]string, 0, 6)
	if created := strings.TrimSpace(key.CreatedAt); created != "" {
		lines = append(lines, renderDetailRow("Created", created))
	}
	if updated := strings.TrimSpace(key.UpdatedAt); updated != "" {
		lines = append(lines, renderDetailRow("Updated", updated))
	}
	if lastUsed := strings.TrimSpace(key.LastUsedAt); lastUsed != "" {
		lines = append(lines, renderDetailRow("Last used", lastUsed))
	}
	if key.Version > 0 {
		lines = append(lines, renderDetailRow("Version", fmt.Sprintf("%d", key.Version)))
	}
	if origin := strings.TrimSpace(key.DeviceOrigin); origin != "" {
		lines = append(lines, renderDetailRow("Device", origin))
	}
	lines = append(lines, renderDetailRow("Git signing", boolLabel(key.GitSigning)))
	return lines
}

func boolLabel(value bool) string {
	if value {
		return "Enabled"
	}
	return "Disabled"
}
