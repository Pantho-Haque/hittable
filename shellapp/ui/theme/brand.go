package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Brand mark, mirroring the web app's logo: a rounded dark tile with a bold
// italic "H", the HITTABLE wordmark in brand cyan, and the tagline.
var (
	BrandCyan   = lipgloss.Color("#00e5cc")
	brandTileBg = lipgloss.Color("#1a1d24")
	brandTileFg = lipgloss.Color("#f4f4f5")
	brandTileBd = lipgloss.Color("#2e323d")

	// Italic "H" in full blocks; each row steps one cell left going down.
	brandH = []string{
		"    ██  ██",
		"   ██  ██ ",
		"  ██████  ",
		" ██  ██   ",
		"██  ██    ",
	}

	WordmarkStyle = lipgloss.NewStyle().Foreground(BrandCyan).Bold(true)
	TaglineStyle  = lipgloss.NewStyle().Foreground(MutedColor)

	// HitMarkStyle is the one-cell "H" chip used as the .hit file icon.
	HitMarkStyle = lipgloss.NewStyle().Background(brandTileBg).Foreground(brandTileFg).Bold(true)
)

// LogoTile renders the "H" tile.
func LogoTile() string {
	art := lipgloss.NewStyle().Foreground(brandTileFg).Background(brandTileBg).
		Render(strings.Join(brandH, "\n"))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(brandTileBd).
		BorderBackground(brandTileBg).
		Background(brandTileBg).
		Padding(1, 3).
		Render(art)
}

// Wordmark returns "H I T T A B L E" in brand cyan.
func Wordmark() string {
	return WordmarkStyle.Render(strings.Join(strings.Split("HITTABLE", ""), " "))
}

// Logo is the tile, wordmark and tagline stacked and centred.
func Logo() string {
	return lipgloss.JoinVertical(lipgloss.Center,
		LogoTile(),
		"",
		Wordmark(),
		TaglineStyle.Render("terminal API client · .hit files, shared with the web app"),
	)
}
