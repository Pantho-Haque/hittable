// Package explorer implements the view rendering for the explorer component.
package explorer

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hittable/shellapp/internal/explorer"
	"github.com/hittable/shellapp/ui/theme"
	zone "github.com/lrstanley/bubblezone"
)

func (e *ExplorerComponent) View(z *zone.Manager) string {
	header := theme.ExplorerHeaderStyle.Render("/" + filepath.Base(e.Root.Path))
	var lines []string
	lines = append(lines, header)

	scrollStart := e.GetScrollStart()
	maxLines := e.Height - 2
	if maxLines < 1 {
		maxLines = 1
	}
	end := scrollStart + maxLines
	if end > len(e.Visible) {
		end = len(e.Visible)
	}
	visible := e.Visible[scrollStart:end]

	for i, node := range visible {
		realIdx := scrollStart + i
		indent := strings.Repeat("  ", explorer.Indent(node))
		icon := explorer.Icon(node)
		name := node.Name
		if node.Kind == explorer.KindDir {
			name += "/"
		}

		if e.Renaming && realIdx == e.RenameNodeIdx {
			display := fmt.Sprintf("%s%s %s", indent, icon, e.RenameInput.View())
			lines = append(lines, display)
			continue
		}

		display := fmt.Sprintf("%s%s %s", indent, icon, name)

		if realIdx == e.Cursor && e.Focused {
			display = theme.SelectedItemStyle.Render(display)
		} else if realIdx == e.HoverRow && e.HoverRow != e.Cursor {
			display = theme.HoverStyle.Render(display)
		}
		lines = append(lines, z.Mark(fmt.Sprintf("explorer_%d", realIdx), display))
	}

	if e.AddingFile || e.AddingFolder {
		kind := "file"
		if e.AddingFolder {
			kind = "folder"
		}
		prompt := fmt.Sprintf("  New %s: %s", kind, e.AddInput.View())
		lines = append(lines, theme.HoverStyle.Render(prompt))
	}

	if e.Deleting && e.DeleteNodeIdx >= 0 && e.DeleteNodeIdx < len(e.Visible) {
		node := e.Visible[e.DeleteNodeIdx]
		prompt := fmt.Sprintf("  Delete %s? (y/n)", node.Name)
		lines = append(lines, lipgloss.NewStyle().Foreground(theme.ErrorColor).Render(prompt))
	}

	for len(lines) < e.Height {
		lines = append(lines, "")
	}

	result := strings.Join(lines, "\n")

	if e.ContextMenuOpen {
		result = e.renderContextMenu(result)
	}

	return lipgloss.NewStyle().
		Width(e.Width).
		Height(e.Height).
		Render(result)
}

func (e *ExplorerComponent) renderContextMenu(baseView string) string {
	menuY := e.ContextMenuRow + 1
	menuItemCount := len(e.ContextMenuItems)
	needed := menuItemCount + 2
	if menuY+needed > e.Height {
		menuY = e.Height - needed
		if menuY < 0 {
			menuY = 0
		}
	}
	menuLines := make([]string, len(e.ContextMenuItems))
	for i, item := range e.ContextMenuItems {
		style := theme.ContextMenuItemStyle
		if i == e.ContextMenuIdx {
			style = theme.ContextMenuItemHoverStyle
		}
		menuLines[i] = style.Render(fmt.Sprintf(" %s ", item))
	}
	menuContent := strings.Join(menuLines, "\n")
	menuWidth := 16
	menu := theme.ContextMenuStyle.Width(menuWidth).Render(menuContent)

	resultLines := strings.Split(baseView, "\n")
	for len(resultLines) < menuY+len(e.ContextMenuItems)+2 {
		resultLines = append(resultLines, "")
	}
	for mi, mline := range strings.Split(menu, "\n") {
		row := menuY + mi
		if row < len(resultLines) {
			resultLines[row] = mline
		}
	}
	return strings.Join(resultLines, "\n")
}
