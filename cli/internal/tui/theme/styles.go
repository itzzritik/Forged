package theme

import (
	"strings"

	"charm.land/lipgloss/v2"
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
			Foreground(ColorText)

	Kicker = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorAccent)

	Wordmark = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	Subtitle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	Context = lipgloss.NewStyle().
		Foreground(ColorSubtle)

	BrandBanner = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	HeaderFrame = lipgloss.NewStyle().
			BorderStyle(NormalBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	Breadcrumb = lipgloss.NewStyle().
			Foreground(ColorText)

	BreadcrumbCurrent = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorAccent)

	BreadcrumbSeparator = lipgloss.NewStyle().
				Foreground(ColorSubtle)

	HeaderSidebar = lipgloss.NewStyle().
			PaddingLeft(1)

	HeaderSeparator = lipgloss.NewStyle().
			Faint(true).
			Foreground(ColorBorder)

	HeaderVersionLabel = lipgloss.NewStyle().
				Foreground(ColorSubtle)

	HeaderVersionValue = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorText)

	HeroTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	HeroMeta = lipgloss.NewStyle().
			Foreground(ColorAccent)

	SectionTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	Body = lipgloss.NewStyle().
		Foreground(ColorMuted)

	BodyMuted = lipgloss.NewStyle().
			Foreground(ColorSubtle)

	BodyStrong = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	RowLabel = lipgloss.NewStyle().
			Foreground(ColorSubtle)

	RowValue = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	Bullet = lipgloss.NewStyle().
		Foreground(ColorAccent)

	Spinner = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorAccent)

	FooterKey = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	FooterLabel = lipgloss.NewStyle().
			Foreground(ColorMuted)

	DividerStyle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	Link = lipgloss.NewStyle().
		Foreground(ColorText)

	LinkHost = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	LinkPath = lipgloss.NewStyle().
			Foreground(ColorMuted)

	Success = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSuccess)

	Warning = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWarning)

	Danger = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorDanger)

	CodeFrame = lipgloss.NewStyle().
			BorderStyle(RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	CodeLabel = lipgloss.NewStyle().
			Foreground(ColorSubtle)

	CodeValue = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	AsideRail = lipgloss.NewStyle().
			BorderStyle(NormalBorder()).
			BorderLeft(true).
			BorderForeground(ColorBorder).
			PaddingLeft(2)

	AlertRail = lipgloss.NewStyle().
			BorderStyle(NormalBorder()).
			BorderLeft(true).
			BorderForeground(ColorDanger).
			PaddingLeft(2)

	FieldLabel = lipgloss.NewStyle().
			Foreground(ColorSubtle)

	FieldHint = lipgloss.NewStyle().
			Foreground(ColorMuted)

	FieldValue = lipgloss.NewStyle().
			Foreground(ColorText)

	FieldLineIdle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	FieldLineActive = lipgloss.NewStyle().
			Foreground(ColorAccent)
)

func Chip(label string, tone Tone) string {
	style := lipgloss.NewStyle().Bold(true)

	switch tone {
	case ToneAccent:
		style = style.Foreground(ColorAccent)
	case ToneSuccess:
		style = style.Foreground(ColorSuccess)
	case ToneWarning:
		style = style.Foreground(ColorWarning)
	case ToneDanger:
		style = style.Foreground(ColorDanger)
	default:
		style = style.Foreground(ColorMuted)
	}

	return style.Render(label)
}

func Divider(width int) string {
	if width <= 0 {
		return ""
	}
	return DividerStyle.Render(strings.Repeat(Glyphs.Horizontal, width))
}
