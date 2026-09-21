// Package texteditor is a VS Code-flavoured code editor. bubbles/textarea is
// the editing model (buffer, cursor, key handling); the view is rendered here
// so lines can be syntax-highlighted (Chroma, Dracula) with a line-number
// gutter, horizontal scrolling instead of soft wrap, click-to-position, undo /
// redo, find and go-to-line.
package texteditor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/hittable/shellapp/ui/theme"
)

// MaxEditableBytes is the per-file size cap for the editor.
const MaxEditableBytes = 1 * 1024 * 1024

const (
	noWrapWidth = 1 << 20 // textarea width large enough that it never soft-wraps
	undoDepth   = 200
	tabSpaces   = "  "
	tabDisplay  = "    " // how an existing \t is drawn (VS Code default: 4)
)

// displayCol maps a rune column in raw to its on-screen column once tabs
// are expanded to tabDisplay.
func displayCol(raw []rune, col int) int {
	if col > len(raw) {
		col = len(raw)
	}
	n := col
	for _, r := range raw[:col] {
		if r == '\t' {
			n += len(tabDisplay) - 1
		}
	}
	return n
}

type snapshot struct {
	value    string
	row, col int
}

type promptKind int

const (
	promptNone promptKind = iota
	promptFind
	promptGoto
)

// buffer is everything that belongs to one open file.
type pos struct{ row, col int }

func (a pos) less(b pos) bool { return a.row < b.row || (a.row == b.row && a.col < b.col) }

type buffer struct {
	ta       textarea.Model
	lexer    chroma.Lexer
	scrollY  int
	scrollX  int
	undo     []snapshot
	redo     []snapshot
	hlCache  map[string]string
	findLast string

	anchor    *pos // selection anchor; nil = no selection
	lastClick pos
	lastAt    time.Time
}

type TextEditor struct {
	Width   int
	Height  int
	Focused bool
	// OnChanged fires after every content-changing Update.
	OnChanged func(content string)

	// TextArea is the active buffer's textarea (kept exported for tests).
	TextArea textarea.Model

	mu          sync.RWMutex
	buffers     map[string]*buffer
	cur         *buffer
	currentPath string

	prompt      promptKind
	promptValue string
	status      string // transient message shown in the status row
	// Hint is a persistent note shown in the status row (e.g. JSON validity).
	Hint string
	// Annotations (one per line) render left of the gutter, e.g. git blame.
	Annotations []string
	AnnotWidth  int
	// LineHint, when set, supplies a status-row note for the cursor line
	// (GitLens-style current-line blame).
	LineHint func(row int) string
	// Wrap soft-wraps long lines at the pane width (prose files); off means
	// horizontal scrolling.
	Wrap bool
}

// vrow is one visual row: a segment [start,end) (rune indexes) of a line.
type vrow struct {
	line       int
	start, end int
	first      bool
}

// layout splits lines into visual rows. Without wrap each line is one row.
func (t *TextEditor) layout(lines []string, avail int) []vrow {
	rows := make([]vrow, 0, len(lines))
	for i, l := range lines {
		r := []rune(l)
		if !t.Wrap || displayCol(r, len(r)) <= avail {
			rows = append(rows, vrow{i, 0, len(r), true})
			continue
		}
		start := 0
		first := true
		for start < len(r) {
			// Advance until the display width would exceed avail.
			end, w := start, 0
			lastSpace := -1
			for end < len(r) {
				cw := 1
				if r[end] == '\t' {
					cw = len(tabDisplay)
				}
				if w+cw > avail {
					break
				}
				if r[end] == ' ' {
					lastSpace = end
				}
				w += cw
				end++
			}
			if end < len(r) && lastSpace > start {
				end = lastSpace + 1 // break after the space
			}
			if end == start {
				end = start + 1
			}
			rows = append(rows, vrow{i, start, end, first})
			first = false
			start = end
		}
	}
	return rows
}

// vrowOf returns the index of the visual row holding (line, col).
func vrowOf(rows []vrow, line, col int) int {
	for i, v := range rows {
		if v.line == line && (col < v.end || (col == v.end && (i+1 >= len(rows) || rows[i+1].line != line))) {
			return i
		}
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].line == line {
			return i
		}
	}
	return 0
}

// SetAnnotations installs per-line gutter annotations (nil clears them).
func (t *TextEditor) SetAnnotations(a []string, width int) {
	t.Annotations, t.AnnotWidth = a, width
	if a == nil {
		t.AnnotWidth = 0
	}
}

