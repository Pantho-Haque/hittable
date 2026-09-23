// Package gitpanel is the Git side of the app: a main-pane panel with
// Status (stage / unstage / discard / commit / push / pull / fetch), Commits
// (repo or file history with search), Branches (checkout / create / delete),
// Stashes (push / pop / drop) and Blame (per-line, with inline editor
// annotations). A detail pane under the list shows the diff / commit /
// stash / blame commit for the selected row.
package gitpanel

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"github.com/hittable/shellapp/internal/commitmsg"
	"github.com/hittable/shellapp/internal/gitx"
	"github.com/hittable/shellapp/ui/components/texteditor"
	"github.com/hittable/shellapp/ui/theme"
)

type Section int

const (
	SecStatus Section = iota
	SecCommits
	SecBranches
	SecStashes
	SecBlame
)

var SectionNames = []string{"Status", "Commits", "Branches", "Stashes", "Blame"}

type promptKind int

const (
	promptNone promptKind = iota
	promptBranch
	promptStash
	promptFilter
	promptConfirm
	promptType // conventional-commit type picker, shown before the compose editor
)

// row is one list entry; header rows are not actionable.
type row struct {
	text   string
	header bool
	group  int    // 0 staged, 1 changes, 2 conflicts (status section)
	action string // "+" stage, "−" unstage (files); "+ stage all" / "− unstage all" (headers)
	undo   string // "⟲" discard file / "⟲ undo all" header button (unstaged rows only)
	file   *gitx.FileStatus
	staged bool
	commit *gitx.Commit
	branch *gitx.Branch
	stash  *gitx.Stash
	blame  int // line index for the blame section
}

// DoneMsg is delivered when an async git operation finishes.
type DoneMsg struct {
	Label string
	Out   string
	Err   error
}

type Panel struct {
	// Spinner is the current animation frame, fed by the screen each frame so
	// a push or pull shows progress rather than a frozen label.
	Spinner string

	// Hover is the zone id the mouse is over, fed by the screen each frame.
	// HoverRow / HoverBtn track the list row and button under the mouse,
	// which are hit-tested by coordinate rather than by zone.
	Hover    string
	HoverRow int
	HoverBtn string

	Repo    *gitx.Repo
	Width   int // outer box width
	Height  int // outer box height
	Section Section

	// Active file (repo-relative) drives file history and blame.
	ActivePath  string
	FileHistory bool
	InlineBlame bool

	Status   *gitx.Status
	Commits  []gitx.Commit
	Branches []gitx.Branch
	Stashes  []gitx.Stash
	Blame    []gitx.BlameLine
	BlameSrc []string // file lines shown beside blame entries
	Filter   string

	rows           []row
	Cursor         int
	listScroll     int
	detail         []string
	detailCache    []string // detailLines() memo
	detailCacheKey string   // width/divider/split/wrap it was built for
	detailScroll   int
	collapsed      [2]bool // status groups

	// Merge state (merge / rebase / cherry-pick waiting on conflicts).
	Merge     bool
	MergeKind string

	// Conflict resolution mode for one file.
	Resolving      bool
	ResolvePath    string
	ResolveRel     string
	ResolveContent string
	Conflicts      []gitx.Conflict
	ConflictIdx    int

	// SplitPos is the width of the diff's old-side column (0 = centred),
	// moved by dragging the divider. Wrap controls whether long diff lines
	// continue on the next row or are clipped.
	SplitPos  int
	Wrap      bool
	dragSplit bool

	// Split renders diffs side by side; Editing edits the working file in
	// the detail pane.
	// IgnoreWS diffs with -w (whitespace-only changes hidden).
	IgnoreWS bool
	// Detail-pane text selection (line range), -1 = none.
	selAnchor, selEnd int

	Split     bool
	Editing   bool
	Editor    *texteditor.TextEditor
	EditPath  string
	editDirty bool

	// Composing edits the commit message in the detail pane, mirroring the
	// Editing submode above. The editor opens pre-filled with the heuristic
	// draft; a model streams over it when one is installed. draft survives
	// esc so pressing c again restores what was typed.
	Composing    bool
	MsgEditor    *texteditor.TextEditor
	draft        string
	draftEdited  bool
	composeType  string
	composeRules commitmsg.Rules
	digest       *commitmsg.Digest

	// Generation state. genSeq supersedes stale chunks the way the find
	// palette does; genBuf accumulates tokens off the UI goroutine and
	// genWake coalesces the redraws, as the terminal does for shell output.
	Generating bool
	genSeq     int
	genCancel  context.CancelFunc
	genMu      sync.Mutex
	genBuf     strings.Builder
	genWake    atomic.Bool
	genStart   time.Time
	userEdited bool

	prompt      promptKind
	promptText  string
	promptTitle string
	confirmFn   func()
	Message     string
	Busy        string

	// Hooks supplied by the screen.
	OnChanged     func()                                        // after any mutation (refresh decorations)
	OnOpenFile    func(abs string)                              // open a file in the editor
	OnGotoLine    func(line int)                                // jump the editor to a line (blame)
	OnBlameToggle func(on bool)                                 // show/hide inline blame in the editor
	Async         func(label string, fn func() (string, error)) // run in background, deliver DoneMsg
	SourceLines   func() []string                               // current editor lines (blame list)
	LoadFile      func(abs string) (string, error)              // working-copy content for edit mode
	SaveFile      func(abs, content string)                     // persist edits (through the document store)
	Send          func(tea.Msg)                                 // deliver an async message to the program
	Drafter       Drafter                                       // nil when no model is installed
}

// Drafter rewrites the heuristic draft into prose. It is an interface rather
// than a concrete client so the panel's tests can script it without HTTP, and
// so nothing here depends on the inference packages.
type Drafter interface {
	DraftStream(ctx context.Context, d *commitmsg.Digest, opts commitmsg.Options, onChunk func(string)) (commitmsg.Message, error)
}

// GenChunkMsg says the generation buffer moved. It carries no text: the buffer
// is read under the mutex when the message is handled, so a burst of tokens
// costs one redraw rather than one per token.
type GenChunkMsg struct{ Seq int }

// GenDoneMsg ends a generation, successfully or not.
//
// Source says where the final text actually came from. It is not cosmetic:
// unusable model output is replaced by the heuristic draft without an error,
// so Err alone cannot tell "the model wrote this" from "the model was
// discarded and you are looking at the mechanical draft". Saying which is the
// difference between a user trusting the label and ignoring it.
type GenDoneMsg struct {
	Seq    int
	Text   string
	Source commitmsg.Source
	Err    error
}

func New(repo *gitx.Repo) *Panel {
	return &Panel{Repo: repo, Width: 80, Height: 24, selAnchor: -1, selEnd: -1, HoverRow: -1, Wrap: true,
		composeRules: commitmsg.DefaultRules()}
}

func (p *Panel) SetSize(w, h int) { p.Width, p.Height = w, h; p.clamp() }

