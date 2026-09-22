// Package gitx is a thin wrapper over the git CLI for the repository that
// contains the working root. Everything is parsed from machine-readable git
// output; nothing here caches, callers refresh when they need to.
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Repo is a git working tree opened from Dir (the app's working root, which
// may be a sub-directory of the repository).
type Repo struct {
	Root   string // repository top level, as git reports it
	Dir    string // the directory Open was given
	Prefix string // Dir relative to Root, with trailing slash ("" at top level)
}

// Open finds the repository containing dir. Returns nil if dir is not in a
// git work tree or git is not installed.
func Open(dir string) *Repo {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel", "--show-prefix").Output()
	if err != nil {
		return nil
	}
	parts := strings.SplitN(strings.TrimRight(string(out), "\n"), "\n", 2)
	r := &Repo{Root: parts[0], Dir: dir}
	if len(parts) == 2 {
		r.Prefix = parts[1]
	}
	return r
}

// Abs converts a repo-relative path to an absolute one under Dir when
// possible (so it matches the explorer's paths even through symlinks).
func (r *Repo) Abs(rel string) string {
	if strings.HasPrefix(rel, r.Prefix) {
		return filepath.Join(r.Dir, filepath.FromSlash(strings.TrimPrefix(rel, r.Prefix)))
	}
	return filepath.Join(r.Root, filepath.FromSlash(rel))
}

// gitMu serialises git invocations: the background status poll and a user
// action must never race for .git/index.lock.
var gitMu sync.Mutex

// Run executes git in the repo and returns stdout (stderr in the error).
// A transient "index.lock" collision (another git process) is retried.
func (r *Repo) Run(args ...string) (string, error) {
	return r.RunEnv(nil, args...)
}

// RunEnv is Run with extra environment variables.
func (r *Repo) RunEnv(env []string, args ...string) (string, error) {
	gitMu.Lock()
	defer gitMu.Unlock()
	var out string
	var err error
	for attempt := 0; attempt < 6; attempt++ {
		cmd := exec.Command("git", append([]string{"-C", r.Root}, args...)...)
		if env != nil {
			cmd.Env = append(os.Environ(), env...)
		}
		var ob, eb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &ob, &eb
		runErr := cmd.Run()
		out = ob.String()
		if runErr == nil {
			return out, nil
		}
		msg := strings.TrimSpace(eb.String())
		if msg == "" {
			msg = runErr.Error()
		}
		err = errors.New(msg)
		if !strings.Contains(msg, "index.lock") {
			break
		}
		time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
	}
	return out, err
}

// Rel converts an absolute path (under Dir) into a repo-relative one.
func (r *Repo) Rel(abs string) string {
	if rel, err := filepath.Rel(r.Dir, abs); err == nil && !strings.HasPrefix(rel, "..") {
		return r.Prefix + filepath.ToSlash(rel)
	}
	rel, err := filepath.Rel(r.Root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

// ---------- status ----------

type FileStatus struct {
	Path     string
	Index    byte // staged change: M A D R C U or ' '
	Worktree byte // unstaged change: M D U or ' ', '?' untracked, '!' ignored
}

func dot(b byte) byte {
	if b == '.' {
		return ' '
	}
	return b
}

func (f FileStatus) Staged() bool { return f.Index != ' ' && f.Index != '?' && f.Index != '!' }
func (f FileStatus) Unstaged() bool {
	return f.Worktree != ' ' && f.Worktree != '?' && f.Worktree != '!'
}
func (f FileStatus) Untracked() bool { return f.Worktree == '?' }
func (f FileStatus) Conflict() bool  { return f.Index == 'U' || f.Worktree == 'U' }

// Badge is the one-letter decoration shown next to the file (VS Code style).
func (f FileStatus) Badge() string {
	switch {
	case f.Conflict():
		return "!"
	case f.Untracked():
		return "U"
	case f.Index == 'A':
		return "A"
	case f.Index == 'D' || f.Worktree == 'D':
		return "D"
	case f.Index == 'R':
		return "R"
	}
	return "M"
}

type Status struct {
	Branch   string
	Upstream string
	Ahead    int
	Behind   int
	Files    []FileStatus
}

// Status runs `git status --porcelain=v2 --branch`.
func (r *Repo) Status() (*Status, error) {
	// --no-optional-locks: status must never write the index (and take
	// index.lock) while the user runs another git command.
	out, err := r.Run("--no-optional-locks", "status", "--porcelain=v2", "--branch", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	st := &Status{}
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			st.Branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.upstream "):
			st.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# branch.ab "):
			fmt.Sscanf(strings.TrimPrefix(line, "# branch.ab "), "+%d -%d", &st.Ahead, &st.Behind)
		case strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 "):
			f := strings.SplitN(line, " ", 9)
			if len(f) < 9 {
				continue
			}
			path := f[8]
			if line[0] == '2' { // rename: "path<TAB>origPath"
				if i := strings.IndexByte(path, '\t'); i >= 0 {
					path = path[:i]
				}
				// v2 rename lines carry a score field before the path
				g := strings.SplitN(line, " ", 10)
				if len(g) == 10 {
					path = g[9]
					if i := strings.IndexByte(path, '\t'); i >= 0 {
						path = path[:i]
					}
				}
			}
			st.Files = append(st.Files, FileStatus{Path: path, Index: dot(f[1][0]), Worktree: dot(f[1][1])})
		case strings.HasPrefix(line, "u "):
			f := strings.SplitN(line, " ", 11)
			if len(f) == 11 {
				st.Files = append(st.Files, FileStatus{Path: f[10], Index: 'U', Worktree: 'U'})
			}
		case strings.HasPrefix(line, "? "):
			st.Files = append(st.Files, FileStatus{Path: line[2:], Index: '?', Worktree: '?'})
		}
	}
	return st, nil
}

func (r *Repo) Stage(paths ...string) error {
	_, err := r.Run(append([]string{"add", "-A", "--"}, paths...)...)
	return err
}

func (r *Repo) Unstage(paths ...string) error {
	_, err := r.Run(append([]string{"reset", "-q", "HEAD", "--"}, paths...)...)
	return err
}

// Discard throws away worktree changes (untracked files are deleted).
func (r *Repo) Discard(f FileStatus) error {
	if f.Untracked() {
		_, err := r.Run("clean", "-f", "--", f.Path)
		return err
	}
	_, err := r.Run("checkout", "--", f.Path)
	return err
}

// DiscardAll throws away every unstaged change and deletes untracked files.
func (r *Repo) DiscardAll() error {
	if _, err := r.Run("checkout", "--", "."); err != nil {
		return err
	}
	_, err := r.Run("clean", "-fd")
	return err
}

func (r *Repo) Commit(msg string) error {
	_, err := r.Run("commit", "-m", msg)
	return err
}

func (r *Repo) Push() (string, error)  { return r.Run("push") }
func (r *Repo) Pull() (string, error)  { return r.Run("pull", "--ff-only") }
func (r *Repo) Fetch() (string, error) { return r.Run("fetch", "--prune") }

// ---------- diffs ----------

// Diff returns the unified diff for a path (staged compares index to HEAD).
// ignoreWS passes -w so whitespace-only changes disappear.
func (r *Repo) Diff(f FileStatus, staged, ignoreWS bool) string {
	if f.Untracked() {
		out, _ := r.Run("diff", "--no-index", "--", "/dev/null", f.Path)
		return out
	}
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	if ignoreWS {
		args = append(args, "-w")
	}
	out, _ := r.Run(append(args, "--", f.Path)...)
	return out
}

// Show returns `git show --stat -p` for a revision.
func (r *Repo) Show(rev string) string {
	out, _ := r.Run("show", "--stat", "-p", "--format=commit %H%nAuthor: %an <%ae>%nDate:   %ad%n%n    %s%n%n%b", rev)
	return out
}

// ---------- history ----------

type Commit struct {
	Hash    string
	Author  string
	Date    string // relative
	Subject string
}

// Log lists up to n commits, optionally limited to a path.
func (r *Repo) Log(n int, path string) ([]Commit, error) {
	args := []string{"log", "-n", strconv.Itoa(n), "--date=relative", "--format=%h%x1f%an%x1f%ad%x1f%s"}
	if path != "" {
		args = append(args, "--follow", "--", path)
	}
	out, err := r.Run(args...)
	if err != nil {
		return nil, err
	}
	var cs []Commit
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, "\x1f")
		if len(f) == 4 {
			cs = append(cs, Commit{f[0], f[1], f[2], f[3]})
		}
	}
	return cs, nil
}