// GotoLine moves the cursor to a 0-based line and scrolls it into view.
func (t *TextEditor) GotoLine(row int) {
	moveTo(&t.TextArea, row, 0)
	t.clearSelection()
	t.writeBack()
	t.followCursor()
}

func New() *TextEditor {
	t := &TextEditor{buffers: make(map[string]*buffer)}
	t.cur = newBuffer("")
	t.TextArea = t.cur.ta
	return t
}

func newBuffer(path string) *buffer {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.MaxWidth = noWrapWidth
	ta.MaxHeight = 0
	ta.SetWidth(noWrapWidth)
	ta.SetHeight(1)
	// VS Code-style word jumps on ctrl+arrows in addition to alt+arrows.
	ta.KeyMap.WordForward = key.NewBinding(key.WithKeys("alt+right", "alt+f", "ctrl+right"))
	ta.KeyMap.WordBackward = key.NewBinding(key.WithKeys("alt+left", "alt+b", "ctrl+left"))
	ta.KeyMap.DeleteWordBackward = key.NewBinding(key.WithKeys("alt+backspace", "ctrl+backspace", "ctrl+w"))
	ta.KeyMap.DeleteWordForward = key.NewBinding(key.WithKeys("alt+delete", "ctrl+delete", "alt+d"))
	// These are app-level shortcuts; unbind them so they never edit text.
	ta.KeyMap.TransposeCharacterBackward = key.NewBinding()
	ta.KeyMap.LowercaseWordForward = key.NewBinding()
	ta.KeyMap.UppercaseWordForward = key.NewBinding()
	ta.KeyMap.CapitalizeWordForward = key.NewBinding()
	ta.KeyMap.Paste = key.NewBinding(key.WithKeys("ctrl+v"))

	b := &buffer{ta: ta, hlCache: map[string]string{}}
	b.lexer = lexerFor(path)
	return b
}

func lexerFor(path string) chroma.Lexer {
	if path == "" {
		return nil
	}
	var l chroma.Lexer
	switch strings.ToLower(filepath.Ext(path)) {
	case ".hit":
		l = lexers.Get("json")
	case "":
		l = nil
	default:
		l = lexers.Match(filepath.Base(path))
	}
	if l == nil {
		return nil
	}
	return chroma.Coalesce(l)
}

// ---------- state sync helpers ----------

// writeBack stores t.TextArea into the current buffer. Callers mutate
// t.TextArea (a value type) and must call this afterwards.
func (t *TextEditor) writeBack() {
	if t.cur != nil {
		t.cur.ta = t.TextArea
	}
}

func (t *TextEditor) SetSize(w, h int) {
	t.Width = w
	t.Height = h
}

// SetContent loads content for path, reusing the buffer (cursor, undo
// history, scroll) if the path was opened before this session.
func (t *TextEditor) SetContent(path, content string) {
	t.mu.Lock()
	b, ok := t.buffers[path]
	if !ok {
		b = newBuffer(path)
		t.buffers[path] = b
	}
	t.mu.Unlock()
	if b.ta.Value() != content {
		row, col := b.ta.Line(), b.ta.LineInfo().ColumnOffset
		b.ta.SetValue(content)
		moveTo(&b.ta, row, col)
	}
	b.ta.Focus()
	if !t.Focused {
		b.ta.Blur()
	}
	t.cur = b
	t.currentPath = path
	t.TextArea = b.ta
	t.prompt = promptNone
	t.status = ""
}

func (t *TextEditor) GetContent() string { return t.TextArea.Value() }

func (t *TextEditor) Focus() {
	t.Focused = true
	t.TextArea.Focus()
	t.writeBack()
}

func (t *TextEditor) Blur() {
	t.Focused = false
	t.TextArea.Blur()
	t.writeBack()
}

func (t *TextEditor) RemoveEditor(path string) {
	t.mu.Lock()
	delete(t.buffers, path)
	t.mu.Unlock()
}

// GetCursorRow returns the 0-based cursor line.
func (t *TextEditor) GetCursorRow() int { return t.TextArea.Line() }

// PromptOpen reports whether find / go-to-line is capturing keys.
func (t *TextEditor) PromptOpen() bool { return t.prompt != promptNone }

