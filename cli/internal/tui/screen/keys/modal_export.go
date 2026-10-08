package keys

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/picker"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const (
	exportW                       = 56
	exportPlaintextWarning        = "This export contains every private key in plaintext. Store it securely, avoid shared or cloud-synced folders, and delete it when finished."
	exportPlaintextSuccessWarning = "Contains plaintext private keys. Store it securely and delete it when finished."
)

const (
	exportAuth = iota
	exportPath
	exportDone
)

type exportAuthMsg struct {
	id    core.ID
	token string
	err   error
}

type exportPickMsg struct {
	id   core.ID
	path string
	err  error
}

type exportWroteMsg struct {
	id  core.ID
	res actions.ExportResult
	err error
}

type exportModal struct {
	step    int
	sec     ui.Secret
	token   string
	def     string
	busy    string
	writing bool
	errMsg  string
	form    *huh.Form
	path    string
	formW   int
	res     actions.ExportResult
	id      core.ID
}

func ExportModal() widget.Modal { return &exportModal{sec: ui.NewSecret(128)} }

func (m *exportModal) Spinning() bool { return m.busy != "" }

func (m *exportModal) Capturing() bool { return m.step == exportAuth || m.step == exportPath }

func (m *exportModal) Actions(*core.State) []core.Action {
	if m.writing {
		return nil
	}
	if m.busy != "" {
		return []core.Action{{Key: "esc", Label: "Cancel"}}
	}
	switch m.step {
	case exportPath:
		return []core.Action{{Key: "enter", Label: "Save"}, {Key: "tab", Label: "Choose file"}, {Key: "esc", Label: "Cancel"}}
	case exportDone:
		return []core.Action{{Key: "enter", Label: "Done"}}
	}
	return []core.Action{{Key: "enter", Label: "Continue"}, {Key: "esc", Label: "Cancel"}}
}

func (m *exportModal) begin(busy string) {
	m.id, m.busy, m.errMsg = core.NextID(), busy, ""
}

func (m *exportModal) reset() {
	m.sec.Wipe()
	m.token, m.busy, m.writing, m.form, m.path = "", "", false, nil, ""
	m.id, m.step = core.NextID(), exportAuth
}

func (m *exportModal) close() tea.Cmd {
	m.reset()
	return core.Close(m)
}

func (m *exportModal) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	return m, m.update(msg, st)
}

func (m *exportModal) update(msg tea.Msg, st *core.State) tea.Cmd {
	switch msg := msg.(type) {
	case core.LockedMsg:
		m.reset()
		m.errMsg = ""
	case exportAuthMsg:
		if msg.id != m.id || m.busy == "" {
			return nil
		}
		m.busy = ""
		if msg.err != nil {
			m.errMsg = st.Reporter.Report("keys", "authorize export", msg.err)
			return nil
		}
		m.token = msg.token
		m.def = st.Deps.DefaultExportPath()
		m.begin("Choosing export file")
		return chooseSave(st, m.id, filepath.Base(m.def))
	case exportPickMsg:
		if msg.id != m.id || m.busy == "" {
			return nil
		}
		m.busy = ""
		if msg.err == nil && strings.TrimSpace(msg.path) != "" {
			return m.write(st, strings.TrimSpace(msg.path))
		}
		m.showPath()
		switch {
		case errors.Is(msg.err, picker.ErrCanceled), msg.err == nil:
		case errors.Is(msg.err, picker.ErrUnavailable):
			st.Reporter.Report("keys", "export file picker", msg.err)
			m.errMsg = "File picker unavailable. Enter an export path instead"
		default:
			st.Reporter.Report("keys", "export file picker", msg.err)
			m.errMsg = "File picker failed. Enter an export path instead"
		}
	case exportWroteMsg:
		if msg.id != m.id || !m.writing {
			return nil
		}
		m.busy, m.writing, m.token = "", false, ""
		if msg.err != nil {
			m.step, m.form = exportAuth, nil
			m.errMsg = st.Reporter.Report("keys", "export vault", msg.err)
			return nil
		}
		m.res, m.step, m.errMsg = msg.res, exportDone, ""
	case tea.PasteMsg:
		if m.busy != "" {
			return nil
		}
		switch m.step {
		case exportAuth:
			m.errMsg = ""
			m.sec.Update(msg)
		case exportPath:
			m.errMsg = ""
			_, cmd := m.form.Update(msg)
			return cmd
		}
	case tea.KeyPressMsg:
		return m.key(msg, st)
	default:
		if m.step == exportPath && m.busy == "" && m.form != nil {
			_, cmd := m.form.Update(msg)
			return cmd
		}
	}
	return nil
}

