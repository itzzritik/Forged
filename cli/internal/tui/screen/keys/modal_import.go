package keys

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/keytypes"
	"github.com/itzzritik/forged/cli/internal/picker"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const importW = 64

const (
	stepSource = iota
	stepPath
	stepPaste
	stepReview
	stepResult
)

var importSources = []struct{ id, label string }{
	{"ssh-dir", "SSH folder (~/.ssh)"},
	{"file", "Key file"},
	{"paste", "Paste a key"},
	{"1password", "1Password export"},
	{"bitwarden", "Bitwarden export"},
	{"forged", "Forged export"},
}

type importPickedMsg struct {
	id   core.ID
	path string
	err  error
}

type importPreviewMsg struct {
	id  core.ID
	res actions.ImportPreviewResult
	err error
}

type importDoneMsg struct {
	id  core.ID
	res actions.ImportResult
	err error
}

type importModal struct {
	step, cur, rcur        int
	src                    string
	busy, errMsg, warn     string
	importing              bool
	form                   *huh.Form
	path                   string
	formW                  int
	previews               []actions.ImportPreview
	discovered, duplicates int
	res                    actions.ImportResult
	vp                     viewport.Model
	id                     core.ID
}

func ImportModal() widget.Modal { return &importModal{} }

func (m *importModal) needsPath() bool {
	return m.src == "file" || m.src == "1password" || m.src == "bitwarden" || m.src == "forged"
}

func (m *importModal) selected() int {
	n := 0
	for _, p := range m.previews {
		if p.Selected {
			n++
		}
	}
	return n
}

func (m *importModal) discard() {
	clear(m.previews)
	m.previews = nil
}

func (m *importModal) begin(busy string) {
	m.id, m.busy, m.errMsg, m.warn = core.NextID(), busy, "", ""
}

func (m *importModal) close() tea.Cmd {
	m.discard()
	return core.Close(m)
}

func (m *importModal) Spinning() bool { return m.busy != "" }

func (m *importModal) Actions(*core.State) []core.Action {
	if m.importing {
		return nil
	}
	if m.busy != "" {
		return []core.Action{{Key: "esc", Label: "Cancel"}}
	}
	switch m.step {
	case stepPath:
		return []core.Action{{Key: "enter", Label: "Continue"}, {Key: "tab", Label: "Choose file"}, {Key: "esc", Label: "Back"}}
	case stepPaste:
		return []core.Action{{Key: "esc", Label: "Back"}}
	case stepReview:
		all := "Select all"
		if m.selected() == len(m.previews) {
			all = "Unselect all"
		}
		out := []core.Action{{Key: "space", Label: "Toggle"}, {Key: "a", Label: all}}
		if m.selected() > 0 {
			out = append(out, core.Action{Key: "enter", Label: "Import"})
		}
		return append(out, core.Action{Key: "esc", Label: "Cancel"})
	case stepResult:
		return []core.Action{{Key: ui.G.UpDown, Label: "Scroll"}, {Key: "enter", Label: "Close"}}
	}
	return []core.Action{{Key: ui.G.UpDown, Label: "Move"}, {Key: "enter", Label: "Choose"}, {Key: "esc", Label: "Cancel"}}
}

func (m *importModal) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	return m, m.update(msg, st)
}

func (m *importModal) update(msg tea.Msg, st *core.State) tea.Cmd {
	switch msg := msg.(type) {
	case core.LockedMsg:
		m.discard()
		m.id, m.busy, m.importing, m.step, m.form = core.NextID(), "", false, stepSource, nil
		m.errMsg, m.warn = "", ""
	case importPickedMsg:
		if msg.id != m.id || m.busy == "" {
			return nil
		}
		return m.onPicked(msg, st)
	case importPreviewMsg:
		if msg.id != m.id || m.busy == "" {
			clear(msg.res.Previews)
			return nil
		}
		return m.onPreview(msg, st)
	case importDoneMsg:
		if msg.id != m.id || !m.importing {
			return nil
		}
		return m.onDone(msg, st)
	case tea.PasteMsg:
		if m.busy != "" {
			return nil
		}
		switch m.step {
		case stepPaste:
			text := msg.Content
			m.begin("Reading pasted key")
			return previewText(st, m.id, text)
		case stepPath:
			m.errMsg = ""
			_, cmd := m.form.Update(msg)
			return cmd
		}
	case tea.KeyPressMsg:
		return m.key(msg, st)
	default:
		if m.step == stepPath && m.busy == "" && m.form != nil {
			_, cmd := m.form.Update(msg)
			return cmd
		}
	}
	return nil
}

