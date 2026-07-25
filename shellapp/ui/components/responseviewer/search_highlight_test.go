package responseviewer

import (
	"strings"
	"testing"
)

func TestFindAllMatchesUTF8(t *testing.T) {
	r := &ResponseViewer{}
	r.SearchQuery = "hello"

	rawLines := []string{
		`{"name": "café", "greeting": "hello world"}`,
		`{"name": "日本語", "greeting": "hello"}`,
	}

	matches := r.findAllMatches(rawLines)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	if matches[0].lineIdx != 0 {
		t.Errorf("match 0 lineIdx: expected 0, got %d", matches[0].lineIdx)
	}
	if matches[0].startCol != 30 {
		t.Errorf("match 0 startCol: expected 30, got %d", matches[0].startCol)
	}
	if matches[0].length != 5 {
		t.Errorf("match 0 length: expected 5, got %d", matches[0].length)
	}

	if matches[1].lineIdx != 1 {
		t.Errorf("match 1 lineIdx: expected 1, got %d", matches[1].lineIdx)
	}
	if matches[1].startCol != 29 {
		t.Errorf("match 1 startCol: expected 29, got %d", matches[1].startCol)
	}
	if matches[1].length != 5 {
		t.Errorf("match 1 length: expected 5, got %d", matches[1].length)
	}
}

func TestApplySearchHighlightUTF8(t *testing.T) {
	r := &ResponseViewer{}
	r.SearchQuery = "world"

	rawLine := `{"data": "café", "target": "hello world"}`
	ansiLine := rawLine

	lineMatches := []matchPos{
		{lineIdx: 0, startCol: 36, length: 5},
	}

	result := r.applySearchHighlight(ansiLine, rawLine, lineMatches)

	if !strings.Contains(result, "hello ") || !strings.Contains(result, "world") {
		t.Errorf("highlight result should contain both 'hello ' and 'world', got %q", result)
	}
}

func TestRuneSearch(t *testing.T) {
	haystack := []rune("café hello")
	needle := []rune("hello")

	idx := runeSearch(haystack, needle)
	if idx != 5 {
		t.Errorf("expected runeSearch to return 5, got %d", idx)
	}

	haystack2 := []rune("日本語 test")
	needle2 := []rune("test")
	idx2 := runeSearch(haystack2, needle2)
	if idx2 != 4 {
		t.Errorf("expected runeSearch to return 4, got %d", idx2)
	}

	haystack3 := []rune("no match here")
	needle3 := []rune("xyz")
	idx3 := runeSearch(haystack3, needle3)
	if idx3 != -1 {
		t.Errorf("expected runeSearch to return -1, got %d", idx3)
	}
}