// ---------- blame ----------

type BlameLine struct {
	Hash        string // short
	Author      string
	Time        time.Time
	Summary     string
	Uncommitted bool
}

// Blame returns one entry per line of the file's working copy.
func (r *Repo) Blame(path string) ([]BlameLine, error) {
	out, err := r.Run("blame", "--line-porcelain", "--", path)
	if err != nil {
		return nil, err
	}
	commits := map[string]BlameLine{}
	var lines []BlameLine
	var cur BlameLine
	var hash string
	for _, l := range strings.Split(out, "\n") {
		switch {
		case len(l) >= 40 && !strings.ContainsAny(l[:40], " \t") && strings.Count(l, " ") >= 2 && hash == "":
			hash = l[:40]
			if c, ok := commits[hash]; ok {
				cur = c
			} else {
				cur = BlameLine{Hash: hash[:7], Uncommitted: strings.Trim(hash, "0") == ""}
			}
		case strings.HasPrefix(l, "author "):
			cur.Author = strings.TrimPrefix(l, "author ")
		case strings.HasPrefix(l, "author-time "):
			if n, err := strconv.ParseInt(strings.TrimPrefix(l, "author-time "), 10, 64); err == nil {
				cur.Time = time.Unix(n, 0)
			}
		case strings.HasPrefix(l, "summary "):
			cur.Summary = strings.TrimPrefix(l, "summary ")
		case strings.HasPrefix(l, "\t"):
			if cur.Uncommitted {
				cur.Author = "You"
				cur.Summary = "Uncommitted changes"
			}
			commits[hash] = cur
			lines = append(lines, cur)
			hash = ""
		}
	}
	return lines, nil
}

