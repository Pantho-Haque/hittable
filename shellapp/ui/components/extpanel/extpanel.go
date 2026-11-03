// Package extpanel is the Extensions view, laid out like VS Code's: a search
// box over collapsible sections (Installed, Popular, results) on the left,
// each entry with its icon, and the selected extension's page on the right —
// a cover (icon, name, publisher, installs, rating, buttons), a setup
// checklist showing every step between "installed" and "running", then
// Details (the README as a Markdown preview), Features and Changelog.
//
// Everything slow loads in the background and the previous content stays up
// with a spinner until the new content arrives, so the panel never blanks.
// The pane under the mouse takes the focus and the wheel scrolls it, so
// nothing needs a click first.
package extpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/internal/extensions"
	"github.com/hittable/shellapp/ui/components/mdpreview"
	"github.com/hittable/shellapp/ui/theme"
)

// Msg carries a background result into the panel; the screen routes every
// Msg to Apply.
type Msg struct{ apply func(p *Panel) }

// Actions are what the panel asks the screen to do. The screen owns
// installing, the extension host and prompts, so the command palette and
// the panel share one path.
type Actions struct {
	Install     func(spec string) // "id", or "id@latest" to update
	Uninstall   func(id string)
	SetEnabled  func(id string, on bool)
	InstallNode func()
	Trust       func()
	SetTheme    func(extID string)
	RestartHost func()
	ShowLog     func()
	State       func() State
}

// State is what the screen knows that the page shows.
type State struct {
	Installing string // label of the install in progress, "" when none
	Progress   int    // percent, -1 when unknown
	Spinner    string // the current spinner frame

	NodeOK      bool
	NodeVersion string
	NodeWhere   string // "Homebrew · /opt/homebrew/bin/node"
	NodeProblem string // why there is no usable Node ("" when NodeOK)
	NodePlan    string // how it would be installed
	NodeBusy    bool   // being installed now
	NodeLine    string // the install's latest output
	NodeErr     string // the last install's failure

	Trusted    bool
	Host       string            // "running", "starting", "stopped", "" (nothing needs it)
	HostNote   string            // why it stopped
	Activation map[string]string // id -> "activating", "active", "failed: …"
}

type entry struct {
	id, name, publisher, desc, version, icon string
	downloads                                int64
	rating                                   float64
	verified, installed, enabled, code       bool
	section                                  string
}

type page struct {
	detail     *extensions.Detail
	loading    bool
	err        string
	readme     string
	readmeDone bool
	changelog  string
	clDone     bool
	clLoading  bool
	manifest   *extensions.Manifest
	mfLoading  bool
	mfErr      string
}

// Tabs of the page.
const (
	TabDetails = iota
	TabFeatures
	TabChangelog
)

var tabNames = []string{"DETAILS", "FEATURES", "CHANGELOG"}

// Sections of the list.
const (
	secInstalled = "INSTALLED"
	secPopular   = "POPULAR ON OPEN VSX"
	secResults   = "OPEN VSX"
)

type row struct {
	header  bool
	section string
	entry   int // index into list, for entry rows
}

const entryLines = 3 // name · description · publisher and installs

// Panel is the Extensions view.
type Panel struct {
	Width, Height int
	Actions       Actions

	Query       string
	FocusDetail bool
	Tab         int

	installed []extensions.Installed
	popular   []entry
	popDone   bool
	popBusy   bool
	popErr    string

	results   []entry
	searching bool
	searchErr string
	seq       int

	list      []entry
	rows      []row
	collapsed map[string]bool
	cursor    int    // index into rows
	sel       string // id whose page shows (stays put on a header)
	scroll    int    // first visible line of the list
	follow    bool   // keep the cursor in view (off after the wheel scrolls)
	hoverRow  int

	pages        map[string]*page
	dscroll      int
	bodyRowsHint int
	render       struct {
		key   string
		lines []string
	}
	icons *icons

	hits []hit
	send func(tea.Msg)
}

type hit struct {
	x0, x1, y0, y1 int
	act            string
	index          int
}

// New makes the panel; send delivers background results to the program.
func New(send func(tea.Msg)) *Panel {
	return &Panel{pages: map[string]*page{}, collapsed: map[string]bool{}, icons: newIcons(),
		send: send, Width: 100, Height: 30, hoverRow: -1, follow: true, bodyRowsHint: 10}
}

// SetSize sets the panel's outer size.
func (p *Panel) SetSize(w, h int) { p.Width, p.Height = w, h }

// Apply installs a background result.
func (p *Panel) Apply(m Msg) { m.apply(p) }

// Busy reports whether anything is loading, so the spinner keeps turning.
func (p *Panel) Busy() bool {
	if p.searching || p.popBusy {
		return true
	}
	pg := p.pages[p.sel]
	return pg != nil && (pg.loading || pg.clLoading || pg.mfLoading)
}

func (p *Panel) state() State {
	if p.Actions.State != nil {
		return p.Actions.State()
	}
	return State{Progress: -1}
}

// Open refreshes the installed list and loads the popular list once.
func (p *Panel) Open() {
	p.Reload()
	if !p.popDone && !p.popBusy {
		p.loadPopular()
	}
}

// Reload re-reads what is installed and keeps the selection.
func (p *Panel) Reload() {
	all, _ := extensions.List()
	p.installed = all
	p.rebuild()
}

func (p *Panel) installedEntry(id string) (extensions.Installed, bool) {
	for _, e := range p.installed {
		if e.ID == id {
			return e, true
		}
	}
	return extensions.Installed{}, false
}

// SetQuery replaces the search text and starts the search it implies.
func (p *Panel) SetQuery(q string) tea.Cmd {
	p.Query = q
	return p.queryChanged()
}

// rebuild recomputes the sections from what is installed, the popular list
// and the search, keeping the same extension selected.
func (p *Panel) rebuild() {
	var out []entry
	q := strings.TrimSpace(p.Query)
	onlyInstalled := strings.HasPrefix(q, "@installed")
	filter := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(q, "@installed")))
	addInstalled := func() {
		for _, e := range p.installed {
			en := fromInstalled(e)
			if filter != "" && !strings.Contains(strings.ToLower(en.name+" "+en.id+" "+en.desc), filter) {
				continue
			}
			out = append(out, en)
		}
	}
	switch {
	case q == "":
		addInstalled()
		for _, e := range p.popular {
			e.section = secPopular
			p.mark(&e)
			out = append(out, e)
		}
	case onlyInstalled:
		addInstalled()
	default:
		for _, e := range p.results {
			e.section = secResults
			p.mark(&e)
			out = append(out, e)
		}
	}
	p.list = out
	p.layoutRows()
	// Keep the selection where it was, or take the first entry.
	p.cursor = -1
	for i, r := range p.rows {
		if !r.header && p.list[r.entry].id == p.sel {
			p.cursor = i
			break
		}
	}
	if p.cursor < 0 {
		p.cursor = 0
		for i, r := range p.rows {
			if !r.header {
				p.cursor = i
				break
			}
		}
		if e, ok := p.cursorEntry(); ok {
			p.sel = e.id
		} else if len(p.list) == 0 {
			p.sel = ""
		}
	}
	p.follow = true
	p.ensurePage()
}

