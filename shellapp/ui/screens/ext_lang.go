package screens

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hittable/shellapp/internal/extensions"
	"github.com/hittable/shellapp/internal/exthost"
	"github.com/hittable/shellapp/ui/components/texteditor"
)

// extEditorKey handles the language-feature keys while the code editor has
// the keyboard. It reports whether the key was used.
func (m *MainScreen) extEditorKey(key string) bool {
	if m.Ext == nil || m.TextEd.CompletionOpen() {
		return false
	}
	switch key {
	case "alt+h":
		m.extHover()
	case "f12":
		m.extLocations("definition")
	case "shift+f12":
		m.extLocations("references")
	case "f2":
		m.extRename()
	case "alt+.":
		m.extCodeActions()
	case "alt+F", "alt+shift+f", "Ï": // ⌥⇧F on a macOS keyboard composes Ï
		m.extFormat()
	case "f8":
		m.extNextProblem()
	default:
		if cmd, args, ok := m.extKeybinding(key); ok {
			var a []any
			if args != nil {
				a = []any{args}
			}
			m.extExecute(cmd, a)
			return true
		}
		return false
	}
	return true
}

// extCursor is the active file and cursor in the host's coordinates.
func (m *MainScreen) extCursor() (path string, pos map[string]int, ok bool) {
	if m.Ext.host == nil {
		m.extWhyStopped()
		return "", nil, false
	}
	if m.ActiveFile == "" || m.ViewMode != ViewText {
		m.StatusBar = "Open a file in the text editor first"
		return "", nil, false
	}
	m.extSyncActive()
	row, col := m.TextEd.Cursor()
	line := lineOf(m.TextEd.GetContent(), row)
	return m.ActiveFile, map[string]int{"line": row, "character": exthost.ToUTF16(line, col)}, true
}

// ---------------------------------------------------------------- completion

type extCompData struct {
	resultID, index int
	additional      []wireEdit
	command         string
}

func (m *MainScreen) extRequestCompletion(path string, row, col int, trigger string) {
	x := m.Ext
	if x.host == nil || x.docs[path] == nil {
		return
	}
	m.extSyncActive()
	x.compSeq++
	seq := x.compSeq
	line := lineOf(m.TextEd.GetContent(), row)
	pos := map[string]int{"line": row, "character": exthost.ToUTF16(line, col)}
	params := map[string]any{"path": path, "position": pos, "token": fmt.Sprintf("c%d", seq)}
	if trigger != "" {
		params["triggerCharacter"] = trigger
	}
	m.extGo("provide/completion", params, 8*time.Second, func(m *MainScreen, raw json.RawMessage, err error) {
		if err != nil || seq != m.Ext.compSeq {
			return
		}
		var res struct {
			ResultID   int  `json:"resultId"`
			Incomplete bool `json:"isIncomplete"`
			Items      []struct {
				Index                                           int
				Label, Detail, SortText, FilterText, InsertText string
				Snippet                                         bool
				Kind                                            int
				Range                                           *wireRange
				AdditionalTextEdits                             []wireEdit
				Command                                         *struct{ Command string }
			}
		}
		if json.Unmarshal(raw, &res) != nil {
			return
		}
		cur := lineOf(m.TextEd.GetContent(), row)
		items := make([]texteditor.ExtItem, 0, len(res.Items))
		for _, it := range res.Items {
			from, to := -1, -1
			if it.Range != nil && it.Range.Start.Line == row {
				from = exthost.FromUTF16(cur, it.Range.Start.Character)
				if it.Range.End.Line == row {
					to = exthost.FromUTF16(cur, it.Range.End.Character)
				}
			}
			data := extCompData{resultID: res.ResultID, index: it.Index}
			for _, e := range it.AdditionalTextEdits {
				e.Path = path
				data.additional = append(data.additional, e)
			}
			if it.Command != nil {
				data.command = it.Command.Command
			}
			items = append(items, texteditor.ExtItem{
				Label: it.Label, Detail: kindName(it.Kind, it.Detail), Insert: it.InsertText, Filter: it.FilterText,
				Sort: it.SortText, Snippet: it.Snippet, From: from, To: to, Data: data,
			})
		}
		m.TextEd.DeliverExtCompletions(path, row, items, trigger != "", res.Incomplete)
	})
}

