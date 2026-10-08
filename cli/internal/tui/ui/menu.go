package ui

type MenuItem struct {
	Key, Label  string
	Danger, Sep bool
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
	body := []string{""}
	for i := start; i < start+n && i < len(items); i++ {
		it := items[i]
		if it.Sep {
			body = append(body, "  "+Paint(Repeat(G.H, inner-4), P().Rule))
			continue
		}
		c := P().Text
		if it.Danger {
			c = P().Danger
		}
		left := ""
		if it.Key != "" {
			left = Bold(Pad(it.Key, 3), P().Muted)
		}
		text := Trunc(it.Label, inner-6-Width(left))
		line := Pad("  "+left+Paint(text, c), inner-1)
		if i == cursor {
			line = SelLine(inner, " "+left+Bold(text, c)+Repeat(" ", inner-3-Width(left)-Width(text)))
		}
		body = append(body, line)
	}
	body = append(body, "")
	return Panel(w, len(body)+2, title, "", true, body)
}
