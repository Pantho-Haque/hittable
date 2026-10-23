package texteditor

import (
	"strings"
	"testing"
)

// Normal lines are coloured; a line past maxHighlightLine (a minified
// response) is shown plain so editing it stays instant.
func TestHighlightSkipsVeryLongLines(t *testing.T) {
	ed := New()
	ed.SetContent("data.json", "{}")

	if got := ed.highlight(`{"a": 1}`); !strings.Contains(got, "\x1b[") {
		t.Fatalf("short JSON line was not highlighted: %q", got)
	}
	long := `{"k":"` + strings.Repeat("v", maxHighlightLine) + `"}`
	if got := ed.highlight(long); got != long {
		t.Fatal("a line over maxHighlightLine must be returned unchanged")
	}
}
