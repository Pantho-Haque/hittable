// Package mdpreview renders Markdown for the terminal (glamour, Dracula
// style) with ```mermaid fences converted to ASCII diagrams (mermaid-ascii:
// flowcharts, sequence and ER diagrams). Rendering is cached per
// (content, width) so live preview stays cheap while typing.
package mdpreview

import (
	"crypto/sha1"
	"fmt"
	"regexp"
	"strings"

	"github.com/AlexanderGrooff/mermaid-ascii/pkg/diagram"
	"github.com/AlexanderGrooff/mermaid-ascii/pkg/render"
	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	cstyles "github.com/alecthomas/chroma/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	gansi "github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/ui/theme"
)

var (
	mermaidFence = regexp.MustCompile("(?s)```mermaid[ \\t]*\\n(.*?)\\n```")
	// Any other fence: language on the opening line, body until the closing fence.
	codeFence = regexp.MustCompile("(?s)(?m)^```([A-Za-z0-9_+-]*)[^\\n]*\\n(.*?)\\n```[ \\t]*$")
)

// codeStyle is Dracula without the red "Error" background: unknown tokens in
// snippets (box-drawing trees, shell prompts…) must not be flagged.
var codeStyle = func() *chroma.Style {
	b := cstyles.Get("dracula").Builder()
	b.Add(chroma.Error, "#f8f8f2")
	st, err := b.Build()
	if err != nil {
		return cstyles.Get("dracula")
	}
	return st
}()

// renderCode highlights a fenced block without wrapping: each line is
// truncated to width, so trees and long commands keep their shape.
func renderCode(lang, code string, width int) string {
	body := code
	if lang != "" && lang != "text" && lang != "txt" && lang != "plain" {
		if lx := lexers.Get(lang); lx != nil {
			it, err := chroma.Coalesce(lx).Tokenise(nil, code)
			if err == nil {
				var sb strings.Builder
				if formatters.Get("terminal256").Format(&sb, codeStyle, it) == nil {
					body = strings.TrimRight(sb.String(), "\n")
				}
			}
		}
	}
	var out []string
	for _, l := range strings.Split(body, "\n") {
		l = strings.ReplaceAll(l, "\t", "    ")
		l = ansi.Truncate(l, width-4, "…")
		pad := width - 4 - lipgloss.Width(l)
		if pad < 0 {
			pad = 0
		}
		out = append(out, "  "+theme.CodeBlockStyle.Render(" "+l+strings.Repeat(" ", pad)+" "))
	}
	return strings.Join(out, "\n")
}

// renderMermaid converts one mermaid block to ASCII; on any failure the
// source is kept as a labelled code block.
func renderMermaid(src string) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = "```mermaid\n" + src + "\n```"
		}
	}()
	cfg := diagram.DefaultConfig()
	cfg.UseAscii = false // box-drawing characters
	text, err := render.RenderDiagram(src, cfg)
	if err != nil || strings.TrimSpace(text) == "" {
		return "```mermaid\n" + src + "\n```"
	}
	return "```text\n" + strings.TrimRight(text, "\n") + "\n```"
}

func sp(v string) *string { return &v }
func bp(v bool) *bool     { return &v }
func up(v uint) *uint     { return &v }

// style is Dracula with document-like headings (no # marks: H1 is a filled
// title bar, H2/H3 coloured and bold), a quote bar, inline-code chips,
// checkbox tasks and a full-width horizontal rule.
func style(width int) gansi.StyleConfig {
	st := styles.DraculaStyleConfig
	st.Document.Margin = up(2)
	st.Heading = gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Bold: bp(true), Color: sp("#bd93f9")}, Margin: up(0)}
	st.H1 = gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{
		Prefix: " ", Suffix: " ", Color: sp("#282a36"), BackgroundColor: sp("#bd93f9"), Bold: bp(true), BlockPrefix: "\n", BlockSuffix: "\n"}}
	st.H2 = gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Prefix: "", Color: sp("#8be9fd"), Bold: bp(true), BlockPrefix: "\n"}}
	st.H3 = gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Prefix: "", Color: sp("#50fa7b"), Bold: bp(true), BlockPrefix: "\n"}}
	st.H4 = gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Prefix: "", Color: sp("#f1fa8c"), Bold: bp(true)}}
	st.H5 = gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Prefix: "", Color: sp("#ffb86c"), Bold: bp(true)}}
	st.H6 = gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Prefix: "", Color: sp("#6272a4"), Bold: bp(true)}}
	st.BlockQuote = gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: sp("#bfbfbf"), Italic: bp(true)}, Indent: up(1), IndentToken: sp("▎ ")}
	st.Code = gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: sp("#50fa7b"), BackgroundColor: sp("#343746"), Prefix: " ", Suffix: " "}}
	st.Link = gansi.StylePrimitive{Color: sp("#8be9fd"), Underline: bp(true)}
	st.LinkText = gansi.StylePrimitive{Color: sp("#f8f8f2"), Bold: bp(true)}
	st.Task = gansi.StyleTask{Ticked: "☑ ", Unticked: "☐ "}
	st.Item = gansi.StylePrimitive{BlockPrefix: "• "}
	st.Enumeration = gansi.StylePrimitive{BlockPrefix: ". ", Color: sp("#bd93f9")}
	st.List.LevelIndent = 2
	rule := strings.Repeat("─", width-4)
	st.HorizontalRule = gansi.StylePrimitive{Color: sp("#44475a"), Format: "\n" + rule + "\n"}
	st.Table.CenterSeparator = sp("┼")
	st.Table.ColumnSeparator = sp("│")
	st.Table.RowSeparator = sp("─")
	return st
}