// SetActiveFile updates the file used by file history and blame.
func (p *Panel) SetActiveFile(abs string) {
	rel := ""
	if abs != "" && p.Repo != nil {
		rel = p.Repo.Rel(abs)
	}
	if rel == p.ActivePath {
		return
	}
	p.ActivePath = rel
	if p.Section == SecBlame || (p.Section == SecCommits && p.FileHistory) {
		p.Refresh()
	}
}

// ---------- data ----------

// Refresh reloads the current section (and the header status).
func (p *Panel) Refresh() {
	if p.Repo == nil {
		return
	}
	st, err := p.Repo.Status()
	if err != nil {
		p.Message = err.Error()
	} else {
		p.Status = st
	}
	p.Merge, p.MergeKind = p.Repo.MergeInProgress()
	switch p.Section {
	case SecCommits:
		path := ""
		if p.FileHistory {
			path = p.ActivePath
		}
		p.Commits, err = p.Repo.Log(300, path)
	case SecBranches:
		p.Branches, err = p.Repo.Branches()
	case SecStashes:
		p.Stashes, err = p.Repo.Stashes()
	case SecBlame:
		p.Blame, p.BlameSrc = nil, nil
		if p.ActivePath != "" {
			p.Blame, err = p.Repo.Blame(p.ActivePath)
			if p.SourceLines != nil {
				p.BlameSrc = p.SourceLines()
			}
		}
	}
	if err != nil {
		p.Message = err.Error()
	}
	p.buildRows()
	p.skipHeader()
	p.loadDetail()
}

// SetBlameSource supplies the editor's current lines for the blame list.
func (p *Panel) SetBlameSource(lines []string) { p.BlameSrc = lines; p.buildRows() }

func (p *Panel) buildRows() {
	p.rows = p.rows[:0]
	switch p.Section {
	case SecStatus:
		if p.Status == nil {
			break
		}
		var staged, changed, conflicts []gitx.FileStatus
		for _, f := range p.Status.Files {
			if p.Filter != "" && !strings.Contains(strings.ToLower(f.Path), strings.ToLower(p.Filter)) {
				continue
			}
			if f.Conflict() {
				conflicts = append(conflicts, f)
				continue
			}
			if f.Staged() && !f.Untracked() {
				staged = append(staged, f)
			}
			if f.Unstaged() || f.Untracked() {
				changed = append(changed, f)
			}
		}
		if p.Merge {
			h := row{text: fmt.Sprintf("⚠ %s in progress · %d conflict(s)", p.MergeKind, len(conflicts)), header: true, group: 2}
			if len(conflicts) == 0 {
				h.action = "✓ commit " + p.MergeKind
			} else {
				h.action = "✕ abort " + p.MergeKind
			}
			p.rows = append(p.rows, h)
			for i := range conflicts {
				p.rows = append(p.rows, row{file: &conflicts[i], group: 2, action: "resolve"})
			}
		}
		groups := []struct {
			title  string
			files  []gitx.FileStatus
			staged bool
			action string
			row    string
		}{
			{"Staged Changes", staged, true, "− unstage all", "−"},
			{"Changes", changed, false, "+ stage all", "+"},
		}
		for gi, g := range groups {
			arrow := "▾"
			if p.collapsed[gi] {
				arrow = "▸"
			}
			h := row{text: fmt.Sprintf("%s %s (%d)", arrow, g.title, len(g.files)), header: true, group: gi}
			if len(g.files) > 0 {
				h.action = g.action
				if !g.staged {
					h.undo = "⟲ undo all"
				}
			}
			p.rows = append(p.rows, h)
			if p.collapsed[gi] {
				continue
			}
			for i := range g.files {
				r := row{file: &g.files[i], staged: g.staged, group: gi, action: g.row}
				if !g.staged {
					r.undo = "⟲"
				}
				p.rows = append(p.rows, r)
			}
		}
	case SecCommits:
		for i := range p.Commits {
			c := &p.Commits[i]
			if p.Filter != "" {
				q := strings.ToLower(p.Filter)
				if !strings.Contains(strings.ToLower(c.Subject), q) && !strings.Contains(strings.ToLower(c.Author), q) && !strings.HasPrefix(c.Hash, q) {
					continue
				}
			}
			p.rows = append(p.rows, row{commit: c})
		}
		if len(p.rows) == 0 {
			p.rows = append(p.rows, row{text: "No commits", header: true})
		}
	case SecBranches:
		for i := range p.Branches {
			p.rows = append(p.rows, row{branch: &p.Branches[i]})
		}
	case SecStashes:
		for i := range p.Stashes {
			p.rows = append(p.rows, row{stash: &p.Stashes[i]})
		}
		if len(p.rows) == 0 {
			p.rows = append(p.rows, row{text: "No stashes", header: true})
		}
	case SecBlame:
		if p.ActivePath == "" {
			p.rows = append(p.rows, row{text: "Open a file to see its blame", header: true})
		}
		for i := range p.Blame {
			p.rows = append(p.rows, row{blame: i})
		}
	}
	p.clamp()
}

func (p *Panel) current() *row {
	if p.Cursor >= 0 && p.Cursor < len(p.rows) {
		return &p.rows[p.Cursor]
	}
	return nil
}

func (p *Panel) loadDetail() {
	if p.Editing {
		p.stopEdit()
	}
	p.detail, p.detailCache = nil, nil
	p.detailScroll = 0
	p.selAnchor, p.selEnd = -1, -1
	r := p.current()
	if r == nil || p.Repo == nil {
		return
	}
	var text string
	switch {
	case r.file != nil && r.file.Conflict():
		b, _ := os.ReadFile(p.Repo.Abs(r.file.Path))
		n := len(gitx.ParseConflicts(string(b)))
		text = fmt.Sprintf("%d conflict block(s) — press ⏎ or [ resolve ] to resolve here, o to open in the editor", n)
	case r.file != nil:
		text = p.Repo.Diff(*r.file, r.staged, p.IgnoreWS)
		if strings.TrimSpace(text) == "" {
			text = "(no diff)"
			if p.IgnoreWS {
				text = "(no diff — only whitespace changed; press w to show it)"
			}
		}
	case r.commit != nil:
		text = p.Repo.Show(r.commit.Hash)
	case r.branch != nil:
		text, _ = p.Repo.Run("log", "-n", "30", "--date=relative", "--format=%h  %ad  %an: %s", r.branch.Name)
	case r.stash != nil:
		text = p.Repo.StashShow(r.stash.Ref)
	case p.Section == SecBlame && r.blame < len(p.Blame):
		b := p.Blame[r.blame]
		if b.Uncommitted {
			text = "Uncommitted changes"
		} else {
			text = p.Repo.Show(b.Hash)
		}
	}
	p.detail, p.detailCache = strings.Split(strings.TrimRight(text, "\n"), "\n"), nil
}