// moveTo places the textarea cursor on (row, col) without soft-wrap effects.
func moveTo(ta *textarea.Model, row, col int) {
	if row < 0 {
		row = 0
	}
	if max := ta.LineCount() - 1; row > max {
		row = max
	}
	for ta.Line() > row {
		ta.CursorUp()
	}
	for ta.Line() < row {
		ta.CursorDown()
	}
	ta.SetCursor(col)
}

func (t *TextEditor) pushUndo(before snapshot) {
	b := t.cur
	b.undo = append(b.undo, before)
	if len(b.undo) > undoDepth {
		b.undo = b.undo[1:]
	}
	b.redo = nil
}

func (t *TextEditor) snap() snapshot {
	return snapshot{t.TextArea.Value(), t.TextArea.Line(), t.TextArea.LineInfo().ColumnOffset}
}

func (t *TextEditor) restore(s snapshot) {
	t.TextArea.SetValue(s.value)
	moveTo(&t.TextArea, s.row, s.col)
	t.writeBack()
}

// ---------- selection ----------

func (t *TextEditor) cursorPos() pos {
	return pos{t.TextArea.Line(), t.TextArea.LineInfo().ColumnOffset}
}

// Selection returns the ordered selection range and whether one exists.
func (t *TextEditor) Selection() (start, end pos, ok bool) {
	b := t.cur
	if b.anchor == nil {
		return pos{}, pos{}, false
	}
	c := t.cursorPos()
	if *b.anchor == c {
		return pos{}, pos{}, false
	}
	if b.anchor.less(c) {
		return *b.anchor, c, true
	}
	return c, *b.anchor, true
}

func (t *TextEditor) HasSelection() bool { _, _, ok := t.Selection(); return ok }

func (t *TextEditor) clearSelection() { t.cur.anchor = nil }

func offsetOf(lines []string, p pos) int {
	n := 0
	for r := 0; r < p.row && r < len(lines); r++ {
		n += len([]rune(lines[r])) + 1
	}
	if p.row < len(lines) {
		c := p.col
		if l := len([]rune(lines[p.row])); c > l {
			c = l
		}
		n += c
	}
	return n
}

// SelectedText returns the selected text ("" if none).
func (t *TextEditor) SelectedText() string {
	s, e, ok := t.Selection()
	if !ok {
		return ""
	}
	lines := strings.Split(t.TextArea.Value(), "\n")
	r := []rune(t.TextArea.Value())
	return string(r[offsetOf(lines, s):offsetOf(lines, e)])
}

// deleteSelection removes the selected text and puts the cursor at its
// start. The caller records the undo snapshot so "replace selection by
// typing" is a single undo step.
func (t *TextEditor) deleteSelection() bool {
	s, e, ok := t.Selection()
	if !ok {
		return false
	}
	lines := strings.Split(t.TextArea.Value(), "\n")
	r := []rune(t.TextArea.Value())
	so, eo := offsetOf(lines, s), offsetOf(lines, e)
	t.TextArea.SetValue(string(r[:so]) + string(r[eo:]))
	moveTo(&t.TextArea, s.row, s.col)
	t.clearSelection()
	t.writeBack()
	return true
}

// SetValue replaces the buffer (undoable) keeping the cursor where possible.
func (t *TextEditor) SetValue(v string) {
	before := t.snap()
	t.TextArea.SetValue(v)
	moveTo(&t.TextArea, before.row, before.col)
	t.clearSelection()
	t.writeBack()
	t.pushUndo(before)
	t.followCursor()
	t.changed()
}

// Copy puts the selection on the clipboard. Returns false if none.
func (t *TextEditor) Copy() bool {
	sel := t.SelectedText()
	if sel == "" {
		return false
	}
	if err := clipboard.WriteAll(sel); err != nil {
		t.status = "clipboard unavailable"
		return false
	}
	t.status = "copied"
	return true
}

// Cut copies then deletes the selection.
func (t *TextEditor) Cut() bool {
	if !t.Copy() {
		return false
	}
	before := t.snap()
	t.deleteSelection()
	t.pushUndo(before)
	t.changed()
	t.status = "cut"
	return true
}

// SelectAll selects the whole buffer.
func (t *TextEditor) SelectAll() {
	t.cur.anchor = &pos{0, 0}
	lines := strings.Split(t.TextArea.Value(), "\n")
	moveTo(&t.TextArea, len(lines)-1, len([]rune(lines[len(lines)-1])))
	t.writeBack()
	t.followCursor()
}

