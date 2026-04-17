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

type ScreenKind string

const (
	ScreenKindRepair ScreenKind = "repair"
	ScreenKindSetup  ScreenKind = "setup"
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
	Kind        ScreenKind
	Title       string
	Context     string
	SetupStatus string
	Tasks       []Task
	StatusRows  []StatusRow
	Error       string
}

func Render(screen TaskScreen, spinner string, width int) string {
	if screen.Kind == ScreenKindSetup {
		return renderSetup(screen, spinner, width)
	}

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

func renderSetup(screen TaskScreen, spinner string, width int) string {
	sections := make([]string, 0, 6)
	if strings.TrimSpace(screen.Context) != "" {
		sections = append(sections, theme.Body.Width(max(28, min(width, theme.HeroMaxWidth))).Render(screen.Context))
	}

	activeLabel := strings.TrimSpace(screen.SetupStatus)
	if activeLabel == "" {
		activeLabel = activeTaskLabel(screen.Tasks)
	}
	if strings.TrimSpace(activeLabel) == "" {
		activeLabel = "Preparing secure access"
	}
	sections = append(sections, "", theme.BodyStrong.Render(theme.Spinner.Render(spinner)+" "+activeLabel))

	if len(screen.Tasks) > 0 {
		lines := make([]string, 0, len(screen.Tasks))
		for _, task := range screen.Tasks {
			lines = append(lines, renderTask(task, spinner))
		}
		sections = append(sections, "", strings.Join(lines, "\n"))
	}

	if screen.Error != "" {
		sections = append(sections, "", theme.Danger.Render("✕ "+screen.Error))
	} else {
		sections = append(sections, "", theme.BodyMuted.Render("This usually takes a few seconds"))
	}

	return strings.Join(sections, "\n")
}

func SetupStatusLabel(label string) string {
	switch label {
	case "Password":
		return "Securing local vault"
	case "Account":
		return "Linking account"
	case "Vault":
		return "Preparing vault"
	case "Service":
		return "Setting background service"
	case "SSH":
		return "Configuring SSH routing"
	case "Agent":
		return "Bringing agent online"
	default:
		return label
	}
}

func activeTaskLabel(tasks []Task) string {
	activeCount := 0
	for _, task := range tasks {
		if task.State == TaskActive {
			activeCount++
		}
	}
	if activeCount > 1 {
		return "Preparing secure access"
	}

	for _, task := range tasks {
		if task.State == TaskActive {
			return SetupStatusLabel(task.Label)
		}
	}
	return ""
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
