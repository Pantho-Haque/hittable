package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/ui/components/requesteditor"
	"github.com/hittable/shellapp/ui/theme"
)

func (m *MainScreen) View() string {
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
	hints := theme.MutedStyle.Render("↑↓ move · ⏎ open · ctrl+n new file · ctrl+j terminal · ? help")
	content := lipgloss.JoinVertical(lipgloss.Center, theme.Logo(), "", hints)
	if lipgloss.Height(content) > h {
		content = lipgloss.JoinVertical(lipgloss.Center, theme.Wordmark(), "", hints) // small terminals
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
		{"ctrl+s", "save now (autosave is always on)"},
		{"ctrl+t", "runner ⇄ text view (.hit)"},
		{"ctrl+y", "copy as curl (or response body)"},
		{"esc", "close file / dismiss"},
		{"ctrl+j  ctrl+`", "toggle integrated terminal"},
		{"?  F1", "this help"},
		{"ctrl+c", "quit"},
	}},
	{"Explorer", []helpRow{
		{"↑↓  j k", "move"},
		{"⏎  l  h", "open / expand / collapse"},
		{"g  G", "top / bottom"},
		{"x", "context menu"},
		{"ctrl+n  ctrl+f", "new file / new folder"},
		{"ctrl+e  ctrl+d", "rename / delete"},
		{"r", "refresh tree"},
	}},
	{"Markdown", []helpRow{
		{"ctrl+t", "Text → Preview → Split (mermaid diagrams rendered)"},
		{"tab", "editor ⇄ preview in split view"},
	}},
	{"Find", []helpRow{
		{"ctrl+p  /", "find file by name (fuzzy)"},
		{"alt+f", "live grep file contents"},
		{"tab", "switch between the two"},
	}},
	{"Git (ctrl+g / alt+g / F5)", []helpRow{
		{"1-5  tab", "Status · Commits · Branches · Stashes · Blame"},
		{"s u a d", "stage · unstage · stage all · discard"},
		{"c  S", "commit · stash"},
		{"p P f", "push · pull · fetch"},
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
}

func (m *MainScreen) renderHelp() string {
	var cols []string
	for _, sec := range helpSections {
		rows := []string{theme.HelpTitle.Render(sec.title)}
		for _, r := range sec.rows {
			rows = append(rows, theme.HelpKeyStyle.Render(r.key)+theme.HelpDescStyle.Render(r.desc))
		}
		cols = append(cols, strings.Join(rows, "\n"))
	}
	inner := m.MainWidth - 2
	var body string
	if inner >= 2*52 {
		var left, right []string
		for i, c := range cols {
			if i%2 == 0 {
				left = append(left, c)
			} else {
				right = append(right, c)
			}
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left, left...), "  ",
			lipgloss.JoinVertical(lipgloss.Left, right...))
	} else {
		body = lipgloss.JoinVertical(lipgloss.Left, cols...)
	}
	body = lipgloss.JoinVertical(lipgloss.Left,
		theme.Wordmark()+theme.MutedStyle.Render("   keyboard reference"),
		body, "", theme.MutedStyle.Render("press ? or esc to close"))
	h := m.MainH - 2
	body = lipgloss.NewStyle().Padding(0, 1).MaxHeight(h).MaxWidth(inner).Render(body)
	return theme.UnfocusedBorderStyle.Width(inner).Height(h).Render(body)
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
	line := st.Render(label) + theme.MutedStyle.Background(theme.SurfaceColor).Render(hint)
	line = ansi.Truncate(line, m.MainWidth, "")
	if w := lipgloss.Width(line); w < m.MainWidth {
		line += lipgloss.NewStyle().Background(theme.SurfaceColor).Render(strings.Repeat(" ", m.MainWidth-w))
	}
	return m.Zones.Mark("term_strip", line)
}