func isWordRune(r rune) bool {
	return r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 127
}

// selectWordAt selects the identifier under (row, col).
func (t *TextEditor) selectWordAt(row, col int) {
	lines := strings.Split(t.TextArea.Value(), "\n")
	if row >= len(lines) {
		return
	}
	r := []rune(lines[row])
	if col >= len(r) {
		col = len(r) - 1
	}
	if col < 0 || !isWordRune(r[col]) {
		return
	}
	s, e := col, col+1
	for s > 0 && isWordRune(r[s-1]) {
		s--
	}
	for e < len(r) && isWordRune(r[e]) {
		e++
	}
	t.cur.anchor = &pos{row, s}
	moveTo(&t.TextArea, row, e)
	t.writeBack()
}

// ---------- Update ----------

func (t *TextEditor) Update(msg tea.Msg) tea.Cmd {
	if !t.Focused {
		return nil
	}
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return t.handleMouse(msg)
	case tea.KeyMsg:
		return t.handleKey(msg)
	case nil:
		return nil
	}
	// e.g. the textarea's async paste result
	before := t.snap()
	var cmd tea.Cmd
	t.TextArea, cmd = t.TextArea.Update(msg)
	t.writeBack()
	if t.TextArea.Value() != before.value {
		t.pushUndo(before)
		t.changed()
	}
	t.followCursor()
	return cmd
}

func (t *TextEditor) handleMouse(msg tea.MouseMsg) tea.Cmd {
	b := t.cur
	switch msg.Type {
	case tea.MouseWheelUp:
		if msg.Shift && !t.Wrap {
			b.scrollX -= 6
		} else {
			b.scrollY -= 3
		}
	case tea.MouseWheelDown:
		if msg.Shift && !t.Wrap {
			b.scrollX += 6
		} else {
			b.scrollY += 3
		}
	case tea.MouseWheelLeft:
		b.scrollX -= 6
	case tea.MouseWheelRight:
		b.scrollX += 6
	case tea.MouseLeft:
		// msg.X / msg.Y are relative to the editor's top-left content cell.
		row := b.scrollY + msg.Y
		col := b.scrollX + msg.X - t.gutterWidth()
		if col < 0 {
			col = 0
		}
		if t.Wrap {
			vrows := t.layout(strings.Split(t.TextArea.Value(), "\n"), t.avail())
			if row >= len(vrows) {
				row = len(vrows) - 1
			}
			if row >= 0 {
				v := vrows[row]
				row = v.line
				col = v.start + col
				if col > v.end {
					col = v.end
				}
			}
		}
		if msg.Action == tea.MouseActionMotion {
			// Drag: extend the selection from where the press happened.
			if b.anchor == nil {
				a := t.cursorPos()
				b.anchor = &a
			}
			moveTo(&t.TextArea, row, col)
			t.writeBack()
			t.followCursor()
			return nil
		}
		now := time.Now()
		here := pos{row, col}
		moveTo(&t.TextArea, row, col)
		t.clearSelection()
		t.writeBack()
		if here == b.lastClick && now.Sub(b.lastAt) < 400*time.Millisecond {
			t.selectWordAt(row, col)
			b.lastAt = time.Time{}
		} else {
			b.lastClick, b.lastAt = here, now
		}
	}
	t.clampScroll()
	return nil
}

