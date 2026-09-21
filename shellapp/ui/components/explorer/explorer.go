// Package explorer is a clean, single-file VS Code-style file tree.
//
// It exposes one type — Explorer — and two callbacks: OnSelect (cursor
// moved via click), OnActivate (click / Enter on a file or folder).
// Directories are loaded lazily on first expand so a large root (node_modules,
// .git) doesn't stall startup.
//
// Click handling is done by reading tea.MouseMsg.X / .Y directly against the
// rendered row positions.
package explorer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/epilande/go-devicons"

	"github.com/hittable/shellapp/ui/theme"
)

// IconMode selects the icon pack: "nerd" (Nerd Font glyphs + per-type colours,
// like VS Code's icon themes; needs a Nerd Font in the terminal) or "emoji".
var IconMode = "nerd"

// Folder glyphs per mode: [closed, open].
func folderIcons() (string, string) {
	if IconMode == "emoji" {
		return "📁", "📂"
	}
	return "", ""
}

// ---------- Tree model ----------

type NodeKind int

const (
	NodeDir NodeKind = iota
	NodeHit
	NodeEnv
	NodeMarkdown
	NodeJSON
	NodeGeneric
)

type Node struct {
	Name     string
	Path     string
	Kind     NodeKind
	Icon     string
	IconHex  string // icon colour (nerd mode), "" = inherit
	Expanded bool
	Loaded   bool // dir children read from disk
	Children []*Node
	Parent   *Node
	Depth    int
}

func classify(name string) NodeKind {
	switch filepath.Ext(name) {
	case ".hit":
		return NodeHit
	case ".md":
		return NodeMarkdown
	case ".json":
		if name == "env.json" {
			return NodeEnv
		}
		return NodeJSON
	}
	return NodeGeneric
}

// resolveIcon returns the glyph and colour for a file. .hit and env.json keep
// app-specific icons; everything else comes from the devicons pack.
func resolveIcon(kind NodeKind, path string) (string, string) {
	if IconMode == "emoji" {
		switch kind {
		case NodeHit:
			return "⚡", ""
		case NodeEnv:
			return "🔧", ""
		case NodeMarkdown:
			return "📝", ""
		case NodeJSON:
			return "📋", ""
		}
		return "📄", ""
	}
	switch kind {
	case NodeHit:
		return "H", "" // rendered as the brand chip (see renderRow)
	case NodeEnv:
		return "", "#ffb86c" // nf-fa-cog
	}
	st := devicons.IconForPath(path)
	if st.Icon == "" {
		return "", "#6272a4" // nf-fa-file_o
	}
	return st.Icon, st.Color
}

// BuildTree stats root and loads its first level. Deeper folders are read on
// expand (see Load).
func BuildTree(rootPath string) (*Node, error) {
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, err
	}
	_, open := folderIcons()
	root := &Node{
		Name:     info.Name(),
		Path:     rootPath,
		Kind:     NodeDir,
		Icon:     open,
		Expanded: true,
		Depth:    -1,
	}
	return root, Load(root)
}

// hiddenNames mirrors VS Code's default files.exclude.
var hiddenNames = map[string]bool{".git": true, ".DS_Store": true, ".svn": true, ".hg": true}

// Load reads a directory's immediate children from disk (once).
func Load(n *Node) error {
	if n.Kind != NodeDir || n.Loaded {
		return nil
	}
	entries, err := os.ReadDir(n.Path)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		di, dj := entries[i].IsDir(), entries[j].IsDir()
		if di != dj {
			return di
		}
		return entries[i].Name() < entries[j].Name()
	})
	n.Children = make([]*Node, 0, len(entries))
	for _, e := range entries {
		if hiddenNames[e.Name()] {
			continue
		}
		c := &Node{
			Name:   e.Name(),
			Path:   filepath.Join(n.Path, e.Name()),
			Parent: n,
			Depth:  n.Depth + 1,
		}
		if e.IsDir() {
			c.Kind = NodeDir
			c.Icon, _ = folderIcons()
		} else {
			c.Kind = classify(e.Name())
			c.Icon, c.IconHex = resolveIcon(c.Kind, c.Path)
		}
		n.Children = append(n.Children, c)
	}
	n.Loaded = true
	return nil
}