// detailWidth is the usable width of the detail pane (the scrollbar column is
// always reserved, so the content never shifts when one appears).
func (p *Panel) detailWidth() int {
	return max(p.Width-3, 1)
}

// detailLines renders the detail pane for the current width: side by side when
// split, otherwise the unified diff. Either way long lines wrap rather than
// being cut off at the border. Memoised, and recomputed on resize or a mode
// flip, so a stale width can never survive a SetSize.
func (p *Panel) detailLines() []string {
	w, half := p.detailWidth(), p.splitHalf()
	key := fmt.Sprintf("%d/%d/%v/%v", w, half, p.Split, p.Wrap)
	if p.detailCache != nil && p.detailCacheKey == key {
		return p.detailCache
	}
	out := []string{}
	if p.Split {
		out = append(out, splitDiff(p.detail, w, half, p.Wrap)...)
	} else {
		for _, l := range p.detail {
			chunks := fullWidth(diffLine(showWhitespace(l)), w, p.Wrap)
			if st, tinted := diffRowStyle(l); tinted {
				// Fill the rest of the row so the tint reaches the edge.
				for i, c := range chunks {
					if n := w - lipgloss.Width(c); n > 0 {
						chunks[i] = c + st.Render(strings.Repeat(" ", n))
					}
				}
			}
			out = append(out, chunks...)
		}
	}
	p.detailCache, p.detailCacheKey = out, key
	return out
}

// EndDrag releases the split divider. The screen calls it on mouse-up, which
// never reaches HandleMouse.
func (p *Panel) EndDrag() { p.dragSplit = false }

// ToggleWrap flips between wrapping long diff lines and clipping them.
func (p *Panel) ToggleWrap() {
	p.Wrap = !p.Wrap
	p.detailScroll = 0
	p.clamp()
}

// ResetSplit re-centres the divider.
func (p *Panel) ResetSplit() { p.SplitPos = 0 }

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// splitHalf is the old-side column width, clamped so both sides stay usable.
func (p *Panel) splitHalf() int {
	w := p.detailWidth()
	half := (w - 1) / 2
	if p.SplitPos > 0 {
		half = p.SplitPos
	}
	if lo, hi := 8, w-9; hi >= lo {
		half = min(max(half, lo), hi)
	}
	return half
}

// dividerX is the panel-relative column of the split divider (1 = inside the
// left border).
func (p *Panel) dividerX() int { return 1 + p.splitHalf() }

// detailTop is the first panel-relative row of the detail pane.
func (p *Panel) detailTop() int { return listTop + p.listRows() + 1 }

// ---------- edit in preview ----------

// canEdit reports whether the selected row is a working-copy file that is
// not staged and not in conflict.
func (p *Panel) canEdit() (*gitx.FileStatus, bool) {
	r := p.current()
	if r == nil || r.file == nil || r.staged || r.file.Conflict() || r.file.Index == 'D' || r.file.Worktree == 'D' {
		return nil, false
	}
	return r.file, true
}

func (p *Panel) startEdit() {
	f, ok := p.canEdit()
	if !ok {
		p.Message = "only unstaged, non-conflicting files can be edited here"
		return
	}
	abs := p.Repo.Abs(f.Path)
	var content string
	var err error
	if p.LoadFile != nil {
		content, err = p.LoadFile(abs)
	} else {
		var b []byte
		b, err = os.ReadFile(abs)
		content = string(b)
	}
	if err != nil {
		p.Message = err.Error()
		return
	}
	ed := texteditor.New()
	ed.SetContent(abs, content)
	ed.SetSize(p.Width-4, p.detailRows()-2)
	ed.Focus()
	ed.OnChanged = func(string) { p.editDirty = true }
	p.Editor, p.EditPath, p.Editing, p.editDirty = ed, abs, true, false
}

func (p *Panel) saveEdit() {
	if !p.Editing || !p.editDirty {
		return
	}
	content := p.Editor.GetContent()
	if p.SaveFile != nil {
		p.SaveFile(p.EditPath, content)
	} else {
		_ = os.WriteFile(p.EditPath, []byte(content), 0o644)
	}
	p.editDirty = false
}

func (p *Panel) stopEdit() {
	if !p.Editing {
		return
	}
	p.saveEdit()
	p.Editing, p.Editor = false, nil
}

// ---------- layout ----------

func (p *Panel) inner() int { return p.Height - 2 }

func (p *Panel) listRows() int {
	n := (p.inner() - 3) * 2 / 5
	if n < 3 {
		n = 3
	}
	return n
}

func (p *Panel) detailRows() int {
	n := p.inner() - 3 - p.listRows()
	if n < 1 {
		n = 1
	}
	return n
}

func (p *Panel) clamp() {
	if len(p.rows) == 0 {
		p.Cursor, p.listScroll = 0, 0
		return
	}
	if p.Cursor < 0 {
		p.Cursor = 0
	}
	if p.Cursor >= len(p.rows) {
		p.Cursor = len(p.rows) - 1
	}
	if p.Cursor < p.listScroll {
		p.listScroll = p.Cursor
	}
	if p.Cursor >= p.listScroll+p.listRows() {
		p.listScroll = p.Cursor - p.listRows() + 1
	}
	if max := len(p.detailLines()) - p.detailRows(); p.detailScroll > max {
		p.detailScroll = max
	}
	if p.detailScroll < 0 {
		p.detailScroll = 0
	}
}

// move steps the cursor, skipping header rows.
func (p *Panel) move(d int) {
	if len(p.rows) == 0 {
		return
	}
	step := 1
	if d < 0 {
		step = -1
	}
	target := p.Cursor + d
	if target < 0 {
		target = 0
	}
	if target >= len(p.rows) {
		target = len(p.rows) - 1
	}
	// Skip headers that carry no action; ones with a button (stage all,
	// commit merge…) stay reachable from the keyboard.
	for target >= 0 && target < len(p.rows) && p.rows[target].header && p.rows[target].action == "" {
		target += step
	}
	if target >= 0 && target < len(p.rows) {
		p.Cursor = target
	}
	p.clamp()
	p.loadDetail()
}

// skipHeader nudges the cursor off a header row onto the next entry.
func (p *Panel) skipHeader() {
	for p.Cursor < len(p.rows)-1 && p.rows[p.Cursor].header {
		p.Cursor++
	}
}

// ---------- actions ----------

func (p *Panel) setSection(s Section) {
	p.Section = s
	p.Cursor, p.listScroll, p.Filter = 0, 0, ""
	p.Refresh()
}

func (p *Panel) changed() {
	p.Refresh()
	if p.OnChanged != nil {
		p.OnChanged()
	}
}

func (p *Panel) run(label string, fn func() error) {
	if err := fn(); err != nil {
		p.Message = label + ": " + err.Error()
		return
	}
	p.Message = label + " ✓"
	p.changed()
}

func (p *Panel) confirm(title string, fn func()) {
	p.prompt = promptConfirm
	p.promptTitle = title + " (y/n)"
	p.confirmFn = fn
}

