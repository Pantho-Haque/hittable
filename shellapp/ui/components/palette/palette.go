// Package palette is the quick-open / live-grep overlay: type to fuzzy-find a
// file (ctrl+p) or to search file contents with ripgrep / git grep (alt+f).
package palette

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"github.com/hittable/shellapp/ui/theme"
)

type Mode int

const (
	ModeFiles Mode = iota
	ModeGrep
)

// Result is one row: a file, optionally at a line.
type Result struct {
	Path  string // absolute
	Rel   string
	Line  int // 1-based, 0 for file results
	Text  string
	score int
}

// ResultsMsg carries async grep results; stale ones (old Seq) are dropped.
type ResultsMsg struct {
	Seq     int
	Results []Result
}

var skipDirs = map[string]bool{".git": true, "node_modules": true, ".next": true, "dist": true, "build": true, "vendor": true, "target": true, ".cache": true, ".DS_Store": true}

const maxFiles = 30000

type Palette struct {
	Root    string
	Mode    Mode
	Open    bool
	Query   string
	Results []Result
	Cursor  int
	Width   int
	Height  int

	OnOpen func(r Result) // called with the chosen result

	files     []string // relative paths, cached
	filesAt   time.Time
	seq       int
	mu        sync.Mutex
	send      func(tea.Msg)
	searching bool
}

func New(root string, send func(tea.Msg)) *Palette {
	return &Palette{Root: root, send: send, Width: 80, Height: 20}
}

func (p *Palette) SetSize(w, h int) { p.Width, p.Height = w, h }

// Show opens the palette in a mode.
func (p *Palette) Show(mode Mode) {
	p.Open = true
	p.Mode = mode
	p.Query = ""
	p.Cursor = 0
	p.Results = nil
	if mode == ModeFiles {
		p.indexFiles()
		p.search()
	}
}

func (p *Palette) Close() { p.Open = false }

