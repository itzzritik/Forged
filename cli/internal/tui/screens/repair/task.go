package repair

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type TaskState string

const (
	TaskPending TaskState = "pending"
	TaskActive  TaskState = "active"
	TaskDone    TaskState = "done"
	TaskFailed  TaskState = "failed"
)

type Task struct {
	Label string
	State TaskState
}

type StatusRow struct {
	Label string
	Value string
}

type TaskScreen struct {
	Title      string
	Context    string
	Tasks      []Task
	StatusRows []StatusRow
	Error      string
}

func Render(screen TaskScreen, spinner string, width int) string {
	sections := make([]string, 0, 4)
	if strings.TrimSpace(screen.Context) != "" {
		sections = append(sections, theme.Body.Width(max(28, min(width, theme.HeroMaxWidth))).Render(screen.Context))
	}

	if len(screen.Tasks) > 0 {
		lines := []string{theme.SectionTitle.Render("Progress")}
		for _, task := range screen.Tasks {
			lines = append(lines, renderTask(task, spinner))
		}
		sections = append(sections, "", strings.Join(lines, "\n"))
	}

	if len(screen.StatusRows) > 0 {
		lines := []string{theme.SectionTitle.Render("Machine")}
		for _, row := range screen.StatusRows {
			lines = append(lines, renderStatusRow(row))
		}
		sections = append(sections, "", strings.Join(lines, "\n"))
	}

	if screen.Error != "" {
		sections = append(sections, "", theme.Danger.Render("✕ "+screen.Error))
	}

	return strings.Join(sections, "\n")
}

func renderTask(task Task, spinner string) string {
	label := theme.Body.Render(task.Label)
	switch task.State {
	case TaskDone:
		return theme.Success.Render("✓") + " " + label
	case TaskFailed:
		return theme.Danger.Render("✕") + " " + label
	case TaskActive:
		return theme.Spinner.Render(spinner) + " " + theme.BodyStrong.Render(task.Label)
	default:
		return theme.BodyMuted.Render("·") + " " + label
	}
}

func renderStatusRow(row StatusRow) string {
	left := theme.RowLabel.Render(strings.ToUpper(row.Label))
	right := theme.RowValue.Render(row.Value)
	gap := max(2, 14-lipgloss.Width(left))
	return left + strings.Repeat(" ", gap) + right
}
