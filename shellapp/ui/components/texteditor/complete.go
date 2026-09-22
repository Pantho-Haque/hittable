package texteditor

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/hittable/shellapp/ui/theme"
)

const (
	// A word has to reach this length before suggestions appear on their own.
	// ctrl+space opens the popup regardless of what has been typed.
	completeMinPrefix = 2
	completeMaxItems  = 200
	completeRows      = 8 // visible suggestion rows
	completeMinWidth  = 16
	completeMaxWidth  = 34
	// Past this size a full-buffer word scan on every keystroke stops being
	// free, so identifiers are dropped and only keywords/schema remain.
	completeMaxScan = 512 << 10
)

// completion is the transient state of the suggestion popup. It hangs off the
// editor rather than the buffer, so switching files dismisses it.
type completion struct {
	items  []string
	idx    int
	scroll int
	row    int    // line the prefix sits on
	from   int    // rune column where the prefix starts
	prefix string // what the user has typed so far
}

// move steps the highlighted item by d, wrapping, and keeps it in view.
func (c *completion) move(d int) {
	n := len(c.items)
	if n == 0 {
		return
	}
	c.idx = (c.idx + d + n) % n
	if c.idx < c.scroll {
		c.scroll = c.idx
	}
	if c.idx >= c.scroll+completeRows {
		c.scroll = c.idx - completeRows + 1
	}
	if max := n - completeRows; c.scroll > max {
		if max < 0 {
			max = 0
		}
		c.scroll = max
	}
}

// CompletionOpen reports whether the suggestion popup is capturing keys.
func (t *TextEditor) CompletionOpen() bool { return t.comp != nil }

// wordStart returns the rune index where the identifier ending at col begins.
func wordStart(r []rune, col int) int {
	s := col
	for s > 0 && isWordRune(r[s-1]) {
		s--
	}
	return s
}

// openCompletion recomputes the suggestion list for the word before the
// cursor. explicit marks a ctrl+space request, which offers candidates even
// with nothing typed yet; otherwise a short prefix just closes the popup.
func (t *TextEditor) openCompletion(explicit bool) {
	lines := strings.Split(t.TextArea.Value(), "\n")
	p := t.cursorPos()
	if p.row >= len(lines) {
		t.comp = nil
		return
	}
	r := []rune(lines[p.row])
	col := p.col
	if col > len(r) {
		col = len(r)
	}
	from := wordStart(r, col)
	prefix := string(r[from:col])

	if !explicit && len([]rune(prefix)) < completeMinPrefix {
		t.comp = nil
		return
	}

	items := t.candidates(lines, p.row, prefix)
	if len(items) == 0 {
		t.comp = nil
		if explicit {
			t.status = "no suggestions"
		}
		return
	}
	t.comp = &completion{items: items, row: p.row, from: from, prefix: prefix}
}

// candidates ranks suggestions in three tiers — request-schema, language
// keywords, then identifiers already in the buffer — alphabetical within each,
// so the most specific help is always at the top of the list.
func (t *TextEditor) candidates(lines []string, row int, prefix string) []string {
	lower := strings.ToLower(prefix)
	seen := map[string]bool{}
	var out []string

	addGroup := func(group []string) {
		var matched []string
		for _, s := range group {
			if s == "" || s == prefix || seen[s] {
				continue
			}
			if !strings.HasPrefix(strings.ToLower(s), lower) {
				continue
			}
			seen[s] = true
			matched = append(matched, s)
		}
		sort.Strings(matched)
		out = append(out, matched...)
	}

	addGroup(t.schemaCandidates(lines, row))
	addGroup(keywordsFor(t.cur))
	addGroup(t.bufferWords(lines))

	if len(out) > completeMaxItems {
		out = out[:completeMaxItems]
	}
	return out
}

// bufferWords collects the identifiers already present in the file. The result
// is cached on the buffer because an edit is the only thing that changes it.
func (t *TextEditor) bufferWords(lines []string) []string {
	b := t.cur
	if !b.wordsDirty {
		return b.words
	}
	b.words = nil
	b.wordsDirty = false

	total := 0
	seen := map[string]bool{}
	for _, line := range lines {
		total += len(line)
		if total > completeMaxScan {
			break
		}
		r := []rune(line)
		for i := 0; i < len(r); {
			if !isWordRune(r[i]) {
				i++
				continue
			}
			j := i
			for j < len(r) && isWordRune(r[j]) {
				j++
			}
			// Single and double letters are noise, never a useful suggestion.
			if j-i >= 3 {
				if w := string(r[i:j]); !seen[w] {
					seen[w] = true
					b.words = append(b.words, w)
				}
			}
			i = j
		}
	}
	return b.words
}