// kindName shortens a completion's detail to fit the popup's right column.
func kindName(kind int, detail string) string {
	if detail != "" {
		if r := []rune(detail); len(r) > 18 {
			return string(r[:17]) + "…"
		}
		return detail
	}
	names := map[int]string{1: "method", 2: "function", 3: "constructor", 4: "field", 5: "variable", 6: "class",
		7: "interface", 8: "module", 9: "property", 11: "value", 12: "enum", 13: "keyword", 14: "snippet",
		16: "file", 18: "folder", 19: "enum member", 20: "constant", 21: "struct", 24: "type param"}
	return names[kind]
}

// extAcceptCompletion runs after an item is inserted: the item is resolved
// (servers often add auto-import edits only then), its additional edits are
// applied and its command runs.
func (m *MainScreen) extAcceptCompletion(path string, it texteditor.ExtItem) {
	data, ok := it.Data.(extCompData)
	if !ok {
		return
	}
	m.extSyncActive()
	m.extGo("completion/resolve", map[string]any{"resultId": data.resultID, "index": data.index}, 3*time.Second, func(m *MainScreen, raw json.RawMessage, err error) {
		edits := data.additional
		cmd := data.command
		if err == nil && len(raw) > 0 && string(raw) != "null" {
			var r struct {
				AdditionalTextEdits []wireEdit
				Command             *struct{ Command string }
			}
			if json.Unmarshal(raw, &r) == nil {
				if len(r.AdditionalTextEdits) > 0 {
					edits = nil
					for _, e := range r.AdditionalTextEdits {
						e.Path = path
						edits = append(edits, e)
					}
				}
				if r.Command != nil {
					cmd = r.Command.Command
				}
			}
		}
		if len(edits) > 0 {
			row, col := m.TextEd.Cursor()
			m.extApplyEdits(edits, nil)
			// The insertion was below these edits (imports at the top), so
			// keep the cursor on the same text.
			shift := 0
			for _, e := range edits {
				if e.Range.Start.Line < row {
					shift += strings.Count(e.NewText, "\n") - (e.Range.End.Line - e.Range.Start.Line)
				}
			}
			m.TextEd.MoveCursor(row+shift, col)
		}
		if cmd != "" && !strings.HasPrefix(cmd, "editor.action.trigger") {
			m.extExecute(cmd, nil)
		}
	})
}

// extSnippetItems are the snippets enabled extensions contribute for the
// file's language, cached per language.
func (m *MainScreen) extSnippetItems(path string) []texteditor.ExtItem {
	lang := extensions.LanguageID(path)
	if items, ok := m.Ext.snippets[lang]; ok {
		return items
	}
	var items []texteditor.ExtItem
	for _, s := range extensions.Snippets(lang) {
		items = append(items, texteditor.ExtItem{Label: s.Prefix, Detail: "snippet", Insert: s.Body, Snippet: true, From: -1, To: -1, Sort: "~" + s.Prefix})
	}
	m.Ext.snippets[lang] = items
	return items
}

// extTyped asks for completions after a trigger character, and for
// signature help after "(" or ",".
func (m *MainScreen) extTyped(path string, r rune) {
	x := m.Ext
	if x.host == nil || x.docs[path] == nil {
		return
	}
	ch := string(r)
	lang := extensions.LanguageID(path)
	if hasTrigger(x.triggers.Completion, lang, ch) {
		row, col := m.TextEd.Cursor()
		m.extRequestCompletion(path, row, col, ch)
	}
	if r == ')' {
		m.TextEd.Signature = ""
	}
	if hasTrigger(x.triggers.Signature, lang, ch) {
		m.extSignature(path, ch)
	}
}

func hasTrigger(t map[string][]string, lang, ch string) bool {
	for _, l := range []string{lang, "*"} {
		for _, c := range t[l] {
			if c == ch {
				return true
			}
		}
	}
	return false
}

func (m *MainScreen) extSignature(path, trigger string) {
	m.extSyncActive()
	row, col := m.TextEd.Cursor()
	line := lineOf(m.TextEd.GetContent(), row)
	m.extGo("provide/signatureHelp", map[string]any{"path": path, "position": map[string]int{"line": row, "character": exthost.ToUTF16(line, col)}, "triggerCharacter": trigger},
		4*time.Second, func(m *MainScreen, raw json.RawMessage, err error) {
			var r *struct{ Label, Parameter string }
			if err != nil || json.Unmarshal(raw, &r) != nil || r == nil {
				return
			}
			label := r.Label
			if r.Parameter != "" {
				label = strings.Replace(label, r.Parameter, "‹"+r.Parameter+"›", 1)
			}
			m.TextEd.Signature = "ƒ " + label
		})
}