// PromptOpen reports whether an inline prompt / confirm / picker is active, or
// the message editor has the keyboard. The screen checks this before treating
// esc as "close the panel".
func (p *Panel) PromptOpen() bool { return p.prompt != promptNone || p.Composing }

// HandleKey processes a key while the panel has focus. Returns false when the
// key is Esc with nothing to dismiss (the screen closes the panel).
func (p *Panel) HandleKey(msg tea.KeyMsg) bool {
	p.Message = ""
	if p.prompt != promptNone {
		p.handlePromptKey(msg)
		return true
	}
	if p.Resolving {
		p.handleResolveKey(msg)
		return true
	}
	if p.Composing {
		p.handleComposeKey(msg)
		return true
	}
	if p.Editing {
		switch msg.String() {
		case "esc":
			p.stopEdit()
			p.Refresh()
		case "ctrl+s":
			p.saveEdit()
			p.Message = "saved"
		default:
			p.Editor.Update(msg)
			p.saveEdit() // autosave through the document store (debounced write)
		}
		return true
	}
	if p.Repo == nil {
		return msg.String() != "esc"
	}
	switch msg.String() {
	case "esc":
		if p.Filter != "" {
			p.Filter = ""
			p.buildRows()
			p.loadDetail()
			return true
		}
		return false
	case "tab", "right", "l":
		p.setSection((p.Section + 1) % Section(len(SectionNames)))
	case "shift+tab", "left", "h":
		p.setSection((p.Section + Section(len(SectionNames)) - 1) % Section(len(SectionNames)))
	case "1", "2", "3", "4", "5":
		p.setSection(Section(msg.String()[0] - '1'))
	case "up", "k":
		p.move(-1)
	case "down", "j":
		p.move(1)
	case "pgup", "ctrl+u":
		p.move(-(p.listRows() - 1))
	case "pgdown", "ctrl+d":
		p.move(p.listRows() - 1)
	case "g", "home":
		p.move(-len(p.rows))
	case "G", "end":
		p.move(len(p.rows))
	case "J", "ctrl+n":
		p.detailScroll++
		p.clamp()
	case "K", "ctrl+p":
		p.detailScroll--
		p.clamp()
	case " ":
		p.detailScroll += p.detailRows() - 1
		p.clamp()
	case "r":
		p.Refresh()
		p.Message = "refreshed"
	case "/":
		p.prompt, p.promptTitle, p.promptText = promptFilter, "filter", p.Filter
	case "enter":
		p.primary()
	case "v":
		p.Split = !p.Split
		p.loadDetail()
	case "w":
		p.IgnoreWS = !p.IgnoreWS
		p.loadDetail()
	case "z", "alt+z", "Ω": // Option+z arrives as Ω on macOS
		p.ToggleWrap()
	case "ctrl+c":
		p.CopySelection()
	case "e":
		if p.Section == SecStatus {
			p.startEdit()
		}
	default:
		p.sectionKey(msg.String())
	}
	return true
}

// toggleGroup collapses / expands a status group.
func (p *Panel) toggleGroup(g int) {
	p.collapsed[g] = !p.collapsed[g]
	p.buildRows()
	p.loadDetail()
}

// rowAction performs the +/− action of a row (file or group header).
func (p *Panel) rowAction(r *row) {
	if r == nil || r.action == "" {
		return
	}
	switch {
	case r.header && r.group == 2:
		kind := p.MergeKind
		if strings.HasPrefix(r.action, "✓") {
			p.run("commit "+kind, func() error { return p.Repo.MergeContinue(kind) })
		} else {
			p.confirm("abort "+kind, func() { p.run("abort "+kind, func() error { return p.Repo.MergeAbort(kind) }) })
		}
	case r.file != nil && r.file.Conflict():
		p.startResolve(*r.file)
	case r.header && r.group == 0:
		p.run("unstage all", func() error { return p.Repo.Unstage(".") })
	case r.header:
		p.run("stage all", func() error { return p.Repo.Stage(".") })
	case r.file != nil && r.staged:
		f := r.file.Path
		p.run("unstage "+f, func() error { return p.Repo.Unstage(f) })
	case r.file != nil:
		f := r.file.Path
		p.run("stage "+f, func() error { return p.Repo.Stage(f) })
	}
}

// rowUndo discards the working-tree changes of a row (one file, or the whole
// unstaged group for a header). Always confirmed — it destroys work.
func (p *Panel) rowUndo(r *row) {
	if r == nil || r.undo == "" {
		return
	}
	if r.header {
		p.confirm("undo all changes", func() { p.run("undo all", p.Repo.DiscardAll) })
		return
	}
	f := *r.file
	p.confirm("discard changes in "+f.Path, func() { p.run("discard "+f.Path, func() error { return p.Repo.Discard(f) }) })
}

func (p *Panel) primary() {
	r := p.current()
	if r == nil {
		return
	}
	switch {
	case r.header && p.Section == SecStatus && r.group == 2:
		p.rowAction(r)
	case r.header && p.Section == SecStatus:
		p.toggleGroup(r.group)
	case r.file != nil && r.file.Conflict():
		p.startResolve(*r.file)
	case r.file != nil && p.OnOpenFile != nil:
		p.OnOpenFile(p.Repo.Abs(r.file.Path))
	case r.branch != nil && !r.branch.Current:
		p.run("checkout "+r.branch.Name, func() error { return p.Repo.Checkout(r.branch.Name) })
	case r.stash != nil:
		ref := r.stash.Ref
		p.confirm("pop "+ref, func() { p.run("pop "+ref, func() error { return p.Repo.StashPop(ref) }) })
	case p.Section == SecBlame && p.OnGotoLine != nil:
		p.OnGotoLine(r.blame)
	}
}