// indexFiles walks the root once per 30s (skipping vendor-ish folders).
func (p *Palette) indexFiles() {
	if time.Since(p.filesAt) < 30*time.Second && p.files != nil {
		return
	}
	var files []string
	filepath.WalkDir(p.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != p.Root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if len(files) >= maxFiles {
			return filepath.SkipAll
		}
		if d.Name() == ".DS_Store" {
			return nil
		}
		rel, _ := filepath.Rel(p.Root, path)
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	p.files, p.filesAt = files, time.Now()
}

// fuzzyScore scores a subsequence match of q in s (higher is better), -1 if
// no match. Basename hits and contiguous runs score higher.
func fuzzyScore(s, q string) int {
	if q == "" {
		return 0
	}
	ls, lq := strings.ToLower(s), strings.ToLower(q)
	if i := strings.Index(ls, lq); i >= 0 {
		score := 1000 - i
		if base := strings.LastIndexByte(ls, '/') + 1; i >= base {
			score += 500
		}
		return score
	}
	score, si, streak := 0, 0, 0
	for _, qc := range lq {
		found := false
		for si < len(ls) {
			c := rune(ls[si])
			si++
			if c == qc {
				found = true
				streak++
				score += 10 + streak*5
				if si >= 2 && (ls[si-2] == '/' || ls[si-2] == '_' || ls[si-2] == '-' || ls[si-2] == '.') {
					score += 20
				}
				break
			}
			streak = 0
		}
		if !found {
			return -1
		}
	}
	return score - len(s)/10
}

func (p *Palette) search() {
	p.Cursor = 0
	if p.Mode == ModeFiles {
		var out []Result
		for _, f := range p.files {
			if sc := fuzzyScore(f, p.Query); sc >= 0 {
				out = append(out, Result{Path: filepath.Join(p.Root, filepath.FromSlash(f)), Rel: f, score: sc})
				if p.Query == "" && len(out) >= 200 {
					break
				}
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
		if len(out) > 200 {
			out = out[:200]
		}
		p.Results = out
		return
	}
	// Grep: async, min 2 chars.
	q := strings.TrimSpace(p.Query)
	if len(q) < 2 {
		p.Results = nil
		return
	}
	p.seq++
	seq := p.seq
	p.searching = true
	go func() {
		res := grep(p.Root, q)
		if p.send != nil {
			p.send(ResultsMsg{Seq: seq, Results: res})
		}
	}()
}

// Deliver installs async results if they are for the latest query.
func (p *Palette) Deliver(msg ResultsMsg) {
	if msg.Seq != p.seq {
		return
	}
	p.searching = false
	p.Results = msg.Results
	p.Cursor = 0
}

// grep uses ripgrep when available, else git grep, else a Go walk.
func grep(root, q string) []Result {
	var out []byte
	var err error
	switch {
	case hasExe("rg"):
		out, err = exec.Command("rg", "--line-number", "--no-heading", "--color=never", "--smart-case", "--max-count=50", "--max-columns=200", "-g", "!node_modules", "-g", "!.next", "--", q, root).Output()
	case hasExe("git") && exec.Command("git", "-C", root, "rev-parse").Run() == nil:
		out, err = exec.Command("git", "-C", root, "grep", "-n", "-i", "--untracked", "--", q).Output()
		out = prefixRoot(out, root)
	default:
		return walkGrep(root, q)
	}
	if err != nil && len(out) == 0 {
		return nil
	}
	var res []Result
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() && len(res) < 500 {
		line := sc.Text()
		// path:line:text  (path may contain ':' on Windows only)
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		j := strings.Index(line[i+1:], ":")
		if j < 0 {
			continue
		}
		n, e := strconv.Atoi(line[i+1 : i+1+j])
		if e != nil {
			continue
		}
		path := line[:i]
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		rel, _ := filepath.Rel(root, path)
		res = append(res, Result{Path: path, Rel: filepath.ToSlash(rel), Line: n, Text: strings.TrimSpace(line[i+1+j+1:])})
	}
	return res
}

func prefixRoot(out []byte, root string) []byte {
	var b bytes.Buffer
	for _, l := range bytes.Split(out, []byte("\n")) {
		if len(l) > 0 {
			b.WriteString(root + "/")
			b.Write(l)
			b.WriteByte('\n')
		}
	}
	return b.Bytes()
}

func hasExe(name string) bool { _, err := exec.LookPath(name); return err == nil }

func walkGrep(root, q string) []Result {
	var res []Result
	lq := strings.ToLower(q)
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || len(res) >= 500 {
			return nil
		}
		if d.IsDir() {
			if path != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for n := 1; sc.Scan(); n++ {
			if strings.Contains(strings.ToLower(sc.Text()), lq) {
				rel, _ := filepath.Rel(root, path)
				res = append(res, Result{Path: path, Rel: filepath.ToSlash(rel), Line: n, Text: strings.TrimSpace(sc.Text())})
			}
		}
		return nil
	})
	return res
}

// ---------- input ----------

func (p *Palette) listRows() int {
	n := p.Height - 5 // border(2) + input + blank + footer
	if n < 1 {
		n = 1
	}
	return n
}

// HandleKey returns true if consumed; Esc closes.
func (p *Palette) HandleKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc":
		p.Close()
	case "enter":
		if p.Cursor < len(p.Results) && p.OnOpen != nil {
			r := p.Results[p.Cursor]
			p.Close()
			p.OnOpen(r)
		}
	case "up", "ctrl+p", "ctrl+k":
		if p.Cursor > 0 {
			p.Cursor--
		}
	case "down", "ctrl+n", "ctrl+j":
		if p.Cursor < len(p.Results)-1 {
			p.Cursor++
		}
	case "pgup":
		p.Cursor -= p.listRows()
		if p.Cursor < 0 {
			p.Cursor = 0
		}
	case "pgdown":
		p.Cursor += p.listRows()
		if p.Cursor >= len(p.Results) {
			p.Cursor = len(p.Results) - 1
		}
	case "tab":
		p.Mode = (p.Mode + 1) % 2
		p.Show(p.Mode)
	case "backspace":
		if r := []rune(p.Query); len(r) > 0 {
			p.Query = string(r[:len(r)-1])
			p.search()
		}
	case "ctrl+u":
		p.Query = ""
		p.search()
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			p.Query += string(msg.Runes)
			p.search()
		}
	}
	return true
}

