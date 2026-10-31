package texteditor

import (
	"fmt"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

// Find & replace, VS Code style: ctrl+f opens the find bar, every match is
// highlighted as you type and the cursor jumps to the nearest one; tab adds a
// replace field. (Not ctrl+h: some terminals send it for backspace.) Matching is case-insensitive
// and literal, the same rule as the response panel's search.

// match is one occurrence of the query: [col, end) in runes on one line.
type match struct{ row, col, end int }

type matchCache struct {
	q, v string
	m    []match
}

func lowerRunes(s string) []rune {
	r := []rune(s)
	for i := range r {
		r[i] = unicode.ToLower(r[i]) // per rune, so columns never shift
	}
	return r
}

// findMatches returns every non-overlapping match of the find query. View
// calls it each frame, so it is cached on (query, content).
func (t *TextEditor) findMatches() []match {
	if t.cur == nil || t.cur.findLast == "" {
		return nil
	}
	q, v := t.cur.findLast, t.TextArea.Value()
	if t.mcache.q == q && t.mcache.v == v {
		return t.mcache.m
	}
	lq := lowerRunes(q)
	var out []match
	for row, line := range strings.Split(v, "\n") {
		lr := lowerRunes(line)
	scan:
		for col := 0; col+len(lq) <= len(lr); col++ {
			for k, r := range lq {
				if lr[col+k] != r {
					continue scan
				}
			}
			out = append(out, match{row, col, col + len(lq)})
			col += len(lq) - 1
		}
	}
	t.mcache = matchCache{q, v, out}
	return out
}

// currentMatch is the index of the match the cursor sits on, or -1.
func (t *TextEditor) currentMatch(ms []match) int {
	c := t.cursorPos()
	for i, m := range ms {
		if m.row == c.row && m.col == c.col {
			return i
		}
	}
	return -1
}

// jumpMatch moves the cursor to the next (dir 1) or previous (dir -1) match
// from p, wrapping around. inclusive lets a match starting exactly at p count,
// which is what typing into the query wants.
func (t *TextEditor) jumpMatch(p pos, dir int, inclusive bool) {
	ms := t.findMatches()
	if len(ms) == 0 {
		if t.cur.findLast != "" {
			t.status = "no match: " + t.cur.findLast
		}
		return
	}
	pick := -1
	if dir > 0 {
		pick = 0
		for i, m := range ms {
			mp := pos{m.row, m.col}
			if p.less(mp) || (inclusive && mp == p) {
				pick = i
				break
			}
		}
	} else {
		pick = len(ms) - 1
		for i := len(ms) - 1; i >= 0; i-- {
			if mp := (pos{ms[i].row, ms[i].col}); mp.less(p) {
				pick = i
				break
			}
		}
	}
	m := ms[pick]
	t.clearSelection()
	moveTo(&t.TextArea, m.row, m.col)
	t.writeBack()
	t.followCursor()
}

// openFind shows the find bar, seeded with a one-line selection if there is
// one. ctrl+f while it is already open adds the replace field.
func (t *TextEditor) openFind(replace bool) {
	if s := t.SelectedText(); s != "" && !strings.Contains(s, "\n") {
		t.cur.findLast = s
		start, _, _ := t.Selection()
		t.clearSelection()
		moveTo(&t.TextArea, start.row, start.col)
		t.writeBack()
	}
	t.prompt = promptFind
	t.inReplace = false
	if replace {
		t.prompt = promptReplace
		t.inReplace = t.cur.findLast != ""
	}
}

// replaceOne replaces the match under the cursor and moves to the next one.
// If the cursor is not on a match it only moves there, so the first ⏎ shows
// what is about to change, as in VS Code.
func (t *TextEditor) replaceOne() {
	ms := t.findMatches()
	i := t.currentMatch(ms)
	if i < 0 {
		t.jumpMatch(t.cursorPos(), 1, true)
		return
	}
	m := ms[i]
	lines := strings.Split(t.TextArea.Value(), "\n")
	r := []rune(lines[m.row])
	lines[m.row] = string(r[:m.col]) + t.replaceValue + string(r[m.end:])
	t.SetValue(strings.Join(lines, "\n"))
	after := pos{m.row, m.col + len([]rune(t.replaceValue))}
	moveTo(&t.TextArea, after.row, after.col)
	t.writeBack()
	t.jumpMatch(after, 1, true)
}

// replaceAll replaces every match as a single undo step.
func (t *TextEditor) replaceAll() {
	ms := t.findMatches()
	if len(ms) == 0 {
		t.status = "no match: " + t.cur.findLast
		return
	}
	lines := strings.Split(t.TextArea.Value(), "\n")
	for i := len(ms) - 1; i >= 0; i-- { // back to front keeps earlier columns valid
		m := ms[i]
		r := []rune(lines[m.row])
		lines[m.row] = string(r[:m.col]) + t.replaceValue + string(r[m.end:])
	}
	t.SetValue(strings.Join(lines, "\n"))
	t.status = fmt.Sprintf("replaced %d", len(ms))
}

// handleFindKey drives the find / replace bar.
func (t *TextEditor) handleFindKey(msg tea.KeyMsg) {
	field := &t.cur.findLast
	if t.inReplace {
		field = &t.replaceValue
	}
	switch msg.String() {
	case "esc":
		t.prompt = promptNone
		t.inReplace = false
	case "tab", "ctrl+f":
		if t.prompt == promptFind {
			t.prompt = promptReplace
			t.inReplace = true
		} else {
			t.inReplace = !t.inReplace
		}
	case "enter", "down", "ctrl+n":
		if t.inReplace && msg.String() == "enter" {
			t.replaceOne()
			return
		}
		t.jumpMatch(t.cursorPos(), 1, false)
	case "shift+enter", "up", "ctrl+p":
		t.jumpMatch(t.cursorPos(), -1, false)
	case "ctrl+a":
		if t.prompt == promptReplace {
			t.replaceAll()
		}
	case "backspace", "ctrl+h":
		if r := []rune(*field); len(r) > 0 {
			*field = string(r[:len(r)-1])
			if !t.inReplace {
				t.jumpMatch(t.cursorPos(), 1, true)
			}
		}
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			// A pasted newline would make the query span lines, which a
			// per-line matcher can never find.
			*field += strings.NewReplacer("\r", "", "\n", "").Replace(string(msg.Runes))
			if !t.inReplace {
				t.jumpMatch(t.cursorPos(), 1, true)
			}
		}
	}
}

// findStatus is the status row while the find / replace bar is open.
func (t *TextEditor) findStatus() (field, hint string) {
	ms := t.findMatches()
	count := "no results"
	switch i := t.currentMatch(ms); {
	case len(ms) > 0 && i >= 0:
		count = fmt.Sprintf("%d of %d", i+1, len(ms))
	case len(ms) > 0:
		count = fmt.Sprintf("%d found", len(ms))
	case t.cur.findLast == "":
		count = ""
	}
	caret := func(on bool) string {
		if on {
			return "▏"
		}
		return ""
	}
	field = " find: " + t.cur.findLast + caret(!t.inReplace)
	if t.prompt == promptReplace {
		field += "   replace: " + t.replaceValue + caret(t.inReplace)
		hint = "  " + count + " · ⏎ replace · ctrl+a all · ↑↓ prev/next · ⇥ field · esc"
		return field, hint
	}
	return field, "  " + count + " · ⏎/↓ next · ↑ prev · ⇥ replace · esc"
}