func (p *Panel) sectionKey(key string) {
	r := p.current()
	switch p.Section {
	case SecStatus:
		switch key {
		case "s", "u", "+", "-":
			p.rowAction(r)
		case "a":
			p.run("stage all", func() error { return p.Repo.Stage(".") })
		case "A":
			p.run("unstage all", func() error { return p.Repo.Unstage(".") })
		case "d", "x":
			if r != nil && r.file != nil {
				f := *r.file
				p.confirm("discard changes in "+f.Path, func() { p.run("discard "+f.Path, func() error { return p.Repo.Discard(f) }) })
			}
		case "D":
			p.confirm("undo all changes", func() { p.run("undo all", p.Repo.DiscardAll) })
		case "c":
			p.startCompose()
		case "S":
			p.prompt, p.promptTitle, p.promptText = promptStash, "stash message (optional)", ""
		case "p":
			p.async("push", p.Repo.Push)
		case "P":
			p.async("pull", p.Repo.Pull)
		case "f":
			p.async("fetch", p.Repo.Fetch)
		case "y":
			p.SyncAction()
		}
	case SecCommits:
		switch key {
		case "f":
			p.FileHistory = !p.FileHistory
			p.Refresh()
		case "y":
			if r != nil && r.commit != nil {
				p.Message = "hash " + r.commit.Hash
			}
		}
	case SecBranches:
		switch key {
		case "n", "c":
			p.prompt, p.promptTitle, p.promptText = promptBranch, "new branch name", ""
		case "d", "x":
			if r != nil && r.branch != nil && !r.branch.Current {
				name := r.branch.Name
				p.confirm("delete branch "+name, func() { p.run("delete "+name, func() error { return p.Repo.DeleteBranch(name) }) })
			}
		case "f":
			p.async("fetch", p.Repo.Fetch)
		}
	case SecStashes:
		switch key {
		case "s", "n":
			p.prompt, p.promptTitle, p.promptText = promptStash, "stash message (optional)", ""
		case "d", "x":
			if r != nil && r.stash != nil {
				ref := r.stash.Ref
				p.confirm("drop "+ref, func() { p.run("drop "+ref, func() error { return p.Repo.StashDrop(ref) }) })
			}
		}
	case SecBlame:
		switch key {
		case "b":
			p.InlineBlame = !p.InlineBlame
			if p.OnBlameToggle != nil {
				p.OnBlameToggle(p.InlineBlame)
			}
		}
	}
}

func (p *Panel) async(label string, fn func() (string, error)) {
	if p.Async == nil {
		out, err := fn()
		p.Done(DoneMsg{label, out, err})
		return
	}
	p.Busy = label + "…"
	p.Async(label, fn)
}

// Done handles the result of an async operation.
func (p *Panel) Done(msg DoneMsg) {
	p.Busy = ""
	if msg.Err != nil {
		p.Message = msg.Label + ": " + msg.Err.Error()
	} else {
		last := strings.TrimSpace(msg.Out)
		if i := strings.LastIndex(last, "\n"); i >= 0 {
			last = last[i+1:]
		}
		p.Message = msg.Label + " ✓ " + last
	}
	p.changed()
}

func (p *Panel) handlePromptKey(msg tea.KeyMsg) {
	if p.prompt == promptType {
		p.handleTypeKey(msg)
		return
	}
	switch msg.String() {
	case "esc":
		p.prompt = promptNone
		p.confirmFn = nil
	case "enter":
		kind, text := p.prompt, strings.TrimSpace(p.promptText)
		p.prompt = promptNone
		switch kind {
		case promptBranch:
			if text != "" {
				p.run("branch "+text, func() error { return p.Repo.CreateBranch(text) })
			}
		case promptStash:
			p.run("stash", func() error { return p.Repo.StashPush(text) })
		case promptFilter:
			p.Filter = text
			p.Cursor = 0
			p.buildRows()
			p.loadDetail()
		case promptConfirm:
			// enter = yes
			if p.confirmFn != nil {
				p.confirmFn()
			}
			p.confirmFn = nil
		}
	case "backspace":
		if r := []rune(p.promptText); len(r) > 0 {
			p.promptText = string(r[:len(r)-1])
		}
	default:
		if p.prompt == promptConfirm {
			if msg.String() == "y" || msg.String() == "Y" {
				fn := p.confirmFn
				p.prompt, p.confirmFn = promptNone, nil
				if fn != nil {
					fn()
				}
			} else if msg.String() == "n" || msg.String() == "N" {
				p.prompt, p.confirmFn = promptNone, nil
			}
			return
		}
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			p.promptText += string(msg.Runes)
		}
	}
}

const listTop = 2 // border + tabs row

// rowAtY maps a panel-relative row to a list index, or -1 if it is not a row.
func (p *Panel) rowAtY(y int) int {
	if y -= listTop; y < 0 || y >= p.listRows() {
		return -1
	}
	if i := p.listScroll + y; i < len(p.rows) {
		return i
	}
	return -1
}

// rowHit names the part of a row the cursor is on: "action" for the
// stage/unstage button, "undo" for the discard button, "" for the row body.
// Click and hover share it so they can never disagree about where a button is.
func (p *Panel) rowHit(r *row, x int) string {
	aLeft := p.Width - 2 - lipgloss.Width(r.action) - 3
	switch {
	case r.action != "" && x >= aLeft:
		return "action"
	case r.undo != "" && x >= aLeft-lipgloss.Width(r.undo)-4:
		return "undo"
	}
	return ""
}

// HandleMouse takes coordinates relative to the panel's top-left corner.
func (p *Panel) HandleMouse(msg tea.MouseMsg) {
	switch msg.Type {
	case tea.MouseMotion:
		p.HoverRow, p.HoverBtn = p.rowAtY(msg.Y), ""
		if p.HoverRow >= 0 {
			p.HoverBtn = p.rowHit(&p.rows[p.HoverRow], msg.X)
		}
	case tea.MouseLeft:
		// Drag the split divider to rebalance the two diff columns.
		if p.Split && !p.Resolving && !p.Editing && !p.Composing && msg.Y >= p.detailTop() {
			if msg.Action == tea.MouseActionPress && abs(msg.X-p.dividerX()) <= 1 {
				p.dragSplit = true
				return
			}
			if p.dragSplit && msg.Action == tea.MouseActionMotion {
				p.SplitPos = msg.X - 1
				return
			}
		}
		if msg.Action == tea.MouseActionPress {
			p.dragSplit = false
		}
		// Text selection in the diff / commit pane: press + drag over rows.
		if !p.Resolving && !p.Editing && !p.Composing && msg.Y >= p.detailTop() {
			line := p.detailScroll + msg.Y - p.detailTop()
			if line >= len(p.detailLines()) {
				line = len(p.detailLines()) - 1
			}
			if line < 0 {
				return
			}
			if msg.Action == tea.MouseActionPress {
				p.selAnchor, p.selEnd = line, line
			} else if msg.Action == tea.MouseActionMotion && p.selAnchor >= 0 {
				p.selEnd = line
			}
			return
		}
		if p.Resolving && msg.Action == tea.MouseActionPress && msg.Y >= listTop+p.listRows()+1 {
			// Map the clicked row back to a conflict block (headers add rows).
			line := p.detailScroll + msg.Y - (listTop + p.listRows() + 1)
			rl := 0
			for i, c := range p.Conflicts {
				rl++ // header row
				if line >= c.Start+i+1 && line <= c.End+i+1 {
					p.ConflictIdx = i
					return
				}
			}
			_ = rl
			return
		}
		if ed := p.detailEditor(); ed != nil && msg.Y >= listTop+p.listRows()+1 {
			rel := msg
			rel.X -= 2
			rel.Y -= listTop + p.listRows() + 2
			ed.Update(rel)
			return
		}
		if msg.Action != tea.MouseActionPress {
			return
		}
		if i := p.rowAtY(msg.Y); i >= 0 {
			{
				r := &p.rows[i]
				switch p.rowHit(r, msg.X) {
				case "action":
					p.Cursor = i
					p.rowAction(r)
					return
				case "undo":
					p.Cursor = i
					p.rowUndo(r)
					return
				}
				if i == p.Cursor {
					p.primary()
				} else {
					p.Cursor = i
					p.clamp()
					p.loadDetail()
				}
			}
		}
	case tea.MouseWheelUp, tea.MouseWheelDown:
		d := 3
		if msg.Type == tea.MouseWheelUp {
			d = -3
		}
		if ed := p.detailEditor(); ed != nil && msg.Y >= listTop+p.listRows()+1 {
			rel := msg
			rel.X -= 2
			rel.Y -= listTop + p.listRows() + 2
			ed.Update(rel)
			return
		}
		if y := msg.Y - listTop; y >= 0 && y < p.listRows() {
			p.listScroll += d
			if p.listScroll < 0 {
				p.listScroll = 0
			}
			if max := len(p.rows) - p.listRows(); p.listScroll > max && max >= 0 {
				p.listScroll = max
			}
		} else {
			p.detailScroll += d
			p.clamp()
		}
	}
}

