package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/ui/components/requesteditor"
	"github.com/hittable/shellapp/ui/theme"
)

func (m *MainScreen) View() string {
	explorerView := m.Explorer.View(m.Zones)

	var mainView string
	if m.ActiveFile == "" {
		mainView = m.renderEmptyMain()
	} else {
		doc := m.Store.Get(m.ActiveFile)
		if doc == nil {
			mainView = m.renderEmptyMain()
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

	split := lipgloss.JoinHorizontal(lipgloss.Top, explorerView, separator, mainView)
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, split, footer)
}

func (m *MainScreen) renderEmptyMain() string {
	empty := lipgloss.NewStyle().
		Foreground(theme.MutedColor).
		Padding(4, 2).
		Render("Select a file from the explorer to get started")

	return theme.UnfocusedBorderStyle.
		Width(m.MainWidth).
		Height(m.Height - 2).
		Render(empty)
}

func (m *MainScreen) renderRunnerView() string {
	var sections []string

	breadcrumb := theme.BreadcrumbStyle.Render(m.relPath())
	sections = append(sections, breadcrumb)
	sections = append(sections, m.URLBar.View(m.Zones))

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

	editorHeight := (m.Height - 14) / 3
	if editorHeight < 3 {
		editorHeight = 3
	}
	editorContainer := theme.UnfocusedBorderStyle.
		Width(m.MainWidth - 4).
		Height(editorHeight).
		Render(editorView)
	sections = append(sections, editorContainer)

	sections = append(sections, m.Response.View(m.Zones))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return theme.UnfocusedBorderStyle.
		Width(m.MainWidth).
		Height(m.Height - 2).
		Render(content)
}

func (m *MainScreen) renderTextView() string {
	breadcrumb := theme.BreadcrumbStyle.Render(m.relPath())
	textContent := m.TextEd.View()
	return lipgloss.JoinVertical(lipgloss.Left, breadcrumb, textContent)
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
	for _, t := range tabs {
		style := theme.TabInactiveStyle
		if m.ActiveTab == t.tab {
			style = theme.TabActiveStyle
		} else if m.HoverZone == "tab_"+t.name {
			style = theme.HoverStyle
		}
		tabLabel := style.Render(fmt.Sprintf("[%s]", t.name))
		parts = append(parts, m.Zones.Mark("tab_"+t.name, tabLabel))
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
	gap := m.MainWidth - lipgloss.Width(partsStr) - lipgloss.Width(toggleStr)
	if gap < 0 {
		gap = 0
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		partsStr,
		strings.Repeat(" ", gap),
		toggleStr,
	)
}
