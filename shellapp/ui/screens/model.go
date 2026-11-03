// Package screens defines the main screen model and types for the application.
package screens

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/gitx"
	"github.com/hittable/shellapp/internal/hitfile"
	"github.com/hittable/shellapp/ui/components/explorer"
	"github.com/hittable/shellapp/ui/components/extpanel"
	"github.com/hittable/shellapp/ui/components/gitpanel"
	"github.com/hittable/shellapp/ui/components/mdpreview"
	"github.com/hittable/shellapp/ui/components/palette"
	"github.com/hittable/shellapp/ui/components/pathprompt"
	"github.com/hittable/shellapp/ui/components/requesteditor"
	"github.com/hittable/shellapp/ui/components/responseviewer"
	"github.com/hittable/shellapp/ui/components/terminal"
	"github.com/hittable/shellapp/ui/components/texteditor"
	zone "github.com/lrstanley/bubblezone"
)

type FocusArea int

const (
	FocusExplorerPane FocusArea = iota
	FocusURLBar
	FocusTabBar
	FocusBody
	FocusResponse
	FocusTextEditor
	FocusMethodDropdown
	FocusPreview
)

// MdMode is how a Markdown file is shown.
type MdMode int

const (
	MdText MdMode = iota
	MdPreview
	MdSplit
)

type ViewMode int

const (
	ViewRunner ViewMode = iota
	ViewText
)

type MainScreen struct {
	RootDir     string
	HittableDir string
	Store       *document.Store
	WriteQueue  *document.WriteQueue
	EnvData     map[string]string
	EnvPath     string
	Zones       *zone.Manager

	Explorer *explorer.Explorer
	URLBar   *requesteditor.URLBar
	Params   *requesteditor.ParamsTab
	Headers  *requesteditor.HeadersTab
	Body     *requesteditor.BodyTab
	Response *responseviewer.ResponseViewer
	TextEd   *texteditor.TextEditor
	Term     *terminal.Terminal
	Repo     *gitx.Repo
	Git      *gitpanel.Panel
	AI       *ai
	Palette  *palette.Palette
	Preview  *mdpreview.Preview
	// PathPrompt picks a file to attach by typing its path (attach.go).
	PathPrompt pathprompt.Prompt
	attaching  bool // the open palette picks a file to attach, not one to open
	MdMode     MdMode

	Focus            FocusArea
	LastFocus        FocusArea
	ActiveFile       string
	ViewMode         ViewMode
	ActiveTab        requesteditor.Tab
	Width            int
	Height           int
	ExplorerWidth    int
	MainWidth        int
	MainH            int // rows of the main pane above the terminal strip
	EditorHeight     int
	TermRows         int // dragged terminal panel height (0 = a third of the window)
	EditorRows       int // dragged runner editor box height (0 = a third of the pane)
	MdSplitLeft      int // dragged markdown split editor width (0 = half)
	TermFocused      bool
	GitOpen          bool
	ExplorerHidden   bool // the user's preference (alt+b, ☰)
	SingleFile       bool // `hittable <file>`: no explorer, Git or Find (EnterSingleFile)
	sidebarWasHidden bool // last laid-out value of explorerHidden()
	BlameOn          bool
	lastGitRefresh   time.Time
	blame            []gitx.BlameLine
	gitPolling       bool
	gitKey           string
	Sending          bool
	cancelSend       context.CancelFunc // aborts the request in flight (esc)
	prodArmed        time.Time          // a production send waits for a second ctrl+r until then
	unknownVars      []string           // <<KEY>>s the request in flight could not resolve
	spinning         bool               // a spinner tick loop is already running
	StatusBar        string
	ExplorerFocused  bool
	dragKind         string // splitter being dragged: explorer · term · editor · md
	sepHover         string // splitter under the mouse (same names), "" = none
	dragMoved        bool   // the mouse moved since the press (a still press is a click)
	dragBase         int    // screen row / column the dragged pane starts at
	DragStartX       int
	termSel          bool     // a drag selection is in progress in the terminal panel
	frameSel         frameSel // drag-selected block of the rendered frame
	lastFrame        []string // the frame as last drawn, for copying the block
	HoverZone        string
	HelpScroll       int       // the help overlay scrolls when it outgrows the pane
	Ext              *extState // VS Code extensions (ext.go)
	ExtPanel         *extpanel.Panel
	ExtOpen          bool    // the Extensions panel fills the main pane
	pendingCmd       tea.Cmd // a command started from a palette pick, returned by Update
	ShowHelp         bool
	Spinner          spinner.Model
}