// Visible returns the flattened list of visible nodes (excluding root).
func Visible(root *Node) []*Node {
	var out []*Node
	var walk func(n *Node)
	walk = func(n *Node) {
		for _, c := range n.Children {
			out = append(out, c)
			if c.Kind == NodeDir && c.Expanded {
				walk(c)
			}
		}
	}
	walk(root)
	return out
}

// FindByPath searches the loaded tree for a node matching path.
func FindByPath(root *Node, path string) *Node {
	if root.Path == path {
		return root
	}
	for _, c := range root.Children {
		if found := FindByPath(c, path); found != nil {
			return found
		}
	}
	return nil
}

// ---------- The component ----------

type SelectCallback func(n *Node)
type ActivateCallback func(n *Node)

type Explorer struct {
	Root    *Node
	Visible []*Node

	ScrollStart int
	Cursor      int
	HoverRow    int
	Width       int
	Height      int
	Focused     bool // cursor renders bright when focused, dim otherwise

	OnSelect   SelectCallback
	OnActivate ActivateCallback

	// GitStatus maps absolute file paths to a one-letter badge (M/A/D/U/!).
	// Folders containing a changed file get a dot.
	GitStatus map[string]string
	gitDirs   map[string]bool

	// Context menu (x key / right-click).
	ContextMenuRequested bool
	ContextMenuRow       int
	MenuOpen             bool
	MenuIdx              int
	menuX, menuY         int // top-left of the floating menu box (set in View)

	// Inline prompt (new file / new folder / rename).
	Prompt      string
	PromptValue string
	PromptKind  PromptMode

	// Delete confirmation.
	DeletePending    bool
	DeletePendingRow int
}

type PromptMode int

const (
	PromptNone PromptMode = iota
	PromptNewFile
	PromptNewFolder
	PromptRename
)

var MenuItems = []string{"New File", "New Folder", "Rename", "Delete"}

func New(rootPath string) (*Explorer, error) {
	root, err := BuildTree(rootPath)
	if err != nil {
		return nil, err
	}
	return &Explorer{
		Root:     root,
		Visible:  Visible(root),
		HoverRow: -1,
		Width:    30,
		Height:   20,
	}, nil
}

// MustNew is like New but falls back to an empty tree on error.
func MustNew(rootPath string) *Explorer {
	e, err := New(rootPath)
	if err != nil {
		root := &Node{Name: filepath.Base(rootPath), Path: rootPath, Kind: NodeDir, Depth: -1}
		e = &Explorer{Root: root, HoverRow: -1, Width: 30, Height: 20}
	}
	return e
}

func (e *Explorer) SetSize(w, h int) {
	if w < 5 {
		w = 5
	}
	if h < 5 {
		h = 5
	}
	e.Width = w
	e.Height = h
	e.clampCursor()
}

// RefreshForTest recomputes the visible list from a synthetic Root.
func (e *Explorer) RefreshForTest() { e.refresh() }

// RebuildVisible recomputes the visible list after Expanded was mutated.
func (e *Explorer) RebuildVisible() { e.refresh() }

// ViewportLinesForTest returns the calculated viewport line count.
func (e *Explorer) ViewportLinesForTest() int { return e.viewportLines() }

// Toggle expands/collapses a folder, loading it from disk on first expand.
func (e *Explorer) Toggle(n *Node) {
	if n.Kind != NodeDir {
		return
	}
	n.Expanded = !n.Expanded
	if n.Expanded && !n.Loaded {
		_ = Load(n) // unreadable dir just shows empty
	}
	e.refresh()
}

