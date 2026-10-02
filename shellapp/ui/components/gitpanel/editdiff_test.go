package gitpanel

import (
	"reflect"
	"strings"
	"testing"
)

func TestLineDiff(t *testing.T) {
	a := strings.Split("a b c d e f", " ")
	cases := []struct {
		b    string
		want []editHunk
	}{
		{"a b c d e f", nil},
		{"X b c d e f", []editHunk{{0, 1, 0, 1}}},
		{"a b c d e Y", []editHunk{{5, 6, 5, 6}}},
		{"a b X c d e f", []editHunk{{2, 2, 2, 3}}},               // insert
		{"a b d e f", []editHunk{{2, 3, 2, 2}}},                   // delete
		{"X b c d Y Z f", []editHunk{{0, 1, 0, 1}, {4, 5, 4, 6}}}, // two hunks
		{"", []editHunk{{0, 6, 0, 1}}},                            // "" splits to one empty line
	}
	for _, c := range cases {
		if got := lineDiff(a, strings.Split(c.b, " ")); !reflect.DeepEqual(got, c.want) {
			t.Errorf("diff(%q): got %v want %v", c.b, got, c.want)
		}
	}
	p := &Panel{editHunks: []editHunk{{0, 1, 0, 1}, {4, 5, 4, 6}}}
	for n, want := range []int{0, 1, 2, 3, 4, -1, 5} {
		if got, _ := p.leftLine(n); got != want {
			t.Errorf("leftLine(%d) = %d want %d", n, got, want)
		}
	}
	if _, del := p.leftLine(4); !del {
		t.Error("line 4 should pair with a removed line")
	}
	p = &Panel{editHunks: []editHunk{{2, 3, 2, 2}}} // "c" deleted: line 2 now shows "d"
	if got, _ := p.leftLine(2); got != 3 {
		t.Errorf("after a deletion leftLine(2) = %d want 3", got)
	}
}