func (m *importModal) key(msg tea.KeyPressMsg, st *core.State) tea.Cmd {
	k := msg.String()
	if m.step != stepPath {
		k = strings.ToLower(k)
	}
	if m.importing {
		return nil
	}
	if m.busy != "" {
		if k == "esc" {
			m.id = core.NextID()
			return m.close()
		}
		return nil
	}
	switch m.step {
	case stepSource:
		n := len(importSources)
		switch k {
		case "esc":
			return m.close()
		case "up", "k":
			m.cur = (m.cur + n - 1) % n
		case "down", "j":
			m.cur = (m.cur + 1) % n
		case "enter":
			return m.choose(st)
		}
	case stepPath:
		switch k {
		case "esc":
			m.toSource()
		case "tab":
			m.begin("Choosing file")
			return chooseFile(st, m.id)
		case "shift+tab", "up", "down":
		case "enter":
			m.form.Update(msg)
			if len(m.form.Errors()) > 0 {
				return nil
			}
			m.begin("Reading file")
			return previewFile(st, m.id, m.src, strings.TrimSpace(m.path))
		default:
			m.errMsg = ""
			_, cmd := m.form.Update(msg)
			return cmd
		}
	case stepPaste:
		if k == "esc" {
			m.toSource()
		}
	case stepReview:
		return m.reviewKey(k, st)
	case stepResult:
		switch k {
		case "esc", "enter":
			return m.close()
		case "up", "k":
			m.vp.ScrollUp(1)
		case "down", "j":
			m.vp.ScrollDown(1)
		case "pgup":
			m.vp.PageUp()
		case "pgdown":
			m.vp.PageDown()
		}
	}
	return nil
}

func (m *importModal) toSource() {
	m.discard()
	m.step, m.form, m.path, m.errMsg, m.warn = stepSource, nil, "", "", ""
}

func (m *importModal) reviewKey(k string, st *core.State) tea.Cmd {
	m.errMsg = ""
	switch k {
	case "esc":
		return m.close()
	case "up", "k":
		m.rcur = max(0, m.rcur-1)
	case "down", "j":
		m.rcur = min(len(m.previews)-1, m.rcur+1)
	case "space", " ":
		if m.rcur < len(m.previews) {
			m.previews[m.rcur].Selected = !m.previews[m.rcur].Selected
		}
	case "a":
		on := m.selected() != len(m.previews)
		for i := range m.previews {
			m.previews[i].Selected = on
		}
	case "enter":
		n := m.selected()
		if n == 0 {
			return nil
		}
		m.begin("Importing " + core.Plural(n, "key"))
		m.importing = true
		return importKeys(st, m.id, m.src, m.discovered, slices.Clone(m.previews))
	}
	return nil
}

func (m *importModal) choose(st *core.State) tea.Cmd {
	m.src = importSources[m.cur].id
	switch {
	case m.src == "ssh-dir":
		m.begin("Scanning ~/.ssh")
		return previewFile(st, m.id, m.src, "")
	case m.src == "paste":
		m.step, m.errMsg, m.warn = stepPaste, "", ""
	default:
		m.begin("Choosing file")
		return chooseFile(st, m.id)
	}
	return nil
}

