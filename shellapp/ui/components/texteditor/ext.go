package texteditor

import (
	"sort"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/internal/extensions"
	"github.com/hittable/shellapp/ui/theme"
)

// syntaxStyle colours every editor. A VS Code colour theme replaces it
// (SetSyntaxStyle); styleGen invalidates each buffer's highlight cache.
var (
	syntaxStyle = styles.Get("dracula")
	styleGen    int
)

// SetSyntaxStyle switches the highlighting style for all editors; nil
// restores the built-in Dracula.
func SetSyntaxStyle(s *chroma.Style) {
	if s == nil {
		s = styles.Get("dracula")
	}
	syntaxStyle = s
	styleGen++
}

// Mark is a diagnostic shown in the editor: an underlined range, a tinted
// line number, and its message in the status row when the cursor is on it.
// Columns are runes.
type Mark struct {
	Line, Start, End int
	Severity         int // 0 error, 1 warning, 2 information, 3 hint
	Message          string
}

// ExtItem is a suggestion from an extension (or a contributed snippet).
type ExtItem struct {
	Label, Detail string
	Insert        string // text to insert; VS Code snippet syntax when Snippet
	Filter        string // what typing filters on (defaults to Label)
	Sort          string
	Snippet       bool
	// From/To replace [From, To) on the cursor row (rune columns). From < 0
	// means "the word before the cursor".
	From, To int
	Data     any // round-tripped to OnExtAccept
}

type item struct {
	label  string
	detail string
	ext    *ExtItem
}

func (it item) row(width int) string {
	if it.detail == "" || lipgloss.Width(it.label)+4 >= width {
		return it.label
	}
	pad := width - lipgloss.Width(it.label) - lipgloss.Width(it.detail) - 1
	if pad < 2 {
		return it.label + "  " + it.detail
	}
	return it.label + strings.Repeat(" ", pad) + it.detail
}

type extState struct {
	marks map[string][]Mark // by path

	// The last extension answer, reused while typing continues the same
	// word: a language server returns the whole list for a word start.
	cachePath string
	cacheRow  int
	cacheFrom int
	cache     []ExtItem
	asked     bool // a request for this word start is out
	noAsk     bool // set while an answer is being shown, so it is not re-asked
	partial   bool // the last answer was incomplete: ask again on every key

	popup []string // hover text, until the next key or click
}

// SetMarks replaces the diagnostics shown for path.
func (t *TextEditor) SetMarks(path string, ms []Mark) {
	if t.ext.marks == nil {
		t.ext.marks = map[string][]Mark{}
	}
	if len(ms) == 0 {
		delete(t.ext.marks, path)
		return
	}
	t.ext.marks[path] = ms
}

// Marks returns the diagnostics shown for path.
func (t *TextEditor) Marks(path string) []Mark { return t.ext.marks[path] }

// CurrentPath is the file in the active buffer.
func (t *TextEditor) CurrentPath() string { return t.currentPath }

var (
	errMark  = lipgloss.NewStyle().Underline(true).Foreground(theme.ErrorColor)
	warnMark = lipgloss.NewStyle().Underline(true).Foreground(theme.WarningColor)
	infoMark = lipgloss.NewStyle().Underline(true).Foreground(theme.PrimaryColor)
)

func sevStyle(sev int) lipgloss.Style {
	switch sev {
	case 0:
		return errMark
	case 1:
		return warnMark
	}
	return infoMark
}

// lineSeverity is the worst error/warning on a line.
func (t *TextEditor) lineSeverity(line int) (int, bool) {
	best, ok := 9, false
	for _, m := range t.ext.marks[t.currentPath] {
		if m.Line == line && m.Severity <= 1 && m.Severity < best {
			best, ok = m.Severity, true
		}
	}
	return best, ok
}

// underlineMarks styles the marked ranges of one rendered line. Hints are
// not drawn (VS Code shows them as faint dots), only listed.
func (t *TextEditor) underlineMarks(line int, raw []rune, shown, full string) string {
	ms := t.ext.marks[t.currentPath]
	if len(ms) == 0 {
		return full
	}
	sr := []rune(shown)
	for _, m := range ms {
		if m.Line != line || m.Severity > 2 {
			continue
		}
		s, e := m.Start, m.End
		if e <= s {
			e = s + 1 // an empty range still marks one cell
		}
		sc, ec := displayCol(raw, s), displayCol(raw, e)
		if ec > len(sr) {
			ec = len(sr)
		}
		if sc >= ec {
			if len(sr) == 0 {
				continue
			}
			sc, ec = max(len(sr)-1, 0), len(sr)
		}
		full = ansi.Cut(full, 0, sc) + sevStyle(m.Severity).Render(string(sr[sc:ec])) + ansi.Cut(full, ec, len(sr))
	}
	return full
}

