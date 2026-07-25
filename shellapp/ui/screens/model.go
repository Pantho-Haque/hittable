// Package screens defines the main screen model and types for the application.
package screens

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/explorer"
	explorerui "github.com/hittable/shellapp/ui/components/explorer"
	"github.com/hittable/shellapp/ui/components/requesteditor"
	"github.com/hittable/shellapp/ui/components/responseviewer"
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

	Explorer *explorerui.ExplorerComponent
	URLBar   *requesteditor.URLBar
	Params   *requesteditor.ParamsTab
	Headers  *requesteditor.HeadersTab
	Body     *requesteditor.BodyTab
	Response *responseviewer.ResponseViewer
	TextEd   *texteditor.TextEditor

	Focus           FocusArea
	LastFocus       FocusArea
	ActiveFile      string
	ViewMode        ViewMode
	ActiveTab       requesteditor.Tab
	Width           int
	Height          int
	ExplorerWidth   int
	MainWidth       int
	Sending         bool
	StatusBar       string
	ExplorerFocused bool
	Dragging        bool
	DragStartX      int
	HoverZone       string
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

	tree, err := explorer.BuildTree(rootDir)
	if err != nil {
		tree = &explorer.FileNode{Name: filepath.Base(rootDir), Path: rootDir, Kind: explorer.KindDir, Expanded: true}
	}

	explorerComp := explorerui.New(tree)
	urlBar := requesteditor.NewURLBar()
	params := requesteditor.NewParamsTab()
	headers := requesteditor.NewHeadersTab()
	body := requesteditor.NewBodyTab()
	response := responseviewer.New()
	textEd := texteditor.New()

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
		Focus:           FocusExplorerPane,
		LastFocus:       FocusURLBar,
		ExplorerFocused: true,
		ExplorerWidth:   30,
	}

	rootDirGlobal = rootDir
	envPathGlobal = envPath

	explorerComp.OnFileOpen = func(path string) {
		m.openFileRaw(path)
	}
	explorerComp.OnFolderOpen = func(path string, expanded bool) {}

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

	m.Explorer.SetSize(m.ExplorerWidth, h-2)
	m.URLBar.SetSize(m.MainWidth - 4)
	m.Params.SetSize(m.MainWidth-4, h/3)
	m.Headers.SetSize(m.MainWidth-4, h/3)
	m.Body.SetSize(m.MainWidth-4, h/3)
	m.Response.SetSize(m.MainWidth-4, h/3)
	m.TextEd.SetSize(m.MainWidth-4, h-4)
}

func (m *MainScreen) OpenFile(path string) {
	m.openFileRaw(path)
}


