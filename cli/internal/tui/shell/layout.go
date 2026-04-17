package shell

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

const (
	ContentLeftInset  = 2
	ContentRightInset = 4
)

func ContentWidth(termWidth int) int {
	if termWidth <= 0 {
		return theme.ShellMaxContentWidth
	}

	available := termWidth - theme.ShellHorizontalInset
	if available < 24 {
		return max(16, termWidth-4)
	}
	if available < theme.ShellMinContentWidth {
		return available
	}
	if available > theme.ShellMaxContentWidth {
		return theme.ShellMaxContentWidth
	}

	return available
}

func BodyWidth(termWidth int) int {
	return max(16, ContentWidth(termWidth)-ContentLeftInset)
}

func IndentBlock(block string, spaces int) string {
	if strings.TrimSpace(block) == "" || spaces <= 0 {
		return block
	}

	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(block, "\n")
	for index, line := range lines {
		if line == "" {
			continue
		}
		lines[index] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func Render(termWidth int, header string, body string, footer string) string {
	chunks := make([]string, 0, 4)

	if header != "" {
		chunks = append(chunks, header)
	}
	if body != "" {
		if len(chunks) > 0 {
			chunks = append(chunks, "")
		}
		chunks = append(chunks, body)
	}
	if footer != "" {
		if len(chunks) > 0 {
			chunks = append(chunks, "", theme.Divider(ContentWidth(termWidth)), "", footer)
		} else {
			chunks = append(chunks, footer)
		}
	}

	container := lipgloss.NewStyle().
		MarginLeft(theme.ShellLeftInset).
		MaxWidth(ContentWidth(termWidth)).
		Render(strings.Join(chunks, "\n"))

	return "\n" + container + "\n"
}

func JoinRow(width int, left string, right string) string {
	if right == "" {
		return left
	}

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap <= 1 {
		return left + "\n" + right
	}
	return left + strings.Repeat(" ", gap) + right
}

func ClampBlockWidth(termWidth int, preferred int) int {
	width := ContentWidth(termWidth)
	if preferred > 0 && preferred < width {
		return preferred
	}
	return width
}
