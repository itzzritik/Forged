package components

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

// StyleTextInput applies the Forged field styles. Bubbles v1 had a single style set for
// every focus state, so focused and blurred inputs render alike.
func StyleTextInput(input *textinput.Model) {
	styles := input.Styles()
	for _, state := range []*textinput.StyleState{&styles.Focused, &styles.Blurred} {
		state.Text = theme.FieldValue
		state.Placeholder = theme.BodyMuted
		state.Prompt = lipgloss.NewStyle()
	}
	styles.Cursor.Color = theme.FooterKey.GetForeground()
	input.SetStyles(styles)
}
