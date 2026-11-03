package gitpanel

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hittable/shellapp/internal/gitx"
	"github.com/hittable/shellapp/ui/theme"
)

// cmdOutput is what the last git command printed, for the output view: the
// whole thing, so a hook that refused a commit (husky, lint-staged, a
// linter, a test run) can be read in full instead of guessed at from one
// line.
type cmdOutput struct {
	label   string
	command string
	text    string
	failed  bool
	code    int
	at      time.Time
	key     string // render cache key
	lines   []string
}

// record keeps a command's output. A failure opens the output view on its
// own, scrolled to the top; a success is kept for ! without interrupting.
func (p *Panel) record(label, out string, err error) {
	o := &cmdOutput{label: label, text: out, at: time.Now(), code: 0}
	if err != nil {
		o.failed = true
		o.text, o.code = err.Error(), -1
		var ce *gitx.CmdError
		if errors.As(err, &ce) {
			o.command, o.code = ce.Command(), ce.Code
			if strings.TrimSpace(ce.Output) != "" {
				o.text = ce.Output
			}
		}
	}
	if strings.TrimSpace(o.text) == "" && !o.failed {
		return // nothing worth showing; keep the previous output
	}
	p.out = o
	if o.failed {
		p.openOutput()
	}
}

func (p *Panel) openOutput() {
	if p.out == nil {
		p.Message = "no command output yet"
		return
	}
	p.ShowOutput, p.FocusDiff = true, true
	p.detailScroll, p.diffCursor = 0, 0
	p.selAnchor, p.selEnd = -1, -1
	p.detailCache = nil
}

func (p *Panel) closeOutput() {
	p.ShowOutput = false
	p.FocusDiff = false
	p.loadDetail()
}

func (p *Panel) toggleOutput() {
	if p.ShowOutput {
		p.closeOutput()
	} else {
		p.openOutput()
	}
}

// outputKey handles keys while the output view is up. Movement and copying
// work as in the diff; esc goes back; anything else closes the view and
// does what it would have done.
func (p *Panel) outputKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc":
		p.closeOutput()
		return true
	case "y":
		if err := clipboard.WriteAll(p.out.text); err != nil {
			p.Message = "clipboard unavailable"
		} else {
			p.Message = "copied the output"
		}
		return true
	case "ctrl+c":
		p.CopySelection()
		return true
	case "up", "k", "down", "j", "shift+up", "shift+down", "pgup", "pgdown", "ctrl+u", "ctrl+d", " ", "g", "G", "home", "end", "J", "K":
		switch msg.String() {
		case "J":
			p.detailScroll++
			p.clamp()
		case "K":
			p.detailScroll--
			p.clamp()
		default:
			p.FocusDiff = true
			p.handleDiffKey(msg)
		}
		return true
	case "tab":
		return true
	}
	p.closeOutput()
	return false
}

var (
	outErr  = lipgloss.NewStyle().Foreground(theme.ErrorColor)
	outWarn = lipgloss.NewStyle().Foreground(theme.WarningColor)
	outOK   = lipgloss.NewStyle().Foreground(theme.AccentColor)
)

// outputLines renders the output view: a header naming the command and its
// result, then every line it printed, wrapped to the pane, with error and
// warning lines tinted so the cause stands out.
func (p *Panel) outputLines() []string {
	o := p.out
	w := p.detailWidth()
	key := fmt.Sprintf("%d|%p", w, o)
	if o.key == key {
		p.detailSrc = make([][]int, len(o.lines))
		for i := range o.lines {
			p.detailSrc[i] = []int{i}
		}
		return o.lines
	}
	var out []string
	head := outOK.Render("✓ " + o.label)
	if o.failed {
		head = outErr.Bold(true).Render("✖ " + o.label + " failed")
		if o.code >= 0 {
			head += outErr.Render(fmt.Sprintf(" (exit %d)", o.code))
		}
	}
	head += theme.MutedStyle.Render("  ·  " + o.at.Format("15:04:05"))
	out = append(out, head)
	if o.command != "" {
		out = append(out, theme.MutedStyle.Render("$ "+o.command))
	}
	if hint := hookHint(o.text); hint != "" {
		out = append(out, outWarn.Render(hint))
	}
	out = append(out, theme.MutedStyle.Render(strings.Repeat("─", w)))
	for _, l := range strings.Split(strings.TrimRight(ansiSeq.ReplaceAllString(o.text, ""), "\n"), "\n") {
		l = strings.ReplaceAll(l, "\t", "    ")
		st := lipgloss.NewStyle()
		low := strings.ToLower(l)
		switch {
		case strings.Contains(low, " error ") || strings.Contains(l, "✖") || strings.Contains(l, "❌") ||
			strings.HasPrefix(low, "error") || strings.HasPrefix(low, "fatal") || strings.Contains(low, "failed"):
			st = outErr
		case strings.Contains(low, " warning ") || strings.HasPrefix(low, "warning") || strings.HasPrefix(low, "hint:"):
			st = outWarn
		case strings.Contains(l, "✔") || strings.HasPrefix(low, "ok "):
			st = outOK
		}
		for _, c := range fullWidth(l, w, true) {
			out = append(out, st.Render(c))
		}
	}
	o.key, o.lines = key, out
	p.detailSrc = make([][]int, len(out))
	for i := range out {
		p.detailSrc[i] = []int{i}
	}
	return out
}

// hookHint names the hook when one refused the command.
func hookHint(text string) string {
	switch {
	case strings.Contains(text, "pre-commit"):
		return "The pre-commit hook refused the commit. Fix what it reports below, then commit again (your message is kept)."
	case strings.Contains(text, "commit-msg"):
		return "The commit-msg hook refused the message. Edit it (c) to follow the rule below."
	case strings.Contains(text, "pre-push"):
		return "The pre-push hook refused the push. Fix what it reports below, then push again."
	}
	return ""
}
