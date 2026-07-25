package theme

import "github.com/charmbracelet/lipgloss"

var (
	PrimaryColor   = lipgloss.Color("#00d4ff")
	SecondaryColor = lipgloss.Color("#7c3aed")
	AccentColor    = lipgloss.Color("#10b981")
	ErrorColor     = lipgloss.Color("#ef4444")
	WarningColor   = lipgloss.Color("#f59e0b")
	MutedColor     = lipgloss.Color("#6b7280")
	BgColor        = lipgloss.Color("#1e1e2e")
	SurfaceColor   = lipgloss.Color("#2d2d3f")
	TextColor      = lipgloss.Color("#cdd6f4")
	SubtextColor   = lipgloss.Color("#a6adc8")
	BorderColor    = lipgloss.Color("#45475a")

	FolderColor     = lipgloss.Color("#89b4fa")
	HitFileColor    = lipgloss.Color("#00d4ff")
	MarkdownColor   = lipgloss.Color("#a6adc8")
	JsonFileColor   = lipgloss.Color("#fab387")
	GenericFileColor = lipgloss.Color("#6b7280")

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			Padding(0, 1)

	ExplorerHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(PrimaryColor).
				Padding(0, 1)

	SelectedItemStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#45475a")).
				Foreground(PrimaryColor).
				Padding(0, 1)

	HoverStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#363650")).
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
			Foreground(lipgloss.Color("#89b4fa"))

	MethodStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f38ba8")).
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
			Foreground(lipgloss.Color("#89b4fa"))

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
				Foreground(TextColor).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(BorderColor).
				Padding(0, 1)

	ContextMenuItemStyle = lipgloss.NewStyle().
				Foreground(TextColor).
				Padding(0, 1)

	ContextMenuItemHoverStyle = lipgloss.NewStyle().
					Background(lipgloss.Color("#45475a")).
					Foreground(PrimaryColor).
					Padding(0, 1)

	BreadcrumbStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)

	SearchHighlightStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#f9e2af")).
				Foreground(lipgloss.Color("#1e1e2e")).
				Bold(true)

	SearchInputStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(PrimaryColor).
				Padding(0, 1)
)
