package ui

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func fill(bg color.Color, content string, w int) string {
	on, _, _ := strings.Cut(Fg(P().Text).Background(bg).Render(" "), " ")
	content = Trunc(content, w)
	pad := Repeat(" ", w-Width(content))
	content = strings.NewReplacer("\x1b[m", "\x1b[m"+on, "\x1b[0m", "\x1b[0m"+on).Replace(content)
	return on + content + pad + "\x1b[m"
}

func Panel(w, h int, title, right string, focused bool, body []string) string {
	if w < 4 || h < 2 {
		return ""
	}
	b := P().Line
	if focused {
		b = P().LineFocus
	}
	tl, rl := "", ""
	if title != "" && w >= 8 {
		tl = " " + Trunc(title, w-6) + " "
	}
	if right != "" && w >= Width(title)+Width(right)+10 {
		rl = " " + right + " "
	}
	var out strings.Builder
	out.WriteString(Paint(G.TL+G.H, b) + Bold(tl, P().Text) + Paint(Repeat(G.H, w-4-Width(tl)-Width(rl)), b) + Paint(rl, P().Muted) + Paint(G.H+G.TR, b))
	for i := 0; i < h-2; i++ {
		line := ""
		if i < len(body) {
			line = Trunc(body[i], w-2)
		}
		out.WriteString("\n" + Paint(G.V, b) + Pad(line, w-2) + Paint(G.V, b))
	}
	out.WriteString("\n" + Paint(G.BL+Repeat(G.H, w-2)+G.BR, b))
	return out.String()
}

func SelLine(w int, content string) string {
	return Fg(P().Accent).Background(P().Sel).Render(G.Bar) + fill(P().Sel, content, w-1)
}

func Row(w int, label, value string, selected bool, valueColor color.Color) string {
	vw := Width(value)
	l := Trunc(label, w-vw-5)
	gap := Repeat(" ", w-3-Width(l)-vw)
	if selected {
		return SelLine(w, Bold(" "+l, P().Text)+gap+Paint(value, valueColor))
	}
	return "  " + Paint(l, P().Text) + gap + Paint(value, valueColor)
}

type ButtonKind int

const (
	Primary ButtonKind = iota
	Secondary
	DangerButton
)

func Button(label string, kind ButtonKind) string {
	s := "  " + label + "  "
	switch kind {
	case Primary:
		return Fg(P().OnAccent).Background(P().Accent).Bold(true).Render(s)
	case DangerButton:
		return Fg(P().OnAccent).Background(P().Danger).Bold(true).Render(s)
	}
	return On(s, P().Text, P().Cap)
}

func Banner(w, h int, tone Tone, text, key, action string) string {
	bg := P().WarnBg
	glyph := G.Warn
	switch tone {
	case ToneBad:
		bg = Mix(P().Bg, P().Danger, 0.12)
	case ToneGood:
		glyph = G.Check
	}
	c := ToneColor(tone)
	right := ""
	if key != "" {
		right = Keycap(key)
		if w >= 44 && action != "" {
			right += " " + Bold(action, P().Text)
		}
	}
	mid := h / 2
	lines := make([]string, h)
	for i := range lines {
		content := ""
		if i == mid {
			t := Trunc(text, w-7-Width(right)-3)
			content = "  " + Bold(glyph, c) + " " + Paint(t, P().Text) + Repeat(" ", w-5-Width(t)-Width(right)-2) + right + " "
		}
		lines[i] = Fg(c).Background(bg).Render(G.Bar) + fill(bg, content, w-1)
	}
	return Join(lines)
}

func Toast(text string, tone Tone) string {
	glyph := G.Check
	switch tone {
	case ToneWarn:
		glyph = G.Warn
	case ToneBad:
		glyph = G.Cross
	}
	return On(" ", P().Text, P().Cap) + Fg(ToneColor(tone)).Background(P().Cap).Render(" "+glyph) + On(" "+text+"  ", P().Text, P().Cap)
}

func Progress(w int, frac float64) string {
	frac = max(0, min(1, frac))
	f := int(float64(w)*frac + 0.5)
	var b strings.Builder
	for i := 0; i < f; i++ {
		b.WriteString(Paint(G.Heavy, SweepAt(i, w)))
	}
	b.WriteString(Paint(Repeat(G.H, w-f), P().Line))
	return b.String()
}

func CodeBlock(w int, text string, padded bool) []string {
	var lines []string
	if padded {
		lines = append(lines, "")
	}
	for _, l := range strings.Split(ansi.Hardwrap(Sanitize(text), max(1, w-2), true), "\n") {
		lines = append(lines, " "+l)
	}
	if padded {
		lines = append(lines, "")
	}
	for i, l := range lines {
		lines[i] = On(Pad(Trunc(l, w), w), P().Text, P().Code)
	}
	return lines
}

func Floor(w, h int) string {
	if h <= 0 {
		return ""
	}
	a := Trunc("Make the window a little larger", w-2)
	b := Trunc("Forged needs at least 40 × 12", w-2)
	top := min(max(0, h/2-1), h-1)
	lines := make([]string, h)
	lines[top] = Repeat(" ", (w-Width(a))/2) + Bold(a, P().Text)
	if top+1 < h {
		lines[top+1] = Repeat(" ", (w-Width(b))/2) + Paint(b, P().Muted)
	}
	return Join(lines)
}