// RebuildTree re-scans disk preserving expanded state of matching paths.
func (e *Explorer) RebuildTree() error {
	expanded := map[string]bool{}
	var walk func(n *Node)
	walk = func(n *Node) {
		if n.Kind == NodeDir && n.Expanded {
			expanded[n.Path] = true
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(e.Root)

	newRoot, err := BuildTree(e.Root.Path)
	if err != nil {
		return err
	}
	var apply func(n *Node)
	apply = func(n *Node) {
		if expanded[n.Path] {
			n.Expanded = true
			_ = Load(n)
		}
		for _, c := range n.Children {
			apply(c)
		}
	}
	apply(newRoot)
	e.Root = newRoot
	e.refresh()
	return nil
}

func (e *Explorer) refresh() {
	e.Visible = Visible(e.Root)
	e.clampCursor()
}

func (e *Explorer) clampCursor() {
	if len(e.Visible) == 0 {
		e.Cursor = 0
		e.HoverRow = -1
		e.ScrollStart = 0
		return
	}
	if e.Cursor < 0 {
		e.Cursor = 0
	}
	if e.Cursor >= len(e.Visible) {
		e.Cursor = len(e.Visible) - 1
	}
	if e.HoverRow >= len(e.Visible) {
		e.HoverRow = -1
	}
	visLines := e.viewportLines()
	if e.Cursor < e.ScrollStart {
		e.ScrollStart = e.Cursor
	}
	if e.Cursor >= e.ScrollStart+visLines {
		e.ScrollStart = e.Cursor - visLines + 1
	}
	if e.ScrollStart < 0 {
		e.ScrollStart = 0
	}
}

// headerLines is the number of rows above the first file row: the header
// plus any prompt / delete-confirm / context-menu rows.
func (e *Explorer) headerLines() int {
	n := 1
	if e.PromptKind != PromptNone {
		n++
	}
	if e.DeletePending {
		n++
	}
	return n
}

func (e *Explorer) viewportLines() int {
	lines := e.Height - e.headerLines()
	if lines < 1 {
		lines = 1
	}
	return lines
}

// rowAtY translates a screen Y to a visible-list index, or -1.
func (e *Explorer) rowAtY(y int) int {
	h := e.headerLines()
	if y < h {
		return -1
	}
	idx := e.ScrollStart + (y - h)
	if idx < 0 || idx >= len(e.Visible) {
		return -1
	}
	return idx
}

// ---------- Mouse handling ----------

// HandleMouse processes one mouse event. Returns true if consumed.
//
//	MouseLeft   -> select; files open, folders toggle (single click).
//	MouseRight  -> context menu on the row.
//	MouseMotion -> hover row.
//	Wheel       -> scroll.
func (e *Explorer) HandleMouse(msg tea.MouseMsg) bool {
	switch msg.Type {
	case tea.MouseLeft:
		if msg.Action == tea.MouseActionMotion {
			return false // drag, not a click
		}
		if e.MenuOpen {
			i := msg.Y - e.menuY - 1 // first row of the box is its border
			if msg.X >= e.menuX && msg.X < e.menuX+menuWidth && i >= 0 && i < len(MenuItems) {
				e.MenuIdx = i
				e.selectMenuItem()
			} else {
				e.MenuOpen = false
			}
			return true
		}
		idx := e.rowAtY(msg.Y)
		if idx < 0 {
			return false
		}
		e.Cursor = idx
		if e.OnSelect != nil {
			e.OnSelect(e.Visible[idx])
		}
		if e.OnActivate != nil {
			e.OnActivate(e.Visible[idx])
		}
		e.clampCursor()
		return true

	case tea.MouseRight:
		idx := e.rowAtY(msg.Y)
		if idx >= 0 {
			e.Cursor = idx
			e.ContextMenuRequested = true
			e.ContextMenuRow = idx
			if e.OnSelect != nil {
				e.OnSelect(e.Visible[idx])
			}
			e.OpenMenu()
		}
		return true

	case tea.MouseMotion:
		if e.MenuOpen {
			if i := msg.Y - e.menuY - 1; msg.X >= e.menuX && msg.X < e.menuX+menuWidth && i >= 0 && i < len(MenuItems) {
				e.MenuIdx = i
			}
			return false
		}
		e.HoverRow = e.rowAtY(msg.Y)
		return false

	case tea.MouseWheelUp:
		if e.ScrollStart > 0 {
			e.ScrollStart--
		}
		return true
	case tea.MouseWheelDown:
		if e.ScrollStart < len(e.Visible)-e.viewportLines() {
			e.ScrollStart++
		}
		return true
	}
	return false
}

// ActivateCursor invokes OnActivate for the selected node.
func (e *Explorer) ActivateCursor() {
	if e.Cursor >= 0 && e.Cursor < len(e.Visible) && e.OnActivate != nil {
		e.OnActivate(e.Visible[e.Cursor])
	}
}

// ToggleCursor toggles the cursor's folder or activates the file (Enter).
func (e *Explorer) ToggleCursor() {
	if e.Cursor < 0 || e.Cursor >= len(e.Visible) {
		return
	}
	n := e.Visible[e.Cursor]
	if n.Kind == NodeDir {
		e.Toggle(n)
	} else if e.OnActivate != nil {
		e.OnActivate(n)
	}
}

func (e *Explorer) MoveUp() {
	if e.Cursor > 0 {
		e.Cursor--
		e.clampCursor()
	}
}

func (e *Explorer) MoveDown() {
	if e.Cursor < len(e.Visible)-1 {
		e.Cursor++
		e.clampCursor()
	}
}

// ---------- View ----------

func (e *Explorer) View() string {
	hs := theme.ExplorerHeaderStyle
	if e.Focused {
		hs = theme.ExplorerHeaderFocusedStyle
	}
	lines := []string{hs.Width(e.Width).Render("/" + filepath.Base(e.Root.Path))}

	if e.PromptKind != PromptNone {
		lines = append(lines, e.renderPrompt())
	}
	if e.DeletePending {
		name := ""
		if e.DeletePendingRow >= 0 && e.DeletePendingRow < len(e.Visible) {
			name = e.Visible[e.DeletePendingRow].Name
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(theme.ErrorColor).Width(e.Width).
			Render(fmt.Sprintf("  Delete %q? (y/n)", name)))
	}
	end := e.ScrollStart + e.viewportLines()
	if end > len(e.Visible) {
		end = len(e.Visible)
	}
	for i := e.ScrollStart; i < end; i++ {
		lines = append(lines, e.renderRow(e.Visible[i], i))
	}
	for len(lines) < e.Height {
		lines = append(lines, strings.Repeat(" ", e.Width))
	}
	if len(lines) > e.Height {
		lines = lines[:e.Height]
	}
	if e.MenuOpen {
		e.overlayMenu(lines)
	}
	return strings.Join(lines, "\n")
}

const menuWidth = 16 // box incl. border

// overlayMenu composites the floating context menu over the rendered rows,
// anchored just below the cursor row (above it when near the bottom).
func (e *Explorer) overlayMenu(lines []string) {
	var box []string
	for i, item := range MenuItems {
		st := theme.ContextMenuItemStyle
		if i == e.MenuIdx {
			st = theme.ContextMenuItemHoverStyle
		}
		box = append(box, st.Width(menuWidth-2).Render(item))
	}
	rendered := strings.Split(theme.ContextMenuStyle.Render(strings.Join(box, "\n")), "\n")

	x := 2
	if e.Cursor >= 0 && e.Cursor < len(e.Visible) {
		x = 2*(e.Visible[e.Cursor].Depth+1) + 3
	}
	if x+menuWidth > e.Width {
		x = e.Width - menuWidth
	}
	if x < 0 {
		x = 0
	}
	y := e.headerLines() + (e.Cursor - e.ScrollStart) + 1
	if y+len(rendered) > len(lines) {
		y = e.headerLines() + (e.Cursor - e.ScrollStart) - len(rendered)
	}
	if y < 0 {
		y = 0
	}
	e.menuX, e.menuY = x, y
	for i, row := range rendered {
		if y+i >= len(lines) {
			break
		}
		base := lines[y+i]
		lines[y+i] = ansi.Cut(base, 0, x) + row + ansi.Cut(base, x+menuWidth, e.Width)
	}
}

func (e *Explorer) renderPrompt() string {
	kind := "name"
	switch e.PromptKind {
	case PromptNewFile:
		kind = "new file"
	case PromptNewFolder:
		kind = "new folder"
	case PromptRename:
		kind = "rename"
	}
	return theme.HoverStyle.Width(e.Width).Render(fmt.Sprintf("  %s: %s", kind, e.PromptValue))
}

func (e *Explorer) renderRow(n *Node, idx int) string {
	icon := n.Icon
	name := n.Name
	if n.Kind == NodeDir {
		closed, open := folderIcons()
		icon = closed
		if n.Expanded {
			icon = open
		}
		name += "/"
	}
	indent := strings.Repeat("  ", n.Depth+1)
	badge := ""
	if n.Kind == NodeDir {
		if e.gitDirs[n.Path] {
			badge = "●"
		}
	} else if b, ok := e.GitStatus[n.Path]; ok {
		badge = b
	}
	display := fmt.Sprintf("%s%s %s", indent, icon, name)
	display = ansi.Truncate(display, e.Width-1-lipgloss.Width(badge)-1, "…")
	spaces := ""
	if w := lipgloss.Width(display); w < e.Width {
		spaces = strings.Repeat(" ", e.Width-w)
	}
	// Right-align the badge inside the row (plain for highlighted rows,
	// coloured for normal rows; built from the space run, never re-sliced).
	pad := spaces
	padStyled := spaces
	if badge != "" && len(spaces) > lipgloss.Width(badge)+1 {
		head := spaces[:len(spaces)-lipgloss.Width(badge)-1]
		pad = head + badge + " "
		st := lipgloss.NewStyle().Foreground(theme.GitStatusColor(badge))
		if n.Kind == NodeDir {
			st = theme.MutedStyle
		}
		padStyled = head + st.Render(badge) + " "
	}
	// Highlighted rows are styled as one run so the background is unbroken;
	// plain rows get a per-type coloured icon.
	switch {
	case idx == e.Cursor && e.Focused:
		return theme.CursorFocusedStyle.Render(display + pad)
	case idx == e.Cursor:
		return theme.CursorUnfocusedStyle.Render(display + pad)
	case idx == e.HoverRow:
		return theme.HoverStyle.Render(display + pad)
	}
	var iconStyled string
	pad = padStyled
	switch {
	case n.Kind == NodeDir:
		iconStyled = theme.FolderStyle.Render(icon)
		return indent + iconStyled + " " + theme.FolderStyle.Render(ansi.Truncate(name, e.Width-1-lipgloss.Width(indent+icon+" "), "…")) + pad
	case n.Kind == NodeHit && IconMode != "emoji":
		iconStyled = theme.HitMarkStyle.Render(icon)
	case n.IconHex != "":
		iconStyled = lipgloss.NewStyle().Foreground(lipgloss.Color(n.IconHex)).Render(icon)
	default:
		iconStyled = icon
	}
	nameStyled := name
	switch {
	case badge != "":
		nameStyled = lipgloss.NewStyle().Foreground(theme.GitStatusColor(badge)).Render(name)
	case n.Kind == NodeHit:
		nameStyled = theme.HitFileStyle.Render(name)
	}
	return ansi.Truncate(indent+iconStyled+" "+nameStyled, e.Width-1, "…") + pad
}

// fit pads/truncates a rendered line to exactly Width cells.
func (e *Explorer) fit(s string) string {
	s = ansi.Truncate(s, e.Width, "")
	if w := lipgloss.Width(s); w < e.Width {
		s += strings.Repeat(" ", e.Width-w)
	}
	return s
}

// SetGitStatus installs explorer decorations from a path→badge map.
func (e *Explorer) SetGitStatus(st map[string]string) {
	e.GitStatus = st
	e.gitDirs = map[string]bool{}
	for p := range st {
		for d := filepath.Dir(p); d != "" && d != "/" && d != e.Root.Path; d = filepath.Dir(d) {
			e.gitDirs[d] = true
		}
	}
}

// ---------- Context menu ----------

func (e *Explorer) OpenMenu() {
	e.MenuOpen = true
	e.MenuIdx = 0
}

func (e *Explorer) selectMenuItem() {
	e.MenuOpen = false
	switch e.MenuIdx {
	case 0:
		e.BeginNewFile()
	case 1:
		e.BeginNewFolder()
	case 2:
		e.BeginRename()
	case 3:
		e.BeginDelete()
	}
}

// ---------- Inline prompt management ----------

func (e *Explorer) BeginNewFile() {
	e.PromptKind = PromptNewFile
	e.PromptValue = ""
}

func (e *Explorer) BeginNewFolder() {
	e.PromptKind = PromptNewFolder
	e.PromptValue = ""
}

func (e *Explorer) BeginRename() {
	if e.Cursor < 0 || e.Cursor >= len(e.Visible) {
		return
	}
	e.PromptKind = PromptRename
	e.PromptValue = e.Visible[e.Cursor].Name
}

func (e *Explorer) CancelPrompt() {
	e.PromptKind = PromptNone
	e.PromptValue = ""
}

func (e *Explorer) AppendPromptChar(c string) { e.PromptValue += c }

func (e *Explorer) BackspacePrompt() {
	if r := []rune(e.PromptValue); len(r) > 0 {
		e.PromptValue = string(r[:len(r)-1])
	}
}

// ConfirmPrompt applies the current prompt value.
// New file/folder: returns (parentPath, name, isDir, true).
// Rename: returns (oldPath, newPath, false, true).
// ok=false means nothing should happen (empty name).
func (e *Explorer) ConfirmPrompt() (string, string, bool, bool) {
	kind := e.PromptKind
	name := strings.TrimSpace(e.PromptValue)
	e.PromptKind = PromptNone
	e.PromptValue = ""
	if name == "" {
		return "", "", false, false
	}
	parent := e.Root.Path
	var cur *Node
	if e.Cursor >= 0 && e.Cursor < len(e.Visible) {
		cur = e.Visible[e.Cursor]
		if cur.Kind == NodeDir {
			parent = cur.Path
		} else if cur.Parent != nil {
			parent = cur.Parent.Path
		}
	}
	if kind == PromptRename {
		if cur == nil {
			return "", "", false, false
		}
		return cur.Path, filepath.Join(filepath.Dir(cur.Path), name), false, true
	}
	return parent, name, kind == PromptNewFolder, true
}

// ---------- Delete confirmation ----------

func (e *Explorer) BeginDelete() {
	if e.Cursor < 0 || e.Cursor >= len(e.Visible) {
		return
	}
	e.DeletePending = true
	e.DeletePendingRow = e.Cursor
}

func (e *Explorer) CancelDelete() {
	e.DeletePending = false
	e.DeletePendingRow = -1
}

// ConfirmDelete returns (path, ok).
func (e *Explorer) ConfirmDelete() (string, bool) {
	if !e.DeletePending || e.DeletePendingRow < 0 || e.DeletePendingRow >= len(e.Visible) {
		e.DeletePending = false
		return "", false
	}
	p := e.Visible[e.DeletePendingRow].Path
	e.DeletePending = false
	e.DeletePendingRow = -1
	return p, true
}

// ---------- Keyboard ----------

// HandleKey processes a key event while the explorer has focus.
// Returns true if it consumed the event. Enter on a prompt / y on a delete
// confirm are reported as consumed but left for the caller to apply via
// ConfirmPrompt / ConfirmDelete.
func (e *Explorer) HandleKey(msg tea.KeyMsg) bool {
	if e.MenuOpen {
		switch msg.String() {
		case "up", "k":
			if e.MenuIdx > 0 {
				e.MenuIdx--
			}
		case "down", "j":
			if e.MenuIdx < len(MenuItems)-1 {
				e.MenuIdx++
			}
		case "enter":
			e.selectMenuItem()
		case "esc", "x":
			e.MenuOpen = false
		}
		return true
	}
	if e.DeletePending {
		switch msg.String() {
		case "n", "N", "esc":
			e.CancelDelete()
		}
		return true
	}
	if e.PromptKind != PromptNone {
		switch msg.String() {
		case "enter":
		case "esc":
			e.CancelPrompt()
		case "backspace":
			e.BackspacePrompt()
		default:
			if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
				e.AppendPromptChar(string(msg.Runes))
			}
		}
		return true
	}

	switch msg.String() {
	case "up", "k":
		e.MoveUp()
	case "down", "j":
		e.MoveDown()
	case "pgup", "ctrl+u":
		e.Cursor -= e.viewportLines() - 1
		e.clampCursor()
	case "pgdown", "ctrl+d":
		e.Cursor += e.viewportLines() - 1
		e.clampCursor()
	case "home", "g":
		e.Cursor = 0
		e.clampCursor()
	case "end", "G":
		e.Cursor = len(e.Visible) - 1
		e.clampCursor()
	case "r":
		_ = e.RebuildTree()
	case "enter", "l":
		e.ToggleCursor()
	case "x":
		if len(e.Visible) > 0 {
			e.OpenMenu()
		}
	case "h":
		if e.Cursor >= 0 && e.Cursor < len(e.Visible) {
			n := e.Visible[e.Cursor]
			if n.Kind == NodeDir && n.Expanded {
				e.Toggle(n)
			} else if n.Parent != nil && n.Parent != e.Root {
				for i, c := range e.Visible {
					if c == n.Parent {
						e.Cursor = i
						e.clampCursor()
						break
					}
				}
			}
		}
	default:
		return false
	}
	return true
}

func (e *Explorer) String() string {
	if len(e.Visible) == 0 {
		return "(empty)"
	}
	return fmt.Sprintf("%d files", len(e.Visible))
}