// HandleMouse takes coordinates relative to the palette box.
func (p *Palette) HandleMouse(msg tea.MouseMsg) {
	const listTop = 3 // border + input + blank
	switch msg.Type {
	case tea.MouseLeft:
		if msg.Action != tea.MouseActionPress {
			return
		}
		if i := msg.Y - listTop + p.scroll(); i >= 0 && i < len(p.Results) {
			if i == p.Cursor {
				p.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
			} else {
				p.Cursor = i
			}
		}
	case tea.MouseWheelUp:
		p.Cursor -= 3
		if p.Cursor < 0 {
			p.Cursor = 0
		}
	case tea.MouseWheelDown:
		p.Cursor += 3
		if p.Cursor >= len(p.Results) {
			p.Cursor = len(p.Results) - 1
		}
	}
}

func (p *Palette) scroll() int {
	rows := p.listRows()
	if p.Cursor < rows {
		return 0
	}
	return p.Cursor - rows + 1
}

// ---------- view ----------

func (p *Palette) View(z *zone.Manager) string {
	inner := p.Width - 2
	title := "Find file"
	prompt := "  "
	if p.Mode == ModeGrep {
		title = "Live grep"
		prompt = "  "
	}
	tabs := theme.TabActiveStyle.Render("["+title+"]") + theme.MutedStyle.Render("  tab: switch to ")
	if p.Mode == ModeGrep {
		tabs += theme.MutedStyle.Render("find file")
	} else {
		tabs += theme.MutedStyle.Render("live grep")
	}
	lines := []string{
		theme.PromptStyle.Render(prompt+p.Query+"▏") + "  " + tabs,
		"",
	}
	start := p.scroll()
	for i := start; i < start+p.listRows(); i++ {
		if i >= len(p.Results) {
			lines = append(lines, "")
			continue
		}
		r := p.Results[i]
		var row string
		if r.Line > 0 {
			row = " " + theme.AuthorStyle.Render(r.Rel) + theme.MutedStyle.Render(":"+strconv.Itoa(r.Line)+"  ") + highlightMatch(r.Text, p.Query)
		} else {
			dir, base := filepath.Split(r.Rel)
			row = " " + highlightMatch(base, p.Query) + "  " + theme.MutedStyle.Render(dir)
		}
		row = ansi.Truncate(row, inner, "…")
		if i == p.Cursor {
			plain := ansi.Strip(row)
			if w := lipgloss.Width(plain); w < inner {
				plain += strings.Repeat(" ", inner-w)
			}
			row = theme.CursorFocusedStyle.Render(plain)
		}
		lines = append(lines, z.Mark(fmt.Sprintf("pal_%d", i), row))
	}
	status := fmt.Sprintf(" %d results", len(p.Results))
	if p.searching {
		status = " searching…"
	} else if p.Mode == ModeGrep && len(strings.TrimSpace(p.Query)) < 2 {
		status = " type at least 2 characters"
	}
	lines = append(lines, theme.MutedStyle.Render(status+" · ↑↓ move · ⏎ open · esc close"))
	return theme.FocusedBorderStyle.Width(inner).Height(p.Height - 2).MaxHeight(p.Height).Render(strings.Join(lines, "\n"))
}

// highlightMatch bolds the query characters inside s (case-insensitive).
func highlightMatch(s, q string) string {
	if q == "" {
		return s
	}
	ls, lq := strings.ToLower(s), strings.ToLower(q)
	if i := strings.Index(ls, lq); i >= 0 {
		return s[:i] + theme.WordmarkStyle.Render(s[i:i+len(q)]) + s[i+len(q):]
	}
	var b strings.Builder
	qi := 0
	qr := []rune(lq)
	for _, c := range s {
		if qi < len(qr) && unicode.ToLower(c) == qr[qi] {
			b.WriteString(theme.WordmarkStyle.Render(string(c)))
			qi++
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}