func (p *Panel) layoutRows() {
	p.rows = p.rows[:0]
	section := ""
	for i, e := range p.list {
		if e.section != section {
			section = e.section
			p.rows = append(p.rows, row{header: true, section: section})
		}
		if !p.collapsed[section] {
			p.rows = append(p.rows, row{section: section, entry: i})
		}
	}
}

func (p *Panel) mark(e *entry) {
	if ie, ok := p.installedEntry(e.id); ok {
		e.installed, e.enabled, e.code = true, ie.Enabled, ie.Manifest.HasCode()
		if e.version == "" {
			e.version = ie.Version
		}
	}
}

func fromInstalled(e extensions.Installed) entry {
	return entry{id: e.ID, name: e.Manifest.Label(), publisher: e.Manifest.Publisher, desc: localized(e.Manifest.Description, e.NLS),
		version: e.Version, installed: true, enabled: e.Enabled, code: e.Manifest.HasCode(), section: secInstalled}
}

func localized(s string, nls map[string]string) string {
	if strings.HasPrefix(s, "%") && strings.HasSuffix(s, "%") {
		if t, ok := nls[strings.Trim(s, "%")]; ok {
			return t
		}
	}
	return s
}

func fromSummary(s extensions.Summary) entry {
	name := s.DisplayName
	if name == "" {
		name = s.ID
	}
	return entry{id: s.ID, name: name, publisher: s.Publisher, desc: s.Description, version: s.Version,
		downloads: s.Downloads, rating: s.Rating, verified: s.Verified, icon: s.Icon}
}

func (p *Panel) cursorEntry() (entry, bool) {
	if p.cursor >= 0 && p.cursor < len(p.rows) && !p.rows[p.cursor].header {
		return p.list[p.rows[p.cursor].entry], true
	}
	return entry{}, false
}

// selected is the extension whose page shows.
func (p *Panel) selected() (entry, bool) {
	for _, e := range p.list {
		if e.id == p.sel {
			return e, true
		}
	}
	return entry{}, false
}

// ---------------------------------------------------------------- loading

func (p *Panel) deliver(f func(p *Panel)) {
	if p.send != nil {
		p.send(Msg{apply: f})
	}
}

func (p *Panel) loadPopular() {
	p.popBusy, p.popErr = true, ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := extensions.Search(ctx, "", 40)
		p.deliver(func(p *Panel) {
			p.popBusy = false
			if err != nil {
				p.popErr = err.Error()
				return
			}
			p.popDone = true
			p.popular = p.popular[:0]
			for _, s := range res {
				p.popular = append(p.popular, fromSummary(s))
			}
			if strings.TrimSpace(p.Query) == "" {
				p.rebuild()
			}
		})
	}()
}

// queryChanged waits for typing to pause, then searches.
func (p *Panel) queryChanged() tea.Cmd {
	p.seq++
	seq := p.seq
	q := strings.TrimSpace(p.Query)
	if q == "" || strings.HasPrefix(q, "@installed") {
		p.searching = false
		p.rebuild()
		return nil
	}
	p.searching = true
	return tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg {
		return Msg{apply: func(p *Panel) {
			if seq == p.seq {
				p.search(seq, q)
			}
		}}
	})
}

func (p *Panel) search(seq int, q string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := extensions.Search(ctx, q, 40)
		p.deliver(func(p *Panel) {
			if seq != p.seq {
				return
			}
			p.searching = false
			if err != nil {
				p.searchErr = err.Error()
				return
			}
			p.searchErr = ""
			p.results = p.results[:0]
			for _, s := range res {
				p.results = append(p.results, fromSummary(s))
			}
			p.collapsed[secResults] = false
			p.sel = ""
			p.rebuild()
		})
	}()
}

// ensurePage starts loading the selected extension's page. An installed
// one shows its local README at once; the registry fills in the rest.
func (p *Panel) ensurePage() {
	id := p.sel
	if id == "" {
		return
	}
	pg := p.pages[id]
	if pg == nil {
		pg = &page{}
		p.pages[id] = pg
		if ie, ok := p.installedEntry(id); ok {
			pg.readme, pg.readmeDone = extensions.LocalDoc(ie, "readme"), true
			if cl := extensions.LocalDoc(ie, "changelog"); cl != "" {
				pg.changelog, pg.clDone = cl, true
			}
		}
		pg.loading = true
		needReadme := !pg.readmeDone
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			d, err := extensions.Details(ctx, id)
			readme := ""
			var rerr error
			if err == nil && d.Readme != "" && needReadme {
				readme, rerr = extensions.FetchText(ctx, d.Readme)
			}
			p.deliver(func(p *Panel) {
				pg.loading = false
				if err != nil {
					if _, ok := p.installedEntry(id); !ok {
						pg.err = err.Error()
					}
					if needReadme {
						pg.readmeDone = true
					}
					return
				}
				pg.detail = &d
				if needReadme {
					pg.readme, pg.readmeDone = readme, true
					if rerr != nil {
						pg.readme = "_The README could not be loaded: " + rerr.Error() + "_"
					}
				}
				p.render.key = ""
			})
		}()
	}
	p.ensureTab()
}

func (p *Panel) ensureTab() {
	id := p.sel
	pg := p.pages[id]
	if pg == nil {
		return
	}
	switch p.Tab {
	case TabChangelog:
		if pg.clDone || pg.clLoading || pg.detail == nil || pg.detail.Changelog == "" {
			return
		}
		pg.clLoading = true
		u := pg.detail.Changelog
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			text, err := extensions.FetchText(ctx, u)
			p.deliver(func(p *Panel) {
				pg.clLoading, pg.clDone = false, true
				pg.changelog = text
				if err != nil {
					pg.changelog = "_The changelog could not be loaded: " + err.Error() + "_"
				}
				p.render.key = ""
			})
		}()
	case TabFeatures:
		if _, ok := p.installedEntry(id); ok || pg.manifest != nil || pg.mfLoading || pg.detail == nil || pg.detail.ManifestURL == "" {
			return
		}
		pg.mfLoading = true
		u := pg.detail.ManifestURL
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			m, err := extensions.FetchManifest(ctx, u)
			p.deliver(func(p *Panel) {
				pg.mfLoading = false
				if err != nil {
					pg.mfErr = err.Error()
					return
				}
				pg.manifest = &m
				p.render.key = ""
			})
		}()
	}
}

