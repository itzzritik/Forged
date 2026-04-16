package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Tone string

const (
	ToneNeutral Tone = "neutral"
	ToneAccent  Tone = "accent"
	ToneSuccess Tone = "success"
	ToneWarning Tone = "warning"
	ToneDanger  Tone = "danger"
)

var (
	AppBackground = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorText))

	Kicker = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorAccent))

	Wordmark = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorText))

	Subtitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted))

	Context = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSubtle))

	BrandBanner = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorAccent))

	HeaderFrame = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Padding(1, 2)

	Breadcrumb = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorText))

	BreadcrumbCurrent = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(ColorAccent))

	BreadcrumbSeparator = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSubtle))

	HeaderSidebar = lipgloss.NewStyle().
			PaddingLeft(1)

	HeaderSeparator = lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color(ColorBorder))

	HeaderVersionLabel = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSubtle))

	HeaderVersionValue = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(ColorText))

	HeroTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorText))

	HeroMeta = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorAccent))

	SectionTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorText))

	Body = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorMuted))

	BodyMuted = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSubtle))

	BodyStrong = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorText))

	RowLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSubtle))

	RowValue = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorText))

	Bullet = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorAccent))

	Spinner = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorAccent))

	FooterKey = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorAccent))

	FooterLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted))

	DividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBorder))

	Link = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorText))

	LinkHost = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorText))

	LinkPath = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted))

	Success = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorSuccess))

	Warning = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorWarning))

	Danger = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorDanger))

	CodeFrame = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Padding(0, 1)

	CodeLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSubtle))

	CodeValue = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorText))

	AsideRail = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(lipgloss.Color(ColorBorder)).
			PaddingLeft(2)

	AlertRail = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(lipgloss.Color(ColorDanger)).
			PaddingLeft(2)

	FieldLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSubtle))

	FieldHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted))

	FieldValue = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorText))

	FieldLineIdle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBorder))

	FieldLineActive = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorAccent))
)

func Chip(label string, tone Tone) string {
	style := lipgloss.NewStyle().Bold(true)

	switch tone {
	case ToneAccent:
		style = style.Foreground(lipgloss.Color(ColorAccent))
	case ToneSuccess:
		style = style.Foreground(lipgloss.Color(ColorSuccess))
	case ToneWarning:
		style = style.Foreground(lipgloss.Color(ColorWarning))
	case ToneDanger:
		style = style.Foreground(lipgloss.Color(ColorDanger))
	default:
		style = style.Foreground(lipgloss.Color(ColorMuted))
	}

	return style.Render(label)
}

func Divider(width int) string {
	if width <= 0 {
		return ""
	}
	return DividerStyle.Render(strings.Repeat("─", width))
}