func (m *exportModal) key(msg tea.KeyPressMsg, st *core.State) tea.Cmd {
	k := msg.String()
	if m.writing {
		return nil
	}
	if m.busy != "" {
		if k == "esc" {
			return m.close()
		}
		return nil
	}
	switch m.step {
	case exportAuth:
		switch k {
		case "esc":
			return m.close()
		case "enter":
			if m.sec.Len() == 0 {
				m.errMsg = "Enter your master password"
				return nil
			}
			m.begin("Verifying password")
			return authorizeExport(st, m.id, m.sec.Take())
		default:
			m.errMsg = ""
			m.sec.Update(msg)
		}
	case exportPath:
		switch k {
		case "esc":
			return m.close()
		case "tab":
			m.begin("Choosing export file")
			return chooseSave(st, m.id, filepath.Base(m.def))
		case "shift+tab", "up", "down":
		case "enter":
			m.form.Update(msg)
			if len(m.form.Errors()) > 0 {
				return nil
			}
			return m.write(st, strings.TrimSpace(m.path))
		default:
			m.errMsg = ""
			_, cmd := m.form.Update(msg)
			return cmd
		}
	case exportDone:
		if k == "enter" || k == "esc" {
			return m.close()
		}
	}
	return nil
}

func (m *exportModal) showPath() {
	m.step = exportPath
	if m.path == "" {
		m.path = m.def
	}
	in := huh.NewInput().
		Value(&m.path).
		CharLimit(512).
		Prompt(ui.G.Next + " ").
		Validate(func(s string) error {
			if strings.TrimSpace(s) == "" {
				return errors.New("Enter an export path")
			}
			return nil
		})
	m.form = huh.NewForm(huh.NewGroup(in)).
		WithTheme(ui.HuhTheme()).
		WithKeyMap(ui.HuhKeyMap()).
		WithShowHelp(false).
		WithShowErrors(false)
	m.form.Init()
	m.formW = 0
}

func (m *exportModal) write(st *core.State, path string) tea.Cmd {
	m.begin("Saving export")
	m.writing = true
	export, token, id := st.Deps.ExportVault, m.token, m.id
	return func() tea.Msg {
		res, err := export(path, token)
		if err != nil {
			err = fmt.Errorf("exporting vault: %w", err)
		}
		return exportWroteMsg{id: id, res: res, err: err}
	}
}

func authorizeExport(st *core.State, id core.ID, pw []byte) tea.Cmd {
	auth := st.Deps.AuthorizeExport
	return func() tea.Msg {
		defer clear(pw)
		token, err := auth(pw)
		if err != nil {
			return exportAuthMsg{id: id, err: fmt.Errorf("authorizing export: %w", err)}
		}
		return exportAuthMsg{id: id, token: strings.TrimSpace(token)}
	}
}

func chooseSave(st *core.State, id core.ID, name string) tea.Cmd {
	choose := st.Deps.ChooseSavePath
	return func() tea.Msg {
		path, err := choose(name)
		if err != nil {
			err = fmt.Errorf("choosing export file: %w", err)
		}
		return exportPickMsg{id: id, path: path, err: err}
	}
}

func warnLines(inner int, text string, glyph bool) []string {
	p := ui.P()
	ls := ui.Wrap(text, inner-2)
	for i, l := range ls {
		lead := "  "
		if i == 0 && glyph {
			lead = ui.Bold(ui.G.Warn, p.Danger) + " "
		}
		ls[i] = lead + ui.Paint(l, p.Danger)
	}
	return ls
}

func (m *exportModal) body(inner, limit, frame int) []string {
	p := ui.P()
	switch m.step {
	case exportPath:
		if m.form != nil && m.formW != inner {
			m.formW = inner
			m.form.WithWidth(inner)
		}
		lines := []string{ui.Paint(ui.Trunc("Enter a path for the export", inner), p.Muted), "", strings.Split(m.form.View(), "\n")[0]}
		errText := m.errMsg
		if errText == "" && m.busy == "" {
			if errs := m.form.Errors(); len(errs) > 0 {
				errText = errs[0].Error()
			}
		}
		return pad(append(lines, status(inner, frame, m.busy, errText)...), limit)
	case exportDone:
		name := ui.Sanitize(filepath.Base(m.res.Path))
		head := []string{ui.Paint(ui.G.Check, p.Good) + " " + ui.Paint(ui.Trunc("Saved "+core.Plural(m.res.KeyCount, "key")+" to "+name, inner-2), p.Text), ""}
		head = append(head, warnLines(inner, exportPlaintextSuccessWarning, false)...)
		tail := append([]string{""}, widget.Buttons(inner, ui.Button("Done", ui.Primary), "")...)
		return pad(append(clipLines(head, limit-len(tail), inner), tail...), limit)
	}
	st := 2
	if limit < 9 {
		st = 1
	}
	tail := []string{""}
	if limit >= 9 {
		tail = append(tail, ui.Paint("Master password", p.Muted))
	}
	tail = append(tail, m.sec.View(inner, m.busy == ""))
	tail = append(tail, status(inner, frame, m.busy, m.errMsg)[:st]...)
	head := clipLines(warnLines(inner, exportPlaintextWarning, true), limit-len(tail), inner)
	return pad(append(head, tail...), limit)
}

func (m *exportModal) View(st *core.State, w, h int) string {
	return widget.ModalView(st, w, h, "Export vault", "", m.body)
}

func (m *exportModal) Size(st *core.State, maxW, maxH int) (int, int) {
	return widget.ModalSize(st, exportW, maxW, maxH, m.body)
}
