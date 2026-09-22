package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/gitx"
	"github.com/hittable/shellapp/internal/hitfile"
	"github.com/hittable/shellapp/ui/components/explorer"
	"github.com/hittable/shellapp/ui/components/gitpanel"
	"github.com/hittable/shellapp/ui/theme"
)

const blameWidth = 34

// wireGit connects the git panel's hooks to the screen.
func (m *MainScreen) wireGit() {
	g := m.Git
	// A discard / checkout / stash pop rewrites working-tree files, so the
	// open documents have to pick that up before the next autosave writes the
	// stale buffer back over it.
	g.OnChanged = func() { m.reloadExternalEdits(); m.refreshGit(true) }
	g.OnOpenFile = func(abs string) {
		m.GitOpen = false
		m.openFileRaw(abs)
	}
	g.OnGotoLine = func(line int) {
		if m.ActiveFile == "" {
			return
		}
		m.GitOpen = false
		if doc := m.Store.Get(m.ActiveFile); doc != nil && doc.Kind == document.KindHit && m.ViewMode == ViewRunner {
			m.toggleViewMode()
		}
		m.Focus = FocusTextEditor
		m.focusCurrent()
		m.TextEd.GotoLine(line)
	}
	g.OnBlameToggle = func(on bool) {
		m.BlameOn = on
		m.applyBlame()
	}
	g.SourceLines = func() []string {
		if m.ActiveFile == "" {
			return nil
		}
		if doc := m.Store.Get(m.ActiveFile); doc != nil {
			return strings.Split(doc.Content, "\n")
		}
		return nil
	}
	g.LoadFile = func(abs string) (string, error) {
		if doc := m.Store.Get(abs); doc != nil {
			return doc.Content, nil
		}
		b, err := os.ReadFile(abs)
		return string(b), err
	}
	g.SaveFile = func(abs, content string) {
		doc := m.Store.Get(abs)
		if doc == nil {
			_ = os.WriteFile(abs, []byte(content), 0o644)
			return
		}
		doc.Lock()
		doc.SetContent(content)
		gen := doc.IncGeneration()
		doc.Unlock()
		m.WriteQueue.Update(abs, content, gen)
		if m.ActiveFile == abs {
			if m.ViewMode == ViewText {
				m.TextEd.SetContent(abs, content)
			} else if h, err := hitfile.Parse([]byte(content)); err == nil {
				doc.HitContent = h
				m.loadHitIntoRunner(h)
			}
		}
	}
	g.Async = func(label string, fn func() (string, error)) {
		if teaProgram == nil {
			out, err := fn()
			g.Done(gitpanel.DoneMsg{Label: label, Out: out, Err: err})
			return
		}
		go func() {
			out, err := fn()
			teaProgram.Send(gitpanel.DoneMsg{Label: label, Out: out, Err: err})
		}()
	}
	m.TextEd.LineHint = func(row int) string {
		if !m.BlameOn || row >= len(m.blame) {
			return ""
		}
		b := m.blame[row]
		return fmt.Sprintf("%s, %s • %s", b.Author, gitx.Ago(b.Time), b.Summary)
	}
}

// refreshGit reloads status decorations (explorer badges, top-bar branch) and
// the blame overlay. Unless force is set it is rate-limited, so it can be
// called from hot paths like autosave.
func (m *MainScreen) refreshGit(force bool) {
	if m.Repo == nil {
		return
	}
	if !force && time.Since(m.lastGitRefresh) < 2*time.Second {
		return
	}
	m.lastGitRefresh = time.Now()
	st, err := m.Repo.Status()
	if err != nil {
		return
	}
	m.gitKey = statusKey(st)
	m.Git.Status = st
	badges := make(map[string]string, len(st.Files))
	for _, f := range st.Files {
		badges[m.Repo.Abs(f.Path)] = f.Badge()
	}
	m.Explorer.SetGitStatus(badges)
	if m.BlameOn {
		m.applyBlame()
	}
}

// applyBlame loads blame for the active file into the editor gutter.
func (m *MainScreen) applyBlame() {
	if !m.BlameOn || m.Repo == nil || m.ActiveFile == "" {
		m.blame = nil
		m.TextEd.SetAnnotations(nil, 0)
		return
	}
	lines, err := m.Repo.Blame(m.Repo.Rel(m.ActiveFile))
	if err != nil {
		m.blame = nil
		m.TextEd.SetAnnotations(nil, 0)
		m.StatusBar = "blame: " + err.Error()
		return
	}
	m.blame = lines
	ann := make([]string, len(lines))
	for i, b := range lines {
		ann[i] = fmt.Sprintf("%-12s %-8s %s", ansi.Truncate(b.Author, 12, "…"), gitx.Ago(b.Time), b.Summary)
	}
	m.TextEd.SetAnnotations(ann, blameWidth)
}

// toggleGit opens / closes the git panel in the main pane.
func (m *MainScreen) toggleGit() {
	if m.GitOpen {
		m.GitOpen = false
		if m.ActiveFile != "" {
			m.focusCurrent()
		} else {
			m.setExplorerFocused(true)
		}
		return
	}
	m.closePanels()
	m.GitOpen = true
	m.blurAll()
	m.setExplorerFocused(false)
	m.Git.SetActiveFile(m.ActiveFile)
	m.Git.Refresh()
}

func (m *MainScreen) handleGitKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+b":
		m.GitOpen = false
		m.setExplorerFocused(true)
		m.Focus = FocusExplorerPane
		return m, nil
	case "ctrl+g":
		m.toggleGit()
		return m, nil
	}
	if !m.Git.HandleKey(msg) {
		m.toggleGit() // esc with nothing to dismiss
	}
	return m, nil
}