// ---------- merge conflict resolution ----------

func (p *Panel) startResolve(f gitx.FileStatus) {
	abs := p.Repo.Abs(f.Path)
	var content string
	var err error
	if p.LoadFile != nil {
		content, err = p.LoadFile(abs)
	} else {
		var b []byte
		b, err = os.ReadFile(abs)
		content = string(b)
	}
	if err != nil {
		p.Message = err.Error()
		return
	}
	p.Resolving, p.ResolvePath, p.ResolveRel = true, abs, f.Path
	p.ResolveContent = content
	p.Conflicts = gitx.ParseConflicts(content)
	p.ConflictIdx = 0
	p.detailScroll = 0
	p.scrollToConflict()
}

func (p *Panel) stopResolve() {
	p.Resolving = false
	p.Conflicts = nil
	p.Refresh()
}

func (p *Panel) persistResolve() {
	if p.SaveFile != nil {
		p.SaveFile(p.ResolvePath, p.ResolveContent)
	} else {
		_ = os.WriteFile(p.ResolvePath, []byte(p.ResolveContent), 0o644)
	}
}

func (p *Panel) resolveCurrent(choice string) {
	if len(p.Conflicts) == 0 {
		return
	}
	p.ResolveContent = gitx.Resolve(p.ResolveContent, p.ConflictIdx, choice)
	p.Conflicts = gitx.ParseConflicts(p.ResolveContent)
	if p.ConflictIdx >= len(p.Conflicts) {
		p.ConflictIdx = len(p.Conflicts) - 1
	}
	if p.ConflictIdx < 0 {
		p.ConflictIdx = 0
	}
	p.persistResolve()
	p.Message = fmt.Sprintf("accepted %s · %d left", choice, len(p.Conflicts))
	p.scrollToConflict()
}

func (p *Panel) scrollToConflict() {
	if p.ConflictIdx < len(p.Conflicts) {
		p.detailScroll = p.Conflicts[p.ConflictIdx].Start - 2
		if p.detailScroll < 0 {
			p.detailScroll = 0
		}
	}
}

func (p *Panel) handleResolveKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc":
		p.stopResolve()
	case "c", "1":
		p.resolveCurrent("ours")
	case "i", "2":
		p.resolveCurrent("theirs")
	case "b", "3":
		p.resolveCurrent("both")
	case "n", "down", "j":
		if p.ConflictIdx < len(p.Conflicts)-1 {
			p.ConflictIdx++
		}
		p.scrollToConflict()
	case "p", "up", "k":
		if p.ConflictIdx > 0 {
			p.ConflictIdx--
		}
		p.scrollToConflict()
	case "J":
		p.detailScroll++
	case "K":
		if p.detailScroll > 0 {
			p.detailScroll--
		}
	case "o":
		if p.OnOpenFile != nil {
			abs := p.ResolvePath
			p.stopResolve()
			p.OnOpenFile(abs)
		}
	case "a", "enter":
		if len(p.Conflicts) > 0 {
			p.Message = fmt.Sprintf("%d conflict(s) still unresolved", len(p.Conflicts))
			return
		}
		rel := p.ResolveRel
		p.persistResolve()
		p.Resolving, p.Conflicts = false, nil
		p.run("mark resolved "+rel, func() error { return p.Repo.Stage(rel) })
	}
}

// resolveLines renders the file with conflict blocks highlighted and a
// CodeLens-style action line above the current block.
func (p *Panel) resolveLines(width int) []string {
	lines := strings.Split(p.ResolveContent, "\n")
	inBlock := map[int]int{} // line -> conflict index
	kind := map[int]string{}
	for i, c := range p.Conflicts {
		for l := c.Start; l <= c.End; l++ {
			inBlock[l] = i
			switch {
			case l == c.Start || l == c.Mid || l == c.End:
				kind[l] = "marker"
			case l < c.Mid:
				kind[l] = "ours"
			default:
				kind[l] = "theirs"
			}
		}
	}
	var out []string
	for i, l := range lines {
		ci, ok := inBlock[i]
		if ok && i == p.Conflicts[ci].Start {
			lens := "   ↳ accept: c current · i incoming · b both · n/p next/prev · a mark resolved"
			if ci == p.ConflictIdx {
				out = append(out, ansi.Truncate(theme.PromptStyle.Render(fmt.Sprintf(" conflict %d/%d ", ci+1, len(p.Conflicts)))+theme.MutedStyle.Render(lens), width, "…"))
			} else {
				out = append(out, theme.MutedStyle.Render(fmt.Sprintf(" conflict %d/%d", ci+1, len(p.Conflicts))))
			}
		}
		text := fmt.Sprintf("%4d %s", i+1, strings.ReplaceAll(l, "\t", "    "))
		st := lipgloss.NewStyle()
		switch kind[i] {
		case "marker":
			st = theme.GitHeaderStyle
		case "ours":
			st = theme.DiffAddStyle
		case "theirs":
			st = theme.DiffHunkStyle
		}
		text = ansi.Truncate(st.Render(text), width, "…")
		if ok && ci == p.ConflictIdx {
			text = theme.HoverStyle.Render(ansi.Strip(text) + strings.Repeat(" ", max(0, width-lipgloss.Width(ansi.Strip(text)))))
			text = st.Render("") + text
		}
		out = append(out, text)
	}
	return out
}

// ---------- view ----------

