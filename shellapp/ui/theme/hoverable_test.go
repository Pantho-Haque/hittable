package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestHoverable(t *testing.T) {
	plain := lipgloss.NewStyle().Foreground(PrimaryColor)
	if Hoverable(false, plain).GetBackground() != lipgloss.TerminalColor(lipgloss.NoColor{}) {
		t.Fatal("not hovered but a background was added")
	}
	if got := Hoverable(true, plain); got.GetBackground() != lipgloss.TerminalColor(HoverBg) {
		t.Fatalf("plain control did not take the hover tint: %v", got.GetBackground())
	}
	// A filled button must keep its own background, or its dark foreground
	// would be drawn on the dark hover tint.
	if got := Hoverable(true, SendButtonStyle); got.GetBackground() != lipgloss.TerminalColor(AccentColor) {
		t.Fatalf("filled button lost its background: %v", got.GetBackground())
	} else if !got.GetUnderline() {
		t.Fatal("filled button got no hover affordance")
	}
}
