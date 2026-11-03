package extpanel

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hittable/shellapp/internal/extensions"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// fakeOpenVSX answers search, the extension page and its README. gate, when
// set, holds every answer until it is closed, to look at loading states.
func fakeOpenVSX(t *testing.T, gate chan struct{}) {
	t.Helper()
	ext := func(ns, name, display, desc string, dl int64) map[string]any {
		return map[string]any{"namespace": ns, "name": name, "displayName": display, "description": desc,
			"version": "2.1.0", "downloadCount": dl, "averageRating": 4.6, "reviewCount": 12, "verified": true,
			"namespaceDisplayName": "Red Hat", "license": "MIT", "timestamp": "2026-10-23T08:14:45Z",
			"engines": map[string]string{"vscode": "^1.80.0"},
			"files":   map[string]string{"readme": "http://" + t.Name() + "/readme"}}
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gate != nil {
			<-gate
		}
		switch {
		case r.URL.Path == "/api/-/search":
			q := r.URL.Query().Get("query")
			list := []any{ext("redhat", "vscode-yaml", "YAML", "YAML Language Support by Red Hat", 9_100_000),
				ext("esbenp", "prettier-vscode", "Prettier - Code formatter", "Code formatter using prettier", 9_700_000)}
			if q != "" && !strings.Contains("yaml", q) {
				list = list[1:]
			}
			json.NewEncoder(w).Encode(map[string]any{"extensions": list})
		case r.URL.Path == "/api/redhat/vscode-yaml":
			e := ext("redhat", "vscode-yaml", "YAML", "YAML Language Support by Red Hat", 9_100_000)
			e["files"] = map[string]string{"readme": srv.URL + "/readme"}
			json.NewEncoder(w).Encode(e)
		case r.URL.Path == "/readme":
			fmt.Fprint(w, "# YAML Language Support\n\nProvides **validation**, completion and formatting.\n\n- Kubernetes schemas\n- Schema store\n")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("HITTABLE_EXT_REGISTRY", srv.URL)
}

// newPanel collects background results so the test can apply them.
func newPanel(t *testing.T) (*Panel, chan tea.Msg) {
	t.Helper()
	t.Setenv("HITTABLE_HOME", t.TempDir())
	ch := make(chan tea.Msg, 64)
	p := New(func(m tea.Msg) { ch <- m })
	p.SetSize(120, 34)
	p.Actions.State = func() State { return State{Spinner: "⠋", Progress: -1} }
	return p, ch
}

func drain(t *testing.T, p *Panel, ch chan tea.Msg, until func() bool) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for !until() {
		select {
		case m := <-ch:
			if pm, ok := m.(Msg); ok {
				p.Apply(pm)
			}
		case <-deadline:
			t.Fatalf("timed out; panel:\n%s", plain(p.View()))
		}
	}
}

func plain(s string) string { return ansi.Strip(s) }

func TestPanelShowsLoadingThenThePage(t *testing.T) {
	gate := make(chan struct{})
	fakeOpenVSX(t, gate)
	p, ch := newPanel(t)
	p.Open()

	// While Open VSX has not answered, the panel says so instead of being blank.
	if v := plain(p.View()); !strings.Contains(v, "Loading popular extensions") {
		t.Fatalf("no loading state:\n%s", v)
	}
	if !p.Busy() {
		t.Error("Busy() should be true while loading, so the spinner turns")
	}
	close(gate)
	drain(t, p, ch, func() bool { return len(p.list) == 2 })
	drain(t, p, ch, func() bool {
		pg := p.pages[p.sel]
		return pg != nil && pg.readmeDone && pg.detail != nil
	})
	v := plain(p.View())
	for _, want := range []string{"▾ POPULAR ON OPEN VSX  2", "Prettier - Code formatter", "⬇ 9.7M", "Install", "DETAILS", "FEATURES", "CHANGELOG"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q in:\n%s", want, v)
		}
	}
	if testing.Verbose() {
		t.Logf("\n%s", v)
	}

	// Moving to YAML loads its page; the README renders.
	for i := 0; i < 5 && p.sel != "redhat.vscode-yaml"; i++ {
		p.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	}
	drain(t, p, ch, func() bool {
		pg := p.pages["redhat.vscode-yaml"]
		return pg != nil && pg.readmeDone && pg.detail != nil
	})
	v = plain(p.View())
	for _, want := range []string{"YAML Language Support", "validation", "Red Hat ✓", "★★★★★ 4.6 (12)", "MIT"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q in:\n%s", want, v)
		}
	}
	// Every line fits the panel: a wrapped line would break the layout.
	for i, l := range strings.Split(p.View(), "\n") {
		if w := ansi.StringWidth(l); w > p.Width {
			t.Errorf("line %d is %d wide (panel %d): %q", i, w, p.Width, plain(l))
		}
	}
	if n := len(strings.Split(p.View(), "\n")); n != p.Height {
		t.Errorf("panel is %d rows, want %d", n, p.Height)
	}
}

