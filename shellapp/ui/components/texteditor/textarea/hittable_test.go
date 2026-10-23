package textarea

import (
	"strings"
	"testing"
	"time"
)

// The editor keeps whole files in this buffer; upstream dropped every row
// past 10000, and the cut buffer was then saved over the file.
func TestNoRowLimit(t *testing.T) {
	ta := New()
	want := strings.Repeat("row\n", 25000) + "end"
	ta.SetValue(want)
	if got := ta.Value(); got != want {
		t.Fatalf("kept %d of %d rows", strings.Count(got, "\n")+1, strings.Count(want, "\n")+1)
	}
	ta.InsertString("\nmore")
	if ta.LineCount() != 25002 {
		t.Errorf("rows after insert = %d", ta.LineCount())
	}
}

// Wrapping a long unbroken line (minified JSON, a saved HTML response) must
// be linear: upstream re-measured the whole word per rune, which took ~40s
// for 64KB and froze the app.
func TestWrapLongWordIsLinear(t *testing.T) {
	// 200K runes, no spaces: milliseconds now, minutes with the quadratic wrap.
	line := []rune(strings.Repeat(`{"k":"v"},`, 20000))
	start := time.Now()
	got := wrap(line, 1<<20)
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("wrap took %s", d)
	}
	total := 0
	for _, l := range got {
		total += len(l)
	}
	if total < len(line) {
		t.Errorf("wrap lost runes: %d of %d", total, len(line))
	}
	// Narrow widths still break the word where upstream did.
	if w := wrap([]rune("abcdefghij"), 4); len(w) != 3 || string(w[0]) != "abcd" || string(w[2]) != "ij " {
		t.Errorf("narrow wrap = %q", w)
	}
}
