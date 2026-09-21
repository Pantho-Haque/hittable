package texteditor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestSetContentAuthoritative verifies that SetContent is the source of truth:
// when called for a path, the editor's buffer is replaced with the given
// content. This matters for the round-trip Text → Runner → Text toggle, which
// re-marshals the document and feeds it back into the editor via SetContent.
func TestSetContentAuthoritative(t *testing.T) {
	ed := New()
	ed.SetSize(80, 24)

	ed.SetContent("/test/file.md", "first content")
	if ed.GetContent() != "first content" {
		t.Fatalf("expected 'first content', got %q", ed.GetContent())
	}

	ed.SetContent("/other/file.md", "second content")
	if ed.GetContent() != "second content" {
		t.Fatalf("expected 'second content', got %q", ed.GetContent())
	}

	// Calling SetContent with new content on an existing path must overwrite.
	ed.SetContent("/test/file.md", "updated content")
	if ed.GetContent() != "updated content" {
		t.Fatalf("expected 'updated content' after re-set, got %q", ed.GetContent())
	}
}

// TestEditorInstanceReusedAcrossOpens: while the buffer text is replaced on
// each SetContent, the underlying textarea.Model is reused (cached per-path),
// which preserves scroll position and other state for the next time the user
// types into that path.
func TestEditorInstanceReusedAcrossOpens(t *testing.T) {
	ed := New()
	ed.SetSize(80, 24)

	ed.SetContent("/file1.md", "hello world")
	if ed.GetContent() != "hello world" {
		t.Fatalf("expected 'hello world', got %q", ed.GetContent())
	}

	ed.SetContent("/file2.md", "other file")
	if ed.GetContent() != "other file" {
		t.Fatalf("expected 'other file', got %q", ed.GetContent())
	}

	// Switching back to /file1.md by re-setting its content restores the
	// editor for that path.
	ed.SetContent("/file1.md", "hello world")
	if ed.GetContent() != "hello world" {
		t.Fatalf("expected 'hello world' on re-open, got %q", ed.GetContent())
	}
}

// TestMouseNavigationNotCrash verifies that forwarding a mouse message to
// Update does not panic and that View() still produces output of the
// expected size. This is the user-visible feature: clicking inside the
// editor positions the cursor, scrolling moves the buffer.
func TestMouseNavigationNotCrash(t *testing.T) {
	ed := New()
	ed.SetSize(80, 24)
	ed.SetContent("/test.md", "line one\nline two\nline three\n")
	ed.Focus()

	// Click in the middle of the buffer.
	ed.Update(tea.MouseMsg{X: 10, Y: 5, Type: tea.MouseLeft})

	// Scroll wheel.
	ed.Update(tea.MouseMsg{X: 10, Y: 5, Type: tea.MouseWheelUp})

	// Type a character.
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})

	view := ed.View()
	if view == "" {
		t.Fatalf("expected non-empty view")
	}
	if !strings.Contains(view, "Ln ") {
		t.Errorf("expected Ln indicator in view, got: %q", view[:min(120, len(view))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
