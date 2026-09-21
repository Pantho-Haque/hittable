package mdpreview

import (
	"strings"
	"testing"
)

const sample = "# Title\n\nSome **bold** text and a [link](https://x.test).\n\n- one\n- two\n\n```go\nfunc main() {}\n```\n\n```mermaid\ngraph LR\n  A[Start] --> B[End]\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"

func TestRenderMarkdownAndMermaid(t *testing.T) {
	out := Render(sample, 80)
	plain := stripANSI(out)
	for _, want := range []string{"Title", "bold", "one", "func main", "Start", "End", "1", "2"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "graph LR") {
		t.Errorf("mermaid source should be replaced by a diagram:\n%s", plain)
	}
	if !strings.Contains(plain, "──") && !strings.Contains(plain, "-->") && !strings.Contains(plain, "───") {
		t.Errorf("no diagram edges rendered:\n%s", plain)
	}

	p := New()
	p.SetSize(60, 10)
	p.SetContent(sample)
	if len(p.lines) < 10 {
		t.Fatalf("expected many lines, got %d", len(p.lines))
	}
	if v := p.View(true); strings.Count(v, "\n") != 9 {
		t.Errorf("view rows = %d", strings.Count(v, "\n")+1)
	}
	p.Scroll(5)
	if p.ScrollY != 5 {
		t.Errorf("scroll: %d", p.ScrollY)
	}
	// Broken mermaid falls back to the source block, never panics.
	if out := Render("```mermaid\nthis is not a diagram %%%\n```", 60); !strings.Contains(stripANSI(out), "not a diagram") {
		t.Errorf("fallback missing:\n%s", out)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			in = true
		case in && (r == 'm' || r == 'z'):
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestCodeBlocksNeverWrapOrFlagErrors(t *testing.T) {
	tree := "```\nproject/\n├── app/          # Providers and layout for the whole application shell\n│   └── layout.tsx\n└── api/\n```\n"
	out := Render(tree, 50)
	plain := stripANSI(out)
	if !strings.Contains(plain, "├── app/") || !strings.Contains(plain, "└── api/") {
		t.Fatalf("tree lost:\n%s", plain)
	}
	for _, l := range strings.Split(plain, "\n") {
		if strings.TrimSpace(l) == "Providers" || strings.HasPrefix(strings.TrimSpace(l), "shell") {
			t.Errorf("code line was wrapped:\n%s", plain)
		}
	}
	if strings.Contains(out, "\x1b[48;5;196m") || strings.Contains(out, "\x1b[41m") {
		t.Errorf("red error background present:\n%q", out)
	}
	// A language fence still highlights.
	if !strings.Contains(Render("```go\nfunc main() {}\n```\n", 60), "\x1b[") {
		t.Error("go block not highlighted")
	}
}