func (t *TextEditor) handleKey(msg tea.KeyMsg) tea.Cmd {
	t.status = ""
	if t.prompt != promptNone {
		t.handlePromptKey(msg)
		return nil
	}
	switch msg.String() {
	case "ctrl+z":
		if n := len(t.cur.undo); n > 0 {
			s := t.cur.undo[n-1]
			t.cur.undo = t.cur.undo[:n-1]
			t.cur.redo = append(t.cur.redo, t.snap())
			t.restore(s)
			t.changed()
		}
		return nil
	case "ctrl+y", "ctrl+shift+z":
		if n := len(t.cur.redo); n > 0 {
			s := t.cur.redo[n-1]
			t.cur.redo = t.cur.redo[:n-1]
			t.cur.undo = append(t.cur.undo, t.snap())
			t.restore(s)
			t.changed()
		}
		return nil
	case "ctrl+f":
		t.prompt = promptFind
		t.promptValue = t.cur.findLast
		return nil
	case "ctrl+g":
		t.prompt = promptGoto
		t.promptValue = ""
		return nil
	case "f3":
		t.findNext(t.cur.findLast)
		return nil
	case "ctrl+a":
		t.SelectAll()
		return nil
	case "alt+z":
		t.Wrap = !t.Wrap
		t.cur.scrollX = 0
		t.followCursor()
		if t.Wrap {
			t.status = "word wrap on"
		} else {
			t.status = "word wrap off"
		}
		return nil
	case "ctrl+c":
		t.Copy()
		return nil
	case "ctrl+x":
		t.Cut()
		return nil
	case "shift+left", "shift+right", "shift+up", "shift+down", "shift+home", "shift+end",
		"ctrl+shift+left", "ctrl+shift+right", "ctrl+shift+home", "ctrl+shift+end":
		if t.cur.anchor == nil {
			a := t.cursorPos()
			t.cur.anchor = &a
		}
		plain := strings.Replace(msg.String(), "shift+", "", 1)
		t.TextArea, _ = t.TextArea.Update(tea.KeyMsg{Type: keyTypeFor(plain)})
		t.writeBack()
		t.followCursor()
		return nil
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tabSpaces)}
	case "shift+tab":
		return nil // owned by the screen (focus cycling)
	case "pgup", "pgdown":
		n := t.contentRows() - 1
		if msg.String() == "pgup" {
			n = -n
		}
		before := t.snap()
		moveTo(&t.TextArea, before.row+n, before.col)
		t.writeBack()
		t.followCursor()
		return nil
	}

	// Editing with a selection replaces it (VS Code); pure movement drops it.
	before := t.snap()
	if t.HasSelection() {
		switch msg.Type {
		case tea.KeyRunes, tea.KeySpace, tea.KeyEnter, tea.KeyBackspace, tea.KeyDelete:
			t.deleteSelection()
			if msg.Type == tea.KeyBackspace || msg.Type == tea.KeyDelete {
				t.pushUndo(before)
				t.changed()
				t.followCursor()
				return nil
			}
		}
	}
	t.clearSelection()

	var cmd tea.Cmd
	t.TextArea, cmd = t.TextArea.Update(msg)
	t.writeBack()
	if t.TextArea.Value() != before.value {
		t.pushUndo(before)
		t.changed()
	}
	t.followCursor()
	return cmd
}

// keyTypeFor maps a plain movement key name to its tea.KeyType.
func keyTypeFor(name string) tea.KeyType {
	switch name {
	case "left":
		return tea.KeyLeft
	case "right":
		return tea.KeyRight
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	case "home":
		return tea.KeyHome
	case "end":
		return tea.KeyEnd
	case "ctrl+left":
		return tea.KeyCtrlLeft
	case "ctrl+right":
		return tea.KeyCtrlRight
	case "ctrl+home":
		return tea.KeyCtrlHome
	case "ctrl+end":
		return tea.KeyCtrlEnd
	}
	return tea.KeyRunes
}

func (t *TextEditor) changed() {
	if t.OnChanged != nil {
		t.OnChanged(t.GetContent())
	}
}

func (t *TextEditor) handlePromptKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc":
		t.prompt = promptNone
	case "enter":
		if t.prompt == promptGoto {
			n, err := strconv.Atoi(strings.TrimSpace(t.promptValue))
			if err == nil {
				moveTo(&t.TextArea, n-1, 0)
				t.writeBack()
				t.followCursor()
			}
			t.prompt = promptNone
			return
		}
		t.cur.findLast = t.promptValue
		t.findNext(t.promptValue)
	case "backspace":
		if r := []rune(t.promptValue); len(r) > 0 {
			t.promptValue = string(r[:len(r)-1])
		}
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			t.promptValue += string(msg.Runes)
		}
	}
}

// findNext moves the cursor to the next case-insensitive match after the
// cursor, wrapping around.
func (t *TextEditor) findNext(q string) {
	if q == "" {
		return
	}
	lines := strings.Split(t.TextArea.Value(), "\n")
	lq := strings.ToLower(q)
	row, col := t.TextArea.Line(), t.TextArea.LineInfo().ColumnOffset
	for i := 0; i <= len(lines); i++ {
		r := (row + i) % len(lines)
		start := 0
		if i == 0 {
			start = col + 1
		}
		lr := []rune(strings.ToLower(lines[r]))
		if start > len(lr) {
			continue
		}
		if idx := strings.Index(string(lr[start:]), lq); idx >= 0 {
			c := start + len([]rune(string(lr[start:])[:idx]))
			moveTo(&t.TextArea, r, c)
			t.writeBack()
			t.followCursor()
			return
		}
	}
	t.status = "no match: " + q
}

