package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type Tone string

const (
	ToneNone    Tone = "none"
	ToneSuccess Tone = "success"
	ToneWarning Tone = "warning"
	ToneDanger  Tone = "danger"
)

type Notice struct {
	Message string
	Tone    Tone
}

type Issue struct {
	Title  string
	Detail string
}

type Option struct {
	Label    string
	Selected bool
}

type Screen struct {
	Title       string
	Context     string
	Snapshot    readiness.Snapshot
	Options     []Option
	Issues      []Issue
	ShowSummary bool
	Notice      Notice
}

func Render(screen Screen, width int) string {
	sections := make([]string, 0, 6)
	if notice := renderNotice(screen.Notice); notice != "" {
		sections = append(sections, notice)
	}

	if strings.TrimSpace(screen.Context) != "" {
		sections = append(sections, theme.Body.Width(max(28, min(width, theme.HeroMaxWidth))).Render(screen.Context))
	}

	if len(screen.Options) > 0 {
		sections = append(sections, "", renderOptions(screen.Options))
	}

	if len(screen.Issues) > 0 {
		sections = append(sections, "", renderIssues(screen.Issues))
	}

	if screen.ShowSummary {
		sections = append(sections, "", renderSummary(screen.Snapshot))
	}
	return strings.Join(sections, "\n")
}

func renderNotice(notice Notice) string {
	if strings.TrimSpace(notice.Message) == "" {
		return ""
	}

	switch notice.Tone {
	case ToneSuccess:
		return theme.Success.Render("✓ " + notice.Message)
	case ToneWarning:
		return theme.Warning.Render("! " + notice.Message)
	case ToneDanger:
		return theme.Danger.Render("✕ " + notice.Message)
	default:
		return theme.BodyStrong.Render(notice.Message)
	}
}

func renderOptions(options []Option) string {
	lines := []string{theme.SectionTitle.Render("Choose how to start")}
	for _, option := range options {
		line := "  " + theme.Body.Render(option.Label)
		if option.Selected {
			line = theme.Kicker.Render("›") + " " + theme.BodyStrong.Render(option.Label)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func renderSummary(snapshot readiness.Snapshot) string {
	daemonState := "Offline"
	if snapshot.Service.Running {
		daemonState = "Running"
	}

	socketState := "Waiting"
	if snapshot.IPCSocketReady && snapshot.AgentSocketReady {
		socketState = "Ready"
	}

	sshState := "Not active"
	if snapshot.SSHEnabled && snapshot.ManagedConfigReady {
		sshState = "Active"
	}

	syncState := "Not linked"
	if snapshot.LoggedIn {
		syncState = "Linked"
	}

	rows := []struct {
		label string
		value string
	}{
		{label: "State", value: string(snapshot.State)},
		{label: "Keys", value: fmt.Sprintf("%d loaded", snapshot.KeyCount)},
		{label: "Daemon", value: daemonState},
		{label: "Sockets", value: socketState},
		{label: "SSH", value: sshState},
		{label: "Sync", value: syncState},
	}

	lines := []string{theme.SectionTitle.Render("Machine")}
	for _, row := range rows {
		left := theme.RowLabel.Render(strings.ToUpper(row.label))
		right := theme.RowValue.Render(row.value)
		gap := max(2, 14-lipgloss.Width(left))
		lines = append(lines, left+strings.Repeat(" ", gap)+right)
	}
	return strings.Join(lines, "\n")
}

func renderIssues(issues []Issue) string {
	lines := []string{theme.SectionTitle.Render("Issues")}
	for index, issue := range issues {
		if index > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, theme.Danger.Render("✕")+" "+theme.BodyStrong.Render(issue.Title))
		if strings.TrimSpace(issue.Detail) != "" {
			lines = append(lines, "  "+theme.Body.Render(issue.Detail))
		}
	}
	return strings.Join(lines, "\n")
}