// ---------------------------------------------------------------- keys

// HandleKey handles a key while the panel is open. It reports whether the
// panel should close (esc with nothing left to clear).
func (p *Panel) HandleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	key := msg.String()
	switch key {
	case "esc":
		if p.FocusDetail {
			p.FocusDetail = false
			return nil, false
		}
		if p.Query != "" {
			p.Query = ""
			return p.queryChanged(), false
		}
		return nil, true
	case "tab", "shift+tab":
		p.FocusDetail = !p.FocusDetail
		return nil, false
	}
	if p.FocusDetail {
		p.detailKey(key)
		return nil, false
	}
	switch key {
	case "up", "ctrl+p", "ctrl+k":
		p.moveCursor(-1)
	case "down", "ctrl+n", "ctrl+j":
		p.moveCursor(1)
	case "pgup":
		p.moveCursor(-max(p.listRows()/entryLines, 1))
	case "pgdown":
		p.moveCursor(max(p.listRows()/entryLines, 1))
	case "left":
		p.setCollapsed(true)
	case "right":
		p.setCollapsed(false)
	case "enter":
		if p.cursor < len(p.rows) && p.rows[p.cursor].header {
			p.toggleSection(p.rows[p.cursor].section)
			return nil, false
		}
		e, ok := p.cursorEntry()
		if !ok {
			return nil, false
		}
		if !e.installed {
			p.do("install")
		} else {
			p.FocusDetail = true
		}
	case "backspace":
		if r := []rune(p.Query); len(r) > 0 {
			p.Query = string(r[:len(r)-1])
			return p.queryChanged(), false
		}
	case "ctrl+u":
		p.Query = ""
		return p.queryChanged(), false
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			p.Query += string(msg.Runes)
			return p.queryChanged(), false
		}
	}
	return nil, false
}

func (p *Panel) detailKey(key string) {
	switch key {
	case "up", "k":
		p.dscroll--
	case "down", "j":
		p.dscroll++
	case "pgup":
		p.dscroll -= p.bodyRowsHint - 1
	case "pgdown", " ":
		p.dscroll += p.bodyRowsHint - 1
	case "g", "home":
		p.dscroll = 0
	case "G", "end":
		p.dscroll = 1 << 20
	case "1", "2", "3":
		p.setTab(int(key[0] - '1'))
	case "left", "h":
		p.setTab((p.Tab + 2) % 3)
	case "right", "l":
		p.setTab((p.Tab + 1) % 3)
	case "i", "enter":
		if e, ok := p.selected(); ok && !e.installed {
			p.do("install")
		}
	case "u":
		p.do("uninstall")
	case "e":
		p.do("toggle")
	case "U":
		p.do("update")
	}
}

func (p *Panel) setTab(t int) {
	if t != p.Tab {
		p.Tab, p.dscroll = t, 0
		p.ensureTab()
	}
}

// moveCursor steps through headers and entries; landing on an entry shows
// its page.
func (p *Panel) moveCursor(d int) {
	if len(p.rows) == 0 {
		return
	}
	p.cursor = max(0, min(len(p.rows)-1, p.cursor+d))
	p.follow = true
	p.selectCursor()
}

func (p *Panel) selectCursor() {
	if e, ok := p.cursorEntry(); ok && e.id != p.sel {
		p.sel, p.dscroll = e.id, 0
		p.ensurePage()
	}
}

func (p *Panel) toggleSection(s string) {
	p.collapsed[s] = !p.collapsed[s]
	p.relayoutKeeping(s)
}

func (p *Panel) setCollapsed(v bool) {
	if p.cursor >= len(p.rows) {
		return
	}
	s := p.rows[p.cursor].section
	if p.collapsed[s] != v {
		p.collapsed[s] = v
		p.relayoutKeeping(s)
	}
}

// relayoutKeeping redoes the rows after a section folds or opens, with the
// cursor on that section's header.
func (p *Panel) relayoutKeeping(section string) {
	p.layoutRows()
	for i, r := range p.rows {
		if r.header && r.section == section {
			p.cursor = i
		}
	}
	p.follow = true
}

// do runs a button's action on the selected extension.
func (p *Panel) do(act string) {
	e, ok := p.selected()
	a := p.Actions
	st := p.state()
	switch act {
	case "install":
		if ok && !e.installed && a.Install != nil && st.Installing == "" {
			a.Install(e.id)
		}
	case "uninstall":
		if ok && e.installed && a.Uninstall != nil {
			a.Uninstall(e.id)
		}
	case "toggle":
		if ok && e.installed && a.SetEnabled != nil {
			a.SetEnabled(e.id, !e.enabled)
		}
	case "update":
		if ok && e.installed && a.Install != nil && st.Installing == "" {
			a.Install(e.id + "@latest")
		}
	case "node":
		if a.InstallNode != nil && !st.NodeBusy {
			a.InstallNode()
		}
	case "trust":
		if a.Trust != nil {
			a.Trust()
		}
	case "theme":
		if ok && a.SetTheme != nil {
			a.SetTheme(e.id)
		}
	case "restart":
		if a.RestartHost != nil {
			a.RestartHost()
		}
	case "log":
		if a.ShowLog != nil {
			a.ShowLog()
		}
	}
}

// ---------------------------------------------------------------- mouse