func (p *Panel) View(z *zone.Manager) string {
	inner := p.Width - 2
	if p.Repo == nil {
		body := lipgloss.Place(inner, p.inner(), lipgloss.Center, lipgloss.Center,
			theme.MutedStyle.Render("Not a git repository.\n\n`git init` in the terminal (ctrl+j)"))
		return theme.FocusedBorderStyle.Width(inner).Height(p.inner()).MaxHeight(p.Height).Render(body)
	}

	// Tabs + branch header.
	var tabs []string
	for i, name := range SectionNames {
		st := theme.TabInactiveStyle
		if Section(i) == p.Section {
			st = theme.TabActiveStyle
		}
		label := name
		if Section(i) == SecCommits && p.FileHistory {
			label = "File History"
		}
		id := fmt.Sprintf("git_tab_%d", i)
		tabs = append(tabs, z.Mark(id, theme.Hoverable(p.Hover == id, st).Render("["+label+"]")))
	}
	head := strings.Join(tabs, " ")
	branch := ""
	if p.Status != nil {
		branch = theme.BranchStyle.Render("⎇ " + p.Status.Branch)
		if p.Status.Ahead > 0 || p.Status.Behind > 0 {
			branch += theme.MutedStyle.Render(fmt.Sprintf(" ↑%d ↓%d", p.Status.Ahead, p.Status.Behind))
		}
	}
	mode := theme.MutedStyle.Render("inline") + " " + theme.TabActiveStyle.Render("split")
	if !p.Split {
		mode = theme.TabActiveStyle.Render("inline") + " " + theme.MutedStyle.Render("split")
	}
	if p.IgnoreWS {
		mode += " " + theme.HashStyle.Render("-w")
	}
	mode = z.Mark("git_diffmode", theme.Hoverable(p.Hover == "git_diffmode", lipgloss.NewStyle()).Render("["+mode+"]"))
	wrapSt := theme.MutedStyle
	if p.Wrap {
		wrapSt = theme.TabActiveStyle
	}
	mode = z.Mark("git_wrap", theme.Hoverable(p.Hover == "git_wrap", wrapSt).Render("[wrap]")) + " " + mode
	right := branch + "  " +
		z.Mark("git_sync", theme.Hoverable(p.Hover == "git_sync", theme.SendButtonStyle).Render(p.SyncLabel())) + "  " + mode
	if gap := inner - lipgloss.Width(head) - lipgloss.Width(right); gap > 0 {
		head += strings.Repeat(" ", gap)
	}
	lines := []string{ansi.Truncate(head+right, inner, "")}

	// List.
	for i := p.listScroll; i < p.listScroll+p.listRows(); i++ {
		if i >= len(p.rows) {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, p.renderRow(&p.rows[i], i == p.Cursor, i == p.HoverRow, inner))
	}
	lines = append(lines, theme.MutedStyle.Render(strings.Repeat("─", inner)))

	// Detail: conflict resolver, editor when editing, otherwise the diff.
	if p.Resolving {
		rl := p.resolveLines(inner)
		if max := len(rl) - p.detailRows(); p.detailScroll > max {
			p.detailScroll = max
		}
		if p.detailScroll < 0 {
			p.detailScroll = 0
		}
		for i := p.detailScroll; i < p.detailScroll+p.detailRows(); i++ {
			if i >= len(rl) {
				lines = append(lines, "")
				continue
			}
			lines = append(lines, rl[i])
		}
	} else if p.Composing && p.MsgEditor != nil {
		p.MsgEditor.SetSize(inner-2, p.detailRows()-2)
		p.MsgEditor.Hint = p.composeHint()
		lines = append(lines, strings.Split(p.MsgEditor.View(), "\n")...)
	} else if p.Editing && p.Editor != nil {
		p.Editor.SetSize(inner-2, p.detailRows()-2)
		p.Editor.Hint = "editing working copy · esc done · ctrl+s save"
		lines = append(lines, strings.Split(p.Editor.View(), "\n")...)
	} else {
		dl := p.detailLines()
		s0, s1 := p.selRange()
		sb := theme.VScrollbar(p.detailRows(), len(dl), p.detailScroll)
		textW := inner - 1 // detailWidth() always reserves the scrollbar column
		for i := p.detailScroll; i < p.detailScroll+p.detailRows(); i++ {
			var line string
			if i < len(dl) {
				line = dl[i] // already styled and wrapped to width
				if s0 >= 0 && i >= s0 && i <= s1 {
					plain := ansi.Strip(line)
					if w := lipgloss.Width(plain); w < textW {
						plain += strings.Repeat(" ", textW-w)
					}
					line = theme.SelectionStyle.Render(plain)
				}
			}
			if sb != nil {
				if w := lipgloss.Width(line); w < textW {
					line += strings.Repeat(" ", textW-w)
				}
				line += sb[i-p.detailScroll]
			}
			lines = append(lines, line)
		}
	}

	// Prompt / hint row.
	var foot string
	switch {
	case p.prompt == promptConfirm:
		foot = theme.PromptStyle.Render(" " + p.promptTitle + " ")
	case p.prompt == promptType:
		foot = p.typePickerFoot(inner)
	case p.prompt != promptNone:
		foot = theme.PromptStyle.Render(" "+p.promptTitle+": "+p.promptText+"▏") + theme.MutedStyle.Render("  ⏎ ok · esc cancel")
	case p.Resolving:
		foot = theme.MutedStyle.Render(fmt.Sprintf(" resolving %s · c/i/b accept current/incoming/both · n/p next/prev · a mark resolved · o open · esc back", p.ResolveRel))
	case p.Busy != "":
		foot = theme.WordmarkStyle.Render(" " + p.Spinner + " " + p.Busy + p.genElapsed())
	case p.Message != "":
		foot = theme.MutedStyle.Render(" " + p.Message)
	default:
		foot = theme.MutedStyle.Render(" " + p.hints())
	}
	lines = append(lines, ansi.Truncate(foot, inner, "…"))

	return theme.FocusedBorderStyle.Width(inner).Height(p.inner()).MaxHeight(p.Height).Render(strings.Join(lines, "\n"))
}

// SyncLabel mirrors VS Code's status-bar sync button.
func (p *Panel) SyncLabel() string {
	st := p.Status
	switch {
	case st == nil:
		return " ⟳ Sync "
	case st.Upstream == "":
		return " ⤴ Publish branch "
	case st.Behind > 0:
		return fmt.Sprintf(" ⟳ Sync ↑%d ↓%d ", st.Ahead, st.Behind)
	case st.Ahead > 0:
		return fmt.Sprintf(" ↑ Push %d ", st.Ahead)
	}
	return " ⟳ Sync "
}

// SyncAction runs pull+push (or publish) in the background.
func (p *Panel) SyncAction() {
	st := p.Status
	label := "sync"
	if st != nil && st.Upstream == "" {
		label = "publish"
	}
	p.async(label, func() (string, error) { return p.Repo.Sync(st) })
}

func (p *Panel) hints() string {
	switch p.Section {
	case SecStatus:
		return "+/− stage/unstage · e edit · v split · z wrap · w ignore-ws · drag+ctrl+c copy · c commit · y sync · S stash · d discard · P pull · f fetch"
	case SecCommits:
		return "⏎/jk browse · J/K scroll diff · f file↔repo history · / search · y hash"
	case SecBranches:
		return "⏎ checkout · n new · d delete · f fetch"
	case SecStashes:
		return "⏎ pop · s stash · d drop"
	case SecBlame:
		return "⏎ go to line · b inline blame in editor · J/K scroll commit"
	}
	return ""
}