// markNote is the message of the diagnostic under (or nearest on the line
// to) the cursor, for the status row.
func (t *TextEditor) markNote(row, col int) string {
	var pick *Mark
	for i, m := range t.ext.marks[t.currentPath] {
		if m.Line != row {
			continue
		}
		if pick == nil || (col >= m.Start && col <= m.End) || m.Severity < pick.Severity {
			pick = &t.ext.marks[t.currentPath][i]
		}
	}
	if pick == nil {
		return ""
	}
	icon := map[int]string{0: "✖ ", 1: "⚠ ", 2: "ℹ ", 3: "· "}[pick.Severity]
	return icon + strings.ReplaceAll(pick.Message, "\n", " ")
}

// extCandidates returns extension suggestions for the word starting at from,
// asking the extensions again when the word start moved or on ctrl+space.
func (t *TextEditor) extCandidates(row, from int, prefix string, explicit bool) []item {
	var out []item
	if t.ExtItems != nil {
		for _, it := range t.ExtItems(t.currentPath) {
			it := it
			if matches(it, prefix) {
				out = append(out, item{label: it.Label, detail: it.Detail, ext: &it})
			}
		}
	}
	if t.ExtComplete == nil {
		return out
	}
	e := &t.ext
	same := e.cachePath == t.currentPath && e.cacheRow == row && e.cacheFrom == from
	if !e.noAsk && (!same || explicit || e.partial) {
		e.cachePath, e.cacheRow, e.cacheFrom, e.cache, e.asked = t.currentPath, row, from, nil, true
		t.ExtComplete(t.currentPath, row, from+len([]rune(prefix)), "")
	}
	return append(filterExt(e.cache, prefix), out...)
}

func matches(it ExtItem, prefix string) bool {
	if prefix == "" {
		return true
	}
	f := it.Filter
	if f == "" {
		f = it.Label
	}
	// Subsequence, case-insensitive: "gUs" finds "getUsers", as VS Code does.
	q := []rune(strings.ToLower(prefix))
	i := 0
	for _, c := range strings.ToLower(f) {
		if i < len(q) && c == q[i] {
			i++
		}
	}
	return i == len(q) && len(q) > 0 && unicode.ToLower([]rune(f)[0]) == q[0]
}

func filterExt(items []ExtItem, prefix string) []item {
	var out []item
	for i := range items {
		if matches(items[i], prefix) {
			it := items[i]
			out = append(out, item{label: it.Label, detail: it.Detail, ext: &it})
		}
	}
	return out
}

// DeliverExtCompletions hands back an extension's answer for (path, row).
// It is dropped when the cursor has since left that row or file. trigger is
// true for an answer to a trigger character, which opens the popup even with
// nothing typed yet.
func (t *TextEditor) DeliverExtCompletions(path string, row int, items []ExtItem, trigger, incomplete bool) {
	if path != t.currentPath || t.TextArea.Line() != row || !t.Focused {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].Sort, items[j].Sort
		if a == "" {
			a = items[i].Label
		}
		if b == "" {
			b = items[j].Label
		}
		return a < b
	})
	lines := strings.Split(t.TextArea.Value(), "\n")
	r := []rune(lines[row])
	col := min(t.cursorPos().col, len(r))
	from := wordStart(r, col)
	e := &t.ext
	e.cachePath, e.cacheRow, e.cacheFrom, e.cache, e.asked, e.partial = path, row, from, items, false, incomplete
	if t.comp == nil && !trigger && col-from < completeMinPrefix {
		return // the user moved on (or deleted the word) while we waited
	}
	if t.comp != nil && t.comp.row != row {
		return
	}
	prev := ""
	if t.comp != nil && t.comp.idx < len(t.comp.items) {
		prev = t.comp.items[t.comp.idx].label
	}
	e.noAsk = true
	t.openCompletion(true)
	e.noAsk = false
	if t.comp != nil && prev != "" {
		for i, it := range t.comp.items {
			if it.label == prev {
				t.comp.idx = i
				t.comp.move(0)
			}
		}
	}
}