// HandleMouse takes coordinates relative to the panel's top-left. The pane
// under the pointer takes the keyboard (no click needed), and the wheel
// scrolls whichever pane it is over without changing the selection.
func (p *Panel) HandleMouse(msg tea.MouseMsg) {
	inLeft := msg.X < p.leftWidth()
	switch msg.Type {
	case tea.MouseMotion:
		p.FocusDetail = !inLeft
		p.hoverRow = -1
		if inLeft {
			for _, h := range p.hits {
				if h.act == "row" && msg.Y >= h.y0 && msg.Y <= h.y1 {
					p.hoverRow = h.index
				}
			}
		}
		return
	case tea.MouseWheelUp, tea.MouseWheelDown:
		d := 1
		if msg.Type == tea.MouseWheelUp {
			d = -1
		}
		p.FocusDetail = !inLeft
		if inLeft {
			p.scroll += d * entryLines // one entry per notch
			p.follow = false
		} else {
			p.dscroll += d * 3
		}
		p.hoverRow = -1 // the row under the pointer just moved
		return
	}
	if msg.Type != tea.MouseLeft || msg.Action != tea.MouseActionPress {
		return
	}
	for _, h := range p.hits {
		if msg.X >= h.x0 && msg.X < h.x1 && msg.Y >= h.y0 && msg.Y <= h.y1 {
			switch h.act {
			case "row":
				p.FocusDetail = false
				p.cursor = h.index
				if p.rows[h.index].header {
					p.toggleSection(p.rows[h.index].section)
				} else {
					p.selectCursor()
				}
				p.follow = false
			case "search":
				p.FocusDetail = false
			case "tab":
				p.setTab(h.index)
			default:
				p.do(h.act)
			}
			return
		}
	}
}

// ---------------------------------------------------------------- view

func (p *Panel) leftWidth() int {
	return max(min(p.Width*2/5, 54), min(30, p.Width/2))
}

// listRows is the list's height in lines: border(2) + search(1) + rule(1) + status(1).
func (p *Panel) listRows() int { return max(p.Height-5, 1) }

// View renders the panel.
func (p *Panel) View() string {
	p.hits = p.hits[:0]
	lw := p.leftWidth()
	view := lipgloss.JoinHorizontal(lipgloss.Top, p.viewList(lw), p.viewPage(lw, p.Width-lw))
	// Image uploads go ahead of the first row: zero width, and written
	// before any placeholder that shows them.
	return p.icons.uploads() + view
}

var (
	errStyle  = lipgloss.NewStyle().Foreground(theme.ErrorColor)
	okStyle   = lipgloss.NewStyle().Foreground(theme.AccentColor)
	warnStyle = lipgloss.NewStyle().Foreground(theme.WarningColor)
	bold      = lipgloss.NewStyle().Bold(true)
)

func (p *Panel) viewList(w int) string {
	inner := w - 2
	st := p.state()
	var lines []string
	caret := ""
	if !p.FocusDetail {
		caret = "▏"
	}
	if p.Query == "" {
		lines = append(lines, theme.PromptStyle.Render("  "+caret)+theme.MutedStyle.Render("Search Open VSX · @installed"))
	} else {
		lines = append(lines, theme.PromptStyle.Render("  "+p.Query+caret))
	}
	p.hits = append(p.hits, hit{x0: 0, x1: w, y0: 1, y1: 1, act: "search"})
	lines = append(lines, theme.MutedStyle.Render(strings.Repeat("─", inner)))

	var status string
	switch {
	case p.searching:
		status = st.Spinner + " Searching Open VSX for “" + strings.TrimSpace(p.Query) + "”…"
	case p.searchErr != "" && p.Query != "":
		status = errStyle.Render("✖ " + p.searchErr)
	case p.Query == "" && p.popBusy:
		status = st.Spinner + " Loading popular extensions…"
	case p.Query == "" && p.popErr != "":
		status = errStyle.Render("✖ offline? " + p.popErr)
	case p.Query != "" && len(p.list) == 0:
		status = "No extensions match “" + strings.TrimSpace(p.Query) + "”"
	default:
		status = fmt.Sprintf("%d extensions · ⏎ on a section folds it", len(p.list))
	}
	lines = append(lines, theme.MutedStyle.Render(ansi.Truncate(" "+status, inner, "…")))

	// Where each row starts, in lines; only the rows inside the window
	// [scroll, scroll+n) are drawn, so a frame costs the same with 40
	// entries as with 4 and the wheel never queues up behind rendering.
	start := make([]int, len(p.rows)+1)
	for i, r := range p.rows {
		h := entryLines
		if r.header {
			h = 1
		}
		start[i+1] = start[i] + h
	}
	total := start[len(p.rows)]
	n := p.listRows()
	if p.follow && p.cursor < len(p.rows) {
		first, last := start[p.cursor], start[p.cursor+1]-1
		if first < p.scroll {
			p.scroll = first
		}
		if last >= p.scroll+n {
			p.scroll = last - n + 1
		}
	}
	p.scroll = max(0, min(p.scroll, max(total-n, 0)))
	window := make([]string, 0, n)
	rowOf := make([]int, 0, n)
	for i, r := range p.rows {
		if start[i+1] <= p.scroll {
			continue
		}
		if start[i] >= p.scroll+n {
			break
		}
		var ls []string
		if r.header {
			ls = []string{p.headerLine(r, inner, i)}
		} else {
			ls = p.entryLines(p.list[r.entry], inner, i)
		}
		for j, l := range ls {
			if k := start[i] + j; k >= p.scroll && k < p.scroll+n {
				window = append(window, l)
				rowOf = append(rowOf, i)
			}
		}
	}
	sb := theme.VScrollbar(n, total, p.scroll)
	for i := 0; i < n; i++ {
		text, rowIdx := "", -1
		if i < len(window) {
			text, rowIdx = window[i], rowOf[i]
		}
		y := 1 + len(lines)
		if sb != nil {
			if pad := inner - 1 - lipgloss.Width(text); pad > 0 {
				text += strings.Repeat(" ", pad)
			}
			text = ansi.Truncate(text, inner-1, "") + sb[i]
		}
		lines = append(lines, text)
		if rowIdx >= 0 {
			p.hits = append(p.hits, hit{x0: 0, x1: w, y0: y, y1: y, act: "row", index: rowIdx})
		}
	}
	box := theme.UnfocusedBorderStyle
	if !p.FocusDetail {
		box = theme.FocusedBorderStyle
	}
	return box.Width(inner).Height(p.Height - 2).MaxHeight(p.Height).Render(strings.Join(lines, "\n"))
}

func (p *Panel) headerLine(r row, inner, i int) string {
	n := 0
	for _, e := range p.list {
		if e.section == r.section {
			n++
		}
	}
	arrow := "▾"
	if p.collapsed[r.section] {
		arrow = "▸"
	}
	text := fmt.Sprintf(" %s %s  %d", arrow, r.section, n)
	w := inner - 1
	switch {
	case i == p.cursor && !p.FocusDetail:
		return theme.CursorFocusedStyle.Width(w).Render(text)
	case i == p.hoverRow:
		return theme.HoverStyle.Bold(true).Width(w).Render(text)
	}
	return theme.GitHeaderStyle.Render(text)
}