// Ago formats a time as GitLens does ("3 days ago").
func Ago(t time.Time) string {
	if t.IsZero() {
		return "now"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%dy ago", int(d.Hours()/24/365))
}

// ---------- branches ----------

type Branch struct {
	Name     string
	Current  bool
	Upstream string
	Track    string // "[ahead 1, behind 2]"
	Date     string
	Subject  string
}

func (r *Repo) Branches() ([]Branch, error) {
	out, err := r.Run("branch", "--sort=-committerdate",
		"--format=%(refname:short)%1f%(HEAD)%1f%(upstream:short)%1f%(upstream:track)%1f%(committerdate:relative)%1f%(subject)")
	if err != nil {
		return nil, err
	}
	var bs []Branch
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, "\x1f")
		if len(f) == 6 {
			bs = append(bs, Branch{f[0], f[1] == "*", f[2], f[3], f[4], f[5]})
		}
	}
	return bs, nil
}

func (r *Repo) Checkout(name string) error {
	_, err := r.Run("checkout", name)
	return err
}

func (r *Repo) CreateBranch(name string) error {
	_, err := r.Run("checkout", "-b", name)
	return err
}

func (r *Repo) DeleteBranch(name string) error {
	_, err := r.Run("branch", "-D", name)
	return err
}

// ---------- stashes ----------

type Stash struct {
	Ref     string // stash@{0}
	Date    string
	Subject string
}

func (r *Repo) Stashes() ([]Stash, error) {
	out, err := r.Run("stash", "list", "--format=%gd%x1f%cr%x1f%s")
	if err != nil {
		return nil, err
	}
	var ss []Stash
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, "\x1f")
		if len(f) == 3 {
			ss = append(ss, Stash{f[0], f[1], f[2]})
		}
	}
	return ss, nil
}

func (r *Repo) StashPush(msg string) error {
	args := []string{"stash", "push", "--include-untracked"}
	if msg != "" {
		args = append(args, "-m", msg)
	}
	_, err := r.Run(args...)
	return err
}

func (r *Repo) StashPop(ref string) error {
	_, err := r.Run("stash", "pop", ref)
	return err
}

