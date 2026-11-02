// Package pathprompt is a one-line file path input with Tab completion, for
// picking a file the project's find palette cannot reach (single-file mode,
// or anything outside the project).
package pathprompt

import (
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/hittable/shellapp/internal/hitfile"
	"github.com/hittable/shellapp/ui/theme"
)

type Prompt struct {
	Open  bool
	Value string
	Base  string // relative paths start here
	note  string // matches or "no match" after a Tab
}

func (p *Prompt) Show(base string) {
	*p = Prompt{Open: true, Base: base}
}

// HandleKey edits the value. On enter it closes and returns the absolute path
// and true; esc closes and returns false.
func (p *Prompt) HandleKey(msg tea.KeyMsg) (string, bool) {
	p.note = ""
	switch msg.String() {
	case "esc":
		p.Open = false
	case "enter":
		if strings.TrimSpace(p.Value) == "" {
			return "", false
		}
		p.Open = false
		return hitfile.ResolvePath(p.Base, strings.TrimSpace(p.Value)), true
	case "tab":
		var matches []string
		p.Value, matches = Complete(p.Base, p.Value)
		switch {
		case len(matches) == 0:
			p.note = "no match"
		case len(matches) > 1:
			p.note = strings.Join(matches, "  ")
		}
	case "backspace", "ctrl+h":
		if r := []rune(p.Value); len(r) > 0 {
			p.Value = string(r[:len(r)-1])
		}
	case "ctrl+w": // back to the previous folder
		v := strings.TrimSuffix(p.Value, "/")
		p.Value = v[:strings.LastIndex(v, "/")+1]
	case "ctrl+u":
		p.Value = ""
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			p.Value += string(msg.Runes)
		}
	}
	return "", false
}

// Complete extends value to the longest name in its folder that every match
// shares (a folder gets its trailing /), and lists the matches. Hidden files
// show only when the typed name starts with a dot.
func Complete(base, value string) (string, []string) {
	head := value[:strings.LastIndex(value, "/")+1]
	prefix := value[len(head):]
	dir := base
	if head != "" {
		dir = hitfile.ResolvePath(base, head)
	} else if value == "~" {
		return "~/", []string{"~/"}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return value, nil
	}
	var matches []string
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, prefix) || strings.HasPrefix(n, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		if e.IsDir() {
			n += "/"
		} else if e.Type()&os.ModeSymlink != 0 {
			if fi, err := os.Stat(dir + "/" + n); err == nil && fi.IsDir() {
				n += "/"
			}
		}
		matches = append(matches, n)
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return value, nil
	}
	common := matches[0]
	for _, m := range matches[1:] {
		for !strings.HasPrefix(m, common) {
			common = common[:len(common)-1]
		}
	}
	return head + common, matches
}

func (p *Prompt) View(width int) string {
	hint := "⇥ complete · ⏎ attach · esc cancel"
	if p.note != "" {
		hint = p.note
	}
	s := theme.PromptStyle.Render(" attach file: "+p.Value+"▏") + theme.MutedStyle.Render("  "+hint)
	return ansi.Truncate(s, width, "…")
}
