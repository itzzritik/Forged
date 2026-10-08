package ui

import (
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

type Secret struct {
	buf   []rune
	limit int
}

func NewSecret(limit int) Secret { return Secret{buf: make([]rune, 0, limit), limit: limit} }

func (s *Secret) add(r rune) {
	if !unicode.IsControl(r) && len(s.buf) < s.limit {
		s.buf = append(s.buf, r)
	}
}

func (s *Secret) Update(msg tea.Msg) bool {
	switch m := msg.(type) {
	case tea.PasteMsg:
		for _, r := range m.Content {
			s.add(r)
		}
		return true
	case tea.KeyPressMsg:
		switch m.String() {
		case "backspace":
			if n := len(s.buf); n > 0 {
				s.buf[n-1] = 0
				s.buf = s.buf[:n-1]
			}
			return true
		case "ctrl+u":
			s.Wipe()
			return true
		}
		if m.Text != "" {
			for _, r := range m.Text {
				s.add(r)
			}
			return true
		}
	}
	return false
}

func (s *Secret) Len() int { return len(s.buf) }

func (s *Secret) Wipe() {
	for i := range s.buf {
		s.buf[i] = 0
	}
	s.buf = s.buf[:0]
}

// Peek returns a copy; the caller must clear it.
func (s *Secret) Peek() []byte {
	out := make([]byte, 0, len(s.buf)*utf8.UTFMax)
	for _, r := range s.buf {
		out = utf8.AppendRune(out, r)
	}
	return out
}

func (s *Secret) Take() []byte {
	out := s.Peek()
	s.Wipe()
	return out
}

func (s *Secret) View(w int, focused bool) string {
	n := min(len(s.buf), max(0, w-3))
	text := " " + Repeat(G.Mask, n)
	cursor := ""
	if focused {
		cursor = On(" ", P().OnAccent, P().Accent)
	}
	return On(text, P().Text, P().Sel) + cursor + On(Repeat(" ", w-Width(text)-Width(cursor)), P().Text, P().Sel)
}
