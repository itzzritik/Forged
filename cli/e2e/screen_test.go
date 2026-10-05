//go:build e2e

package e2e

import (
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
)

// screen is a deliberately small VT emulator: enough of ECMA-48 for what
// ConPTY emits so tests can assert on rendered text instead of raw bytes.
type screen struct {
	mu         sync.Mutex
	cols, rows int
	cells      [][]rune
	x, y       int
	savedX     int
	savedY     int
	top        int
	bottom     int
	parser     *ansi.Parser
	reply      func([]byte)
}

func newScreen(cols, rows int, reply func([]byte)) *screen {
	s := &screen{cols: cols, rows: rows, reply: reply}
	s.reset()
	s.parser = ansi.NewParser()
	s.parser.SetParamsSize(32)
	s.parser.SetDataSize(4096)
	s.parser.SetHandler(ansi.Handler{
		Print:     s.print,
		Execute:   s.execute,
		HandleCsi: s.csi,
		HandleEsc: s.esc,
	})
	return s
}

func (s *screen) reset() {
	s.cells = make([][]rune, s.rows)
	for i := range s.cells {
		s.cells[i] = blankLine(s.cols)
	}
	s.x, s.y = 0, 0
	s.top, s.bottom = 0, s.rows-1
}

func blankLine(cols int) []rune {
	line := make([]rune, cols)
	for i := range line {
		line[i] = ' '
	}
	return line
}

func (s *screen) Write(b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.parser.Parse(b)
}

func (s *screen) print(r rune) {
	width := ansi.StringWidth(string(r))
	if width <= 0 {
		return
	}
	if s.x+width > s.cols {
		s.x = 0
		s.lineFeed()
	}
	s.cells[s.y][s.x] = r
	for i := 1; i < width && s.x+i < s.cols; i++ {
		s.cells[s.y][s.x+i] = 0
	}
	s.x += width
	if s.x > s.cols {
		s.x = s.cols
	}
}

func (s *screen) execute(b byte) {
	switch b {
	case '\r':
		s.x = 0
	case '\n', '\v', '\f':
		s.lineFeed()
	case '\b':
		if s.x > 0 {
			s.x--
		}
	case '\t':
		s.x = min((s.x/8+1)*8, s.cols-1)
	}
}

func (s *screen) lineFeed() {
	if s.y == s.bottom {
		s.scrollUp(1)
		return
	}
	if s.y < s.rows-1 {
		s.y++
	}
}

func (s *screen) scrollUp(n int) {
	for ; n > 0; n-- {
		copy(s.cells[s.top:s.bottom], s.cells[s.top+1:s.bottom+1])
		s.cells[s.bottom] = blankLine(s.cols)
	}
}

func (s *screen) scrollDown(n int) {
	for ; n > 0; n-- {
		copy(s.cells[s.top+1:s.bottom+1], s.cells[s.top:s.bottom])
		s.cells[s.top] = blankLine(s.cols)
	}
}

func (s *screen) clampCursor() {
	s.x = max(0, min(s.x, s.cols-1))
	s.y = max(0, min(s.y, s.rows-1))
}

func (s *screen) esc(cmd ansi.Cmd) {
	switch cmd.Final() {
	case '7':
		s.savedX, s.savedY = s.x, s.y
	case '8':
		s.x, s.y = s.savedX, s.savedY
	case 'M':
		if s.y == s.top {
			s.scrollDown(1)
		} else if s.y > 0 {
			s.y--
		}
	case 'c':
		s.reset()
	}
}

func (s *screen) csi(cmd ansi.Cmd, params ansi.Params) {
	p := func(i, def int) int {
		v, _, _ := params.Param(i, def)
		if v == 0 && def > 0 {
			return def
		}
		return v
	}
	if cmd.Prefix() == '?' {
		if mode, _, _ := params.Param(0, 0); mode == 1049 || mode == 47 || mode == 1047 {
			if cmd.Final() == 'h' || cmd.Final() == 'l' {
				s.reset()
			}
		}
		return
	}
	switch cmd.Final() {
	case 'A':
		s.y -= p(0, 1)
	case 'B', 'e':
		s.y += p(0, 1)
	case 'C', 'a':
		s.x += p(0, 1)
	case 'D':
		s.x -= p(0, 1)
	case 'E':
		s.y += p(0, 1)
		s.x = 0
	case 'F':
		s.y -= p(0, 1)
		s.x = 0
	case 'G', '`':
		s.x = p(0, 1) - 1
	case 'd':
		s.y = p(0, 1) - 1
	case 'H', 'f':
		s.y = p(0, 1) - 1
		s.x = p(1, 1) - 1
	case 'J':
		s.eraseDisplay(p(0, 0))
	case 'K':
		s.eraseLine(p(0, 0))
	case 'X':
		for i := 0; i < p(0, 1) && s.x+i < s.cols; i++ {
			s.cells[s.y][s.x+i] = ' '
		}
	case 'P':
		n := min(p(0, 1), s.cols-s.x)
		line := s.cells[s.y]
		copy(line[s.x:], line[s.x+n:])
		for i := s.cols - n; i < s.cols; i++ {
			line[i] = ' '
		}
	case '@':
		n := min(p(0, 1), s.cols-s.x)
		line := s.cells[s.y]
		copy(line[s.x+n:], line[s.x:s.cols-n])
		for i := s.x; i < s.x+n; i++ {
			line[i] = ' '
		}
	case 'L', 'M':
		if s.y < s.top || s.y > s.bottom {
			return
		}
		savedTop := s.top
		s.top = s.y
		if cmd.Final() == 'L' {
			s.scrollDown(p(0, 1))
		} else {
			s.scrollUp(p(0, 1))
		}
		s.top = savedTop
	case 'S':
		s.scrollUp(p(0, 1))
	case 'T':
		s.scrollDown(p(0, 1))
	case 'r':
		s.top = p(0, 1) - 1
		s.bottom = p(1, s.rows) - 1
		if s.top >= s.bottom || s.bottom >= s.rows {
			s.top, s.bottom = 0, s.rows-1
		}
		s.x, s.y = 0, 0
	case 'n':
		if p(0, 0) == 6 && s.reply != nil {
			s.reply([]byte(ansi.CursorPositionReport(s.y+1, s.x+1)))
		}
	}
	s.clampCursor()
}

func (s *screen) eraseDisplay(mode int) {
	switch mode {
	case 0:
		s.eraseLine(0)
		for y := s.y + 1; y < s.rows; y++ {
			s.cells[y] = blankLine(s.cols)
		}
	case 1:
		s.eraseLine(1)
		for y := 0; y < s.y; y++ {
			s.cells[y] = blankLine(s.cols)
		}
	case 2, 3:
		for y := range s.cells {
			s.cells[y] = blankLine(s.cols)
		}
	}
}

func (s *screen) eraseLine(mode int) {
	line := s.cells[s.y]
	from, to := s.x, s.cols
	switch mode {
	case 1:
		from, to = 0, min(s.x+1, s.cols)
	case 2:
		from, to = 0, s.cols
	}
	for i := from; i < to; i++ {
		line[i] = ' '
	}
}

// Text returns the visible screen with trailing spaces trimmed per row.
func (s *screen) Text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	for _, line := range s.cells {
		var row strings.Builder
		for _, r := range line {
			if r != 0 {
				row.WriteRune(r)
			}
		}
		b.WriteString(strings.TrimRight(row.String(), " "))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