var rootDirGlobal string
var envPathGlobal string

func NewMainScreen(rootDir string, zones *zone.Manager) *MainScreen {
	hittableDir := filepath.Join(rootDir, "hittable")
	envPath := filepath.Join(hittableDir, "env.json")

	envData := hitfile.LoadEnv(envPath) // env.json + env.local.json

	store := document.NewStore()
	wq := document.NewWriteQueue()

	explorerComp := explorer.MustNew(rootDir)
	urlBar := requesteditor.NewURLBar()
	params := requesteditor.NewParamsTab()
	headers := requesteditor.NewHeadersTab()
	body := requesteditor.NewBodyTab()
	response := responseviewer.New()
	textEd := texteditor.New()
	term := terminal.New(rootDir, func(msg tea.Msg) { send(msg) })

	repo := gitx.Open(rootDir)
	git := gitpanel.New(repo)
	aiState := newAI(rootDir)
	git.Drafter = aiState.drafter()
	pal := palette.New(rootDir, func(msg tea.Msg) { send(msg) })

	m := &MainScreen{
		RootDir:         rootDir,
		HittableDir:     hittableDir,
		Store:           store,
		WriteQueue:      wq,
		EnvData:         envData,
		EnvPath:         envPath,
		Zones:           zones,
		Explorer:        explorerComp,
		URLBar:          urlBar,
		Params:          params,
		Headers:         headers,
		Body:            body,
		Response:        response,
		TextEd:          textEd,
		Term:            term,
		Repo:            repo,
		Git:             git,
		AI:              aiState,
		Palette:         pal,
		Preview:         mdpreview.New(),
		ExtPanel:        extpanel.New(func(msg tea.Msg) { send(msg) }),
		Focus:           FocusExplorerPane,
		LastFocus:       FocusURLBar,
		ExplorerFocused: true,
		ExplorerWidth:   30,
		Spinner:         spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
	explorerComp.Focused = true
	m.StatusBar = InitialStatus
	m.wireGit()
	m.refreshGit(true)
	pal.OnOpen = func(r palette.Result) {
		if m.attaching {
			m.attachFile(r.Path)
			return
		}
		m.openFileRaw(r.Path)
		if r.Line > 0 && m.ActiveFile == r.Path {
			if doc := m.Store.Get(r.Path); doc != nil && doc.Kind == document.KindHit && m.ViewMode == ViewRunner {
				m.toggleViewMode()
			}
			m.Focus = FocusTextEditor
			m.focusCurrent()
			m.TextEd.GotoLine(r.Line - 1)
		}
	}

	rootDirGlobal = rootDir
	envPathGlobal = envPath

	explorerComp.OnSelect = func(n *explorer.Node) {
		// No-op for now; cursor movement is the visible feedback.
	}
	m.extInit()

	explorerComp.OnActivate = func(n *explorer.Node) {
		if n.Kind == explorer.NodeDir {
			explorerComp.Toggle(n)
		} else {
			m.openFileRaw(n.Path)
		}
	}

	return m
}

func (m *MainScreen) SetSize(w, h int) {
	m.Width = w
	m.Height = h
	if m.ExplorerWidth < 15 {
		m.ExplorerWidth = 15
	}
	if m.ExplorerWidth > w-20 {
		m.ExplorerWidth = w - 20
	}
	m.MainWidth = w - m.ExplorerWidth - 1
	if m.explorerHidden() {
		m.MainWidth = w
	}
	m.sidebarWasHidden = m.explorerHidden()

	// Main column: main pane (MainH rows) + terminal strip (1) + terminal
	// panel when open; footer takes the last row.
	// The panel never squeezes the main pane below minMainH (the smallest
	// runner layout that still fits: 18 rows).
	const minMainH = 18
	termRows := m.termRows()
	if max := h - 3 - minMainH; termRows > max {
		termRows = max
	}
	if termRows < 3 {
		termRows = 3
	}
	m.Term.SetSize(m.MainWidth, termRows)
	m.MainH = h - 3 // top bar + terminal strip + footer
	if m.Term.Open {
		m.MainH -= termRows
	}
	if m.MainH < 10 {
		m.MainH = 10 // ponytail: below this a terminal is too small to lay out anyway
	}

	// Inside the main pane's border: breadcrumb(1) + url bar(3) + tab bar(1)
	// + editor box + response box.
	// The runner stacks breadcrumb(1) + url bar(3) + tab bar(1) + editor box
	// + response box, so its height is exactly 5+avail. Flooring avail above
	// MainH-7 made that sum outgrow the pane, which wrapped the frame and
	// scrolled the terminal; keep the identity and let the frame clip handle
	// panes too short for any layout.
	avail := max(m.MainH-7, 4)
	m.EditorHeight = avail / 3 // outer rows of the editor box (incl. border)
	if m.EditorRows > 0 {
		m.EditorHeight = m.EditorRows
	}
	m.EditorHeight = max(m.EditorHeight, 4)
	m.EditorHeight = min(m.EditorHeight, avail-2)
	respInner := max(avail-m.EditorHeight-2, 0)

	m.Explorer.SetSize(m.ExplorerWidth, h-2)
	m.Git.SetSize(m.MainWidth, m.MainH)
	m.ExtPanel.SetSize(m.MainWidth, m.MainH)
	m.Palette.SetSize(m.MainWidth, m.MainH)
	m.URLBar.SetSize(m.MainWidth - 4)
	m.Params.SetSize(m.MainWidth-4, m.EditorHeight-2)
	m.Headers.SetSize(m.MainWidth-4, m.EditorHeight-2)
	m.Body.SetSize(m.MainWidth-4, m.EditorHeight-2)
	m.Response.SetSize(m.MainWidth-4, respInner)
	m.TextEd.SetSize(m.MainWidth-2, m.MainH-3)
	m.Preview.SetSize(m.MainWidth, m.MainH-1)
	if m.isMarkdown() && m.MdMode == MdSplit {
		left := m.MainWidth / 2
		if m.MdSplitLeft > 0 && m.MainWidth >= 24 {
			left = min(max(m.MdSplitLeft, 12), m.MainWidth-12)
		}
		m.TextEd.SetSize(left-2, m.MainH-3)
		m.Preview.SetSize(m.MainWidth-left, m.MainH-1)
	}
}

// isMarkdown reports whether the active file is a .md document.
func (m *MainScreen) isMarkdown() bool {
	if m.ActiveFile == "" {
		return false
	}
	doc := m.Store.Get(m.ActiveFile)
	return doc != nil && doc.Kind == document.KindMarkdown
}

// termRows is the terminal panel height when open: the dragged height, else
// a third of the window.
func (m *MainScreen) termRows() int {
	r := (m.Height - 2) / 3
	if r < 6 {
		r = 6
	}
	if m.TermRows > 0 {
		r = m.TermRows
	}
	return r
}

func (m *MainScreen) OpenFile(path string) {
	m.openFileRaw(path)
}

// EnterSingleFile turns the window into an editor for one file: no explorer,
// Git or Find, just the file and the terminal. A .hit file still opens in the
// runner, and its <<KEY>> tokens resolve against the nearest hittable/env.json
// above it, so a request sent from here behaves as it does in the project.
func (m *MainScreen) EnterSingleFile(path string) {
	m.SingleFile = true
	if env := findEnv(filepath.Dir(path)); env != "" {
		m.EnvPath, m.HittableDir = env, filepath.Dir(env)
		envPathGlobal = env
		m.EnvData = hitfile.LoadEnv(env)
	}
	m.openFileRaw(path)
	m.SetSize(m.Width, m.Height)
}

// findEnv walks up from dir to the closest hittable/env.json: either dir is
// inside a hittable/ folder, or a parent holds one.
func findEnv(dir string) string {
	for {
		if filepath.Base(dir) == "hittable" {
			if p := filepath.Join(dir, "env.json"); fileExists(p) {
				return p
			}
		}
		if p := filepath.Join(dir, "hittable", "env.json"); fileExists(p) {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// explorerHidden reports whether the sidebar is off screen. The find palette
// takes the full width while it is up, so the file tree gets out of the way
// and comes back when it closes; ExplorerHidden itself stays the user's own
// preference. The Git panel keeps the tree: clicking a file there feeds the
// Blame tab.
func (m *MainScreen) explorerHidden() bool {
	return m.SingleFile || m.ExplorerHidden || m.Palette.Open || m.ExtOpen
}