// ---------- scrolling ----------

func (t *TextEditor) contentRows() int {
	n := t.Height - 1 // status / prompt row
	if n < 1 {
		n = 1
	}
	return n
}

func (t *TextEditor) gutterWidth() int {
	w := len(strconv.Itoa(t.TextArea.LineCount())) + 2
	if t.AnnotWidth > 0 {
		w += t.AnnotWidth + 1
	}
	return w
}

// avail is the text width: pane minus gutter minus the scrollbar column.
func (t *TextEditor) avail() int {
	a := t.Width - t.gutterWidth() - 1
	if a < 1 {
		a = 1
	}
	return a
}

func (t *TextEditor) followCursor() {
	b := t.cur
	row, col := t.TextArea.Line(), t.TextArea.LineInfo().ColumnOffset
	lines := strings.Split(t.TextArea.Value(), "\n")
	vr := vrowOf(t.layout(lines, t.avail()), row, col)
	rows := t.contentRows()
	if vr < b.scrollY {
		b.scrollY = vr
	}
	if vr >= b.scrollY+rows {
		b.scrollY = vr - rows + 1
	}
	if t.Wrap {
		b.scrollX = 0
	} else {
		dc := col
		if row < len(lines) {
			dc = displayCol([]rune(lines[row]), col)
		}
		if dc < b.scrollX {
			b.scrollX = dc
		}
		if dc >= b.scrollX+t.avail() {
			b.scrollX = dc - t.avail() + 1
		}
	}
	t.clampScroll()
}

func (t *TextEditor) clampScroll() {
	b := t.cur
	total := t.TextArea.LineCount()
	if t.Wrap {
		total = len(t.layout(strings.Split(t.TextArea.Value(), "\n"), t.avail()))
	}
	if max := total - t.contentRows(); b.scrollY > max {
		b.scrollY = max
	}
	if b.scrollY < 0 {
		b.scrollY = 0
	}
	if !t.Wrap {
		if max := t.longestLine() - t.avail(); b.scrollX > max {
			b.scrollX = max
		}
	}
	if b.scrollX < 0 {
		b.scrollX = 0
	}
}

// longestLine is the display width of the widest line in the buffer.
func (t *TextEditor) longestLine() int {
	w := 0
	for _, l := range strings.Split(t.TextArea.Value(), "\n") {
		r := []rune(l)
		if n := displayCol(r, len(r)); n > w {
			w = n
		}
	}
	return w
}

// ---------- View ----------

func (t *TextEditor) highlight(line string) string {
	b := t.cur
	if b.lexer == nil || strings.TrimSpace(line) == "" {
		return line
	}
	if hl, ok := b.hlCache[line]; ok {
		return hl
	}
	it, err := b.lexer.Tokenise(nil, line)
	if err != nil {
		return line
	}
	var sb strings.Builder
	if err := formatters.Get("terminal256").Format(&sb, styles.Get("dracula"), it); err != nil {
		return line
	}
	hl := strings.ReplaceAll(sb.String(), "\n", "") // lexers with EnsureNL add one
	if len(b.hlCache) > 4000 {
		b.hlCache = map[string]string{} // ponytail: crude cache reset; LRU if it ever matters
	}
	b.hlCache[line] = hl
	return hl
}