func TestSearchKeepsTheListWhileSearching(t *testing.T) {
	fakeOpenVSX(t, nil)
	p, ch := newPanel(t)
	p.Open()
	drain(t, p, ch, func() bool { return len(p.list) == 2 })

	var cmd tea.Cmd
	for _, r := range "pret" {
		cmd, _ = p.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	v := plain(p.View())
	if !strings.Contains(v, "Searching Open VSX for “pret”") || !strings.Contains(v, "Prettier") {
		t.Fatalf("while searching, the old list should stay with a spinner:\n%s", v)
	}
	// The debounce fires, the search runs, results replace the list.
	if m, ok := cmd().(Msg); ok {
		p.Apply(m)
	}
	drain(t, p, ch, func() bool { return !p.searching })
	if len(p.list) != 1 || p.list[0].id != "esbenp.prettier-vscode" {
		t.Errorf("results: %+v", p.list)
	}
	if v := plain(p.View()); !strings.Contains(v, "▾ OPEN VSX  1") {
		t.Errorf("results header missing:\n%s", v)
	}
	// esc clears the search before it closes the panel.
	if _, closed := p.HandleKey(tea.KeyMsg{Type: tea.KeyEsc}); closed || p.Query != "" {
		t.Errorf("esc with a query should clear it (closed=%v, query=%q)", closed, p.Query)
	}
	if _, closed := p.HandleKey(tea.KeyMsg{Type: tea.KeyEsc}); !closed {
		t.Error("esc with nothing to clear should close the panel")
	}
}

func TestButtonsAndClicks(t *testing.T) {
	fakeOpenVSX(t, nil)
	p, ch := newPanel(t)
	var installed string
	p.Actions.Install = func(spec string) { installed = spec }
	p.Open()
	drain(t, p, ch, func() bool { return len(p.list) == 2 })
	p.View()

	// Click the Install button where it was drawn.
	clicked := false
	for _, h := range p.hits {
		if h.act == "install" {
			p.HandleMouse(tea.MouseMsg{X: h.x0 + 1, Y: h.y0, Type: tea.MouseLeft, Action: tea.MouseActionPress})
			clicked = true
		}
	}
	if !clicked || installed != p.list[0].id {
		t.Errorf("install button: clicked=%v installed=%q", clicked, installed)
	}
	// Click the second entry and the Features tab.
	p.View()
	for _, h := range p.hits {
		if h.act == "row" && !p.rows[h.index].header && p.rows[h.index].entry == 1 {
			p.HandleMouse(tea.MouseMsg{X: 3, Y: h.y0, Type: tea.MouseLeft, Action: tea.MouseActionPress})
			break
		}
	}
	if p.sel != p.list[1].id {
		t.Errorf("click on an entry selected %q", p.sel)
	}
	p.View()
	for _, h := range p.hits {
		if h.act == "tab" && h.index == TabFeatures {
			p.HandleMouse(tea.MouseMsg{X: h.x0, Y: h.y0, Type: tea.MouseLeft, Action: tea.MouseActionPress})
		}
	}
	if p.Tab != TabFeatures {
		t.Errorf("tab click: tab %d", p.Tab)
	}
}

func TestAccordionHoverAndWheel(t *testing.T) {
	fakeOpenVSX(t, nil)
	p, ch := newPanel(t)
	p.SetSize(120, minHeight) // the smallest panel, so the list scrolls
	p.Open()
	drain(t, p, ch, func() bool { return len(p.list) == 2 })
	for i := 0; i < 5; i++ {
		p.popular = append(p.popular, entry{id: fmt.Sprintf("acme.more%d", i), name: "More", publisher: "acme"})
	}
	p.rebuild()
	p.View()

	// The pointer over the page focuses it; back over the list, the list.
	p.HandleMouse(tea.MouseMsg{X: p.leftWidth() + 5, Y: 10, Type: tea.MouseMotion})
	if !p.FocusDetail {
		t.Error("hovering the page should focus it")
	}
	p.HandleMouse(tea.MouseMsg{X: 3, Y: 6, Type: tea.MouseMotion})
	if p.FocusDetail || p.hoverRow < 0 {
		t.Errorf("hovering the list: focus %v, hover row %d", p.FocusDetail, p.hoverRow)
	}

	// The wheel scrolls the list without changing the selection.
	sel := p.sel
	p.HandleMouse(tea.MouseMsg{X: 3, Y: 6, Type: tea.MouseWheelDown})
	p.View()
	if p.scroll != entryLines || p.sel != sel {
		t.Errorf("wheel: scroll %d, selection %q (was %q)", p.scroll, p.sel, sel)
	}

	// Enter on a section header folds it; again opens it.
	p.cursor = 0 // the Popular header
	p.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !p.collapsed[secPopular] || len(p.rows) != 1 {
		t.Fatalf("collapse: %v, %d rows", p.collapsed, len(p.rows))
	}
	v := plain(p.View())
	rows := 0
	for _, h := range p.hits {
		if h.act == "row" {
			rows++
		}
	}
	if !strings.Contains(v, "▸ POPULAR ON OPEN VSX  7") || rows != 1 {
		t.Errorf("collapsed section still shows its entries (%d rows):\n%s", rows, v)
	}
	p.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if p.collapsed[secPopular] || len(p.rows) != 8 {
		t.Errorf("expand: %v, %d rows", p.collapsed, len(p.rows))
	}
}

func TestCleanMarkdown(t *testing.T) {
	in := "<p align=\"center\"><img src=\"logo.png\"></p>\n\n" +
		"[![Build](https://x/b.svg)](https://ci) [![npm](https://x/n.svg)](https://npm)\n\n" +
		"<h2 align=\"center\">Fast <b>formatting</b></h2>\n\n\n\n\n" +
		"See <a href=\"https://prettier.io\">the site</a>.<br>Next line &amp; more.\n\n" +
		"```js\n<div>kept in code</div>\n```\n"
	out := CleanMarkdown(in)
	for _, want := range []string{"## Fast **formatting**", "[the site](https://prettier.io)", "Next line & more.", "<div>kept in code</div>"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	for _, gone := range []string{"<img", "<p", "b.svg", "\n\n\n"} {
		if strings.Contains(out, gone) {
			t.Errorf("%q survived in:\n%s", gone, out)
		}
	}
}

func TestHalfBlockIcon(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			c := color.RGBA{255, 0, 0, 255} // red top half
			if y >= 2 {
				c = color.RGBA{0, 0, 255, 255} // blue bottom half
			}
			img.Set(x, y, c)
		}
	}
	lines := halfBlocks(img, 2, 1, "#000000")
	if len(lines) != 1 || ansi.StringWidth(lines[0]) != 2 || !strings.Contains(lines[0], "▀") {
		t.Fatalf("lines = %q", lines)
	}
	px := sample(img, 2, 2)
	if px[0][0].r < 0.99 || px[1][0].b < 0.99 {
		t.Errorf("sampled %+v", px)
	}
	if got := over(rgba{0, 0, 0, 0}, "#336699"); got != "#336699" {
		t.Errorf("transparent pixel over a background = %s", got)
	}
	// The tile fallback has the size asked for.
	if tl := tileLines("acme.widget", 6, 3, ""); len(tl) != 3 || ansi.StringWidth(tl[1]) != 6 || !strings.Contains(ansi.Strip(tl[1]), "WI") {
		t.Errorf("tile = %q", tl)
	}
}