// ---------------------------------------------------------------- hover, navigation

func (m *MainScreen) extHover() {
	path, pos, ok := m.extCursor()
	if !ok {
		return
	}
	m.extGo("provide/hover", map[string]any{"path": path, "position": pos}, 5*time.Second, func(m *MainScreen, raw json.RawMessage, err error) {
		var r *struct{ Markdown string }
		if err != nil {
			m.extNote("error", "hover: "+err.Error())
			return
		}
		if json.Unmarshal(raw, &r) != nil || r == nil || strings.TrimSpace(r.Markdown) == "" {
			m.StatusBar = "No hover information here"
			return
		}
		if m.ActiveFile == path {
			m.TextEd.ShowPopup(plainMarkdown(r.Markdown))
		}
	})
}

var mdLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// plainMarkdown makes hover markdown readable as plain terminal text.
func plainMarkdown(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	r := strings.NewReplacer("**", "", "__", "", "\\_", "_", "\\*", "*", "\\`", "`", "\\#", "#", "\\.", ".", "\\-", "-", "&nbsp;", " ", "<br>", "\n", "---", strings.Repeat("─", 24))
	return r.Replace(s)
}

type extLocation struct {
	Path       string
	Line, Char int // UTF-16 character
}

func (m *MainScreen) extLocations(kind string) {
	path, pos, ok := m.extCursor()
	if !ok {
		return
	}
	title := strings.ToUpper(kind[:1]) + kind[1:]
	wanted := m.extLoading(title, "asking the language server")
	m.extGo("provide/"+kind, map[string]any{"path": path, "position": pos}, 10*time.Second, func(m *MainScreen, raw json.RawMessage, err error) {
		if !wanted() {
			return
		}
		if err != nil {
			m.extLoaded()
			m.extNote("error", kind+": "+err.Error())
			return
		}
		var locs []struct {
			Path  string
			Range wireRange
		}
		_ = json.Unmarshal(raw, &locs)
		out := make([]extLocation, 0, len(locs))
		seen := map[string]bool{}
		for _, l := range locs {
			k := fmt.Sprintf("%s:%d:%d", l.Path, l.Range.Start.Line, l.Range.Start.Character)
			if !seen[k] {
				seen[k] = true
				out = append(out, extLocation{Path: l.Path, Line: l.Range.Start.Line, Char: l.Range.Start.Character})
			}
		}
		if len(out) == 0 {
			m.extLoaded()
			m.StatusBar = "No " + kind + " found"
			return
		}
		m.extPickLocation(title, out)
	})
}

// extPickLocation jumps straight to a single location, or lists several.
func (m *MainScreen) extPickLocation(title string, locs []extLocation) {
	if len(locs) == 1 {
		m.extLoaded()
		m.extJump(locs[0].Path, locs[0].Line, locs[0].Char)
		return
	}
	items := make([]paletteItem, len(locs))
	for i, l := range locs {
		rel, err := filepath.Rel(m.RootDir, l.Path)
		if err != nil || strings.HasPrefix(rel, "..") {
			rel = l.Path
		}
		items[i] = paletteItem{Label: fmt.Sprintf("%s:%d", rel, l.Line+1), Detail: strings.TrimSpace(lineOf(m.extDocText(l.Path), l.Line))}
	}
	m.extAsk(func() {
		m.showItems(paletteItems{Title: fmt.Sprintf("%s (%d)", title, len(locs)), Items: items,
			OnPick: func(i int, _ string, _ []int) {
				if i >= 0 {
					m.extJump(locs[i].Path, locs[i].Line, locs[i].Char)
				}
				m.extNextPrompt()
			},
			OnCancel: m.extNextPrompt,
		})
	})
}

// extJump opens path in the text editor at a line and UTF-16 column.
func (m *MainScreen) extJump(path string, line, char16 int) {
	m.openFileRaw(path)
	if m.ActiveFile != path {
		return
	}
	if m.ViewMode == ViewRunner {
		m.toggleViewMode()
	}
	m.Focus = FocusTextEditor
	m.focusCurrent()
	col := exthost.FromUTF16(lineOf(m.TextEd.GetContent(), line), char16)
	m.TextEd.MoveCursor(line, col)
	m.extSendActive()
}