// entryLines draws one entry: its icon beside three lines of text.
func (p *Panel) entryLines(e entry, inner, i int) []string {
	bg, st := "", lipgloss.NewStyle()
	switch {
	case e.id == p.sel && i == p.cursor && !p.FocusDetail:
		bg, st = "#44475a", lipgloss.NewStyle().Background(lipgloss.Color("#44475a"))
	case e.id == p.sel:
		bg, st = "#343746", lipgloss.NewStyle().Background(lipgloss.Color("#343746"))
	case i == p.hoverRow:
		bg, st = "#3a3d4d", lipgloss.NewStyle().Background(lipgloss.Color("#3a3d4d"))
	}
	const iconW = 6
	icon := p.icon(e.id, p.iconSource(e), iconW, entryLines, bg)
	textW := inner - 1 - iconW - 3 // gutter, icon, gaps, scrollbar
	badge := ""
	switch {
	case e.installed && !e.enabled:
		badge = "  ⏸ disabled"
	case e.installed:
		badge = "  ✓"
	}
	right := ""
	if e.version != "" {
		right = "v" + e.version
	}
	name := ansi.Truncate(e.name, max(textW-lipgloss.Width(badge)-lipgloss.Width(right)-1, 6), "…")
	l1 := padBetween(name+badge, right, textW)
	desc := e.desc
	if desc == "" {
		desc = e.id
	}
	l2 := ansi.Truncate(desc, textW, "…")
	pub := e.publisher
	if e.verified {
		pub += " ✓"
	}
	var meta []string
	if e.downloads > 0 {
		meta = append(meta, "⬇ "+human(e.downloads))
	}
	if e.rating > 0 {
		meta = append(meta, fmt.Sprintf("★ %.1f", e.rating))
	}
	l3 := padBetween(ansi.Truncate(pub, textW/2, "…"), strings.Join(meta, "  "), textW)

	nameSt := st.Bold(true).Foreground(theme.TextColor)
	if e.id == p.sel {
		nameSt = nameSt.Foreground(theme.PrimaryColor)
	}
	gap := st.Render(" ")
	return []string{
		gap + icon[0] + st.Render("  ") + nameSt.Width(textW).Render(l1),
		gap + icon[1] + st.Render("  ") + st.Foreground(theme.SubtextColor).Width(textW).Render(l2),
		gap + icon[2] + st.Render("  ") + st.Foreground(theme.MutedColor).Width(textW).Render(l3),
	}
}

func padBetween(left, right string, w int) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return ansi.Truncate(left, w, "…")
	}
	return left + strings.Repeat(" ", gap) + right
}

// ---------------------------------------------------------------- the page

func (p *Panel) viewPage(x0, w int) string {
	inner := w - 2
	box := theme.UnfocusedBorderStyle
	if p.FocusDetail {
		box = theme.FocusedBorderStyle
	}
	e, ok := p.selected()
	if !ok {
		msg := theme.MutedStyle.Render("\n  Search for an extension on the left.\n\n  Themes, snippets and language definitions work right away.\n  Extensions with code run in Node.js — the one on this machine, or\n  one installed for you in a single step.")
		return box.Width(inner).Height(p.Height - 2).MaxHeight(p.Height).Render(msg)
	}
	st := p.state()
	pg := p.pages[e.id]
	if pg == nil {
		pg = &page{}
	}
	d := pg.detail
	ie, installed := p.installedEntry(e.id)

	var out []string
	y := func() int { return 1 + len(out) } // panel row of the next line
	buttonsAt := func(xStart int) func(label, act string, primary bool) string {
		bx := xStart
		return func(label, act string, primary bool) string {
			s := theme.TopBarButtonStyle.Render(" " + label + " ")
			if primary {
				s = theme.TopBarActiveStyle.Render(" " + label + " ")
			}
			p.hits = append(p.hits, hit{x0: bx, x1: bx + lipgloss.Width(s), y0: y(), y1: y(), act: act})
			bx += lipgloss.Width(s) + 1
			return s
		}
	}

	// ---- cover: a 12×12-pixel icon beside the title block
	name, desc, publisher := e.name, e.desc, e.publisher
	version := e.version
	downloads, rating, reviews, verified := e.downloads, e.rating, 0, e.verified
	if d != nil {
		if d.DisplayName != "" {
			name = d.DisplayName
		}
		if d.Description != "" {
			desc = d.Description
		}
		if d.PublisherName != "" {
			publisher = d.PublisherName
		}
		downloads, rating, reviews, verified = d.Downloads, d.Rating, d.Reviews, d.Verified
	}
	if installed {
		version = ie.Version
	}
	const coverIcon = 12
	icon := p.icon(e.id, p.iconSource(e), coverIcon, 6, "")
	textW := inner - coverIcon - 4
	title := bold.Foreground(theme.TextColor).Render(ansi.Truncate(name, max(textW-14, 10), "…"))
	if version != "" {
		title += theme.MutedStyle.Render("  v" + version)
	}
	pubLine := publisher
	if verified {
		pubLine += lipgloss.NewStyle().Foreground(theme.PrimaryColor).Render(" ✓")
	}
	pubLine += theme.MutedStyle.Render("  ·  " + e.id)
	var stats []string
	if downloads > 0 {
		stats = append(stats, "⬇ "+human(downloads)+" installs")
	}
	if rating > 0 {
		s := stars(rating) + fmt.Sprintf(" %.1f", rating)
		if reviews > 0 {
			s += fmt.Sprintf(" (%d)", reviews)
		}
		stats = append(stats, s)
	}
	if d != nil && d.License != "" {
		stats = append(stats, d.License)
	}
	if d != nil && len(d.Updated) >= 10 {
		stats = append(stats, "updated "+d.Updated[:10])
	}
	statLine := theme.MutedStyle.Render(strings.Join(stats, "  ·  "))
	if pg.loading && d == nil {
		statLine = theme.MutedStyle.Render(st.Spinner + " loading from Open VSX…")
	}
	cover := []string{title, ansi.Truncate(pubLine, textW, "…"), ansi.Truncate(statLine, textW, "…"), ""}
	cover = append(cover, wrap(desc, textW, 2)...)
	for len(cover) < 6 {
		cover = append(cover, "")
	}
	for i := 0; i < 6; i++ {
		out = append(out, " "+icon[i]+"   "+cover[i])
	}

	// ---- buttons, a blank line under the cover
	out = append(out, "")
	btn := buttonsAt(x0 + 2)
	var btns []string
	installingThis := st.Installing != "" && strings.Contains(st.Installing, e.id)
	switch {
	case installingThis:
		label := st.Spinner + " Installing"
		if st.Progress >= 0 {
			label += fmt.Sprintf(" %d%%", st.Progress)
		}
		btns = append(btns, theme.TopBarActiveStyle.Render(" "+label+" "))
	case !installed:
		btns = append(btns, btn("Install", "install", true))
	default:
		btns = append(btns, btn("Uninstall", "uninstall", false))
		if ie.Enabled {
			btns = append(btns, btn("Disable", "toggle", false))
		} else {
			btns = append(btns, btn("Enable", "toggle", true))
		}
		if d != nil && d.Version != "" && newerVersion(d.Version, ie.Version) && ie.Source != "vsix" {
			btns = append(btns, btn("Update to "+d.Version, "update", true))
		}
		if len(ie.Manifest.Contributes.Themes) > 0 {
			btns = append(btns, btn("Set Color Theme", "theme", false))
		}
	}
	out = append(out, " "+strings.Join(btns, " "))

	// ---- setup checklist: every step between "installed" and "running"
	switch {
	case installed || installingThis:
		out = append(out, "")
		out = append(out, p.setupLines(e, ie, installed, installingThis, st, x0, inner, func() int { return 1 + len(out) })...)
	case pg.err != "":
		out = append(out, "", " "+errStyle.Render("✖ "+pg.err))
	case d != nil && d.Engine != "" && !extensions.EngineAllows(d.Engine, extensions.APIVersion):
		out = append(out, "", " "+theme.MutedStyle.Render("ℹ Its latest version needs VS Code "+d.Engine+"; the newest compatible one installs instead"))
	default:
		out = append(out, "", " "+theme.MutedStyle.Render("⏎ or Install · checked against Open VSX's SHA-256 and signature"))
	}

	// ---- tabs
	out = append(out, theme.MutedStyle.Render(strings.Repeat("─", inner)))
	tx := x0 + 2
	var tabs []string
	for i, n := range tabNames {
		s := theme.TabInactiveStyle.Render(" " + n + " ")
		if i == p.Tab {
			s = theme.TabActiveStyle.Render(" " + n + " ")
		}
		p.hits = append(p.hits, hit{x0: tx, x1: tx + lipgloss.Width(s), y0: y(), y1: y(), act: "tab", index: i})
		tx += lipgloss.Width(s) + 1
		tabs = append(tabs, s)
	}
	out = append(out, " "+strings.Join(tabs, " "))

	// ---- body
	rows := max(p.Height-2-len(out), 1)
	p.bodyRowsHint = rows
	body := p.bodyLines(e, pg, inner-3, st.Spinner)
	p.dscroll = max(0, min(p.dscroll, max(len(body)-rows, 0)))
	sb := theme.VScrollbar(rows, len(body), p.dscroll)
	for i := 0; i < rows; i++ {
		line := ""
		if p.dscroll+i < len(body) {
			line = " " + body[p.dscroll+i]
		}
		line = ansi.Truncate(line, inner-1, "")
		if sb != nil {
			if pad := inner - 1 - lipgloss.Width(line); pad > 0 {
				line += strings.Repeat(" ", pad)
			}
			line += sb[i]
		}
		out = append(out, line)
	}
	for i := range out {
		out[i] = ansi.Truncate(out[i], inner, "")
	}
	return box.Width(inner).Height(p.Height - 2).MaxHeight(p.Height).Render(strings.Join(out, "\n"))
}

