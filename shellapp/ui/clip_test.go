package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestClipToTerminal(t *testing.T) {
	frame := strings.Join([]string{
		"top bar that is far too wide for the terminal",
		"row two",
		"row three",
		"row four",
	}, "\n")

	out := clipToTerminal(frame, 10, 3)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d rows, want 3", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 10 {
			t.Errorf("row %d is %d columns wide", i, w)
		}
	}
	if lines[0] != "top bar th" {
		t.Errorf("first row = %q", lines[0])
	}

	// Styling must survive the cut, and a frame that already fits is untouched.
	styled := "\x1b[31m" + strings.Repeat("x", 20) + "\x1b[0m"
	if got := clipToTerminal(styled, 5, 1); !strings.Contains(got, "\x1b[") {
		t.Errorf("clipping dropped the styling: %q", got)
	}
	fits := "ab\ncd"
	if got := clipToTerminal(fits, 10, 5); got != fits {
		t.Errorf("a frame that fits was changed: %q", got)
	}
}
