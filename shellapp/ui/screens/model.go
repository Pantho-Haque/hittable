// Package screens defines the main screen model and types for the application.
package screens

import (
	"encoding/json"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/gitx"
	"github.com/hittable/shellapp/ui/components/explorer"
	"github.com/hittable/shellapp/ui/components/gitpanel"
	"github.com/hittable/shellapp/ui/components/mdpreview"
	"github.com/hittable/shellapp/ui/components/palette"
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
	Palette  *palette.Palette
	Preview  *mdpreview.Preview
	MdMode   MdMode

	Focus           FocusArea
	LastFocus       FocusArea
	ActiveFile      string
	ViewMode        ViewMode
	ActiveTab       requesteditor.Tab
	Width           int
	Height          int
	ExplorerWidth   int
	MainWidth       int
	MainH           int // rows of the main pane above the terminal strip
	EditorHeight    int
	TermFocused     bool
	GitOpen         bool
	ExplorerHidden  bool
	BlameOn         bool
	lastGitRefresh  time.Time
	blame           []gitx.BlameLine
	gitPolling      bool
	gitKey          string
	Sending         bool
	StatusBar       string
	ExplorerFocused bool
	Dragging        bool
	DragStartX      int
	HoverZone       string
	HelpScroll      int // the help overlay scrolls when it outgrows the pane
	ShowHelp        bool
	Spinner         spinner.Model
}

var rootDirGlobal string
var envPathGlobal string

func NewMainScreen(rootDir string, zones *zone.Manager) *MainScreen {
	hittableDir := filepath.Join(rootDir, "hittable")
	envPath := filepath.Join(hittableDir, "env.json")

	envData := make(map[string]string)
	if data, err := os.ReadFile(envPath); err == nil {
		json.Unmarshal(data, &envData)
	}

	store := document.NewStore()
	wq := document.NewWriteQueue()

	explorerComp := explorer.MustNew(rootDir)
	urlBar := requesteditor.NewURLBar()
	params := requesteditor.NewParamsTab()
	headers := requesteditor.NewHeadersTab()
	body := requesteditor.NewBodyTab()
	response := responseviewer.New()
	textEd := texteditor.New()
	term := terminal.New(rootDir, func(msg tea.Msg) {
		if teaProgram != nil {
			teaProgram.Send(msg)
		}
	})

	repo := gitx.Open(rootDir)
	git := gitpanel.New(repo)
	pal := palette.New(rootDir, func(msg tea.Msg) {
		if teaProgram != nil {
			teaProgram.Send(msg)
		}
	})

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
		Palette:         pal,
		Preview:         mdpreview.New(),
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
	if m.ExplorerHidden {
		m.MainWidth = w
	}

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
	avail := m.MainH - 7
	if avail < 11 {
		avail = 11
	}
	if avail < 8 {
		avail = 8
	}
	m.EditorHeight = avail / 3 // outer rows of the editor box (incl. border)
	if m.EditorHeight < 4 {
		m.EditorHeight = 4
	}
	respInner := avail - m.EditorHeight - 2

	m.Explorer.SetSize(m.ExplorerWidth, h-2)
	m.Git.SetSize(m.MainWidth, m.MainH)
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

// termRows is the terminal panel height when open.
func (m *MainScreen) termRows() int {
	r := (m.Height - 2) / 3
	if r < 6 {
		r = 6
	}
	return r
}

func (m *MainScreen) OpenFile(path string) {
	m.openFileRaw(path)
}