// BenchmarkScrollFrame is one wheel notch over a long list: the frame must
// stay cheap so wheel events never queue up.
func BenchmarkScrollFrame(b *testing.B) {
	b.Setenv("HITTABLE_HOME", b.TempDir())
	p := New(nil)
	p.SetSize(150, 44)
	p.Actions.State = func() State { return State{Spinner: "⠋", Progress: -1} }
	for i := 0; i < 40; i++ {
		p.popular = append(p.popular, entry{id: fmt.Sprintf("acme.ext%d", i), name: fmt.Sprintf("Extension %d", i),
			publisher: "acme", desc: "Does useful things for testing the panel", version: "1.0.0", downloads: 123456})
	}
	p.rebuild()
	p.View()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.HandleMouse(tea.MouseMsg{X: 3, Y: 10, Type: tea.MouseWheelDown})
		if i%20 == 0 {
			p.scroll = 0
		}
		p.View()
	}
}

func TestKittyIcons(t *testing.T) {
	p, _ := newPanel(t)
	p.icons.mode = gKitty
	img := image.NewNRGBA(image.Rect(0, 0, 300, 300)) // bigger than the 128 px upload cap
	p.icons.img["acme.x"] = img

	lines := p.icon("acme.x", "", 6, 3, "#44475a")
	if len(lines) != 3 {
		t.Fatalf("rows: %d", len(lines))
	}
	for _, l := range lines {
		if w := ansi.StringWidth(l); w != 6 {
			t.Errorf("placeholder row is %d cells wide, want 6: %q", w, l)
		}
		if strings.Count(l, string(placeholder)) != 6 {
			t.Errorf("want 6 placeholder cells: %q", l)
		}
	}
	// The first frame carries the upload: PNG, virtual placement, no reply.
	up := p.icons.uploads()
	for _, want := range []string{"\x1b_Ga=T,U=1,f=100,t=d,", ",c=6,r=3,q=2,", "\x1b\\"} {
		if !strings.Contains(up, want) {
			t.Errorf("upload is missing %q", want)
		}
	}
	for _, chunk := range strings.Split(up, "\x1b\\") {
		if i := strings.IndexByte(chunk, ';'); i >= 0 && len(chunk)-i-1 > 4096 {
			t.Errorf("a chunk carries %d bytes; the protocol allows 4096", len(chunk)-i-1)
		}
	}
	// After its window it is not sent again; after a resize it is.
	for _, k := range p.icons.kitty {
		k.until = time.Now().Add(-time.Millisecond)
	}
	p.icon("acme.x", "", 6, 3, "#44475a")
	if up := p.icons.uploads(); up != "" {
		t.Errorf("re-sent after its window: %d bytes", len(up))
	}
	p.Retransmit()
	p.icon("acme.x", "", 6, 3, "#44475a")
	if up := p.icons.uploads(); !strings.Contains(up, "a=T") {
		t.Error("Retransmit should send the image again")
	}
	// A different size is its own image.
	p.icon("acme.x", "", 12, 6, "")
	if len(p.icons.kitty) != 2 {
		t.Errorf("sizes share an image: %d", len(p.icons.kitty))
	}
}

