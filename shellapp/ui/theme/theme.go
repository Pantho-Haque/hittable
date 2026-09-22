package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	PrimaryColor   = lipgloss.Color("#8be9fd")
	SecondaryColor = lipgloss.Color("#bd93f9")
	AccentColor    = lipgloss.Color("#50fa7b")
	ErrorColor     = lipgloss.Color("#ff5555")
	WarningColor   = lipgloss.Color("#f1fa8c")
	MutedColor     = lipgloss.Color("#6272a4")
	BgColor        = lipgloss.Color("#282a36")
	SurfaceColor   = lipgloss.Color("#343746")
	TextColor      = lipgloss.Color("#f8f8f2")
	SubtextColor   = lipgloss.Color("#bfbfbf")
	BorderColor    = lipgloss.Color("#44475a")

	FolderColor      = lipgloss.Color("#bd93f9")
	HitFileColor     = lipgloss.Color("#8be9fd")
	MarkdownColor    = lipgloss.Color("#f8f8f2")
	JsonFileColor    = lipgloss.Color("#ffb86c")
	GenericFileColor = lipgloss.Color("#6272a4")

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			Padding(0, 1)

	ExplorerHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(PrimaryColor).
				Padding(0, 1)

	SelectedItemStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#44475a")).
				Foreground(PrimaryColor).
				Padding(0, 1)

	HoverStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3a3d4d")).
			Foreground(TextColor)

	FocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(PrimaryColor)

	UnfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(BorderColor)

	StatusOKStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	StatusErrorStyle = lipgloss.NewStyle().
				Foreground(ErrorColor).
				Bold(true)

	URLStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f8f8f2"))

	MethodStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff79c6")).
			Bold(true)

	TabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			Underline(true)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(MutedColor)

	MutedStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	FooterStyle = lipgloss.NewStyle().
			Foreground(SubtextColor).
			Background(SurfaceColor).
			Padding(0, 1)

	ResponseHeaderStyle = lipgloss.NewStyle().
				Foreground(AccentColor).
				Bold(true)

	JsonKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f8f8f2"))

	JsonStringStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6e3a1"))

	JsonNumberStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fab387"))

	JsonBoolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cba6f7"))

	JsonNullStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	ContextMenuStyle = lipgloss.NewStyle().
				Background(SurfaceColor).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(PrimaryColor).
				BorderBackground(SurfaceColor)

	ContextMenuItemStyle = lipgloss.NewStyle().
				Background(SurfaceColor).
				Foreground(TextColor).
				Padding(0, 1)

	ContextMenuItemHoverStyle = lipgloss.NewStyle().
					Background(lipgloss.Color("#44475a")).
					Foreground(PrimaryColor).
					Bold(true).
					Padding(0, 1)

	BreadcrumbStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)

	SearchHighlightStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#f1fa8c")).
				Foreground(lipgloss.Color("#282a36")).
				Bold(true)

	SearchInputStyle = lipgloss.NewStyle().
				Foreground(TextColor)

	StatusWarnStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	// Explorer row styles: focused cursor is bright, unfocused is dim so the
	// user can tell where keystrokes will go.
	CursorFocusedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#44475a")).
				Foreground(PrimaryColor).
				Bold(true)
	CursorUnfocusedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#343746")).
				Foreground(SubtextColor)
	ExplorerHeaderFocusedStyle = ExplorerHeaderStyle.
					Background(lipgloss.Color("#343746"))
	FolderStyle  = lipgloss.NewStyle().Foreground(FolderColor)
	HitFileStyle = lipgloss.NewStyle().Foreground(HitFileColor)

	LinkStyle       = lipgloss.NewStyle().Foreground(PrimaryColor).Underline(true)
	SendButtonStyle = lipgloss.NewStyle().Background(AccentColor).Foreground(BgColor).Bold(true)

	// Tab bar focus ring.
	TabBarFocusedStyle = lipgloss.NewStyle().Foreground(PrimaryColor)

	// Code editor.
	GutterStyle       = lipgloss.NewStyle().Foreground(MutedColor)
	GutterActiveStyle = lipgloss.NewStyle().Foreground(TextColor).Bold(true)
	CursorCellStyle   = lipgloss.NewStyle().Reverse(true)
	SelectionStyle    = lipgloss.NewStyle().Background(lipgloss.Color("#44475a")).Foreground(TextColor)
	PromptStyle       = lipgloss.NewStyle().Foreground(BgColor).Background(WarningColor).Bold(true)

	// Integrated terminal strip.
	TermStripStyle        = lipgloss.NewStyle().Background(SurfaceColor).Foreground(SubtextColor).Bold(true).Padding(0, 1)
	TermStripFocusedStyle = lipgloss.NewStyle().Background(SurfaceColor).Foreground(AccentColor).Bold(true).Padding(0, 1)

	// Git.
	BranchStyle    = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true)
	HashStyle      = lipgloss.NewStyle().Foreground(WarningColor)
	AuthorStyle    = lipgloss.NewStyle().Foreground(PrimaryColor)
	GitHeaderStyle = lipgloss.NewStyle().Foreground(TextColor).Bold(true)
	DiffAddStyle   = lipgloss.NewStyle().Foreground(AccentColor)
	DiffDelStyle   = lipgloss.NewStyle().Foreground(ErrorColor)

	// Whole-row tints for added / removed diff lines. Kept separate from the
	// foreground-only styles above, which also colour the stage / unstage
	// buttons in the file list.
	DiffAddLineStyle = lipgloss.NewStyle().Foreground(AccentColor).Background(lipgloss.Color("#223a2b"))
	DiffDelLineStyle = lipgloss.NewStyle().Foreground(ErrorColor).Background(lipgloss.Color("#3d2328"))
	DiffHunkStyle    = lipgloss.NewStyle().Foreground(PrimaryColor)
	BlameStyle       = lipgloss.NewStyle().Foreground(MutedColor).Italic(true)
	GitModified      = lipgloss.Color("#e2c08d")
	GitAdded         = lipgloss.Color("#73c991")
	GitDeleted       = lipgloss.Color("#f14c4c")
	GitConflict      = lipgloss.Color("#e4676b")

	// Top bar: a dark strip with pill buttons.
	TopBarBg          = lipgloss.Color("#21222c")
	TopBarStyle       = lipgloss.NewStyle().Background(TopBarBg).Foreground(SubtextColor)
	TopBarDimStyle    = lipgloss.NewStyle().Background(TopBarBg).Foreground(MutedColor)
	TopBarTextStyle   = lipgloss.NewStyle().Background(TopBarBg).Foreground(TextColor)
	TopBarButtonStyle = lipgloss.NewStyle().Background(lipgloss.Color("#343746")).Foreground(TextColor).Padding(0, 2)
	TopBarHoverStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#44475a")).Foreground(TextColor).Bold(true).Padding(0, 2)
	TopBarActiveStyle = lipgloss.NewStyle().Background(BrandCyan).Foreground(lipgloss.Color("#21222c")).Bold(true).Padding(0, 2)
	TopBarBrandStyle  = lipgloss.NewStyle().Background(TopBarBg).Foreground(BrandCyan).Bold(true)
	TopBarBranchStyle = lipgloss.NewStyle().Background(lipgloss.Color("#343746")).Foreground(SecondaryColor).Bold(true).Padding(0, 2)

	// Markdown preview code blocks.
	CodeBlockStyle = lipgloss.NewStyle().Background(lipgloss.Color("#21222c"))

	// Help overlay.
	HelpKeyStyle  = lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true)
	HelpDescStyle = lipgloss.NewStyle().Foreground(TextColor)
	HelpTitle     = lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
)