func (m *MainScreen) extRename() {
	path, pos, ok := m.extCursor()
	if !ok {
		return
	}
	wanted := m.extLoading("Rename symbol", "checking what can be renamed")
	m.extGo("provide/prepareRename", map[string]any{"path": path, "position": pos}, 5*time.Second, func(m *MainScreen, raw json.RawMessage, err error) {
		if !wanted() {
			return
		}
		var p struct{ Placeholder, Error string }
		_ = json.Unmarshal(raw, &p)
		if err == nil && p.Error != "" {
			err = fmt.Errorf("%s", p.Error)
		}
		if err != nil {
			m.extLoaded()
			m.extNote("error", "rename: "+err.Error())
			return
		}
		m.extAsk(func() {
			m.showItems(paletteItems{Title: "Rename “" + p.Placeholder + "” to", Value: p.Placeholder, Input: true,
				OnPick: func(_ int, name string, _ []int) {
					name = strings.TrimSpace(name)
					if name != "" && name != p.Placeholder {
						m.extGo("provide/rename", map[string]any{"path": path, "position": pos, "newName": name}, 20*time.Second, func(m *MainScreen, raw json.RawMessage, err error) {
							var r struct {
								OK      bool
								Message string
							}
							_ = json.Unmarshal(raw, &r)
							switch {
							case err != nil:
								m.extNote("error", "rename: "+err.Error())
							case !r.OK:
								m.extNote("warning", "rename: "+r.Message)
							default:
								m.StatusBar = "Renamed to " + name
							}
						})
					}
					m.extNextPrompt()
				},
				OnCancel: m.extNextPrompt,
			})
		})
	})
}

func (m *MainScreen) extCodeActions() {
	path, pos, ok := m.extCursor()
	if !ok {
		return
	}
	rng := map[string]any{"start": pos, "end": pos}
	if m.TextEd.HasSelection() {
		if s, e, ok := m.TextEd.SelectionRange(); ok {
			text := m.TextEd.GetContent()
			rng = map[string]any{
				"start": map[string]int{"line": s[0], "character": exthost.ToUTF16(lineOf(text, s[0]), s[1])},
				"end":   map[string]int{"line": e[0], "character": exthost.ToUTF16(lineOf(text, e[0]), e[1])},
			}
		}
	}
	wanted := m.extLoading("Quick fix", "asking for fixes and refactorings")
	m.extGo("provide/codeActions", map[string]any{"path": path, "range": rng}, 8*time.Second, func(m *MainScreen, raw json.RawMessage, err error) {
		if !wanted() {
			return
		}
		var r struct {
			ResultID int `json:"resultId"`
			Items    []struct {
				Index       int
				Title, Kind string
				Preferred   bool
			}
		}
		if err != nil {
			m.extLoaded()
			m.extNote("error", "code actions: "+err.Error())
			return
		}
		_ = json.Unmarshal(raw, &r)
		if len(r.Items) == 0 {
			m.extLoaded()
			m.StatusBar = "No quick fixes or refactorings here"
			return
		}
		sort.SliceStable(r.Items, func(i, j int) bool { return r.Items[i].Preferred && !r.Items[j].Preferred })
		items := make([]paletteItem, len(r.Items))
		for i, a := range r.Items {
			d := a.Kind
			if a.Preferred {
				d = "★ " + d
			}
			items[i] = paletteItem{Label: a.Title, Detail: d}
		}
		m.extAsk(func() {
			m.showItems(paletteItems{Title: "Quick fix", Items: items,
				OnPick: func(i int, _ string, _ []int) {
					if i >= 0 {
						m.extGo("codeAction/apply", map[string]any{"resultId": r.ResultID, "index": r.Items[i].Index}, 30*time.Second, func(m *MainScreen, _ json.RawMessage, err error) {
							if err != nil {
								m.extNote("error", err.Error())
							}
						})
					}
					m.extNextPrompt()
				},
				OnCancel: m.extNextPrompt,
			})
		})
	})
}