func (t *TextEditor) View() string {
	border := theme.UnfocusedBorderStyle
	if t.Focused {
		border = theme.FocusedBorderStyle
	}
	b := t.cur
	lines := strings.Split(t.TextArea.Value(), "\n")
	row, col := t.TextArea.Line(), t.TextArea.LineInfo().ColumnOffset
	gw := t.gutterWidth()
	_ = gw
	avail := t.avail() // reserves the scrollbar column
	rows := t.contentRows()
	t.clampScroll()
	selS, selE, hasSel := t.Selection()

	vrows := t.layout(lines, avail)
	numW := len(strconv.Itoa(t.TextArea.LineCount())) + 1
	sb := theme.VScrollbar(rows, len(vrows), b.scrollY)
	var out []string
	cut := false // any visible line truncated on the right
	for vi := b.scrollY; vi < b.scrollY+rows; vi++ {
		if vi >= len(vrows) {
			out = append(out, "")
			continue
		}
		v := vrows[vi]
		i := v.line
		numStyle := theme.GutterStyle
		if i == row {
			numStyle = theme.GutterActiveStyle
		}
		num := numStyle.Render(fmt.Sprintf("%*d ", numW, i+1))
		if !v.first {
			num = strings.Repeat(" ", numW+1)
		}
		if t.AnnotWidth > 0 {
			a := ""
			if i < len(t.Annotations) && v.first {
				a = t.Annotations[i]
			}
			a = ansi.Truncate(a, t.AnnotWidth, "…")
			a += strings.Repeat(" ", t.AnnotWidth-lipgloss.Width(a))
			st := theme.BlameStyle
			if i == row {
				st = theme.GutterActiveStyle
			}
			num = st.Render(a) + " " + num
		}
		raw := []rune(lines[i])
		shown := strings.ReplaceAll(lines[i], "\t", tabDisplay)
		full := t.highlight(shown)
		if hasSel && i >= selS.row && i <= selE.row {
			sc, ec := 0, displayCol(raw, len(raw))
			if i == selS.row {
				sc = displayCol(raw, selS.col)
			}
			if i == selE.row {
				ec = displayCol(raw, selE.col)
			}
			if ec > sc {
				sr := []rune(shown)
				full = ansi.Cut(full, 0, sc) + theme.SelectionStyle.Render(string(sr[sc:ec])) + ansi.Cut(full, ec, len(sr))
			}
		}
		// Visible window of this segment in display columns (never past the
		// segment end, so a wrapped row does not bleed into the next one).
		segStart := displayCol(raw, v.start) + b.scrollX
		segEnd := displayCol(raw, v.end)
		if t.Wrap && segEnd > segStart+avail {
			segEnd = segStart + avail
		}
		if !t.Wrap {
			segEnd = segStart + avail
		}
		text := ansi.Cut(full, segStart, segEnd)
		if !t.Wrap && displayCol(raw, len(raw)) > segStart+avail {
			cut = true
		}
		if i == row && t.Focused && col >= v.start && (col < v.end || (col == v.end && (vi+1 >= len(vrows) || vrows[vi+1].line != i))) {
			c := displayCol(raw, col) - segStart
			if c >= 0 && c < avail { // cursor scrolled out of view: don't draw it
				cell := " "
				if col < len(raw) && raw[col] != '\t' {
					cell = string(raw[col])
				}
				text = ansi.Cut(text, 0, c) + theme.CursorCellStyle.Render(cell) + ansi.Cut(text, c+1, avail)
			}
		}
		if w := lipgloss.Width(text); w < avail {
			text += strings.Repeat(" ", avail-w)
		}
		out = append(out, num+text)
	}
	// Scrollbar column on the right.
	for i := range out {
		if sb != nil {
			if w := lipgloss.Width(out[i]); w < t.Width-1 {
				out[i] += strings.Repeat(" ", t.Width-1-w)
			}
			out[i] += sb[i]
		}
	}

	var status string
	switch t.prompt {
	case promptFind:
		status = theme.PromptStyle.Render(" find: "+t.promptValue+"▏") + theme.MutedStyle.Render("  ⏎ next · esc close")
	case promptGoto:
		status = theme.PromptStyle.Render(" go to line: "+t.promptValue+"▏") + theme.MutedStyle.Render("  ⏎ go · esc close")
	default:
		lang := "plain"
		if b.lexer != nil {
			lang = strings.ToLower(b.lexer.Config().Name)
		}
		note := t.status
		if note == "" {
			note = t.Hint
		}
		if note == "" && t.LineHint != nil {
			note = t.LineHint(row)
		}
		if hasSel {
			note = fmt.Sprintf("%d selected · ctrl+c copy · ctrl+x cut", len([]rune(t.SelectedText())))
		} else if cut || b.scrollX > 0 {
			note = fmt.Sprintf("⟷ col %d · shift+wheel scrolls · alt+z wraps", b.scrollX+1)
		}
		status = theme.MutedStyle.Render(fmt.Sprintf(" Ln %d, Col %d  ·  %s  ·  %s", row+1, col+1, lang, note))
	}
	out = append(out, ansi.Truncate(status, t.Width, "…"))
	return border.Width(t.Width).Height(t.Height).Render(lipgloss.JoinVertical(lipgloss.Left, out...))
}