// acceptExt inserts an extension suggestion as one undo step: its range
// replaced, a snippet expanded with the cursor on the first tabstop, and
// continuation lines indented like the line it lands on.
func (t *TextEditor) acceptExt(c *completion, it ExtItem, lines []string, r []rune, col int) {
	from, to := c.from, col
	if it.From >= 0 {
		from = min(it.From, len(r))
	}
	if it.To > to && it.To <= len(r) {
		to = it.To
	}
	text, cur := it.Insert, len([]rune(it.Insert))
	if it.Snippet {
		text, cur = extensions.ExpandSnippet(it.Insert)
	}
	indent := leadingSpace(lines[c.row])
	if strings.Contains(text, "\n") {
		parts := strings.Split(text, "\n")
		// Re-measure the cursor offset as indentation is added line by line.
		runeOff, added := 0, 0
		for i := range parts {
			if i > 0 {
				if runeOff <= cur {
					added += len([]rune(indent))
				}
				parts[i] = indent + parts[i]
			}
			runeOff += len([]rune(parts[i])) + 1
		}
		text = strings.Join(parts, "\n")
		cur += added
	}
	before := t.snap()
	lines[c.row] = string(r[:from]) + text + string(r[to:])
	t.TextArea.SetValue(strings.Join(lines, "\n"))
	ins := []rune(text)
	row, colOut := c.row, from
	for _, ch := range ins[:min(cur, len(ins))] {
		if ch == '\n' {
			row++
			colOut = 0
		} else {
			colOut++
		}
	}
	moveTo(&t.TextArea, row, colOut)
	t.clearSelection()
	t.writeBack()
	t.pushUndo(before)
	t.changed()
	t.followCursor()
	if t.OnExtAccept != nil {
		t.OnExtAccept(t.currentPath, it)
	}
}

func leadingSpace(s string) string {
	for i, c := range s {
		if c != ' ' && c != '\t' {
			return s[:i]
		}
	}
	return s
}

// ShowPopup shows text in a box at the cursor until the next key or click
// (hover information). Long text is wrapped and cut to a dozen rows.
func (t *TextEditor) ShowPopup(text string) {
	width := min(72, max(t.Width-6, 20))
	var rows []string
	for _, l := range strings.Split(strings.TrimSpace(text), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			continue
		}
		for _, w := range wrapLine(l, width) {
			rows = append(rows, w)
		}
	}
	if len(rows) > 12 {
		rows = append(rows[:11], "…")
	}
	if len(rows) == 0 {
		t.ext.popup = nil
		return
	}
	t.ext.popup = rows
}

// PopupOpen reports whether a hover box is showing.
func (t *TextEditor) PopupOpen() bool { return t.ext.popup != nil }

func wrapLine(s string, width int) []string {
	r := []rune(s)
	if len(r) <= width {
		return []string{s}
	}
	var out []string
	for len(r) > width {
		cut := width
		for i := width; i > width/2; i-- {
			if r[i] == ' ' {
				cut = i
				break
			}
		}
		out = append(out, string(r[:cut]))
		r = []rune(strings.TrimLeft(string(r[cut:]), " "))
	}
	return append(out, string(r))
}

func (t *TextEditor) overlayPopup(out []string, curX, curY int) {
	w := 0
	for _, l := range t.ext.popup {
		w = max(w, lipgloss.Width(l))
	}
	var box []string
	for _, l := range t.ext.popup {
		box = append(box, theme.ContextMenuItemStyle.Width(w+2).Render(l))
	}
	rendered := strings.Split(theme.ContextMenuStyle.Render(strings.Join(box, "\n")), "\n")
	t.overlayBox(out, rendered, curX, curY)
}

// ApplyContent replaces path's text after an extension edit (formatting, a
// quick fix, a rename). For the active buffer it is one undo step and the
// cursor stays on the same line; other open buffers are updated in place.
func (t *TextEditor) ApplyContent(path, content string, row, col int) {
	if path == t.currentPath {
		if t.TextArea.Value() == content {
			return
		}
		before := t.snap()
		t.TextArea.SetValue(content)
		if row < 0 {
			row, col = before.row, before.col
		}
		moveTo(&t.TextArea, row, col)
		t.clearSelection()
		t.writeBack()
		t.pushUndo(before)
		t.changed()
		t.followCursor()
		return
	}
	t.mu.Lock()
	b := t.buffers[path]
	t.mu.Unlock()
	if b != nil && b.ta.Value() != content {
		r, c := b.ta.Line(), b.ta.LineInfo().ColumnOffset
		b.ta.SetValue(content)
		moveTo(&b.ta, r, c)
		b.wordsDirty = true
	}
}

// SelectionRange is the selection as [row, col] start and end (rune columns).
func (t *TextEditor) SelectionRange() (start, end [2]int, ok bool) {
	s, e, ok := t.Selection()
	return [2]int{s.row, s.col}, [2]int{e.row, e.col}, ok
}