func (m *importModal) showPath() {
	m.step = stepPath
	in := huh.NewInput().
		Value(&m.path).
		CharLimit(512).
		Prompt(ui.G.Next + " ").
		Validate(func(s string) error {
			if strings.TrimSpace(s) == "" {
				return errors.New("Enter a file path")
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

func (m *importModal) retry() {
	switch {
	case m.needsPath():
		m.showPath()
	case m.src == "paste":
		m.step = stepPaste
	default:
		m.step = stepSource
	}
}

func (m *importModal) onPicked(msg importPickedMsg, st *core.State) tea.Cmd {
	m.busy = ""
	if msg.err == nil && strings.TrimSpace(msg.path) != "" {
		m.begin("Reading file")
		return previewFile(st, m.id, m.src, strings.TrimSpace(msg.path))
	}
	m.showPath()
	switch {
	case errors.Is(msg.err, picker.ErrCanceled), msg.err == nil:
	case errors.Is(msg.err, picker.ErrUnavailable):
		st.Reporter.Report("keys", "import file picker", msg.err)
		m.errMsg = "File picker unavailable. Enter a file path instead"
	default:
		st.Reporter.Report("keys", "import file picker", msg.err)
		m.errMsg = "File picker failed. Enter a file path instead"
	}
	return nil
}

func (m *importModal) onPreview(msg importPreviewMsg, st *core.State) tea.Cmd {
	m.busy = ""
	m.discard()
	m.discovered, m.duplicates = msg.res.Discovered, msg.res.Duplicates
	switch {
	case msg.err != nil:
		m.retry()
		m.errMsg = st.Reporter.Report("keys", "preview import", msg.err)
	case len(msg.res.Previews) == 0:
		m.retry()
		switch m.src {
		case "ssh-dir":
			m.warn = "No new SSH keys found in ~/.ssh"
		case "paste":
			m.warn = "No new SSH key found in the pasted text"
		default:
			m.warn = "No new SSH key found in this file"
		}
	default:
		m.previews, m.step, m.rcur = msg.res.Previews, stepReview, 0
	}
	return nil
}

func (m *importModal) onDone(msg importDoneMsg, st *core.State) tea.Cmd {
	m.busy, m.importing = "", false
	if msg.err != nil {
		m.errMsg = st.Reporter.Report("keys", "import keys", msg.err)
		return nil
	}
	res := msg.res
	switch {
	case len(res.Failures) > 0:
		m.discard()
		m.res, m.step = res, stepResult
		m.vp = viewport.New()
		return nil
	case res.Imported == 0:
		m.errMsg = "No keys were imported"
		return nil
	}
	return tea.Sequence(m.close(), core.Toast("Imported "+core.Plural(res.Imported, "key"), ui.ToneGood))
}

func chooseFile(st *core.State, id core.ID) tea.Cmd {
	choose := st.Deps.ChooseFile
	return func() tea.Msg {
		path, err := choose()
		if err != nil {
			err = fmt.Errorf("choosing import file: %w", err)
		}
		return importPickedMsg{id: id, path: path, err: err}
	}
}

func previewFile(st *core.State, id core.ID, src, file string) tea.Cmd {
	preview := st.Deps.PreviewImport
	return func() tea.Msg {
		res, err := preview(src, file)
		if err != nil {
			err = fmt.Errorf("reading import source: %w", err)
		}
		return importPreviewMsg{id: id, res: res, err: err}
	}
}

func previewText(st *core.State, id core.ID, text string) tea.Cmd {
	preview := st.Deps.PreviewImportText
	return func() tea.Msg {
		res, err := preview(text)
		if err != nil {
			err = fmt.Errorf("reading pasted key: %w", err)
		}
		return importPreviewMsg{id: id, res: res, err: err}
	}
}

func importKeys(st *core.State, id core.ID, src string, discovered int, previews []actions.ImportPreview) tea.Cmd {
	imp := st.Deps.ImportPreviews
	return func() tea.Msg {
		defer clear(previews)
		res, err := imp(src, discovered, previews)
		if err != nil {
			err = fmt.Errorf("importing keys: %w", err)
		}
		return importDoneMsg{id: id, res: res, err: err}
	}
}

func logImportFailures(st *core.State, res actions.ImportResult) tea.Cmd {
	logErr, version := st.Deps.LogError, st.Deps.AppVersion
	const maxReasons = 3
	n := min(len(res.Failures), maxReasons)
	reasons := make([]string, 0, n+1)
	for _, f := range res.Failures[:n] {
		reasons = append(reasons, ui.Trunc(ui.Sanitize(f.Reason), 200))
	}
	if extra := len(res.Failures) - n; extra > 0 {
		reasons = append(reasons, fmt.Sprintf("%d additional failures", extra))
	}
	event := actions.DiagnosticErrorEvent{
		Route:   "keys",
		Action:  "keys.import.partial",
		Version: version,
		Message: fmt.Sprintf("%d import failures: %s", len(res.Failures), strings.Join(reasons, "; ")),
	}
	return func() tea.Msg {
		logErr(event)
		return nil
	}
}

func keyKind(public string) string {
	f := strings.Fields(public)
	if len(f) == 0 {
		return "key"
	}
	return keytypes.FromSSHPublicKeyType(f[0])
}

func rowLine(inner int, content string, sel bool) string {
	if sel {
		return widget.Flush + ui.SelLine(inner+4, " "+content)
	}
	return widget.Flush + "  " + content
}

func (m *importModal) body(inner, limit, frame int) []string {
	switch m.step {
	case stepPath:
		return m.bodyPath(inner, limit, frame)
	case stepPaste:
		return m.bodyPaste(inner, limit, frame)
	case stepReview:
		return m.bodyReview(inner, limit, frame)
	case stepResult:
		return m.bodyResult(inner, limit)
	}
	return m.bodySource(inner, limit, frame)
}

func (m *importModal) bodySource(inner, limit, frame int) []string {
	p := ui.P()
	n := len(importSources)
	stN := 2
	if limit-2-stN < n {
		stN = 1
	}
	rows := max(1, min(n, limit-2-stN))
	lines := []string{ui.Paint(ui.Trunc("Choose where to import from", inner), p.Muted), ""}
	for i := windowStart(m.cur, rows, n); i < n && i < windowStart(m.cur, rows, n)+rows; i++ {
		label := ui.Paint(ui.Trunc(importSources[i].label, inner), p.Text)
		if i == m.cur {
			label = ui.Bold(ui.Trunc(importSources[i].label, inner), p.Text)
		}
		lines = append(lines, rowLine(inner, label, i == m.cur))
	}
	lines = append(lines, notice(inner, stN, frame, m.busy, m.errMsg, m.warn)...)
	return pad(lines, limit)
}

func (m *importModal) sizeForm(inner int) {
	if m.form != nil && m.formW != inner {
		m.formW = inner
		m.form.WithWidth(inner)
	}
}

func (m *importModal) bodyPath(inner, limit, frame int) []string {
	m.sizeForm(inner)
	lines := []string{ui.Paint(ui.Trunc("Enter the path to the file", inner), ui.P().Muted), "", strings.Split(m.form.View(), "\n")[0]}
	errText := m.errMsg
	if errText == "" && m.busy == "" && m.warn == "" {
		if errs := m.form.Errors(); len(errs) > 0 {
			errText = errs[0].Error()
		}
	}
	lines = append(lines, notice(inner, 2, frame, m.busy, errText, m.warn)...)
	return pad(lines, limit)
}

func (m *importModal) bodyPaste(inner, limit, frame int) []string {
	p := ui.P()
	lines := []string{ui.Paint(ui.Trunc("Paste your private key now", inner), p.Text)}
	for _, l := range ui.Wrap("It will not be shown on screen.", inner) {
		lines = append(lines, ui.Paint(l, p.Muted))
	}
	lines = append(lines, "")
	lines = append(lines, notice(inner, 2, frame, m.busy, m.errMsg, m.warn)...)
	return pad(lines, limit)
}

func (m *importModal) reviewRow(inner, i int) string {
	p := ui.P()
	pv := m.previews[i]
	box, c := ui.G.Box, p.Muted
	if pv.Selected {
		box, c = ui.G.BoxOn, p.Accent
	}
	bw := max(ui.Width(ui.G.Box), ui.Width(ui.G.BoxOn))
	typeCol := bw + 22
	noteCol := typeCol + 12
	name := ui.Sanitize(pv.Key.Name)
	nameW := inner - bw - 1
	typeOK, noteOK := inner >= bw+33, inner >= bw+55
	if typeOK {
		nameW = typeCol - bw - 3
	}
	nameS := ui.Paint(ui.Trunc(name, nameW), p.Text)
	if i == m.rcur {
		nameS = ui.Bold(ui.Trunc(name, nameW), p.Text)
	}
	line := ui.Pad(ui.Paint(box, c), bw+1) + nameS
	if typeOK {
		line = ui.Pad(line, typeCol) + ui.Paint(ui.Trunc(keyKind(pv.Key.PublicKey), 10), p.Muted)
	}
	if noteOK && pv.Converted {
		line = ui.Pad(line, noteCol) + ui.Paint("Converts to OpenSSH", p.Steel)
	}
	return rowLine(inner, line, i == m.rcur)
}

func (m *importModal) bodyReview(inner, limit, frame int) []string {
	p := ui.P()
	total := len(m.previews)
	where := " in this file"
	switch m.src {
	case "ssh-dir":
		where = " in ~/.ssh"
	case "paste":
		where = ""
	}
	found := "Found 1 new key"
	if total != 1 {
		found = "Found " + strconv.Itoa(total) + " new keys"
	}
	title := ui.Paint(ui.Trunc(found+where, inner), p.Text)
	var note []string
	if m.duplicates > 0 {
		s := strconv.Itoa(m.duplicates) + " keys are already in your vault and will be skipped."
		if m.duplicates == 1 {
			s = "1 key is already in your vault and will be skipped."
		}
		note = ui.Wrap(s, inner)
		for i, l := range note {
			note[i] = ui.Paint(l, p.Muted)
		}
	}
	if m.selected() == 0 {
		note = []string{ui.Paint("Select keys you want to import.", p.Muted)}
	}
	hasStatus := m.busy != "" || m.errMsg != ""
	noteN := max(1, len(note))
	label := "Import " + core.Plural(m.selected(), "key")
	btns := widget.Buttons(inner, ui.Button(label, ui.Primary), ui.Button("Cancel", ui.Secondary))
	if m.selected() == 0 {
		btns = widget.Buttons(inner, ui.Button("Cancel", ui.Secondary), "")
	}
	type attempt struct {
		pad  int
		info int
		gaps bool
	}
	infoMin := 0
	if hasStatus {
		infoMin = 1
	}
	attempts := []attempt{{1, noteN, true}, {0, noteN, true}, {0, infoMin, true}, {0, infoMin, false}}
	var out []string
	for ai, a := range attempts {
		var info []string
		if a.info > 0 {
			info = note
			if hasStatus || len(note) == 0 {
				info = statusN(inner, a.info, frame, m.busy, m.errMsg)
			}
			info = clipLines(info, a.info, inner)
		}
		build := func(n int) []string {
			o := make([]string, a.pad, limit+4)
			o = append(o, title)
			if a.gaps {
				o = append(o, "")
			}
			if n > 0 {
				start := windowStart(m.rcur, n, total)
				for i := start; i < total && i < start+n; i++ {
					o = append(o, m.reviewRow(inner, i))
				}
			}
			if len(info) > 0 {
				if a.gaps {
					o = append(o, "")
				}
				o = append(o, info...)
			}
			if a.gaps {
				o = append(o, "")
			}
			o = append(o, btns...)
			return append(o, make([]string, a.pad)...)
		}
		fixed := len(build(0))
		n := min(total, limit-fixed)
		if n >= min(2, total) || ai == len(attempts)-1 {
			out = build(max(1, n))
			break
		}
	}
	return out[:min(len(out), max(0, limit))]
}

func (m *importModal) bodyResult(inner, limit int) []string {
	p := ui.P()
	added := ui.Paint(ui.Trunc(core.Plural(m.res.Imported, "key")+" added", inner), p.Good)
	failed := ui.Paint(ui.Trunc(core.Plural(len(m.res.Failures), "key")+" failed", inner), p.Danger)
	var content []string
	for i, f := range m.res.Failures {
		if i > 0 {
			content = append(content, "")
		}
		content = append(content, ui.Bold(ui.Trunc(ui.Sanitize(f.Name), inner), p.Text))
		if f.Fingerprint != "" {
			content = append(content, ui.Paint(ui.MidTrunc(ui.Sanitize(f.Fingerprint), inner), p.Muted))
		}
		for _, l := range ui.Wrap(ui.Sanitize(f.Reason), inner) {
			content = append(content, ui.Paint(l, p.Danger))
		}
	}
	btn := ui.Button("Close", ui.Primary)
	head := []string{added, failed, ""}
	tail := append([]string{""}, widget.Buttons(inner, btn, "")...)
	vh := max(1, min(len(content), limit-len(head)-len(tail)))
	m.vp.SetWidth(inner)
	m.vp.SetHeight(vh)
	m.vp.SetContentLines(content)
	out := append(head, strings.Split(m.vp.View(), "\n")...)
	out = append(out, tail...)
	return out[:min(len(out), max(0, limit))]
}

func (m *importModal) View(st *core.State, w, h int) string {
	right := ""
	if m.step == stepReview && w >= 40 {
		right = "Review"
	}
	return widget.ModalView(st, w, h, "Import keys", right, m.body)
}

func (m *importModal) Size(st *core.State, maxW, maxH int) (int, int) {
	return widget.ModalSize(st, importW, maxW, maxH, m.body)
}