var (
	hitKeys      = []string{"body", "headers", "method", "params", "response", "url"}
	httpMethods  = []string{"DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"}
	contentTypes = []string{
		"application/json", "application/x-www-form-urlencoded", "application/xml",
		"multipart/form-data", "text/html", "text/plain",
	}
	commonHeaders = []string{
		"Accept", "Accept-Encoding", "Accept-Language", "Authorization", "Cache-Control",
		"Connection", "Content-Length", "Content-Type", "Cookie", "Host", "If-Match",
		"If-None-Match", "Origin", "Referer", "User-Agent", "X-Api-Key",
		"X-Correlation-Id", "X-Request-Id",
	}
)

// schemaCandidates offers the .hit request schema: verbs after "method",
// header names inside the "headers" block, and the top-level keys otherwise.
func (t *TextEditor) schemaCandidates(lines []string, row int) []string {
	if !strings.HasSuffix(strings.ToLower(t.currentPath), ".hit") || row >= len(lines) {
		return nil
	}
	line := strings.ToLower(lines[row])
	switch {
	case strings.Contains(line, `"method"`):
		return httpMethods
	case strings.Contains(line, "content-type"):
		return contentTypes
	case inHeadersBlock(lines, row):
		return commonHeaders
	}
	return hitKeys
}

// inHeadersBlock walks outwards from row to find the object that encloses it,
// and reports whether that object is the request's "headers".
func inHeadersBlock(lines []string, row int) bool {
	depth := 0
	for i := row - 1; i >= 0; i-- {
		depth += strings.Count(lines[i], "}") - strings.Count(lines[i], "{")
		if depth < 0 {
			return strings.Contains(strings.ToLower(lines[i]), `"headers"`)
		}
	}
	return false
}

// keywordsFor returns the reserved words of the buffer's language, so
// suggestions are useful in a file that has barely been written yet.
func keywordsFor(b *buffer) []string {
	if b == nil || b.lexer == nil {
		return nil
	}
	return languageKeywords[strings.ToLower(b.lexer.Config().Name)]
}

var languageKeywords = map[string][]string{
	"go": {
		"append", "break", "cap", "case", "chan", "close", "const", "continue", "copy",
		"default", "defer", "delete", "else", "error", "fallthrough", "false", "for",
		"func", "go", "goto", "if", "import", "interface", "len", "make", "map", "new",
		"nil", "package", "panic", "range", "recover", "return", "select", "string",
		"struct", "switch", "true", "type", "var",
	},
	"python": {
		"and", "as", "assert", "async", "await", "break", "class", "continue", "def",
		"del", "elif", "else", "except", "False", "finally", "for", "from", "global",
		"if", "import", "in", "is", "lambda", "None", "nonlocal", "not", "or", "pass",
		"raise", "return", "True", "try", "while", "with", "yield",
	},
	"javascript": {
		"async", "await", "break", "case", "catch", "class", "const", "continue",
		"default", "delete", "else", "export", "extends", "false", "finally", "for",
		"function", "if", "import", "instanceof", "let", "new", "null", "return",
		"static", "super", "switch", "this", "throw", "true", "try", "typeof", "undefined",
		"var", "void", "while", "yield",
	},
	"typescript": {
		"abstract", "any", "as", "async", "await", "boolean", "break", "case", "catch",
		"class", "const", "continue", "declare", "default", "delete", "else", "enum",
		"export", "extends", "false", "finally", "for", "function", "if", "implements",
		"import", "instanceof", "interface", "let", "never", "new", "null", "number",
		"private", "protected", "public", "readonly", "return", "static", "string",
		"super", "switch", "this", "throw", "true", "try", "type", "typeof", "undefined",
		"unknown", "var", "void", "while", "yield",
	},
	"rust": {
		"async", "await", "break", "const", "continue", "crate", "dyn", "else", "enum",
		"false", "fn", "for", "if", "impl", "in", "let", "loop", "match", "mod", "move",
		"mut", "pub", "ref", "return", "self", "static", "struct", "super", "trait",
		"true", "type", "unsafe", "use", "where", "while",
	},
	"json": {"false", "null", "true"},
	"yaml": {"false", "null", "true"},
}