var (
	itemStart = regexp.MustCompile(`^(\s*)(• |\d+\. |[☑☐] )`)
)

// hangIndent gives wrapped list-item lines the same indent as the item's
// text, which glamour does not do on its own.
func hangIndent(out string) string {
	lines := strings.Split(out, "\n")
	indent := -1 // text column of the current item, -1 = not in an item
	for i, l := range lines {
		plain := ansi.Strip(l)
		trimmed := strings.TrimRight(plain, " ")
		switch {
		case trimmed == "":
			indent = -1
		case itemStart.MatchString(plain):
			m := itemStart.FindStringSubmatch(plain)
			indent = len([]rune(m[1])) + lipgloss.Width(m[2])
		case indent >= 0:
			lead := len(plain) - len(strings.TrimLeft(plain, " "))
			if lead < indent {
				lines[i] = strings.Repeat(" ", indent-lead) + l
			}
		}
	}
	return strings.Join(lines, "\n")
}

// Render turns markdown into styled terminal text wrapped to width. Fenced
// code (and mermaid diagrams) are rendered separately and spliced back in so
// they are never word-wrapped.
func Render(md string, width int) string {
	if width < 20 {
		width = 20
	}
	md = mermaidFence.ReplaceAllStringFunc(md, func(block string) string {
		m := mermaidFence.FindStringSubmatch(block)
		if len(m) < 2 {
			return block
		}
		return renderMermaid(m[1])
	})
	var blocks []string
	md = codeFence.ReplaceAllStringFunc(md, func(block string) string {
		m := codeFence.FindStringSubmatch(block)
		if len(m) < 3 {
			return block
		}
		blocks = append(blocks, renderCode(strings.ToLower(m[1]), m[2], width))
		return fmt.Sprintf("\n\nCODEBLOCK%dMARK\n\n", len(blocks)-1)
	})
	out := renderMarkdown(md, width)
	if len(blocks) == 0 {
		return out
	}
	lines := strings.Split(out, "\n")
	var res []string
	for _, l := range lines {
		plain := strings.TrimSpace(ansi.Strip(l))
		var idx int
		if n, _ := fmt.Sscanf(plain, "CODEBLOCK%dMARK", &idx); n == 1 && idx < len(blocks) {
			res = append(res, strings.Split(blocks[idx], "\n")...)
			continue
		}
		res = append(res, l)
	}
	return strings.Join(res, "\n")
}

func renderMarkdown(md string, width int) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style(width)),
		glamour.WithWordWrap(width-6), // slack for list hanging indents
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return hangIndent(strings.Trim(out, "\n"))
}

// Preview is a scrollable rendered-markdown pane.
type Preview struct {
	Width, Height int
	ScrollY       int
	lines         []string
	key           string
}

func New() *Preview { return &Preview{Width: 80, Height: 24} }

func (p *Preview) SetSize(w, h int) { p.Width, p.Height = w, h }

// SetContent re-renders when the content or width changed.
func (p *Preview) SetContent(md string) {
	inner := p.Width - 2
	key := fmt.Sprintf("%x:%d", sha1.Sum([]byte(md)), inner)
	if key == p.key {
		return
	}
	p.key = key
	p.lines = strings.Split(Render(md, inner), "\n")
	p.clamp()
}

func (p *Preview) rows() int {
	n := p.Height - 2
	if n < 1 {
		n = 1
	}
	return n
}

func (p *Preview) clamp() {
	if max := len(p.lines) - p.rows(); p.ScrollY > max {
		p.ScrollY = max
	}
	if p.ScrollY < 0 {
		p.ScrollY = 0
	}
}

func (p *Preview) Scroll(delta int) { p.ScrollY += delta; p.clamp() }

// ScrollToFraction positions the view proportionally (editor ↔ preview sync).
func (p *Preview) ScrollToFraction(f float64) {
	p.ScrollY = int(f * float64(len(p.lines)-p.rows()))
	p.clamp()
}

func (p *Preview) HandleKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "up", "k":
		p.Scroll(-1)
	case "down", "j":
		p.Scroll(1)
	case "pgup", "ctrl+u", "b":
		p.Scroll(-(p.rows() - 1))
	case "pgdown", "ctrl+d", " ", "f":
		p.Scroll(p.rows() - 1)
	case "home", "g":
		p.ScrollY = 0
	case "end", "G":
		p.ScrollY = len(p.lines)
		p.clamp()
	default:
		return false
	}
	return true
}

func (p *Preview) View(focused bool) string {
	border := theme.UnfocusedBorderStyle
	if focused {
		border = theme.FocusedBorderStyle
	}
	inner := p.Width - 2
	var out []string
	for i := p.ScrollY; i < p.ScrollY+p.rows(); i++ {
		if i >= len(p.lines) {
			out = append(out, "")
			continue
		}
		out = append(out, ansi.Truncate(p.lines[i], inner, "…"))
	}
	if len(p.lines) > p.rows() {
		pct := 100 * (p.ScrollY + p.rows()) / len(p.lines)
		if pct > 100 {
			pct = 100
		}
		out[len(out)-1] = ansi.Truncate(out[len(out)-1], inner-6, "") +
			strings.Repeat(" ", max(0, inner-6-lipgloss.Width(out[len(out)-1]))) + theme.MutedStyle.Render(fmt.Sprintf("%3d%%", pct))
	}
	return border.Width(inner).Height(p.rows()).MaxHeight(p.Height).Render(strings.Join(out, "\n"))
}
