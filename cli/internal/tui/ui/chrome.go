package ui

import (
	"image/color"
	"math"
	"strconv"
	"strings"
)

type Tone int

const (
	ToneGood Tone = iota
	ToneWarn
	ToneBad
	ToneBusy
)

func ToneColor(t Tone) color.Color {
	switch t {
	case ToneWarn:
		return P().Warn
	case ToneBad:
		return P().Danger
	case ToneBusy:
		return P().Accent
	}
	return P().Good
}

func SpinnerGlyph(frame int) string {
	c := SweepAt(int(math.Round((math.Sin(float64(frame)/9)+1)*10)), 21)
	return Paint(G.Spinner[frame%len(G.Spinner)], c)
}

type Hint struct{ Key, Label string }

type Chip struct {
	Glyph, Label string
	Tone         Tone
}

func Keycap(k string) string { return On(" "+k+" ", P().Text, P().Cap) }

func HintsWidth(h []Hint, labels bool) int {
	if len(h) == 0 {
		return 0
	}
	w, gap := 0, 2
	if labels {
		gap = 3
	}
	for _, x := range h {
		w += Width(x.Key) + 2
		if labels {
			w += 1 + Width(x.Label)
		}
	}
	return w + gap*(len(h)-1)
}

func Hints(h []Hint, labels bool) string {
	var b strings.Builder
	for i, x := range h {
		if i > 0 {
			if labels {
				b.WriteString("   ")
			} else {
				b.WriteString("  ")
			}
		}
		b.WriteString(Fg(P().Text).Bold(true).Background(P().Cap).Render(" " + x.Key + " "))
		if labels {
			b.WriteString(" " + Paint(x.Label, P().Muted))
		}
	}
	return b.String()
}

func Footer(w int, left, right []Hint) string {
	avail := w - 4
	more := right
	if len(more) > 1 {
		more = right[:1]
	}
	first := func(n int) []Hint { return left[:min(n, len(left))] }
	cands := []struct {
		l, r []Hint
		lab  bool
	}{
		{left, right, true}, {left, more, true}, {first(3), more, true}, {first(2), more, true},
		{first(1), more, true}, {first(3), more, false}, {first(2), more, false}, {nil, more, true}, {nil, more, false},
	}
	for _, c := range cands {
		lw, rw := HintsWidth(c.l, c.lab), HintsWidth(c.r, c.lab)
		gap := 0
		if lw > 0 && rw > 0 {
			gap = 3
		}
		if lw+rw+gap <= avail {
			return "  " + Hints(c.l, c.lab) + Repeat(" ", w-4-lw-rw) + Hints(c.r, c.lab) + "  "
		}
	}
	return ""
}

func Header(w int, tabs []string, active int, chip Chip) string {
	chipLabel := w >= 64
	chipW := 1
	if chipLabel {
		chipW = Width(chip.Label) + 2
	}
	chipX := w - 2 - chipW
	brand := "  " + Bold("forged", P().Accent)
	compactX := 10
	under := make([]string, w)
	for i := range under {
		under[i] = " "
		if i >= 2 && i < w-2 {
			under[i] = Paint(G.H, P().Rule)
		}
	}

	gap := 2
	if w >= 90 {
		gap = 4
	}
	full := 0
	for _, t := range tabs {
		full += Width(t)
	}
	full += gap * (len(tabs) - 1)
	mark := func(from, n int) {
		for i := 0; i < n && from+i < w; i++ {
			if from+i >= 0 {
				under[from+i] = Paint(G.Heavy, SweepAt(i, n))
			}
		}
	}
	var line strings.Builder
	line.WriteString(brand)
	x := max(Width(brand)+2, min((w-full)/2, chipX-2-full))
	if x+full <= chipX-2 {
		line.WriteString(Repeat(" ", x-Width(brand)))
		for i, t := range tabs {
			if i == active {
				line.WriteString(Bold(t, P().Text))
				mark(x-1, Width(t)+2)
			} else {
				line.WriteString(Paint(t, P().Muted))
			}
			if i < len(tabs)-1 {
				line.WriteString(Repeat(" ", gap))
			}
			x += Width(t) + gap
		}
		x -= gap
	} else {
		x = compactX
		t := tabs[active]
		pos := strconv.Itoa(active+1) + "/" + strconv.Itoa(len(tabs))
		line.WriteString(Repeat(" ", x-Width(brand)) + Paint(G.Prev, P().Muted) + " " + Bold(t, P().Text) + " " + Paint(G.Next, P().Muted) + "  " + Paint(pos, P().Faint))
		mark(x+1, Width(t)+2)
		x += 6 + Width(t) + Width(pos)
	}
	line.WriteString(Repeat(" ", chipX-x))
	c := ToneColor(chip.Tone)
	line.WriteString(Bold(chip.Glyph, c))
	if chipLabel {
		lc := P().Muted
		if chip.Tone != ToneGood {
			lc = c
		}
		line.WriteString(" " + Paint(chip.Label, lc))
	}
	line.WriteString("  ")
	return Trunc(line.String(), w) + "\n" + strings.Join(under, "")
}