func (p *Panel) renderRow(r *row, selected, hovered bool, width int) string {
	var text string
	switch {
	case r.header:
		text = theme.GitHeaderStyle.Render(r.text)
	case r.file != nil:
		badge := r.file.Badge()
		text = "    " + theme.GitBadgeStyle(badge).Render(badge) + " " + r.file.Path
	case r.commit != nil:
		c := r.commit
		text = " " + theme.HashStyle.Render(c.Hash) + " " + theme.MutedStyle.Render(fmt.Sprintf("%-14s", ansi.Truncate(c.Date, 14, ""))) + " " + theme.AuthorStyle.Render(ansi.Truncate(c.Author, 14, "…")) + "  " + c.Subject
	case r.branch != nil:
		b := r.branch
		mark := "  "
		if b.Current {
			mark = theme.StatusOKStyle.Render("* ")
		}
		text = " " + mark + theme.BranchStyle.Render(b.Name) + " " + theme.MutedStyle.Render(b.Track+" "+b.Date) + "  " + b.Subject
	case r.stash != nil:
		s := r.stash
		text = " " + theme.HashStyle.Render(s.Ref) + " " + theme.MutedStyle.Render(s.Date) + "  " + s.Subject
	case p.Section == SecBlame && r.blame < len(p.Blame):
		b := p.Blame[r.blame]
		src := ""
		if r.blame < len(p.BlameSrc) {
			src = strings.ReplaceAll(p.BlameSrc[r.blame], "\t", "    ")
		}
		hash := theme.HashStyle.Render(b.Hash)
		if b.Uncommitted {
			hash = theme.MutedStyle.Render("·······")
		}
		text = fmt.Sprintf(" %4d %s %s %s %s │ %s", r.blame+1, hash,
			theme.AuthorStyle.Render(fmt.Sprintf("%-12s", ansi.Truncate(b.Author, 12, "…"))),
			theme.MutedStyle.Render(fmt.Sprintf("%-8s", gitx.Ago(b.Time))),
			theme.MutedStyle.Render(fmt.Sprintf("%-20s", ansi.Truncate(b.Summary, 20, "…"))), src)
	default:
		text = r.text
	}
	// Right-aligned buttons (undo, then stage / unstage) for status rows.
	action, undo := "", ""
	if r.action != "" {
		action = "[ " + r.action + " ]"
	}
	if r.undo != "" {
		undo = "[ " + r.undo + " ]"
	}
	btns := undo + action
	text = ansi.Truncate(text, width-lipgloss.Width(btns)-1, "…")
	pad := ""
	if w := lipgloss.Width(text) + lipgloss.Width(btns); w < width {
		pad = strings.Repeat(" ", width-w)
	}
	if selected {
		plain := ansi.Strip(text) + pad + btns
		return theme.CursorFocusedStyle.Render(plain)
	}
	if undo != "" {
		undo = theme.Hoverable(hovered && p.HoverBtn == "undo", theme.HashStyle).Render(undo)
	}
	if action != "" {
		st := theme.DiffAddStyle
		switch {
		case strings.HasPrefix(r.action, "−"), strings.HasPrefix(r.action, "✕"):
			st = theme.DiffDelStyle
		case r.action == "resolve":
			st = theme.HashStyle
		}
		action = theme.Hoverable(hovered && p.HoverBtn == "action", st).Render(action)
	}
	if hovered && p.HoverBtn == "" {
		// Hovering the row body: tint the text, leave the buttons alone so
		// their own hover state stays distinguishable.
		text = theme.HoverStyle.Render(ansi.Strip(text))
	}
	return text + pad + undo + action
}

// selRange returns the ordered selected detail lines (-1,-1 if none).
func (p *Panel) selRange() (int, int) {
	if p.selAnchor < 0 || p.selEnd < 0 {
		return -1, -1
	}
	if p.selAnchor <= p.selEnd {
		return p.selAnchor, p.selEnd
	}
	return p.selEnd, p.selAnchor
}

// HasSelection reports whether detail lines are selected.
func (p *Panel) HasSelection() bool { a, _ := p.selRange(); return a >= 0 }

// CopySelection puts the selected detail lines (plain text) on the clipboard.
func (p *Panel) CopySelection() bool {
	s0, s1 := p.selRange()
	if s0 < 0 {
		return false
	}
	dl := p.detailLines()
	var out []string
	for i := s0; i <= s1 && i < len(dl); i++ {
		out = append(out, strings.TrimRight(ansi.Strip(dl[i]), " "))
	}
	if err := clipboard.WriteAll(strings.Join(out, "\n")); err != nil {
		p.Message = "clipboard unavailable"
		return false
	}
	p.Message = fmt.Sprintf("copied %d line(s)", len(out))
	return true
}

// visibleWhitespace expands tabs and marks trailing spaces, so a tab→spaces
// reindent is not an invisible "everything changed" diff.
func visibleWhitespace(body string) string {
	if trimmed := strings.TrimRight(body, " "); len(trimmed) < len(body) {
		body = trimmed + strings.Repeat("·", len(body)-len(trimmed))
	}
	return theme.ExpandTabs(body)
}

// showWhitespace applies it to a unified-diff line, keeping the +/- marker.
func showWhitespace(l string) string {
	if l == "" || (l[0] != '+' && l[0] != '-' && l[0] != ' ') {
		return l
	}
	return l[:1] + visibleWhitespace(l[1:])
}

// diffLine colours unified-diff lines.
// diffRowStyle is the whole-row tint for an added / removed diff line. The
// prefix tests mirror diffLine's, where +++ / --- are file headers rather than
// changed lines.
func diffRowStyle(l string) (lipgloss.Style, bool) {
	switch {
	case strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"):
		return lipgloss.Style{}, false
	case strings.HasPrefix(l, "+"):
		return theme.DiffAddLineStyle, true
	case strings.HasPrefix(l, "-"):
		return theme.DiffDelLineStyle, true
	}
	return lipgloss.Style{}, false
}

func diffLine(l string) string {
	switch {
	case strings.HasPrefix(l, "+++") || strings.HasPrefix(l, "---") || strings.HasPrefix(l, "diff ") || strings.HasPrefix(l, "index "):
		return theme.MutedStyle.Render(l)
	case strings.HasPrefix(l, "@@"):
		return theme.DiffHunkStyle.Render(l)
	case strings.HasPrefix(l, "+"):
		return theme.DiffAddLineStyle.Render(l)
	case strings.HasPrefix(l, "-"):
		return theme.DiffDelLineStyle.Render(l)
	case strings.HasPrefix(l, "commit ") || strings.HasPrefix(l, "Author:") || strings.HasPrefix(l, "Date:"):
		return theme.HashStyle.Render(l)
	}
	return l
}
