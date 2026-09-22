package screens

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/ui/components/requesteditor"
	"github.com/hittable/shellapp/ui/theme"
)

func (m *MainScreen) View() string {
	// Components that mark their own zones render their own hover state.
	m.Git.Hover, m.Palette.Hover, m.URLBar.Hover, m.Response.Hover = m.HoverZone, m.HoverZone, m.HoverZone, m.HoverZone

	explorerView := m.Explorer.View()

	var mainView string
	if m.Palette.Open {
		mainView = m.Zones.Mark("palette", m.Palette.View(m.Zones))
	} else if m.GitOpen {
		mainView = m.Zones.Mark("git_panel", m.Git.View(m.Zones))
	} else if m.ShowHelp {
		mainView = m.renderHelp()
	} else if m.ActiveFile == "" {
		mainView = m.renderEmptyMain()
	} else {
		doc := m.Store.Get(m.ActiveFile)
		if doc == nil {
			mainView = m.renderEmptyMain()
		} else if doc.Kind == document.KindBinary {
			mainView = m.renderBinaryPlaceholder()
		} else if doc.Kind == document.KindHit && m.ViewMode == ViewRunner {
			mainView = m.renderRunnerView()
		} else {
			mainView = m.renderTextView()
		}
	}

	sepStyle := lipgloss.NewStyle().
		Foreground(theme.BorderColor).
		Background(theme.BgColor)
	separator := sepStyle.Render("│")

	mainCol := lipgloss.JoinVertical(lipgloss.Left, mainView, m.renderTermStrip())
	if m.Term.Open {
		mainCol = lipgloss.JoinVertical(lipgloss.Left, mainCol, m.Zones.Mark("term_view", m.Term.View()))
	}
	split := lipgloss.JoinHorizontal(lipgloss.Top, explorerView, separator, mainCol)
	if m.ExplorerHidden {
		split = mainCol
	}
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, m.renderTopBar(), split, footer)
}

func (m *MainScreen) renderEmptyMain() string {
	inner, h := m.MainWidth-2, m.MainH-2
	hints := theme.MutedStyle.Render("↑↓ move · ⏎ open · ctrl+n new file · ctrl+p find · ctrl+j terminal · ? help")
	cmd := func(c, d string) string {
		return theme.HelpKeyStyle.Width(30).Render(c) + theme.HelpDescStyle.Render(d)
	}
	commands := lipgloss.JoinVertical(lipgloss.Left,
		theme.HelpTitle.Render("Get started from the shell"),
		cmd("hittable init", "create the hittable/ template here"),
		cmd("hittable -i <file>", "import a Postman or Insomnia collection"),
		cmd("hittable -e postman", "export hittable/ as a Postman collection"),
		cmd("hittable -e insomnia", "export hittable/ as an Insomnia collection"),
		cmd("hittable uninstall", "remove hittable from this machine"),
		cmd("hittable -h", "all options"),
	)
	if _, err := os.Stat(m.HittableDir); err != nil {
		commands = lipgloss.JoinVertical(lipgloss.Left, commands, "",
			theme.MutedStyle.Render("No hittable/ folder here yet — `hittable init` or import a collection, or just create a .hit file."))
	}
	if lipgloss.Width(commands) > inner {
		commands = "" // narrow pane: keep the logo only
	}
	content := lipgloss.JoinVertical(lipgloss.Center, theme.Logo(), "", hints, "", commands)
	if lipgloss.Height(content) > h {
		content = lipgloss.JoinVertical(lipgloss.Center, theme.Wordmark(), "", hints, "", commands)
	}
	if lipgloss.Height(content) > h {
		content = lipgloss.JoinVertical(lipgloss.Center, theme.Wordmark(), "", hints) // tiny terminals
	}
	body := lipgloss.Place(inner, h, lipgloss.Center, lipgloss.Center, content)
	return theme.UnfocusedBorderStyle.Width(inner).Height(h).Render(body)
}

func (m *MainScreen) renderRunnerView() string {
	var sections []string

	breadcrumb := theme.BreadcrumbStyle.Render(m.relPath())
	sections = append(sections, breadcrumb)
	sections = append(sections, m.Zones.Mark("urlbar", m.URLBar.View(m.Zones)))

	tabs := m.renderTabs()
	sections = append(sections, tabs)

	var editorView string
	switch m.ActiveTab {
	case requesteditor.TabParams:
		editorView = m.Params.View()
	case requesteditor.TabHeaders:
		editorView = m.Headers.View()
	case requesteditor.TabBody:
		editorView = m.Body.View()
	}

	sections = append(sections, m.Zones.Mark("editor", editorView))

	sections = append(sections, m.Zones.Mark("response", m.Response.View(m.Zones, m.Focus == FocusResponse)))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return theme.UnfocusedBorderStyle.
		Width(m.MainWidth - 2).
		Height(m.MainH - 2).
		Render(content)
}

