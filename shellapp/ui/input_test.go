package ui

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestPartialMouseTail(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"complete SGR", "abc\x1b[<66;51;23M", 0},
		{"complete SGR release", "abc\x1b[<0;1;1m", 0},
		{"SGR cut mid-coords", "abc\x1b[<66;51;2", 10},
		{"SGR cut after <", "abc\x1b[<", 3},
		{"complete X10", "ab\x1b[M\x20\x21\x22", 0},
		{"X10 cut short", "ab\x1b[M\x20", 4},
		{"bare ESC left to bubbletea", "abc\x1b", 0},
		{"bare CSI left to bubbletea", "abc\x1b[", 0},
		{"arrow key untouched", "\x1b[A", 0},
		{"not a mouse report", "\x1b[<12;ab", 0},
		{"plain text", "hello", 0},
		{"empty", "", 0},
	}
	for _, c := range cases {
		if got := partialMouseTail([]byte(c.in)); got != c.want {
			t.Errorf("%s: partialMouseTail(%q) = %d, want %d", c.name, c.in, got, c.want)
		}
	}
}

// The regression: a scroll burst split mid-report was re-parsed as keystrokes
// and typed into the editor. The wrapper must pass every byte through
// unchanged while never ending a read inside a report — whatever the split.
func TestReadsNeverEndInsideAMouseReport(t *testing.T) {
	const report = "\x1b[<66;51;23M"
	full := strings.Repeat(report, 40) + "hello\x1b[A"

	for _, split := range []int{1, 2, 3, 5, 8, 11, 12, 250, 255, 256, 257} {
		if split >= len(full) {
			continue
		}
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			w.Write([]byte(full[:split]))
			w.Write([]byte(full[split:]))
			w.Close()
		}()

		var out bytes.Buffer
		in := MouseSafeInput(r)
		buf := make([]byte, 256)
		for {
			n, readErr := in.Read(buf)
			chunk := buf[:n]
			if cut := partialMouseTail(chunk); cut != 0 {
				t.Fatalf("split=%d: a read ended %d bytes into a mouse report", split, cut)
			}
			out.Write(chunk)
			if readErr != nil {
				break
			}
		}
		r.Close()

		if out.String() != full {
			t.Fatalf("split=%d: byte stream changed\n got %q\nwant %q", split, out.String(), full)
		}
	}
}
