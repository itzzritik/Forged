package shell

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

const (
	ContentLeftInset  = 2
	ContentRightInset = 4
	fullBleedMarker   = "\x00full-bleed\x00"
	bottomDockMarker  = "\x00dock-bottom\x00"
	fixedBodyHeight   = 19
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
	return max(16, ContentWidth(termWidth)-ContentLeftInset-ContentRightInset)
}

func IndentBlock(block string, spaces int) string {
	if strings.TrimSpace(block) == "" || spaces <= 0 {
		return block
	}

	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(block, "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, fullBleedMarker) {
			lines[index] = strings.TrimPrefix(line, fullBleedMarker)
			continue
		}
		if line == bottomDockMarker {
			continue
		}
		if line == "" {
			continue
		}
		lines[index] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func FullBleed(line string) string {
	return fullBleedMarker + line
}

func DockBottom(top string, bottom string) string {
	if strings.TrimSpace(bottom) == "" {
		return strings.TrimRight(top, "\n")
	}
	if strings.TrimSpace(top) == "" {
		return bottomDockMarker + "\n" + strings.TrimLeft(bottom, "\n")
	}
	return top + "\n" + bottomDockMarker + "\n" + bottom
}

func CenterInFixedBody(width int, block string) string {
	if width <= 0 || strings.TrimSpace(block) == "" {
		return block
	}
	return lipgloss.Place(width, fixedBodyHeight, lipgloss.Center, lipgloss.Center, block)
}

func Render(termWidth int, termHeight int, header string, body string, footer string, tightFooter bool, tightBody bool) string {
	body, bodyPresent := fitBodyHeight(termWidth, termHeight, header, body, footer, tightFooter, tightBody)
	chunks := make([]string, 0, 4)

	if header != "" {
		chunks = append(chunks, header)
	}
	if bodyPresent || body != "" {
		if len(chunks) > 0 {
			if !tightBody {
				chunks = append(chunks, "")
			}
		}
		chunks = append(chunks, body)
	}
	if footer != "" {
		if len(chunks) > 0 {
			if tightFooter {
				chunks = append(chunks, theme.Divider(ContentWidth(termWidth)), "", footer)
			} else {
				chunks = append(chunks, "", theme.Divider(ContentWidth(termWidth)), "", footer)
			}
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
	width := BodyWidth(termWidth)
	if preferred > 0 && preferred < width {
		return preferred
	}
	return width
}

func fitBodyHeight(termWidth int, termHeight int, header string, body string, footer string, tightFooter bool, tightBody bool) (string, bool) {
	bodyHeight := fixedBodyHeight
	if termHeight > 0 {
		available := availableBodyHeight(termWidth, termHeight, header, footer, tightFooter, tightBody)
		if available <= 0 {
			return "", false
		}
		bodyHeight = min(bodyHeight, available)
	}

	return fitBodyBlock(body, bodyHeight), true
}

func availableBodyHeight(termWidth int, termHeight int, header string, footer string, tightFooter bool, tightBody bool) int {
	reserved := 2
	if header != "" {
		reserved += lipgloss.Height(header)
	}
	if footer != "" {
		reserved += lipgloss.Height(footer)
		if tightFooter {
			reserved += 2
		} else {
			reserved += 3
		}
	}

	bodyHeight := termHeight - reserved
	if bodyHeight > 0 && header != "" && !tightBody {
		bodyHeight--
	}
	return max(0, bodyHeight)
}

func fitBodyBlock(body string, height int) string {
	if height <= 0 {
		return ""
	}

	top, bottom, docked := splitDockedBody(body)
	if !docked {
		return blockFromLines(fitLines(blockLines(body), height))
	}
	return blockFromLines(fitDockedLines(blockLines(top), blockLines(bottom), height))
}

func splitDockedBody(body string) (string, string, bool) {
	parts := strings.SplitN(body, bottomDockMarker, 2)
	if len(parts) != 2 {
		return body, "", false
	}

	top := strings.TrimSuffix(parts[0], "\n")
	bottom := strings.TrimPrefix(parts[1], "\n")
	return top, bottom, true
}

func blockLines(body string) []string {
	if body == "" {
		return nil
	}
	return strings.Split(body, "\n")
}

func fitLines(lines []string, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(lines) > height {
		return lines[:height]
	}
	if len(lines) < height {
		padding := make([]string, height-len(lines))
		lines = append(lines, padding...)
	}
	return lines
}

func fitDockedLines(top []string, bottom []string, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(bottom) == 0 {
		return fitLines(top, height)
	}
	if len(bottom) >= height {
		return bottom[len(bottom)-height:]
	}

	availableTop := height - len(bottom)
	if len(top) > availableTop {
		top = top[:availableTop]
	}

	padding := make([]string, max(0, height-len(top)-len(bottom)))
	lines := make([]string, 0, height)
	lines = append(lines, top...)
	lines = append(lines, padding...)
	lines = append(lines, bottom...)
	return lines
}

func blockFromLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