func (m *MainScreen) renderTextView() string {
	breadcrumb := theme.BreadcrumbStyle.Render(m.relPath())
	if !m.isMarkdown() {
		return lipgloss.JoinVertical(lipgloss.Left, breadcrumb, m.Zones.Mark("editor", m.TextEd.View()))
	}

	// Markdown: header with the Text / Preview / Split toggle.
	seg := func(id, label string, mode MdMode) string {
		st := theme.MutedStyle
		if m.MdMode == mode {
			st = theme.TabActiveStyle
		} else if m.HoverZone == id {
			st = theme.HoverStyle
		}
		return m.Zones.Mark(id, st.Render(label))
	}
	toggle := "[ " + seg("md_text", "Text", MdText) + " | " + seg("md_preview", "Preview", MdPreview) + " | " + seg("md_split", "Split", MdSplit) + " ]"
	gap := m.MainWidth - lipgloss.Width(breadcrumb) - lipgloss.Width(toggle)
	if gap < 1 {
		gap = 1
	}
	header := breadcrumb + strings.Repeat(" ", gap) + toggle

	var body string
	switch m.MdMode {
	case MdPreview:
		m.Preview.SetContent(m.TextEd.GetContent())
		body = m.Zones.Mark("md_pane", m.Preview.View(m.Focus == FocusPreview))
	case MdSplit:
		m.Preview.SetContent(m.TextEd.GetContent())
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.Zones.Mark("editor", m.TextEd.View()),
			m.Zones.Mark("md_pane", m.Preview.View(m.Focus == FocusPreview)))
	default:
		body = m.Zones.Mark("editor", m.TextEd.View())
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

// setMdMode switches the Markdown view and re-lays out the panes.
func (m *MainScreen) setMdMode(mode MdMode) {
	m.MdMode = mode
	m.SetSize(m.Width, m.Height)
	switch mode {
	case MdPreview:
		m.blurAll()
		m.Focus = FocusPreview
		m.Preview.ScrollY = 0
	default:
		m.Focus = FocusTextEditor
		m.focusCurrent()
	}
}

// renderBinaryPlaceholder shows a friendly "Binary file" message instead of
// dumping non-text bytes into the text editor. Used when openFileRaw
// detected null bytes in the first 8KB or a file larger than the editor cap.
func (m *MainScreen) renderBinaryPlaceholder() string {
	breadcrumb := theme.BreadcrumbStyle.Render(m.relPath())
	doc := m.Store.Get(m.ActiveFile)
	size := ""
	if doc != nil {
		size = humanSize(len(doc.RawContent))
	}
	msg := lipgloss.NewStyle().
		Foreground(theme.MutedColor).
		Padding(2, 2).
		Render(fmt.Sprintf("Binary file (%s)\n\nPreview disabled. Open this file in an external editor to inspect its contents.", size))
	return lipgloss.JoinVertical(lipgloss.Left, breadcrumb, msg)
}

func (m *MainScreen) renderTabs() string {
	tabs := []struct {
		name string
		key  string
		tab  requesteditor.Tab
	}{
		{"Params", "1", requesteditor.TabParams},
		{"Headers", "2", requesteditor.TabHeaders},
		{"Body", "3", requesteditor.TabBody},
	}

	var parts []string
	if m.Focus == FocusTabBar {
		parts = append(parts, theme.TabBarFocusedStyle.Render("▸"))
	} else {
		parts = append(parts, " ")
	}
	for _, t := range tabs {
		style := theme.TabInactiveStyle
		if m.ActiveTab == t.tab {
			style = theme.TabActiveStyle
		} else if m.HoverZone == "tab_"+t.name {
			style = theme.HoverStyle
		}
		label := t.name
		if t.tab == requesteditor.TabParams && m.Params.InvalidJSON || t.tab == requesteditor.TabHeaders && m.Headers.InvalidJSON {
			label += " ⚠"
		}
		parts = append(parts, m.Zones.Mark("tab_"+t.name, style.Render(fmt.Sprintf("[%s]", label))))
	}

	runnerStyle := theme.MutedStyle
	textStyle := theme.MutedStyle
	if m.ViewMode == ViewRunner {
		runnerStyle = theme.TabActiveStyle
	} else {
		textStyle = theme.TabActiveStyle
	}
	if m.HoverZone == "toggle_runner" {
		runnerStyle = theme.HoverStyle
	}
	if m.HoverZone == "toggle_text" {
		textStyle = theme.HoverStyle
	}

	runnerText := m.Zones.Mark("toggle_runner", runnerStyle.Render("Runner"))
	textText := m.Zones.Mark("toggle_text", textStyle.Render("Text"))
	toggleStr := "[ " + runnerText + " | " + textText + " ]"

	partsStr := strings.Join(parts, " ")
	gap := m.MainWidth - 2 - lipgloss.Width(partsStr) - lipgloss.Width(toggleStr)
	if gap < 0 {
		gap = 0
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		partsStr,
		strings.Repeat(" ", gap),
		toggleStr,
	)
}

type helpRow struct{ key, desc string }

var helpSections = []struct {
	title string
	rows  []helpRow
}{
	{"Global", []helpRow{
		{"ctrl+b", "toggle explorer / main pane"},
		{"ctrl+r  ctrl+⏎", "send request"},
		{"ctrl+s", "save now (autosave is on)"},
		{"ctrl+t", "runner ⇄ text view (.hit)"},
		{"ctrl+y", "copy as curl / response body"},
		{"ctrl+p  alt+f", "find file · live grep"},
		{"ctrl+j  ctrl+`", "toggle integrated terminal"},
		{"ctrl+g  alt+g  F5", "toggle Git panel"},
		{"alt+b", "hide / show the sidebar"},
		{"esc", "close file / dismiss"},
		{"?  F1", "this help"},
		{"ctrl+c", "quit (copies a selection first)"},
	}},
	{"Editor", []helpRow{
		{"ctrl+z  ctrl+y", "undo · redo"},
		{"ctrl+c  ctrl+x  ctrl+v", "copy · cut · paste"},
		{"ctrl+a", "select all"},
		{"drag  shift+←→↑↓", "select · double-click a word"},
		{"ctrl+f  ⏎  F3", "find · next match"},
		{"ctrl+g", "go to line"},
		{"ctrl+l", "format JSON body"},
		{"alt+z  ⌥z", "word wrap on / off"},
		{"shift+wheel", "scroll sideways (wrap off)"},
		{"ctrl+←/→  alt+←/→", "jump by word"},
		{"tab  shift+tab", "indent 2 spaces · leave"},
	}},
	{"Explorer", []helpRow{
		{"↑↓  j k", "move"},
		{"⏎  l  h", "open · expand · collapse"},
		{"g  G", "top / bottom"},
		{"pgup pgdn", "page up / down"},
		{"x  right-click", "context menu"},
		{"ctrl+n  ctrl+f", "new file / new folder"},
		{"ctrl+e  ctrl+d", "rename / delete"},
		{"r", "refresh tree"},
		{"/", "find file by name"},
	}},
	{"Terminal (ctrl+j)", []helpRow{
		{"ctrl+b", "back to the explorer"},
		{"ctrl+c", "copy selection, else interrupt"},
		{"drag", "select output text"},
		{"wheel  shift+↑↓", "scroll back (5000 lines)"},
		{"ctrl+v", "paste"},
		{"any key", "restart the shell after exit"},
	}},
	{"Git (ctrl+g)", []helpRow{
		{"1-5  tab", "switch section"},
		{"s u  a A", "stage · unstage · all · all"},
		{"d  D", "discard file · undo all changes"},
		{"c  S", "commit · stash"},
		{"p P f  y", "push · pull · fetch · sync"},
		{"e", "edit working copy in preview"},
		{"v  z  w", "inline⇄split · wrap · ignore ws"},
		{"drag │", "resize the diff columns"},
		{"drag  ctrl+c", "select diff lines · copy"},
		{"n d", "new / delete branch"},
		{"b", "inline blame in the editor"},
		{"/", "search commits / filter files"},
	}},
	{"Runner", []helpRow{
		{"tab  shift+tab", "url → tabs → editor → response"},
		{"⏎ on url", "method picker"},
		{"ctrl+←/→", "cycle method"},
		{"alt+1/2/3", "params / headers / body"},
		{"ctrl+l", "format body JSON"},
		{"ctrl+f", "search response"},
		{"h", "response headers"},
	}},
	{"Markdown", []helpRow{
		{"ctrl+t", "Text → Preview → Split"},
		{"tab", "editor ⇄ preview in split view"},
		{"jk ↑↓ g G", "scroll the preview"},
	}},
	{"Find (ctrl+p)", []helpRow{
		{"ctrl+p  /", "find file by name (fuzzy)"},
		{"alt+f", "live grep file contents"},
		{"tab", "switch between the two"},
		{"↑↓  ⏎", "move · open at that line"},
	}},
}

// helpBody lays the reference out in as many columns as the pane fits,
// filling the shortest column each time so they come out even. Every entry is
// built to exactly one line — a styled fixed-width key column would wrap the
// longer chords and slide the two halves out of step.
func (m *MainScreen) helpBody(inner int) []string {
	keyW := 0
	for _, sec := range helpSections {
		for _, r := range sec.rows {
			keyW = max(keyW, lipgloss.Width(r.key))
		}
	}
	keyW += 2

	pad := func(s string, w int) string {
		return s + strings.Repeat(" ", max(w-lipgloss.Width(s), 0))
	}
	var blocks [][]string
	colWidth := 0
	for _, sec := range helpSections {
		lines := []string{"", theme.HelpTitle.Render(sec.title)}
		for _, r := range sec.rows {
			lines = append(lines, theme.HelpKeyStyle.Render(pad(r.key, keyW))+theme.HelpDescStyle.Render(r.desc))
		}
		for _, l := range lines {
			colWidth = max(colWidth, lipgloss.Width(l))
		}
		blocks = append(blocks, lines)
	}

	const gap = 3
	n := min(max((inner+gap)/(colWidth+gap), 1), len(blocks))
	cols := make([][]string, n)
	for _, b := range blocks {
		i := 0
		for j := range cols {
			if len(cols[j]) < len(cols[i]) {
				i = j
			}
		}
		cols[i] = append(cols[i], b...)
	}

	rows := 0
	for _, c := range cols {
		rows = max(rows, len(c))
	}
	out := make([]string, rows)
	for y := range out {
		var line string
		for _, c := range cols {
			cell := ""
			if y < len(c) {
				cell = c[y]
			}
			line += pad(cell, colWidth+gap)
		}
		out[y] = strings.TrimRight(line, " ")
	}
	return out
}

func (m *MainScreen) renderHelp() string {
	inner, h := m.MainWidth-2, m.MainH-2
	head := theme.Wordmark() + theme.MutedStyle.Render("   keyboard reference")
	body := m.helpBody(inner - 2) // the padding below eats two columns

	// Taller than the pane: scroll it rather than silently cutting shortcuts.
	avail := max(h-lipgloss.Height(head)-2, 1)
	foot := "press ? or esc to close"
	if len(body) > avail {
		m.HelpScroll = min(max(m.HelpScroll, 0), len(body)-avail)
		body = body[m.HelpScroll : m.HelpScroll+avail]
		foot = "↑↓ / wheel scroll · ? or esc to close"
	} else {
		m.HelpScroll = 0
	}

	out := lipgloss.JoinVertical(lipgloss.Left,
		head, strings.Join(body, "\n"), "", theme.MutedStyle.Render(foot))
	out = lipgloss.NewStyle().Padding(0, 1).MaxHeight(h).MaxWidth(inner).Render(out)
	return theme.UnfocusedBorderStyle.Width(inner).Height(h).Render(out)
}

// renderTermStrip is the one-row, clickable "TERMINAL" bar between the main
// pane and the terminal panel.
func (m *MainScreen) renderTermStrip() string {
	arrow := "▸"
	if m.Term.Open {
		arrow = "▾"
	}
	label := arrow + " TERMINAL"
	st := theme.TermStripStyle
	if m.TermFocused {
		st = theme.TermStripFocusedStyle
	}
	hint := "  ctrl+j toggle"
	if m.TermFocused {
		hint = "  ctrl+j hide · ctrl+b explorer · wheel scrolls back"
	}
	if off := m.Term.ScrollOffset; off > 0 {
		hint = fmt.Sprintf("  ↑ scrollback %d/%d · any key returns", off, m.Term.ScrollbackLen())
	}
	line := theme.Hoverable(m.HoverZone == "term_strip", st).Render(label) +
		theme.MutedStyle.Background(theme.SurfaceColor).Render(hint)
	line = ansi.Truncate(line, m.MainWidth, "")
	if w := lipgloss.Width(line); w < m.MainWidth {
		line += lipgloss.NewStyle().Background(theme.SurfaceColor).Render(strings.Repeat(" ", m.MainWidth-w))
	}
	return m.Zones.Mark("term_strip", line)
}