func (m *MainScreen) extFormat() {
	path, _, ok := m.extCursor()
	if !ok {
		return
	}
	m.extGo("provide/formatting", map[string]any{"path": path, "tabSize": 2, "insertSpaces": true}, 15*time.Second, func(m *MainScreen, raw json.RawMessage, err error) {
		if err != nil {
			m.extNote("error", "format: "+err.Error())
			return
		}
		if string(raw) == "null" || len(raw) == 0 {
			m.StatusBar = "No installed extension formats " + extensions.LanguageID(path) + " files"
			return
		}
		var edits []wireEdit
		_ = json.Unmarshal(raw, &edits)
		if len(edits) == 0 {
			m.StatusBar = "Already formatted"
			return
		}
		for i := range edits {
			edits[i].Path = path
		}
		if m.extApplyEdits(edits, nil) {
			m.StatusBar = "Formatted"
		}
	})
}

func (m *MainScreen) extSymbols() {
	path, _, ok := m.extCursor()
	if !ok {
		return
	}
	wanted := m.extLoading("Go to symbol", "reading the file's symbols")
	m.extGo("provide/documentSymbols", map[string]any{"path": path}, 10*time.Second, func(m *MainScreen, raw json.RawMessage, err error) {
		if !wanted() {
			return
		}
		var syms []struct {
			Name, Detail string
			Kind, Depth  int
			Range        wireRange
		}
		if err != nil {
			m.extLoaded()
			m.extNote("error", err.Error())
			return
		}
		_ = json.Unmarshal(raw, &syms)
		if len(syms) == 0 {
			m.extLoaded()
			m.StatusBar = "No symbols in this file"
			return
		}
		items := make([]paletteItem, len(syms))
		for i, s := range syms {
			items[i] = paletteItem{Label: strings.Repeat("  ", s.Depth) + s.Name, Detail: fmt.Sprintf("%s · line %d", s.Detail, s.Range.Start.Line+1)}
		}
		m.extAsk(func() {
			m.showItems(paletteItems{Title: "Go to symbol", Items: items,
				OnPick: func(i int, _ string, _ []int) {
					if i >= 0 {
						m.extJump(path, syms[i].Range.Start.Line, syms[i].Range.Start.Character)
					}
					m.extNextPrompt()
				},
				OnCancel: m.extNextPrompt,
			})
		})
	})
}

func (m *MainScreen) cmdProblems() tea.Cmd {
	type row struct {
		d    extDiag
		path string
	}
	var rows []row
	for p, ds := range m.Ext.diags {
		for _, d := range ds {
			rows = append(rows, row{d, p})
		}
	}
	if len(rows) == 0 {
		m.StatusBar = "No problems"
		return nil
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.d.severity != b.d.severity {
			return a.d.severity < b.d.severity
		}
		if a.path != b.path {
			return a.path < b.path
		}
		return a.d.line < b.d.line
	})
	items := make([]paletteItem, len(rows))
	for i, r := range rows {
		rel, err := filepath.Rel(m.RootDir, r.path)
		if err != nil {
			rel = r.path
		}
		icon := map[int]string{0: "✖", 1: "⚠", 2: "ℹ", 3: "·"}[r.d.severity]
		items[i] = paletteItem{Label: icon + " " + r.d.message, Detail: fmt.Sprintf("%s:%d", rel, r.d.line+1)}
	}
	m.extAsk(func() {
		m.showItems(paletteItems{Title: fmt.Sprintf("Problems (%d)", len(rows)), Items: items,
			OnPick: func(i int, _ string, _ []int) {
				if i >= 0 {
					m.extJump(rows[i].path, rows[i].d.line, rows[i].d.start)
				}
				m.extNextPrompt()
			},
			OnCancel: m.extNextPrompt,
		})
	})
	return nil
}

// extNextProblem moves to the next diagnostic after the cursor in this
// file, wrapping to the first.
func (m *MainScreen) extNextProblem() {
	if m.ActiveFile == "" {
		return
	}
	ds := append([]extDiag(nil), m.Ext.diags[m.ActiveFile]...)
	if len(ds) == 0 {
		m.StatusBar = "No problems in this file"
		return
	}
	sort.Slice(ds, func(i, j int) bool {
		if ds[i].line != ds[j].line {
			return ds[i].line < ds[j].line
		}
		return ds[i].start < ds[j].start
	})
	row, col := m.TextEd.Cursor()
	c16 := exthost.ToUTF16(lineOf(m.TextEd.GetContent(), row), col)
	next := ds[0]
	for _, d := range ds {
		if d.line > row || (d.line == row && d.start > c16) {
			next = d
			break
		}
	}
	m.extJump(m.ActiveFile, next.line, next.start)
}