// renderTopBar draws the navbar: brand block, project name, pill buttons
// (hover / active states) and a branch pill on the right.
func (m *MainScreen) renderTopBar() string {
	bg := theme.TopBarStyle
	sp := func(n int) string { return bg.Render(strings.Repeat(" ", n)) }
	btn := func(id, label string, active bool) string {
		st := theme.TopBarButtonStyle
		switch {
		case active:
			st = theme.TopBarActiveStyle
		case m.HoverZone == id:
			st = theme.TopBarHoverStyle
		}
		return m.Zones.Mark(id, st.Render(label))
	}
	gitIcon, findIcon, termIcon, helpIcon := "", "", "", "?"
	if explorer.IconMode == "emoji" {
		gitIcon, findIcon, termIcon = "⑂", "🔍", "▤"
	}

	sidebar := btn("top_sidebar", "☰", !m.explorerHidden())
	left := sidebar + sp(1) + theme.HitMarkStyle.Render(" H ") + sp(1) + theme.TopBarBrandStyle.Render("HITTABLE") +
		theme.TopBarDimStyle.Render("  ›  ") + theme.TopBarTextStyle.Render(filepath.Base(m.RootDir)) + sp(4)
	buttons := btn("top_git", gitIcon+" Git", m.GitOpen) + sp(1) +
		btn("top_find", findIcon+" Find", m.Palette.Open) + sp(1) +
		btn("top_term", termIcon+" Terminal", m.Term.Open) + sp(1) +
		btn("top_help", helpIcon+" Help", m.ShowHelp)

	right := ""
	switch {
	case m.Git.Status != nil:
		st := m.Git.Status
		badge := gitIcon + " " + st.Branch
		if n := len(st.Files); n > 0 {
			badge += theme.MutedStyle.Background(lipgloss.Color("#343746")).Render(fmt.Sprintf("  ●%d", n))
		}
		syncSt := theme.TopBarButtonStyle
		if m.HoverZone == "top_sync" {
			syncSt = theme.TopBarHoverStyle
		}
		if m.Git.Busy != "" {
			syncSt = theme.TopBarActiveStyle
		}
		right = m.Zones.Mark("top_branch",
			theme.Hoverable(m.HoverZone == "top_branch", theme.TopBarBranchStyle).Render(badge)) + sp(1) +
			m.Zones.Mark("top_sync", syncSt.Render(strings.TrimSpace(m.Git.SyncLabel()))) + sp(1)
	case m.Repo == nil:
		right = theme.TopBarDimStyle.Render("not a git repo") + sp(1)
	}

	line := left + buttons
	if gap := m.Width - lipgloss.Width(line) - lipgloss.Width(right); gap > 0 {
		line += sp(gap)
	}
	line += right
	return ansi.Truncate(line, m.Width, "")
}

// ---------- realtime status ----------

type gitTickMsg struct{}
type gitStatusMsg struct {
	st  *gitx.Status
	key string
}

const gitPollInterval = 2 * time.Second

func gitTick() tea.Cmd {
	return tea.Tick(gitPollInterval, func(time.Time) tea.Msg { return gitTickMsg{} })
}

// pollGit runs `git status` off the UI goroutine; the result arrives as a
// gitStatusMsg and is applied only when something changed.
func (m *MainScreen) pollGit() tea.Cmd {
	if m.Repo == nil || m.gitPolling {
		return gitTick()
	}
	m.gitPolling = true
	repo := m.Repo
	go func() {
		st, err := repo.Status()
		if err != nil || teaProgram == nil {
			if teaProgram != nil {
				teaProgram.Send(gitStatusMsg{})
			}
			return
		}
		teaProgram.Send(gitStatusMsg{st: st, key: statusKey(st)})
	}()
	return gitTick()
}

// statusKey fingerprints a status so unchanged polls are ignored.
func statusKey(st *gitx.Status) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|%d|%d|", st.Branch, st.Upstream, st.Ahead, st.Behind)
	for _, f := range st.Files {
		b.WriteString(f.Path)
		b.WriteByte(f.Index)
		b.WriteByte(f.Worktree)
		b.WriteByte(';')
	}
	return b.String()
}

// applyStatus installs a polled status: explorer badges, top bar, tree
// (new / deleted files) and the open git panel.
func (m *MainScreen) applyStatus(msg gitStatusMsg) {
	m.gitPolling = false
	if msg.st == nil || msg.key == m.gitKey {
		return
	}
	m.gitKey = msg.key
	m.lastGitRefresh = time.Now()
	m.Git.Status = msg.st
	badges := make(map[string]string, len(msg.st.Files))
	for _, f := range msg.st.Files {
		badges[m.Repo.Abs(f.Path)] = f.Badge()
	}
	_ = m.Explorer.RebuildTree()
	m.Explorer.SetGitStatus(badges)
	if m.BlameOn {
		m.applyBlame()
	}
	if m.GitOpen && !m.Git.Editing && !m.Git.PromptOpen() {
		m.Git.Refresh()
	}
}

// toggleExplorer hides / shows the sidebar (alt+b, ☰ button).
func (m *MainScreen) toggleExplorer() {
	m.ExplorerHidden = !m.ExplorerHidden
	if m.ExplorerHidden && m.ExplorerFocused {
		m.setExplorerFocused(false)
		if m.ActiveFile != "" {
			m.Focus = m.LastFocus
			if m.Focus == FocusExplorerPane {
				m.Focus = FocusURLBar
			}
			m.focusCurrent()
		}
	}
	m.SetSize(m.Width, m.Height)
}