// setupLines is the checklist from download to "active". Each step is done
// (✓), in progress (spinner), waiting on the user (○ with the button that
// does it) or failed (✖ with what to do).
func (p *Panel) setupLines(e entry, ie extensions.Installed, installed, installing bool, st State, x0, inner int, y func() int) []string {
	var out []string
	add := func(mark, text string, buttons ...[2]string) {
		line := " " + mark + " " + text
		bx := x0 + 1 + lipgloss.Width(line)
		row := y() + len(out)
		for _, b := range buttons {
			s := theme.TopBarActiveStyle.Render(" " + b[0] + " ")
			p.hits = append(p.hits, hit{x0: bx + 2, x1: bx + 2 + lipgloss.Width(s), y0: row, y1: row, act: b[1]})
			line += "  " + s
			bx += 2 + lipgloss.Width(s)
		}
		out = append(out, ansi.Truncate(line, inner, "…"))
	}
	done := okStyle.Render("✓")
	todo := warnStyle.Render("○")
	fail := errStyle.Render("✖")
	spin := lipgloss.NewStyle().Foreground(theme.PrimaryColor).Render(st.Spinner)
	wait := theme.MutedStyle.Render("·")
	dim := theme.MutedStyle.Render

	out = append(out, " "+theme.GitHeaderStyle.Render("SETUP"))
	if installing {
		label := "Downloading and verifying"
		if st.Progress >= 0 {
			label += fmt.Sprintf(" · %d%%", st.Progress)
		}
		add(spin, label)
		add(wait, dim("Install"))
		add(wait, dim("Run"))
		return out
	}
	sig := "Downloaded · signature verified"
	if !ie.Signed {
		sig = "Installed from a .vsix file (no registry signature)"
		if ie.Source == "openvsx" {
			sig = "Downloaded · not signed by the registry"
		}
	}
	add(done, sig+dim("  v"+ie.Version))
	if !ie.Enabled {
		add(todo, "Disabled", [2]string{"Enable", "toggle"})
		return out
	}
	if !ie.Manifest.HasCode() {
		var what []string
		c := ie.Manifest.Contributes
		if len(c.Themes) > 0 {
			what = append(what, "themes")
		}
		if len(c.Snippets) > 0 {
			what = append(what, "snippets")
		}
		if len(c.Languages) > 0 || len(c.Grammars) > 0 {
			what = append(what, "language definitions")
		}
		text := "Ready — no code to run"
		if len(what) > 0 {
			text += " (" + strings.Join(what, ", ") + ")"
		}
		add(done, text)
		return out
	}

	// Node.js
	switch {
	case st.NodeOK:
		add(done, "Node.js "+st.NodeVersion+dim("  "+st.NodeWhere))
	case st.NodeBusy:
		line := st.NodeLine
		if st.Progress >= 0 {
			line = fmt.Sprintf("downloading %d%%", st.Progress)
		}
		add(spin, "Installing Node.js"+dim("  "+line))
	case st.NodeErr != "":
		add(fail, "Node.js install failed: "+st.NodeErr, [2]string{"Try again", "node"})
	default:
		add(todo, st.NodeProblem)
		add(" ", dim("→ "+st.NodePlan), [2]string{"Install Node.js", "node"})
	}
	// Trust
	if st.Trusted {
		add(done, "Folder trusted to run extension code")
	} else {
		add(todo, "This folder is not trusted to run extension code", [2]string{"Trust this folder", "trust"})
	}
	// Host and activation
	ready := st.NodeOK && st.Trusted
	act := st.Activation[e.id]
	switch {
	case !ready:
		add(wait, dim("Extension host — waits for the steps above"))
	case st.Host == "stopped":
		add(fail, "Extension host stopped: "+st.HostNote, [2]string{"Restart", "restart"}, [2]string{"Log", "log"})
	case st.Host == "running":
		add(done, "Extension host running")
	default:
		add(spin, "Starting the extension host")
	}
	switch {
	case !ready || st.Host != "running":
	case act == "active":
		add(done, okStyle.Render("Active"))
	case act == "activating":
		add(spin, "Activating")
	case strings.HasPrefix(act, "failed"):
		add(fail, "Failed to start: "+strings.TrimPrefix(act, "failed: "), [2]string{"Log", "log"})
	default:
		add(todo, "Waiting — "+activationHint(ie.Manifest))
	}
	return out
}

