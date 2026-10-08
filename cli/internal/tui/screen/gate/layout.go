package gate

import (
	"image/color"

	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/flame"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type placement struct {
	compact      bool
	cardX, cardY int
	cardW, cardH int
	mark         int
	markY        int
	fireTop      int
}

func place(w, h int, fire bool, height func(cardW int, compact bool) int) placement {
	cw := min(52, w-4)
	fireTop := -1
	if fire && w >= 40 && h >= 16 {
		fireTop = h - int(float64(h)*0.33+0.5)
	}
	choose := func(lim int) (placement, bool) {
		var opts []placement
		for _, c := range []bool{false, true} {
			ch := height(cw, c)
			y := (h - ch) / 2
			if y+ch > lim {
				y = lim - ch
			}
			if y >= 1 {
				opts = append(opts, placement{compact: c, cardY: y, cardH: ch})
			}
		}
		for _, need := range []int{6, 3} {
			for _, o := range opts {
				if o.cardY-1 >= need {
					return o, true
				}
			}
		}
		if len(opts) > 0 {
			return opts[len(opts)-1], true
		}
		return placement{}, false
	}
	lim := h
	if fireTop >= 0 {
		lim = fireTop
	}
	p, ok := choose(lim)
	if !ok {
		fireTop = -1
		if p, ok = choose(h); !ok {
			ch := height(cw, true)
			p = placement{compact: true, cardY: max(0, (h-ch)/2), cardH: ch}
		}
	}
	p.cardW, p.cardX, p.fireTop = cw, (w-cw)/2, fireTop
	room := p.cardY - 1
	switch {
	case room >= 6 && w >= 30:
		p.mark, p.markY = 3, max(1, max((p.cardY-3)/2, p.cardY-7))
	case room >= 3:
		p.mark, p.markY = 1, p.cardY-2
	}
	return p
}

type rect struct{ x, y, w, h int }

func (r rect) has(x, y int) bool { return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h }

func (m *Model) syncFire(w, rows int) {
	if rows < 4 {
		m.fire = nil
		return
	}
	if fw, fr := m.fire.Size(); fw == w && fr == rows {
		return
	}
	m.fire = flame.New(w, rows)
	m.fire.SetHeat(m.heat())
}

func (m *Model) heat() float64 {
	if m.busy != "" {
		return 1.25
	}
	return 1
}

func (m *Model) View(st *core.State, w, h int) string {
	if w < 40 || h < 12 {
		return ui.Floor(w, h)
	}
	p := place(w, h, !m.noFire, func(cw int, compact bool) int { return len(m.rows(st, cw, compact)) + 2 })
	if p.fireTop >= 0 {
		m.syncFire(w, h-p.fireTop)
	} else {
		m.fire = nil
	}
	m.fire.SetHeat(m.heat())

	card := rect{p.cardX, p.cardY, p.cardW, p.cardH}
	var layers []ui.Layer
	var mark rect
	switch p.mark {
	case 3:
		wm := ui.Wordmark()
		ww := 0
		for _, l := range wm {
			ww = max(ww, ui.Width(l))
		}
		mark = rect{(w - ww) / 2, p.markY, ww, len(wm)}
		layers = append(layers, ui.Layer{Content: ui.Join(wm), X: mark.x, Y: mark.y})
	case 1:
		mark = rect{(w - 6) / 2, p.markY, 6, 1}
		layers = append(layers, ui.Layer{Content: ui.Bold("forged", ui.P().Accent), X: mark.x, Y: mark.y})
	}
	if m.fire != nil {
		layers = append([]ui.Layer{{Content: ui.Join(flame.Render(m.fire)), Y: p.fireTop}}, layers...)
	}
	layers = append(layers, ui.Layer{Content: ui.Panel(p.cardW, p.cardH, "", "", false, m.rows(st, p.cardW, p.compact)), X: p.cardX, Y: p.cardY})
	if m.fire != nil {
		for _, s := range m.fire.Sparks(p.fireTop) {
			if s.X < 0 || s.X >= w || s.Row < 0 || s.Row >= h || card.has(s.X, s.Row) || mark.has(s.X, s.Row) {
				continue
			}
			layers = append(layers, ui.Layer{Content: ui.Paint(ui.G.Spark, sparkColor(s.Life)), X: s.X, Y: s.Row})
		}
	}
	return ui.Compose(w, h, "", layers...)
}

func sparkColor(life float64) color.Color {
	sp := ui.P().Spark
	if life > 0.5 {
		return ui.Mix(sp[1], sp[0], (life-0.5)*2)
	}
	return ui.Mix(ui.P().Bg, sp[1], life*2)
}
