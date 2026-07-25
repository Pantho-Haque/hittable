package responseviewer

import (
	"strings"
	"testing"
)

func TestSearchHighlightCorrectSubstring(t *testing.T) {
	r := New()
	r.SetResponse(200, "OK", 100, 50, true, `{
  "title": "veniam",
  "name": "title"
}`)

	r.SearchQuery = "ve"
	r.doSearch()

	rawLines := strings.Split(r.RawContent, "\n")

	matches := r.findAllMatches(rawLines)

	if len(matches) == 0 {
		t.Fatal("expected at least one match for 've'")
	}

	for _, m := range matches {
		if m.lineIdx < 0 || m.lineIdx >= len(rawLines) {
			t.Errorf("match lineIdx %d out of range", m.lineIdx)
			continue
		}
		line := rawLines[m.lineIdx]
		if m.startCol >= len(line) || m.startCol+m.length > len(line) {
			t.Errorf("match col %d+%d out of range for line %q", m.startCol, m.length, line)
			continue
		}
		matched := line[m.startCol : m.startCol+m.length]
		if strings.ToLower(matched) != "ve" {
			t.Errorf("expected match 've', got %q at col %d in line %q", matched, m.startCol, line)
		}
	}

	for _, m := range matches {
		line := rawLines[m.lineIdx]
		context := line
		if len(context) > 40 {
			if m.startCol > 20 {
				start := m.startCol - 20
				context = "..." + line[start:start+40] + "..."
			} else {
				context = line[:40] + "..."
			}
		}
		t.Logf("match at line %d, col %d: %q", m.lineIdx, m.startCol, context)
	}
}

func TestSearchHighlightDoesNotTouchTitle(t *testing.T) {
	r := New()
	r.SetResponse(200, "OK", 100, 50, true, `{"title":"test","data":"veniam"}`)

	r.SearchQuery = "ve"
	r.doSearch()

	rawLines := strings.Split(r.RawContent, "\n")
	matches := r.findAllMatches(rawLines)

	titleMatchFound := false
	for _, m := range matches {
		if m.lineIdx < len(rawLines) {
			line := rawLines[m.lineIdx]
			if m.startCol < len(line) {
				ctx := line
				if m.startCol+10 < len(line) {
					ctx = line[m.startCol : m.startCol+10]
				}
				if strings.Contains(ctx, "title") && !strings.Contains(ctx, "veniam") {
					titleMatchFound = true
				}
			}
		}
	}

	if titleMatchFound {
		t.Error("search incorrectly matched 've' inside 'title' instead of 'veniam'")
	}
}

func TestSearchHighlightRendering(t *testing.T) {
	r := New()
	r.SetResponse(200, "OK", 100, 50, true, `{"data":"veniam"}`)

	r.SearchQuery = "veniam"
	r.doSearch()

	rawLines := strings.Split(r.RawContent, "\n")
	ansiLine := r.highlightJSON(`  "data": "veniam"`)

	matches := r.findAllMatches(rawLines)
	var dataLineMatches []matchPos
	for _, m := range matches {
		if m.lineIdx < len(rawLines) && strings.Contains(rawLines[m.lineIdx], "veniam") {
			dataLineMatches = append(dataLineMatches, m)
		}
	}

	if len(dataLineMatches) == 0 {
		t.Fatal("expected match on data line")
	}

	result := r.applySearchHighlight(ansiLine, rawLines[dataLineMatches[0].lineIdx], dataLineMatches)
	if !strings.Contains(result, "\x1b[") {
		t.Error("expected ANSI codes in highlighted result")
	}
}