// activationHint says, in plain words, what starts an extension.
func activationHint(m extensions.Manifest) string {
	for _, ev := range m.Activation {
		switch {
		case ev == "*" || ev == "onStartupFinished":
			return "starts with hittable"
		case strings.HasPrefix(ev, "onLanguage:"):
			return "starts when you open a " + strings.TrimPrefix(ev, "onLanguage:") + " file"
		case strings.HasPrefix(ev, "workspaceContains:"):
			return "starts in folders containing " + strings.TrimPrefix(ev, "workspaceContains:")
		case strings.HasPrefix(ev, "onCommand:"):
			return "starts when you run one of its commands (alt+p)"
		}
	}
	if len(m.Contributes.Languages) > 0 {
		return "starts when you open a " + m.Contributes.Languages[0].ID + " file"
	}
	if len(m.Contributes.Commands) > 0 {
		return "starts when you run one of its commands (alt+p)"
	}
	return "starts when it is needed"
}

// wrap breaks s into at most maxLines lines of width w.
func wrap(s string, w, maxLines int) []string {
	var lines []string
	cur := ""
	for _, wd := range strings.Fields(s) {
		switch {
		case cur == "":
			cur = wd
		case lipgloss.Width(cur)+1+lipgloss.Width(wd) <= w:
			cur += " " + wd
		default:
			lines = append(lines, cur)
			cur = wd
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = ansi.Truncate(lines[maxLines-1], w-1, "") + "…"
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], w, "…")
	}
	return lines
}

// bodyLines renders the active tab with the app's Markdown preview, cached
// per (extension, tab, width, content): glamour is far too slow per frame.
func (p *Panel) bodyLines(e entry, pg *page, width int, spin string) []string {
	var md, loading string
	switch p.Tab {
	case TabDetails:
		md = pg.readme
		if !pg.readmeDone {
			loading = "loading the README…"
		} else if strings.TrimSpace(md) == "" {
			md = "_This extension has no README._"
		}
	case TabFeatures:
		m := pg.manifest
		if ie, ok := p.installedEntry(e.id); ok {
			m = &ie.Manifest
		}
		switch {
		case m != nil:
			md = featuresMarkdown(*m)
		case pg.mfErr != "":
			md = "_Could not read the manifest: " + pg.mfErr + "_"
		default:
			loading = "loading what it contributes…"
		}
	case TabChangelog:
		md = pg.changelog
		switch {
		case pg.clDone:
		case pg.detail != nil && pg.detail.Changelog == "":
			md = "_No changelog published._"
		default:
			loading = "loading the changelog…"
		}
	}
	if loading != "" {
		return []string{"", spin + " " + theme.MutedStyle.Render(loading)}
	}
	key := fmt.Sprintf("%s|%d|%d|%d", e.id, p.Tab, width, len(md))
	if p.render.key != key {
		p.render.key = key
		p.render.lines = strings.Split(strings.TrimRight(mdpreview.Render(CleanMarkdown(md), width), "\n"), "\n")
	}
	return p.render.lines
}

var (
	reComment  = regexp.MustCompile(`(?s)<!--.*?-->`)
	reImgTag   = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	reMdImage  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reMdImgRef = regexp.MustCompile(`!\[[^\]]*\]\[[^\]]*\]`)
	reEmptyLnk = regexp.MustCompile(`\[\s*\]\([^)]*\)`)
	reBr       = regexp.MustCompile(`(?i)<br\s*/?>`)
	reHeading  = regexp.MustCompile(`(?is)<h([1-6])\b[^>]*>(.*?)</h[1-6]>`)
	reLink     = regexp.MustCompile(`(?is)<a\b[^>]*href\s*=\s*["']([^"']*)["'][^>]*>(.*?)</a>`)
	reStrong   = regexp.MustCompile(`(?is)<(b|strong)\b[^>]*>(.*?)</(b|strong)>`)
	reEm       = regexp.MustCompile(`(?is)<(i|em)\b[^>]*>(.*?)</(i|em)>`)
	reCode     = regexp.MustCompile(`(?is)<code\b[^>]*>(.*?)</code>`)
	reSummary  = regexp.MustCompile(`(?is)<summary\b[^>]*>(.*?)</summary>`)
	reLi       = regexp.MustCompile(`(?i)<li\b[^>]*>`)
	reCell     = regexp.MustCompile(`(?i)</t[dh]>\s*<t[dh]\b[^>]*>`)
	reRowEnd   = regexp.MustCompile(`(?i)</tr>`)
	reTag      = regexp.MustCompile(`(?s)</?[a-zA-Z][^>]*>`)
	reBlank    = regexp.MustCompile(`\n[ \t]*\n([ \t]*\n)+`)
)

// CleanMarkdown turns a README written for a browser into Markdown that
// reads well in a terminal: badges and images go (a terminal cannot show
// them), inline HTML becomes its Markdown equivalent or plain text, and the
// blank space HTML layout leaves behind collapses. Code blocks are kept as
// they are.
func CleanMarkdown(md string) string {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	parts := strings.Split(md, "```")
	for i := 0; i < len(parts); i += 2 { // even parts are outside fences
		parts[i] = cleanProse(parts[i])
	}
	md = strings.Join(parts, "```")
	return strings.TrimSpace(md) + "\n"
}