// handleCompletionKey routes a key while the popup is open. It reports whether
// the key was consumed; anything else falls through to normal editing and then
// refreshes the list.
func (t *TextEditor) handleCompletionKey(msg tea.KeyMsg) bool {
	c := t.comp
	if c == nil {
		return false
	}
	switch msg.String() {
	case "esc":
		t.comp = nil
		t.status = "suggestions dismissed"
		return true
	case "up", "ctrl+p":
		c.move(-1)
		return true
	case "down", "ctrl+n":
		c.move(1)
		return true
	case "tab", "enter":
		t.acceptCompletion()
		return true
	case "left", "right", "home", "end", "pgup", "pgdown":
		// Moving off the word invalidates the prefix the list was built from.
		t.comp = nil
		return false
	}
	return false
}

// acceptCompletion replaces the typed prefix with the highlighted item as a
// single undo step.
func (t *TextEditor) acceptCompletion() {
	c := t.comp
	t.comp = nil
	if c == nil || c.idx < 0 || c.idx >= len(c.items) {
		return
	}
	lines := strings.Split(t.TextArea.Value(), "\n")
	if c.row >= len(lines) {
		return
	}
	r := []rune(lines[c.row])
	col := t.cursorPos().col
	if col > len(r) {
		col = len(r)
	}
	if c.from > col {
		return
	}

	before := t.snap()
	word := c.items[c.idx]
	lines[c.row] = string(r[:c.from]) + word + string(r[col:])
	t.TextArea.SetValue(strings.Join(lines, "\n"))
	moveTo(&t.TextArea, c.row, c.from+len([]rune(word)))
	t.clearSelection()
	t.writeBack()
	t.pushUndo(before)
	t.changed()
	t.followCursor()
}

// overlayCompletion composites the popup over the rendered rows, anchored at
// the start of the word being completed and flipped above the cursor when
// there is no room below.
func (t *TextEditor) overlayCompletion(out []string, curX, curY int) {
	c := t.comp
	if c == nil || len(out) == 0 {
		return
	}

	width := completeMinWidth
	for _, item := range c.items {
		if w := lipgloss.Width(item) + 2; w > width {
			width = w
		}
	}
	if width > completeMaxWidth {
		width = completeMaxWidth
	}

	end := c.scroll + completeRows
	if end > len(c.items) {
		end = len(c.items)
	}
	var box []string
	for i := c.scroll; i < end; i++ {
		st := theme.ContextMenuItemStyle
		if i == c.idx {
			st = theme.ContextMenuItemHoverStyle
		}
		box = append(box, st.Width(width).Render(ansi.Truncate(c.items[i], width, "…")))
	}
	if len(c.items) > completeRows {
		box = append(box, theme.MutedStyle.Render(
			ansi.Truncate(" "+itoa(c.idx+1)+"/"+itoa(len(c.items)), width, "")))
	}
	rendered := strings.Split(theme.ContextMenuStyle.Render(strings.Join(box, "\n")), "\n")
	boxW := lipgloss.Width(rendered[0])

	// Align the box with the word, not the cursor, so the list sits under
	// what it is completing.
	x := curX - lipgloss.Width(c.prefix)
	if x+boxW > t.Width-1 {
		x = t.Width - 1 - boxW
	}
	if x < 0 {
		x = 0
	}

	y := curY + 1
	if y+len(rendered) > len(out) {
		y = curY - len(rendered)
	}
	if y < 0 {
		y = 0
	}

	for i, row := range rendered {
		ry := y + i
		if ry < 0 || ry >= len(out) {
			continue
		}
		// Rows past the end of the file are empty, and cutting an empty string
		// yields nothing — pad first or the box slides to column 0.
		base := out[ry]
		if w := lipgloss.Width(base); w < x {
			base += strings.Repeat(" ", x-w)
		}
		out[ry] = ansi.Cut(base, 0, x) + row + ansi.Cut(base, x+boxW, t.Width)
	}
}
