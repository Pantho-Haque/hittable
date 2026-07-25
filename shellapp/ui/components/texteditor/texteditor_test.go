package texteditor

import (
	"testing"
)

func TestSetContentReusesEditorAcrossOpens(t *testing.T) {
	ed := New()
	ed.SetSize(80, 24)

	ed.SetContent("/test/file.md", "first content")
	ed.Focus()

	if ed.GetContent() != "first content" {
		t.Fatalf("expected 'first content', got %q", ed.GetContent())
	}

	ed.SetContent("/other/file.md", "second content")
	ed.Focus()

	if ed.GetContent() != "second content" {
		t.Fatalf("expected 'second content', got %q", ed.GetContent())
	}

	ed.SetContent("/test/file.md", "")
	ed.Focus()

	if ed.GetContent() != "first content" {
		t.Fatalf("expected 'first content' on re-open, got %q", ed.GetContent())
	}
}

func TestEditorStatePreservedAcrossSwitches(t *testing.T) {
	ed := New()
	ed.SetSize(80, 24)

	ed.SetContent("/file1.md", "hello world")
	ed.Focus()

	ed.SetContent("/file2.md", "other file")
	ed.Focus()

	ed.SetContent("/file1.md", "")
	ed.Focus()

	content := ed.GetContent()
	if content != "hello world" {
		t.Fatalf("expected editor state preserved, got %q", content)
	}
}