func cleanProse(md string) string {
	md = reComment.ReplaceAllString(md, "")
	md = reImgTag.ReplaceAllString(md, "")
	md = reMdImgRef.ReplaceAllString(md, "")
	md = reMdImage.ReplaceAllString(md, "")
	md = reEmptyLnk.ReplaceAllString(md, "") // a badge's link, now holding nothing
	md = reBr.ReplaceAllString(md, "\n")
	md = reLink.ReplaceAllStringFunc(md, func(s string) string {
		m := reLink.FindStringSubmatch(s)
		text := strings.TrimSpace(reTag.ReplaceAllString(m[2], ""))
		if text == "" {
			return ""
		}
		return "[" + text + "](" + m[1] + ")"
	})
	md = reStrong.ReplaceAllString(md, "**$2**")
	md = reEm.ReplaceAllString(md, "_${2}_")
	md = reCode.ReplaceAllString(md, "`$1`")
	md = reHeading.ReplaceAllStringFunc(md, func(s string) string {
		m := reHeading.FindStringSubmatch(s)
		n := int(m[1][0] - '0')
		return "\n\n" + strings.Repeat("#", n) + " " + strings.TrimSpace(reTag.ReplaceAllString(m[2], "")) + "\n\n"
	})
	md = reSummary.ReplaceAllString(md, "\n**$1**\n")
	md = reLi.ReplaceAllString(md, "\n- ")
	md = reCell.ReplaceAllString(md, " · ")
	md = reRowEnd.ReplaceAllString(md, "\n")
	md = reTag.ReplaceAllString(md, "")
	md = html.UnescapeString(md)
	// Lines that held only HTML layout are now runs of spaces; HTML-indented
	// text would read as a code block, so its indentation goes too.
	lines := strings.Split(md, "\n")
	for i, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case t == "":
			lines[i] = ""
		case strings.HasPrefix(l, "    ") && !strings.HasPrefix(t, "-") && !strings.HasPrefix(t, "*") && !startsNumbered(t):
			lines[i] = t
		}
	}
	md = strings.Join(lines, "\n")
	return reBlank.ReplaceAllString(md, "\n\n")
}

func startsNumbered(s string) bool {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i > 0 && i < len(s) && (s[i] == '.' || s[i] == ')')
}

// featuresMarkdown describes what a manifest contributes (VS Code's
// Features tab) as Markdown.
func featuresMarkdown(m extensions.Manifest) string {
	var b strings.Builder
	c := m.Contributes
	if m.HasCode() {
		fmt.Fprintf(&b, "**Runs code** in the extension host (Node.js) — %s.\n\n", activationHint(m))
	} else {
		b.WriteString("**Declarative only** — no code runs; works without Node.js.\n\n")
	}
	if eng := m.Engines["vscode"]; eng != "" {
		fmt.Fprintf(&b, "Needs VS Code `%s` (hittable provides `%s`).\n\n", eng, extensions.APIVersion)
	}
	section := func(title string, n int) { fmt.Fprintf(&b, "## %s (%d)\n\n", title, n) }
	if len(c.Commands) > 0 {
		section("Commands", len(c.Commands))
		b.WriteString("| Command | ID |\n| --- | --- |\n")
		for _, cmd := range c.Commands {
			fmt.Fprintf(&b, "| %s | `%s` |\n", esc(cmd.CommandTitle(nil)), cmd.Command)
		}
		b.WriteString("\n")
	}
	if props := settings(c.Configuration); len(props) > 0 {
		section("Settings", len(props))
		b.WriteString("| Setting | Default |\n| --- | --- |\n")
		for _, s := range props {
			fmt.Fprintf(&b, "| `%s` | %s |\n", s[0], esc(s[1]))
		}
		b.WriteString("\n")
	}
	if len(c.Languages) > 0 {
		section("Languages", len(c.Languages))
		for _, l := range c.Languages {
			fmt.Fprintf(&b, "- **%s** %s\n", l.ID, strings.Join(l.Extensions, " "))
		}
		b.WriteString("\n")
	}
	if len(c.Themes) > 0 {
		section("Color themes", len(c.Themes))
		for _, t := range c.Themes {
			fmt.Fprintf(&b, "- %s (%s)\n", t.Label, t.UITheme)
		}
		b.WriteString("\n")
	}
	if len(c.Snippets) > 0 {
		section("Snippets", len(c.Snippets))
		for _, s := range c.Snippets {
			fmt.Fprintf(&b, "- %s\n", s.Language)
		}
		b.WriteString("\n")
	}
	if len(c.Keybindings) > 0 {
		section("Keyboard shortcuts", len(c.Keybindings))
		b.WriteString("| Key | Command |\n| --- | --- |\n")
		for _, k := range c.Keybindings {
			key := k.Key
			if k.Mac != "" {
				key += " / mac " + k.Mac
			}
			fmt.Fprintf(&b, "| `%s` | `%s` |\n", key, k.Command)
		}
		b.WriteString("\n")
	}
	var views []string
	for _, vs := range c.Views {
		for _, v := range vs {
			views = append(views, v.Name)
		}
	}
	if len(views) > 0 {
		section("Views", len(views))
		for _, v := range views {
			fmt.Fprintf(&b, "- %s (alt+p › View: Open View…)\n", v)
		}
		b.WriteString("\n")
	}
	if len(m.Dependencies) > 0 {
		fmt.Fprintf(&b, "## Depends on\n\n- %s\n\n", strings.Join(m.Dependencies, "\n- "))
	}
	if u := m.Unsupported(); len(u) > 0 {
		fmt.Fprintf(&b, "## Not available in the terminal\n\n- %s\n", strings.Join(u, "\n- "))
	}
	return b.String()
}

func esc(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " ") }

// settings lists contributes.configuration properties as (key, default).
func settings(raw []byte) [][2]string {
	if len(raw) == 0 {
		return nil
	}
	type block struct {
		Properties map[string]struct {
			Default any `json:"default"`
		} `json:"properties"`
	}
	var blocks []block
	if raw[0] == '[' {
		_ = json.Unmarshal(raw, &blocks)
	} else {
		var one block
		if json.Unmarshal(raw, &one) == nil {
			blocks = []block{one}
		}
	}
	var out [][2]string
	for _, bl := range blocks {
		for k, v := range bl.Properties {
			def := "—"
			if v.Default != nil {
				def = fmt.Sprintf("`%v`", v.Default)
				if len(def) > 40 {
					def = def[:39] + "…`"
				}
			}
			out = append(out, [2]string{k, def})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

func stars(r float64) string {
	full := int(r + 0.5)
	return strings.Repeat("★", full) + strings.Repeat("☆", 5-full)
}

func human(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.0fK", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

func newerVersion(a, b string) bool { return a != b && extensions.EngineAllows(">"+b, a) }