// MethodColor returns the badge colour for an HTTP method.
func MethodColor(method string) lipgloss.Color {
	switch method {
	case "GET":
		return lipgloss.Color("#50fa7b")
	case "POST":
		return lipgloss.Color("#f1fa8c")
	case "PUT", "PATCH":
		return lipgloss.Color("#ffb86c")
	case "DELETE":
		return lipgloss.Color("#ff5555")
	}
	return lipgloss.Color("#bd93f9")
}

// GitStatusColor is the explorer name colour for a git status badge.
func GitStatusColor(badge string) lipgloss.Color {
	switch badge {
	case "U", "A":
		return GitAdded
	case "D":
		return GitDeleted
	case "!":
		return GitConflict
	}
	return GitModified
}

// GitBadgeStyle styles a one-letter status badge.
func GitBadgeStyle(badge string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(GitStatusColor(badge)).Bold(true)
}

// VScrollbar returns one glyph per row for a vertical scrollbar (track "░",
// thumb "█") or nil when everything fits.
func VScrollbar(rows, total, offset int) []string {
	if rows <= 0 || total <= rows {
		return nil
	}
	thumb := rows * rows / total
	if thumb < 1 {
		thumb = 1
	}
	maxOff := total - rows
	if offset > maxOff {
		offset = maxOff
	}
	if offset < 0 {
		offset = 0
	}
	pos := 0
	if maxOff > 0 {
		pos = offset * (rows - thumb) / maxOff
	}
	out := make([]string, rows)
	track := lipgloss.NewStyle().Foreground(lipgloss.Color("#343746")).Render("░")
	bar := lipgloss.NewStyle().Foreground(MutedColor).Render("█")
	for i := range out {
		if i >= pos && i < pos+thumb {
			out[i] = bar
		} else {
			out[i] = track
		}
	}
	return out
}

// IndentWidth is how many cells a literal tab occupies on screen (VS Code's
// default).
const IndentWidth = 4

// ExpandTabs renders tabs as spaces so every caller measures the same width.
func ExpandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", strings.Repeat(" ", IndentWidth))
}

// HoverBg tints a control the mouse is over.
var HoverBg = lipgloss.Color("#44475a")

// Hoverable returns st as it should render under the mouse. A control with no
// background of its own takes the hover tint; one that is already a filled
// button (Send, the top-bar pills) keeps its colours and is underlined
// instead, so its dark foreground never lands on the dark hover tint.
func Hoverable(on bool, st lipgloss.Style) lipgloss.Style {
	if !on {
		return st
	}
	if st.GetBackground() == lipgloss.TerminalColor(lipgloss.NoColor{}) {
		return st.Background(HoverBg).Bold(true)
	}
	return st.Underline(true).Bold(true)
}