func TestTinyPanelNeverPanics(t *testing.T) {
	fakeOpenVSX(t, nil)
	p, ch := newPanel(t)
	p.Open()
	drain(t, p, ch, func() bool { return len(p.list) == 2 })
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {10, 3}, {59, 30}, {120, 13}, {60, 14}, {61, 15}} {
		p.SetSize(sz[0], sz[1])
		v := p.View()
		lines := strings.Split(v, "\n")
		if sz[0] > 0 && sz[1] > 0 && len(lines) != sz[1] {
			t.Errorf("%v: %d rows", sz, len(lines))
		}
		for i, l := range lines {
			if w := ansi.StringWidth(l); w > sz[0] {
				t.Errorf("%v: row %d is %d wide", sz, i, w)
			}
		}
		p.HandleMouse(tea.MouseMsg{X: 1, Y: 1, Type: tea.MouseWheelDown})
		p.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	}
}

// In a common 80×24 terminal (a 21-row panel), an installed extension with
// code and setup left to do still shows its tabs and README.
func TestShortPageKeepsTabsVisible(t *testing.T) {
	fakeOpenVSX(t, nil)
	p, ch := newPanel(t)
	vsix := filepath.Join(t.TempDir(), "c.vsix")
	f, _ := os.Create(vsix)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("extension/package.json")
	w.Write([]byte(`{"name":"c","publisher":"acme","version":"1.0.0","engines":{"vscode":"^1.80.0"},"main":"./m.js","activationEvents":["onLanguage:yaml"]}`))
	w, _ = zw.Create("extension/m.js")
	w.Write([]byte("exports.activate=()=>{}"))
	w, _ = zw.Create("extension/README.md")
	w.Write([]byte("# Readme\n\nREADME-BODY here.\n"))
	zw.Close()
	f.Close()
	if _, err := extensions.Install(context.Background(), vsix, nil); err != nil {
		t.Fatal(err)
	}
	p.Actions.State = func() State {
		return State{Spinner: "⠋", Progress: -1, NodeProblem: "Needs Node.js 20+", NodePlan: "Install Node.js with Homebrew"}
	}
	p.SetSize(80, 21)
	p.Open()
	drain(t, p, ch, func() bool { return len(p.list) >= 1 && p.sel == "acme.c" })
	v := plain(p.View())
	for _, want := range []string{"DETAILS", "README-BODY", "Install Node.js", "Trust this folder"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q at 80×21:\n%s", want, v)
		}
	}
}

// Every button the page draws is fully on screen where its hit area says.
func TestButtonsAreNeverCutOff(t *testing.T) {
	fakeOpenVSX(t, nil)
	p, ch := newPanel(t)
	p.Actions.State = func() State {
		return State{Spinner: "⠋", Progress: -1, NodeProblem: "Needs Node.js 20+, and none is installed", NodePlan: "Install Node.js 24 to ~/.local"}
	}
	p.Open()
	drain(t, p, ch, func() bool { return len(p.list) == 2 })
	for _, w := range []int{60, 80, 100, 150} {
		p.SetSize(w, 30)
		v := strings.Split(plain(p.View()), "\n")
		for _, h := range p.hits {
			if h.act == "row" || h.act == "search" || h.y0 >= len(v) {
				continue
			}
			if h.x1 > w {
				t.Errorf("width %d: %s button ends at %d", w, h.act, h.x1)
			}
		}
	}
}
