package gitpanel

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestWrapCells(t *testing.T) {
	if got := wrapCells("abcdefgh", 3); strings.Join(got, "|") != "abc|def|gh" {
		t.Fatalf("plain: got %q", got)
	}
	if got := wrapCells("abc", 8); len(got) != 1 || got[0] != "abc" {
		t.Fatalf("short line was split: %q", got)
	}
	if got := wrapCells("", 8); len(got) != 1 {
		t.Fatalf("empty line must stay one row, got %d", len(got))
	}

	// Styling survives the wrap, and no text is dropped.
	// A raw SGR run: lipgloss renders unstyled under test (no TTY profile).
	styled := "\x1b[31m" + strings.Repeat("ab", 20) + "\x1b[0m"
	rows := wrapCells(styled, 7)
	var plain strings.Builder
	for _, r := range rows {
		if w := lipgloss.Width(r); w > 7 {
			t.Fatalf("row wider than the limit: %d", w)
		}
		if !strings.Contains(r, "\x1b[") {
			t.Fatalf("row lost its styling: %q", r)
		}
		plain.WriteString(ansi.Strip(r))
	}
	if plain.String() != strings.Repeat("ab", 20) {
		t.Fatalf("content changed: %q", plain.String())
	}
}

func TestSplitDiffWrapsInsteadOfClipping(t *testing.T) {
	const width, half = 41, 20 // 15 cells of text per column
	long := strings.Repeat("x", 60)
	in := []string{"@@ -1,1 +1,1 @@", "-old", "+" + long}
	out := splitDiff(in, width, half, true)

	var right strings.Builder
	for _, l := range out {
		if w := lipgloss.Width(l); w > width {
			t.Fatalf("row overflows the pane (%d > %d): %q", w, width, ansi.Strip(l))
		}
		if strings.Contains(ansi.Strip(l), "…") {
			t.Fatalf("row was clipped: %q", ansi.Strip(l))
		}
		if i := strings.Index(ansi.Strip(l), "│"); i >= 0 {
			right.WriteString(strings.TrimSpace(ansi.Strip(l)[i+len("│"):]))
		}
	}
	if !strings.Contains(right.String(), long) {
		t.Fatalf("the long added line did not survive the wrap: %q", right.String())
	}
}

// With wrap off the same line is clipped to one row per side.
func TestSplitDiffWrapOffClips(t *testing.T) {
	const width, half = 41, 20
	long := strings.Repeat("x", 60)
	in := []string{"@@ -1,1 +1,1 @@", "-old", "+" + long}

	wrapped := splitDiff(in, width, half, true)
	clipped := splitDiff(in, width, half, false)
	if len(clipped) >= len(wrapped) {
		t.Fatalf("wrap off produced %d rows, wrap on %d", len(clipped), len(wrapped))
	}
	if !strings.Contains(ansi.Strip(strings.Join(clipped, "\n")), "…") {
		t.Fatal("clipped output has no ellipsis")
	}
}

// Moving the divider moves the column boundary without changing the row width.
func TestSplitDiffDividerPosition(t *testing.T) {
	in := []string{"@@ -1,1 +1,1 @@", " ctx"}
	for _, half := range []int{10, 20, 30} {
		out := splitDiff(in, 41, half, true)
		row := ansi.Strip(out[len(out)-1])
		if lipgloss.Width(row) != 41 {
			t.Fatalf("half=%d: row width %d, want 41", half, lipgloss.Width(row))
		}
		if i := strings.Index(row, "│"); i != half {
			t.Errorf("half=%d: divider at column %d", half, i)
		}
	}
}
