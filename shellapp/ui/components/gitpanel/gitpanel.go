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
	"regexp"
	"sort"
	"strconv"
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
	blame  int // index into blameCommits (blame section)
}

// blameCommit is one commit that wrote lines of the blamed file.
type blameCommit struct {
	gitx.BlameLine
	lines, first int // line count and first line index
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

	Status       *gitx.Status
	Commits      []gitx.Commit
	Branches     []gitx.Branch
	Stashes      []gitx.Stash
	Blame        []gitx.BlameLine
	BlameSrc     []string // file lines shown beside blame entries
	blameCommits []blameCommit
	Filter       string

	rows           []row
	Cursor         int
	listScroll     int
	detail         []string
	detailCache    []string // detailLines() memo
	detailSrc      [][]int  // rendered row → detail line(s) it shows (memo)
	detailCacheKey string   // width/divider/split/wrap it was built for
	detailScroll   int
	collapsed      [2]bool // status groups

	// Two columns: the list (with the commit graph under it) on the left,
	// the diff on the right. ListW / ListRows are the user's drag
	// preferences (0 = default); the drag flags live until EndDrag.
	ListW, ListRows    int
	dragList, dragRows bool
	HoverSep           string   // divider under the mouse: list · rows · split
	graph              []string // commit graph of the selected branch
	graphFor           string
	graphScroll        int

	// FocusDiff moves the keyboard to the diff: diffCursor is the rendered
	// row under it, and line ops (stage / revert / unstage) act on the
	// selection or the cursor's hunk. hunkBtns is the button strip drawn
	// on hunk headers ("" when the diff cannot be patched by line).
	FocusDiff  bool
	diffCursor int
	hunkBtns   string
	hoverHunk  int    // rendered row whose hunk button is hovered, -1 = none
	hoverHunkB string // "stage" / "undo"

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

	Split      bool
	Editing    bool
	Editor     *texteditor.TextEditor
	EditPath   string
	editDirty  bool
	editOld    []string // index side, tabs expanded like the editor's buffer
	editOldRaw []string // index side as git has it (patch "-" lines)
	editHunks  []editHunk
	editHover  int    // editor line whose gutter button is hovered, -1 = none
	editHoverB string // "revert" / "stage"

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
	return &Panel{Repo: repo, Width: 80, Height: 24, selAnchor: -1, selEnd: -1, HoverRow: -1, hoverHunk: -1, Wrap: true,
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
	p.loadGraph(true)
}

// graphRef is the branch the graph pane follows: the branch under the cursor
// in the Branches section, otherwise the checked-out one.
func (p *Panel) graphRef() string {
	if p.Section == SecBranches {
		if r := p.current(); r != nil && r.branch != nil {
			return r.branch.Name
		}
	}
	if p.Status != nil && p.Status.Branch != "" && p.Status.Branch != "HEAD" {
		return p.Status.Branch
	}
	return ""
}

// loadGraph reloads the commit graph. Unless forced it only runs when the
// followed branch changed, so moving the cursor stays cheap.
func (p *Panel) loadGraph(force bool) {
	ref := p.graphRef()
	if !force && ref == p.graphFor {
		return
	}
	if p.Repo == nil {
		return
	}
	p.graphFor = ref
	p.graph, _ = p.Repo.Graph(ref, 200)
	p.graphScroll = 0
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
			h := row{text: fmt.Sprintf("⚠ %s · %d conflict(s)", p.MergeKind, len(conflicts)), header: true, group: 2}
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
			p.rows = append(p.rows, row{text: "Click a file in the explorer to see its blame", header: true})
		} else {
			p.rows = append(p.rows, row{text: " " + p.ActivePath, header: true})
		}
		p.blameCommits = groupBlame(p.Blame)
		for i := range p.blameCommits {
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
	p.detailScroll, p.diffCursor = 0, 0
	p.selAnchor, p.selEnd = -1, -1
	p.hunkBtns = ""
	r := p.current()
	if r == nil || p.Repo == nil {
		return
	}
	if p.canPatch(r) {
		p.hunkBtns = "[ − unstage ]"
		if !r.staged {
			p.hunkBtns = "[ ⟲ revert ] [ + stage ]"
		}
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
	case p.Section == SecBlame && r.blame < len(p.blameCommits):
		p.detail = p.BlameSrc
		p.detailScroll = max(p.blameCommits[r.blame].first-2, 0)
		p.clamp()
		return
	}
	p.detail, p.detailCache = strings.Split(strings.TrimRight(text, "\n"), "\n"), nil
}

// detailWidth is the usable width of the detail pane (the scrollbar column is
// always reserved, so the content never shifts when one appears).
func (p *Panel) detailWidth() int {
	return max(p.rightW()-1, 1)
}

// detailLines renders the detail pane for the current width: side by side when
// split, otherwise the unified diff. Either way long lines wrap rather than
// being cut off at the border. Memoised, and recomputed on resize or a mode
// flip, so a stale width can never survive a SetSize.
func (p *Panel) detailLines() []string {
	w, half := p.detailWidth(), p.splitHalf()
	hot := p.dragSplit || p.HoverSep == "split"
	// The hovered hunk button is baked into the rows, so it is part of the key.
	key := fmt.Sprintf("%d/%d/%v/%v/%v/%s/%d/%s", w, half, p.Split, p.Wrap, hot, p.hunkBtns, p.hoverHunk, p.hoverHunkB)
	if p.detailCache != nil && p.detailCacheKey == key {
		return p.detailCache
	}
	out, src := []string{}, [][]int{}
	if p.Section == SecBlame {
		for i, l := range p.blameLines(w) {
			out = append(out, l)
			src = append(src, []int{i})
		}
	} else if p.Split {
		out, src = splitDiff(p.detail, w, half, p.Wrap, sepStyle(hot))
	} else {
		for i, l := range p.detail {
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
			for range chunks {
				src = append(src, []int{i})
			}
		}
	}
	// Hunk headers carry the line-op buttons, right-aligned.
	if p.hunkBtns != "" {
		bw := lipgloss.Width(p.hunkBtns)
		for i, l := range out {
			if s := src[i][0]; s < len(p.detail) && strings.HasPrefix(p.detail[s], "@@") && (i == 0 || src[i-1][0] != s) && w > bw+8 {
				head := ansi.Truncate(l, w-bw-1, "…")
				out[i] = head + strings.Repeat(" ", w-bw-lipgloss.Width(head)) + p.renderHunkBtns(i)
			}
		}
	}
	p.detailCache, p.detailSrc, p.detailCacheKey = out, src, key
	return out
}

// renderHunkBtns styles the hunk button strip, tinting the hovered one.
func (p *Panel) renderHunkBtns(row int) string {
	hov := func(name string) bool { return p.hoverHunk == row && p.hoverHunkB == name }
	if strings.HasPrefix(p.hunkBtns, "[ −") {
		return theme.Hoverable(hov("undo"), theme.DiffDelStyle).Render(p.hunkBtns)
	}
	return theme.Hoverable(hov("undo"), theme.HashStyle).Render("[ ⟲ revert ]") + " " +
		theme.Hoverable(hov("stage"), theme.DiffAddStyle).Render("[ + stage ]")
}

// hunkBtnAt names the hunk button at a right-pane column of a rendered row:
// "stage", "undo" or "".
func (p *Panel) hunkBtnAt(row, col int) string {
	dl := p.detailLines()
	if p.hunkBtns == "" || row < 0 || row >= len(dl) {
		return ""
	}
	s := p.detailSrc[row][0]
	if !strings.HasPrefix(p.detail[s], "@@") || (row > 0 && p.detailSrc[row-1][0] == s) {
		return ""
	}
	w, bw := p.detailWidth(), lipgloss.Width(p.hunkBtns)
	if w <= bw+8 || col < w-bw { // detailLines draws no buttons on a pane this narrow
		return ""
	}
	if !strings.HasPrefix(p.hunkBtns, "[ −") && col >= w-lipgloss.Width("[ + stage ]") {
		return "stage"
	}
	return "undo"
}

// EndDrag releases every divider. The screen calls it on mouse-up, which
// never reaches HandleMouse.
func (p *Panel) EndDrag() { p.dragSplit, p.dragList, p.dragRows = false, false, false }

// ClearHover drops every hover highlight; the screen calls it when the mouse
// leaves the panel.
func (p *Panel) ClearHover() {
	p.HoverRow, p.HoverBtn, p.HoverSep = -1, "", ""
	p.hoverHunk, p.hoverHunkB = -1, ""
	p.editHover, p.editHoverB = -1, ""
}

// sepAt names the draggable divider at a panel-relative cell, "" if none.
func (p *Panel) sepAt(x, y int) string {
	inBody := y >= listTop && y < listTop+p.bodyRows()
	switch {
	case !inBody:
		return ""
	case x == p.listW()+1:
		return "list"
	case x >= 1 && x <= p.listW() && y == p.ruleY():
		return "rows"
	case p.Split && !p.Resolving && !p.Editing && !p.Composing && abs(x-p.dividerX()) <= 1:
		return "split"
	}
	return ""
}

// ResizeAxis is the pointer shape a divider under the mouse (or being
// dragged) asks for: "ew-resize", "ns-resize" or "".
func (p *Panel) ResizeAxis() string {
	switch {
	case p.dragRows, p.HoverSep == "rows":
		return "ns-resize"
	case p.dragList, p.dragSplit, p.HoverSep != "":
		return "ew-resize"
	}
	return ""
}

// sepStyle is the divider style, lit while it is hovered or dragged.
func sepStyle(hot bool) lipgloss.Style {
	if hot {
		return lipgloss.NewStyle().Foreground(theme.PrimaryColor).Bold(true)
	}
	return theme.MutedStyle
}

// newLineAt is the working-copy line number shown on a rendered diff row
// (0 when the row carries no new-side line).
func (p *Panel) newLineAt(row int) int {
	dl := p.detailLines()
	if row < 0 || row >= len(dl) {
		return 0
	}
	src := p.detailSrc[row]
	want := src[len(src)-1]
	n, inHunk := 0, false
	for i := 0; i <= want && i < len(p.detail); i++ {
		l := p.detail[i]
		switch {
		case strings.HasPrefix(l, "@@"):
			if m := hunkNewStart.FindStringSubmatch(l); m != nil {
				n, _ = strconv.Atoi(m[1])
				n--
				inHunk = true
			}
		case strings.HasPrefix(l, "-"), strings.HasPrefix(l, "\\"):
		default:
			n++
		}
	}
	if !inHunk {
		return 0
	}
	if strings.HasPrefix(p.detail[want], "-") {
		return n + 1
	}
	return n
}

var hunkNewStart = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)`)

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

// dividerX is the panel-relative column of the split divider.
func (p *Panel) dividerX() int { return p.rightX() + p.splitHalf() }

// detailTop is the first panel-relative row of the detail pane.
func (p *Panel) detailTop() int { return listTop }

// editCell maps a right-pane column / panel row in split edit mode to the
// editor line on that row and the column relative to the gutter's start
// (negative = index side, 0..editGutterW-1 = gutter, beyond = editor).
func (p *Panel) editCell(col, y int) (line, gx int) {
	lines, first := p.Editor.RowLines()
	if i := y - listTop - 1; i >= 0 && i < len(lines) && first[i] {
		line = lines[i]
	} else {
		line = -1
	}
	return line, col - p.splitHalf()
}

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
	ed.SetSize(p.rightW()-2, p.detailRows()-2)
	ed.Focus()
	ed.OnChanged = func(c string) {
		p.editDirty = true
		p.editHunks = lineDiff(p.editOld, splitLines(c))
	}
	ed.RowStyle = func(line int) (lipgloss.Style, bool) {
		if h := p.hunkAt(line); h != nil && h.ns < h.ne {
			return theme.DiffAddLineStyle, true
		}
		return lipgloss.Style{}, false
	}
	p.Editor, p.EditPath, p.Editing, p.editDirty = ed, abs, true, false
	p.editHover = -1
	p.loadEditOld(f.Path)
	p.editHunks = lineDiff(p.editOld, splitLines(ed.GetContent()))
}

// loadEditOld reads the index side of the edited file (what a stage would
// build on); an untracked file has none, so every line is an addition. The
// editor's buffer holds tabs as spaces, so the copy it is diffed against
// is expanded the same way; the raw lines go into patches.
func (p *Panel) loadEditOld(rel string) {
	p.editOld, p.editOldRaw = nil, nil
	if old, err := p.Repo.Run("show", ":"+rel); err == nil {
		p.editOldRaw = splitLines(old)
		p.editOld = make([]string, len(p.editOldRaw))
		for i, l := range p.editOldRaw {
			p.editOld[i] = theme.ExpandTabs(l)
		}
	}
}

func splitLines(s string) []string { return strings.Split(strings.TrimSuffix(s, "\n"), "\n") }

// editGutterW is the column between the index side and the editor in split
// edit mode: "⟲ + │" — revert and stage buttons for the hunk starting on
// that row, then the divider.
const editGutterW = 5

// revertEditHunk puts the index side's lines back in the editor (undoable).
func (p *Panel) revertEditHunk(h editHunk) {
	cur := p.Editor.GetContent()
	lines := splitLines(cur)
	out := append(append(append([]string{}, lines[:h.ns]...), p.editOld[h.os:h.oe]...), lines[h.ne:]...)
	s := strings.Join(out, "\n")
	if strings.HasSuffix(cur, "\n") {
		s += "\n"
	}
	p.Editor.SetValue(s) // fires OnChanged: dirty + hunks
	p.saveEdit()
	p.Message = "hunk reverted · ctrl+z undoes"
}

// stageEditHunk applies this hunk to the index straight from the editor
// buffer; the working file is untouched and the editor stays open.
// ponytail: a file without a trailing newline gets no "\ No newline" marker.
func (p *Panel) stageEditHunk(h editHunk) {
	rel := p.Repo.Rel(p.EditPath)
	lines := splitLines(p.Editor.GetContent())
	rng := func(start, n int) string {
		if n == 0 {
			return fmt.Sprintf("%d,0", start)
		}
		return fmt.Sprintf("%d,%d", start+1, n)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n@@ -%s +%s @@\n", rel, rel, rng(h.os, h.oe-h.os), rng(h.ns, h.ne-h.ns))
	for _, l := range p.editOldRaw[h.os:h.oe] {
		b.WriteString("-" + l + "\n")
	}
	for _, l := range lines[h.ns:h.ne] {
		b.WriteString("+" + l + "\n")
	}
	if err := p.Repo.Apply(b.String(), true, false); err != nil {
		p.Message = "stage hunk: " + err.Error()
		return
	}
	p.Message = "hunk staged ✓"
	p.loadEditOld(rel)
	p.editHunks = lineDiff(p.editOld, lines)
	if st, err := p.Repo.Status(); err == nil { // file list only; the editor keeps running
		p.Status = st
		p.buildRows()
	}
	if p.OnChanged != nil {
		p.OnChanged()
	}
}

// editGutter renders the gutter cell for editor line n.
func (p *Panel) editGutter(n int, hunkRow bool) string {
	if !hunkRow {
		return "    " + sepStyle(false).Render("│")
	}
	hov := func(b string) bool { return p.editHover == n && p.editHoverB == b }
	return theme.Hoverable(hov("revert"), theme.HashStyle).Render("⟲") + " " +
		theme.Hoverable(hov("stage"), theme.DiffAddStyle).Render("+") + " " + sepStyle(false).Render("│")
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

// bodyRows is the height of both columns: everything between the tabs row
// and the hint row.
func (p *Panel) bodyRows() int { return max(p.inner()-2, 3) }

// listW is the left column's width, clamped so both columns stay usable.
func (p *Panel) listW() int {
	inner := p.Width - 2
	w := p.ListW
	if w <= 0 {
		w = min(max(inner*2/5, 20), 60)
	}
	if lo, hi := 16, inner-1-20; hi >= lo {
		w = min(max(w, lo), hi)
	}
	return max(w, 1)
}

// rightX is the panel-relative column where the diff pane starts (after the
// left border, the list and the column divider).
func (p *Panel) rightX() int { return 1 + p.listW() + 1 }

// rightW is the diff pane's width.
func (p *Panel) rightW() int { return max(p.Width-2-p.listW()-1, 1) }

// listRows is the height of the file / commit list; the graph takes the rest
// of the left column under a one-row rule.
func (p *Panel) listRows() int {
	body := p.bodyRows()
	n := p.ListRows
	if n <= 0 {
		n = body * 3 / 5
	}
	if lo, hi := 3, body-2; hi >= lo {
		n = min(max(n, lo), hi)
	}
	return max(n, 1)
}

// ruleY is the panel-relative row of the list / graph divider.
func (p *Panel) ruleY() int { return listTop + p.listRows() }

func (p *Panel) graphRows() int { return max(p.bodyRows()-p.listRows()-1, 0) }

func (p *Panel) detailRows() int { return p.bodyRows() }

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
	if n := len(p.detailLines()); p.diffCursor >= n {
		p.diffCursor = n - 1
	}
	if p.diffCursor < 0 {
		p.diffCursor = 0
	}
	if max := len(p.graph) - p.graphRows(); p.graphScroll > max {
		p.graphScroll = max
	}
	if p.graphScroll < 0 {
		p.graphScroll = 0
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
	p.loadGraph(false)
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
	p.FocusDiff = false
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
		case "alt+z", "Ω": // Option+z arrives as Ω on macOS
			p.ToggleWrap()
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
	if p.FocusDiff && p.handleDiffKey(msg) {
		return true
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
	case "tab":
		p.FocusDiff = len(p.detail) > 0
	case "right", "l":
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
	case p.Section == SecBlame && p.OnGotoLine != nil && !r.header && r.blame < len(p.blameCommits):
		line := p.blameCommits[r.blame].first
		if p.FocusDiff {
			line = p.diffCursor
		}
		p.OnGotoLine(line)
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
	aLeft := p.listW() - lipgloss.Width(r.action) - 3
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
	inLeft := msg.X >= 1 && msg.X <= p.listW()
	inRight := msg.X >= p.rightX()
	inBody := msg.Y >= listTop && msg.Y < listTop+p.bodyRows()
	// The right pane's own coordinates: column inside it, rendered row.
	col := msg.X - p.rightX()
	drow := p.detailScroll + msg.Y - p.detailTop()
	editorTop := listTop // the editors draw their own border from the first body row

	switch msg.Type {
	case tea.MouseMotion:
		p.HoverRow, p.HoverBtn = -1, ""
		p.hoverHunk, p.hoverHunkB = -1, ""
		p.HoverSep = p.sepAt(msg.X, msg.Y)
		p.editHover, p.editHoverB = -1, ""
		if p.Editing && p.Split && inRight && inBody {
			if n, gx := p.editCell(col, msg.Y); gx == 0 || gx == 2 {
				if h := p.hunkAt(n); h != nil && h.ns == n {
					p.editHover, p.editHoverB = n, map[int]string{0: "revert", 2: "stage"}[gx]
				}
			}
		}
		if inLeft {
			p.HoverRow = p.rowAtY(msg.Y)
			if p.HoverRow >= 0 {
				p.HoverBtn = p.rowHit(&p.rows[p.HoverRow], msg.X)
			}
		} else if inRight && inBody && !p.Resolving && !p.Editing && !p.Composing {
			if b := p.hunkBtnAt(drow, col); b != "" {
				p.hoverHunk, p.hoverHunkB = drow, b
			}
		}
	case tea.MouseLeft:
		// Column divider and list / graph rule: drag to resize.
		if msg.Action == tea.MouseActionPress && inBody && msg.X == p.listW()+1 {
			p.dragList = true
			return
		}
		if msg.Action == tea.MouseActionPress && inLeft && msg.Y == p.ruleY() {
			p.dragRows = true
			return
		}
		if msg.Action == tea.MouseActionMotion {
			if p.dragList {
				p.ListW = msg.X - 1
				p.detailCache = nil
				p.clamp()
				return
			}
			if p.dragRows {
				p.ListRows = msg.Y - listTop
				p.clamp()
				return
			}
		}
		// Drag the split divider to rebalance the two diff columns.
		if p.Split && !p.Resolving && !p.Editing && !p.Composing && inBody {
			if msg.Action == tea.MouseActionPress && abs(msg.X-p.dividerX()) <= 1 {
				p.dragSplit = true
				return
			}
			if p.dragSplit && msg.Action == tea.MouseActionMotion {
				p.SplitPos = msg.X - p.rightX()
				return
			}
		}
		if msg.Action == tea.MouseActionPress {
			p.dragSplit = false
		}
		if inRight && inBody {
			if p.Resolving {
				if msg.Action != tea.MouseActionPress {
					return
				}
				// Map the clicked row back to a conflict block (headers add rows).
				for i, c := range p.Conflicts {
					if drow >= c.Start+i+1 && drow <= c.End+i+1 {
						p.ConflictIdx = i
						return
					}
				}
				return
			}
			if ed := p.detailEditor(); ed != nil {
				rel := msg
				rel.X = col - 1
				rel.Y -= editorTop + 1
				if p.Editing && p.Split {
					n, gx := p.editCell(col, msg.Y)
					if gx < editGutterW {
						if h := p.hunkAt(n); msg.Action == tea.MouseActionPress && h != nil && h.ns == n {
							switch gx {
							case 0:
								p.revertEditHunk(*h)
							case 2:
								p.stageEditHunk(*h)
							}
						}
						return // index side and gutter are not the editor
					}
					rel.X = gx - editGutterW - 1
				}
				ed.Update(rel)
				return
			}
			// Text selection in the diff / commit pane: press + drag over rows.
			if n := len(p.detailLines()); drow >= n {
				drow = n - 1
			}
			if drow < 0 {
				return
			}
			if msg.Action == tea.MouseActionPress {
				p.FocusDiff = true
				if b := p.hunkBtnAt(drow, col); b != "" {
					p.selAnchor, p.selEnd = -1, -1
					p.diffCursor = drow
					p.lineOp(p.diffOp(b), drow)
					return
				}
				// The new side of a split diff is the working copy: click it to edit there.
				if p.Split && p.Section == SecStatus && col > p.splitHalf() {
					if _, ok := p.canEdit(); ok {
						line := p.newLineAt(drow)
						p.startEdit()
						if p.Editing && line > 0 {
							p.Editor.GotoLine(line - 1)
						}
						return
					}
				}
				p.selAnchor, p.selEnd = drow, drow
				p.diffCursor = drow
			} else if msg.Action == tea.MouseActionMotion && p.selAnchor >= 0 {
				p.selEnd = drow
				p.diffCursor = drow
			}
			return
		}
		if msg.Action != tea.MouseActionPress || !inLeft {
			return
		}
		if i := p.rowAtY(msg.Y); i >= 0 {
			p.FocusDiff = false
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
				p.loadGraph(false)
			}
		}
	case tea.MouseWheelLeft, tea.MouseWheelRight:
		if ed := p.detailEditor(); inRight && ed != nil {
			ed.Update(msg)
		}
	case tea.MouseWheelUp, tea.MouseWheelDown:
		d := 3
		if msg.Type == tea.MouseWheelUp {
			d = -3
		}
		switch {
		case inRight && p.detailEditor() != nil:
			rel := msg
			rel.X = col - 1
			rel.Y -= editorTop + 1
			p.detailEditor().Update(rel)
		case inRight:
			p.detailScroll += d
			p.clamp()
		case inLeft && msg.Y > p.ruleY():
			p.graphScroll += d
			p.clamp()
		case inLeft:
			p.listScroll += d
			if p.listScroll < 0 {
				p.listScroll = 0
			}
			if max := len(p.rows) - p.listRows(); p.listScroll > max && max >= 0 {
				p.listScroll = max
			}
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

	// Two columns: list + graph on the left, the detail pane on the right.
	left, right2 := p.leftColumn(), p.rightColumn()
	sep := sepStyle(p.dragList || p.HoverSep == "list").Render("│")
	for i := 0; i < p.bodyRows(); i++ {
		lines = append(lines, left[i]+sep+right2[i])
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

// pad fills a rendered line to exactly w cells.
func pad(s string, w int) string {
	s = ansi.Truncate(s, w, "")
	if n := w - lipgloss.Width(s); n > 0 {
		s += strings.Repeat(" ", n)
	}
	return s
}

// leftColumn is the list, a rule naming the graphed branch, and the commit
// graph — bodyRows lines of listW cells.
func (p *Panel) leftColumn() []string {
	w := p.listW()
	out := make([]string, 0, p.bodyRows())
	for i := p.listScroll; i < p.listScroll+p.listRows(); i++ {
		if i >= len(p.rows) {
			out = append(out, pad("", w))
			continue
		}
		out = append(out, pad(p.renderRow(&p.rows[i], i == p.Cursor && !p.FocusDiff, i == p.HoverRow, w), w))
	}
	title := " ⎇ " + p.graphFor + " "
	if p.graphFor == "" {
		title = " graph "
	}
	rs := sepStyle(p.dragRows || p.HoverSep == "rows")
	rule := rs.Render("─") + theme.BranchStyle.Render(title)
	out = append(out, pad(rule+rs.Render(strings.Repeat("─", max(w-lipgloss.Width(rule), 0))), w))
	for i := p.graphScroll; len(out) < p.bodyRows(); i++ {
		if i >= len(p.graph) {
			out = append(out, pad("", w))
			continue
		}
		out = append(out, pad(renderGraphLine(p.graph[i]), w))
	}
	return out[:p.bodyRows()]
}

// renderGraphLine colours one `git log --graph --oneline` row: glyphs muted,
// the hash and the decoration highlighted.
func renderGraphLine(l string) string {
	i := 0
	for i < len(l) && strings.ContainsRune("*|/\\ _-.", rune(l[i])) {
		i++
	}
	var glyphs strings.Builder
	for _, c := range l[:i] {
		if c == '*' {
			glyphs.WriteString(theme.DiffAddStyle.Render("●"))
		} else {
			glyphs.WriteString(theme.MutedStyle.Render(string(c)))
		}
	}
	rest := l[i:]
	hash, rest, _ := strings.Cut(rest, " ")
	out := glyphs.String() + theme.HashStyle.Render(hash)
	if strings.HasPrefix(rest, "(") {
		if j := strings.Index(rest, ")"); j >= 0 {
			out += " " + theme.BranchStyle.Render(rest[:j+1])
			rest = strings.TrimPrefix(rest[j+1:], " ")
		}
	}
	if rest != "" {
		out += " " + rest
	}
	return out
}

// rightColumn is the detail pane: conflict resolver, an editor, or the diff —
// bodyRows lines of rightW cells.
func (p *Panel) rightColumn() []string {
	w, rows := p.rightW(), p.detailRows()
	var lines []string
	switch {
	case p.Resolving:
		rl := p.resolveLines(w)
		if max := len(rl) - rows; p.detailScroll > max {
			p.detailScroll = max
		}
		if p.detailScroll < 0 {
			p.detailScroll = 0
		}
		for i := p.detailScroll; i < p.detailScroll+rows; i++ {
			if i < len(rl) {
				lines = append(lines, rl[i])
			}
		}
	case p.Composing && p.MsgEditor != nil:
		p.MsgEditor.SetSize(w-2, rows-2)
		p.MsgEditor.Hint = p.composeHint()
		lines = strings.Split(p.MsgEditor.View(), "\n")
	case p.Editing && p.Editor != nil && p.Split:
		// Index side on the left (removed lines red, paired with the editor
		// line that replaced them), the working copy in the editor on the
		// right (added lines green), hunk buttons in the gutter between.
		half := p.splitHalf()
		p.Editor.SetSize(w-half-editGutterW-2, rows-2)
		p.Editor.Wrap = p.Wrap
		p.Editor.Hint = "editing working copy · ⟲ revert hunk · + stage hunk · ⌥z wrap · esc done · ctrl+s save"
		ed := strings.Split(p.Editor.View(), "\n")
		rowLines, rowFirst := p.Editor.RowLines()
		numW := len(strconv.Itoa(len(p.editOld))) + 1
		textW := max(half-numW-1, 0)
		sx := p.Editor.ScrollX() // the index side pans with the editor
		clip := func(s string) string { return ansi.Truncate(ansi.Cut(s, sx, sx+textW+1), textW, "…") }
		for i := 0; i < rows; i++ {
			var l, gut string
			gut = p.editGutter(-1, false)
			if n := i - 1; n >= 0 && n < len(rowLines) && rowLines[n] >= 0 && rowFirst[n] {
				n = rowLines[n]
				j, del := p.leftLine(n)
				if j >= 0 && j < len(p.editOld) {
					l = fmt.Sprintf("%*d ", numW, j+1) + clip(p.editOld[j])
					if del {
						l = theme.DiffDelLineStyle.Render(pad(l, half))
					} else {
						l = theme.MutedStyle.Render(l[:numW+1]) + l[numW+1:]
					}
				}
				h := p.hunkAt(n)
				if h != nil && h.ns == h.ne && h.ns == n {
					// A pure deletion has no editor row to sit beside: show what
					// went, folded onto the row that follows it.
					gone := strings.Join(p.editOld[h.os:h.oe], " ⏎ ")
					l = theme.DiffDelLineStyle.Render(pad(fmt.Sprintf("%*s ", numW, "−")+clip(gone), half))
				}
				gut = p.editGutter(n, h != nil && h.ns == n)
			}
			r := ""
			if i < len(ed) {
				r = ed[i]
			}
			lines = append(lines, pad(l, half)+gut+r)
		}
	case p.Editing && p.Editor != nil:
		p.Editor.SetSize(w-2, rows-2)
		p.Editor.Wrap = p.Wrap
		p.Editor.Hint = "editing working copy · ⌥z wrap · esc done · ctrl+s save"
		lines = strings.Split(p.Editor.View(), "\n")
	default:
		dl := p.detailLines()
		s0, s1 := p.selRange()
		sb := theme.VScrollbar(rows, len(dl), p.detailScroll)
		textW := w - 1 // detailWidth() always reserves the scrollbar column
		for i := p.detailScroll; i < p.detailScroll+rows; i++ {
			var line string
			if i < len(dl) {
				line = dl[i] // already styled and wrapped to width
				switch {
				case s0 >= 0 && i >= s0 && i <= s1:
					line = theme.SelectionStyle.Render(pad(ansi.Strip(line), textW))
				case p.FocusDiff && i == p.diffCursor:
					line = theme.CursorFocusedStyle.Render(pad(ansi.Strip(line), textW))
				}
			}
			line = pad(line, textW)
			if sb != nil {
				line += sb[i-p.detailScroll]
			}
			lines = append(lines, line)
		}
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = pad(lines[i], w)
	}
	return lines[:rows]
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
	if p.FocusDiff {
		if p.hunkBtns != "" {
			return "jk move · shift+↑↓ select · s stage lines · u revert/unstage lines · e edit · ctrl+c copy · drag │ resize · tab/esc list"
		}
		return "jk move · shift+↑↓ select · ctrl+c copy · v split · z wrap · tab/esc list"
	}
	switch p.Section {
	case SecStatus:
		return "tab diff · +/− stage · e edit · v split · z wrap · w -w · c commit · y sync · S stash · d discard · P pull · f fetch"
	case SecCommits:
		return "⏎/jk browse · J/K scroll diff · f file↔repo history · / search · y hash"
	case SecBranches:
		return "⏎ checkout · n new · d delete · f fetch"
	case SecStashes:
		return "⏎ pop · s stash · d drop"
	case SecBlame:
		return "⏎ open in editor at line · tab/click lines · b inline blame in editor · J/K scroll"
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
	case p.Section == SecBlame && r.blame < len(p.blameCommits):
		b := p.blameCommits[r.blame]
		text = fmt.Sprintf(" %s %s %s %s  %s", blameHash(b.BlameLine),
			theme.AuthorStyle.Render(fmt.Sprintf("%-12s", ansi.Truncate(b.Author, 12, "…"))),
			theme.MutedStyle.Render(fmt.Sprintf("%-8s", gitx.Ago(b.Time))),
			theme.MutedStyle.Render(fmt.Sprintf("%4d ln", b.lines)), b.Summary)
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

// ---------- blame ----------

// groupBlame folds per-line blame into one entry per commit, newest first,
// uncommitted lines on top.
func groupBlame(lines []gitx.BlameLine) []blameCommit {
	idx := map[string]int{}
	var out []blameCommit
	for i, b := range lines {
		k := b.Hash
		if b.Uncommitted {
			k = ""
		}
		j, ok := idx[k]
		if !ok {
			j = len(out)
			idx[k] = j
			out = append(out, blameCommit{BlameLine: b, first: i})
		}
		out[j].lines++
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Uncommitted != out[b].Uncommitted {
			return out[a].Uncommitted
		}
		return out[a].Time.After(out[b].Time)
	})
	return out
}

func blameHash(b gitx.BlameLine) string {
	if b.Uncommitted {
		return theme.MutedStyle.Render("·······")
	}
	return theme.HashStyle.Render(b.Hash)
}

// blameLines is the annotated source: hash, author and age in front of
// every line, the selected commit's lines marked so they stand out.
func (p *Panel) blameLines(w int) []string {
	sel := ""
	selUn := false
	if r := p.current(); r != nil && r.blame < len(p.blameCommits) {
		sel, selUn = p.blameCommits[r.blame].Hash, p.blameCommits[r.blame].Uncommitted
	}
	numW := len(strconv.Itoa(len(p.Blame)))
	out := make([]string, 0, len(p.Blame))
	for i, b := range p.Blame {
		src := ""
		if i < len(p.BlameSrc) {
			src = theme.ExpandTabs(p.BlameSrc[i])
		}
		mark, author := theme.MutedStyle.Render("  "), theme.MutedStyle
		if (b.Uncommitted && selUn) || (!b.Uncommitted && b.Hash == sel) {
			mark, author = theme.StatusOKStyle.Render("▌ "), theme.AuthorStyle
		}
		gutter := mark + blameHash(b) + " " + author.Render(fmt.Sprintf("%-12s", ansi.Truncate(b.Author, 12, "…"))) +
			" " + theme.MutedStyle.Render(fmt.Sprintf("%-8s %*d │ ", gitx.Ago(b.Time), numW, i+1))
		out = append(out, ansi.Truncate(gutter+src, w, "…"))
	}
	return out
}
