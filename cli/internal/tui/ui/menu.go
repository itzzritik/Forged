package ui

type MenuItem struct {
	Key, Icon, Label string
	Danger, Sep      bool
}

func Menu(w int, title string, items []MenuItem, cursor, maxH int) string {
	if len(items)+4 > maxH {
		kept := items[:0:0]
		newCursor := 0
		for i, it := range items {
			if !it.Sep {
				kept = append(kept, it)
				if i < cursor {
					newCursor++
				}
			}
		}
		items, cursor = kept, min(newCursor, len(kept)-1)
	}
	n := max(1, min(len(items), maxH-4))
	start := max(0, min(cursor-n+1, len(items)-n))
	inner := w - 2
	iw := 0
	for _, it := range items {
		iw = max(iw, IconWidth(it.Icon))
	}
	body := []string{""}
	for i := start; i < start+n && i < len(items); i++ {
		it := items[i]
		if it.Sep {
			body = append(body, "  "+Paint(Repeat(G.H, inner-4), P().Rule))
			continue
		}
		c, ic := P().Text, P().Muted
		if it.Danger {
			c, ic = P().Danger, P().Danger
		}
		kw := Width(it.Key)
		text := Trunc(it.Label, inner-4-iw-kw)
		gap := Repeat(" ", inner-3-iw-Width(text)-kw)
		key := Bold(it.Key, P().Muted)
		line := "  " + Pad(Icon(it.Icon, ic), iw) + Paint(text, c) + gap + key
		if i == cursor {
			if !it.Danger {
				ic = P().Accent
			}
			line = SelLine(inner, " "+Pad(Icon(it.Icon, ic), iw)+Bold(text, c)+gap+key+" ")
		}
		body = append(body, line)
	}
	body = append(body, "")
	return Panel(w, len(body)+2, title, "", true, body)
}