func (r *Repo) StashDrop(ref string) error {
	_, err := r.Run("stash", "drop", ref)
	return err
}

func (r *Repo) StashShow(ref string) string {
	out, _ := r.Run("stash", "show", "-p", "--include-untracked", ref)
	return out
}

// ---------- sync / publish ----------

// Sync mirrors VS Code's default: pull then push; with no upstream it
// publishes the branch to origin.
func (r *Repo) Sync(st *Status) (string, error) {
	if st != nil && st.Upstream == "" {
		return r.Run("push", "-u", "origin", "HEAD")
	}
	out, err := r.Run("pull")
	if err != nil {
		return out, err
	}
	out2, err := r.Run("push")
	return out + out2, err
}

// ---------- merge conflicts ----------

// MergeInProgress reports whether a merge (or rebase / cherry-pick) is
// waiting for conflict resolution, and which kind.
func (r *Repo) MergeInProgress() (bool, string) {
	for _, k := range []struct{ ref, name string }{{"MERGE_HEAD", "merge"}, {"REBASE_HEAD", "rebase"}, {"CHERRY_PICK_HEAD", "cherry-pick"}} {
		if _, err := r.Run("rev-parse", "-q", "--verify", k.ref); err == nil {
			return true, k.name
		}
	}
	return false, ""
}

func (r *Repo) MergeAbort(kind string) error {
	switch kind {
	case "rebase":
		_, err := r.Run("rebase", "--abort")
		return err
	case "cherry-pick":
		_, err := r.Run("cherry-pick", "--abort")
		return err
	}
	_, err := r.Run("merge", "--abort")
	return err
}

// MergeContinue commits the resolved merge (or continues the rebase).
func (r *Repo) MergeContinue(kind string) error {
	switch kind {
	case "rebase":
		_, err := r.RunEnv([]string{"GIT_EDITOR=true"}, "rebase", "--continue")
		return err
	case "cherry-pick":
		_, err := r.RunEnv([]string{"GIT_EDITOR=true"}, "cherry-pick", "--continue")
		return err
	}
	_, err := r.Run("commit", "--no-edit")
	return err
}

// Conflict is one <<<<<<< / ======= / >>>>>>> block: line indexes into the
// file (0-based, Start = marker line, End = closing marker line).
type Conflict struct {
	Start, Mid, End int
	Ours, Theirs    []string
	OursLabel       string
	TheirsLabel     string
}

// ParseConflicts finds conflict blocks in file content.
func ParseConflicts(content string) []Conflict {
	lines := strings.Split(content, "\n")
	var out []Conflict
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "<<<<<<<") {
			continue
		}
		c := Conflict{Start: i, Mid: -1, End: -1, OursLabel: strings.TrimSpace(strings.TrimPrefix(lines[i], "<<<<<<<"))}
		for j := i + 1; j < len(lines); j++ {
			switch {
			case c.Mid < 0 && strings.HasPrefix(lines[j], "======="):
				c.Mid = j
			case c.Mid >= 0 && strings.HasPrefix(lines[j], ">>>>>>>"):
				c.End = j
				c.TheirsLabel = strings.TrimSpace(strings.TrimPrefix(lines[j], ">>>>>>>"))
			}
			if c.End >= 0 {
				break
			}
		}
		if c.Mid < 0 || c.End < 0 {
			break
		}
		c.Ours = lines[c.Start+1 : c.Mid]
		c.Theirs = lines[c.Mid+1 : c.End]
		out = append(out, c)
		i = c.End
	}
	return out
}

// Resolve replaces conflict block idx with the chosen side(s):
// "ours", "theirs" or "both".
func Resolve(content string, idx int, choice string) string {
	cs := ParseConflicts(content)
	if idx < 0 || idx >= len(cs) {
		return content
	}
	c := cs[idx]
	lines := strings.Split(content, "\n")
	var repl []string
	switch choice {
	case "ours":
		repl = c.Ours
	case "theirs":
		repl = c.Theirs
	default:
		repl = append(append([]string{}, c.Ours...), c.Theirs...)
	}
	out := append([]string{}, lines[:c.Start]...)
	out = append(out, repl...)
	out = append(out, lines[c.End+1:]...)
	return strings.Join(out, "\n")
}
