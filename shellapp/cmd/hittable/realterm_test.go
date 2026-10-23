package main

import (
	"bytes"
	"errors"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

// These tests run the real binary in a real pseudo-terminal, under the
// conditions people actually have: odd TERM values, no UTF-8 locale, tiny
// windows, resizes, no git, no HOME, strange project folders, a cat on the
// keyboard. Each one must start, draw, quit cleanly and give the terminal back.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

func buildBinary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds and runs the real binary")
	}
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "hittable-bin-")
		if err != nil {
			binErr = err
			return
		}
		binPath = filepath.Join(dir, "hittable")
		out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput()
		if err != nil {
			binErr = errors.New(string(out))
		}
	})
	if binErr != nil {
		t.Fatalf("build: %v", binErr)
	}
	return binPath
}

type session struct {
	t    *testing.T
	pty  *os.File
	cmd  *exec.Cmd
	vt   vt10x.Terminal
	mu   sync.Mutex
	raw  bytes.Buffer // everything the app wrote, for panic and restore checks
	done chan error
}

// startApp starts the binary on root in a cols×rows terminal. env replaces the
// defaults (TERM, HOME, HITTABLE_HOME, PATH, LANG); the AI is always off and
// the shell panel runs cat, so random keys never reach a real shell.
func startApp(t *testing.T, root string, cols, rows int, env map[string]string) *session {
	t.Helper()
	bin := buildBinary(t)
	base := map[string]string{
		"TERM":          "xterm-256color",
		"HOME":          t.TempDir(),
		"HITTABLE_HOME": t.TempDir(),
		"HITTABLE_AI":   "0",
		"PATH":          os.Getenv("PATH"),
		"LANG":          "en_US.UTF-8",
		"SHELL":         "/bin/cat",
	}
	for k, v := range env {
		base[k] = v
	}
	cmd := exec.Command(bin, root)
	cmd.Dir = root
	cmd.Env = nil
	for k, v := range base {
		if v != "<unset>" {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	// WithWriter: like a real terminal, answer device queries (cursor position
	// and the like). Bubble Tea asks at startup and waits up to 5s for a reply.
	s := &session{t: t, pty: f, cmd: cmd, vt: vt10x.New(vt10x.WithSize(cols, rows), vt10x.WithWriter(f)), done: make(chan error, 1)}
	go func() {
		buf := make([]byte, 32<<10)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				s.mu.Lock()
				s.raw.Write(buf[:n])
				_, _ = s.vt.Write(buf[:n])
				s.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	go func() { s.done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = f.Close()
	})
	return s
}

func (s *session) screen() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.vt.String()
}

// waitFor waits until the screen shows text.
func (s *session) waitFor(text string) {
	s.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(s.screen(), text) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.t.Fatalf("screen never showed %q:\n%s", text, s.screen())
}

// started waits until the app has taken over the terminal (alternate screen
// on). Before that the tty is still cooked and ctrl+c is a SIGINT.
func (s *session) started() {
	s.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		ok := strings.Contains(s.raw.String(), "\x1b[?1049h")
		s.mu.Unlock()
		if ok {
			time.Sleep(100 * time.Millisecond)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.t.Fatal("the app never switched to the alternate screen")
}

func (s *session) keys(k string) {
	_, _ = s.pty.WriteString(k)
	time.Sleep(30 * time.Millisecond)
}

func (s *session) resize(cols, rows int) {
	_ = pty.Setsize(s.pty, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	s.mu.Lock()
	s.vt.Resize(cols, rows)
	s.mu.Unlock()
	time.Sleep(60 * time.Millisecond)
}

// quit presses ctrl+c (twice if a selection or prompt eats the first) and
// checks a clean exit: status 0, no panic, the alternate screen left and
// mouse reporting switched off, so the user's terminal works afterwards.
func (s *session) quit() {
	s.t.Helper()
	s.started()
	for i := 0; i < 4; i++ {
		s.keys("\x1b") // close any prompt or menu
		// Esc then a key within a few ms reads as alt+key; let it stand alone.
		time.Sleep(150 * time.Millisecond)
		s.keys("\x03")
		select {
		case err := <-s.done:
			s.mu.Lock()
			out := s.raw.String()
			s.mu.Unlock()
			if err != nil {
				s.t.Fatalf("exit: %v\n%s", err, tail(out))
			}
			if strings.Contains(out, "panic:") || strings.Contains(out, "goroutine ") {
				s.t.Fatalf("the app panicked:\n%s", tail(out))
			}
			if !strings.Contains(out, "\x1b[?1049l") {
				s.t.Error("the alternate screen was never left: the user's terminal stays blank")
			}
			if !strings.Contains(out, "\x1b[?1003l") && !strings.Contains(out, "\x1b[?1000l") {
				s.t.Error("mouse reporting was never switched off: clicks print garbage afterwards")
			}
			return
		case <-time.After(1500 * time.Millisecond):
		}
	}
	s.t.Fatalf("ctrl+c did not quit:\n%s", s.screen())
}

func tail(s string) string {
	if len(s) > 3000 {
		return s[len(s)-3000:]
	}
	return s
}

// project makes a folder with the usual layout plus whatever extra files.
func project(t *testing.T, name string, extra map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	files := map[string]string{
		"hittable/env.json":       `{"BASE":"http://127.0.0.1:9"}`,
		"hittable/users/list.hit": `{"method":"GET","url":"<<BASE>>/users","headers":{},"params":{},"body":"","response":null}`,
		"hittable/notes.md":       "# Notes\n\nhello\n",
	}
	for k, v := range extra {
		files[k] = v
	}
	for p, c := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRealTerminalEveryday(t *testing.T) {
	root := project(t, "api", nil)
	s := startApp(t, root, 120, 32, nil)
	s.waitFor("hittable")
	s.keys("\x1b[B\x1b[B\r") // down, down, enter: walk the explorer
	s.keys("?")              // help
	s.waitFor("ctrl")
	s.keys("\x1b")
	s.quit()
}

func TestRealTerminalUnusualTerminals(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"dumb terminal", map[string]string{"TERM": "dumb"}},
		{"no TERM at all", map[string]string{"TERM": "<unset>"}},
		{"NO_COLOR", map[string]string{"NO_COLOR": "1"}},
		{"plain C locale", map[string]string{"LANG": "C", "LC_ALL": "C"}},
		{"16-colour linux console", map[string]string{"TERM": "linux"}},
		{"screen/tmux", map[string]string{"TERM": "screen-256color"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := startApp(t, project(t, "api", nil), 100, 30, c.env)
			s.waitFor("hittable")
			s.keys("\x1b[B\r")
			s.quit()
		})
	}
}

// Like the Linux console: the terminal answers a cursor-position query but not
// the background-colour one. Startup must not wait out the colour timeout.
func TestRealTerminalStartsFast(t *testing.T) {
	for _, term := range []string{"xterm-256color", "linux", "<unset>"} {
		start := time.Now()
		s := startApp(t, project(t, "api", nil), 100, 30, map[string]string{"TERM": term})
		s.started()
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("TERM=%s: took %v to start", term, d.Round(10*time.Millisecond))
		}
		s.quit()
	}
}

func TestRealTerminalTinyAndResizing(t *testing.T) {
	for _, size := range [][2]int{{20, 6}, {10, 3}, {1, 1}} {
		s := startApp(t, project(t, "api", nil), size[0], size[1], nil)
		s.started() // too small to show any text reliably
		s.keys("\x1b[B\r?")
		s.quit()
	}
	// Dragging the window corner: dozens of resizes, down to nothing and back.
	s := startApp(t, project(t, "api", nil), 100, 30, nil)
	s.waitFor("hittable")
	for i := 0; i < 30; i++ {
		s.resize(5+i*7%200, 2+i*3%60)
	}
	s.resize(0, 0)
	s.resize(120, 40)
	s.waitFor("hittable")
	s.quit()
}

func TestRealTerminalHostileEnvironments(t *testing.T) {
	t.Run("no git on PATH", func(t *testing.T) {
		s := startApp(t, project(t, "api", nil), 100, 30, map[string]string{"PATH": t.TempDir()})
		s.waitFor("hittable")
		s.keys("\x07") // ctrl+g opens the Git panel: must explain, not crash
		time.Sleep(300 * time.Millisecond)
		s.keys("\x1b")
		s.quit()
	})
	t.Run("no HOME", func(t *testing.T) {
		s := startApp(t, project(t, "api", nil), 100, 30, map[string]string{"HOME": "<unset>", "HITTABLE_HOME": "<unset>"})
		s.waitFor("hittable")
		s.quit()
	})
	t.Run("a folder with no hittable/ yet", func(t *testing.T) {
		s := startApp(t, t.TempDir(), 100, 30, nil)
		s.started()
		s.quit()
	})
}

func TestRealTerminalStrangeProjects(t *testing.T) {
	big := `{"method":"POST","url":"http://x","headers":{},"params":{},"body":"` + strings.Repeat("a", 2<<20) + `","response":null}`
	root := project(t, "Проект 東京 ☕ with spaces", map[string]string{
		"hittable/empty.hit":            "",
		"hittable/broken.hit":           `{"method": "GET", "url": `,
		"hittable/array.hit":            `[1,2,3]`,
		"hittable/huge.hit":             big,
		"hittable/emoji 🚀 name.hit":     `{"method":"GET","url":"http://x","headers":{},"params":{},"body":"","response":null}`,
		"hittable/binary.png":           "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00",
		"hittable/notes/very-long.md":   strings.Repeat("a very long line ", 5000),
		"hittable/deep/a/b/c/d/e/f.hit": `{"method":"GET","url":"http://x"}`,
	})
	// An unreadable file and a symlink loop, as left behind by other tools.
	locked := filepath.Join(root, "hittable", "locked.hit")
	if err := os.WriteFile(locked, []byte(`{}`), 0o000); err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink(filepath.Join(root, "hittable", "loop"), filepath.Join(root, "hittable", "loop"))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })

	s := startApp(t, root, 140, 40, nil)
	s.waitFor("hittable")
	// Open every entry in the explorer from top to bottom.
	for i := 0; i < 25; i++ {
		s.keys("\x1b[B\r")
		s.keys("\x02") // ctrl+b back to the explorer
	}
	s.quit()
}

func TestRealTerminalReadOnlyProject(t *testing.T) {
	root := project(t, "readonly", nil)
	hd := filepath.Join(root, "hittable")
	for _, p := range []string{filepath.Join(hd, "users", "list.hit"), filepath.Join(hd, "users"), hd, root} {
		if err := os.Chmod(p, 0o555); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, p := range []string{root, hd, filepath.Join(hd, "users")} {
			_ = os.Chmod(p, 0o755)
		}
	})
	s := startApp(t, root, 120, 32, nil)
	s.waitFor("hittable")
	s.keys("\x1b[B\x1b[B\r") // open list.hit
	s.keys("xyz")            // edit it: saving must fail politely
	s.keys("\x13")           // ctrl+s
	time.Sleep(300 * time.Millisecond)
	s.keys("\x0e") // ctrl+n: new file in a read-only folder
	s.keys("new.hit\r")
	time.Sleep(300 * time.Millisecond)
	s.quit()
}

// A cat on the keyboard: thousands of random keys, escape sequences and mouse
// reports. Nothing the user can type may crash the app.
func TestRealTerminalKeyboardMash(t *testing.T) {
	pieces := []string{
		"a", "z", "Q", "1", " ", "/", "?", ":", "<<", ">>", "{", "}", "\"", "\\", "é", "東", "🚀",
		"\r", "\t", "\x1b[Z", "\x7f", "\x1b", "\x1b[A", "\x1b[B", "\x1b[C", "\x1b[D", "\x1b[H", "\x1b[F",
		"\x1b[5~", "\x1b[6~", "\x1b[3~", "\x01", "\x02", "\x05", "\x06", "\x07", "\x0b", "\x0e", "\x10",
		"\x12", "\x13", "\x14", "\x15", "\x17", "\x18", "\x19", "\x1a", "\x1bOP", "\x1b[15~", "\x1bf", "\x1bb",
		"\x1b[<0;10;5M", "\x1b[<0;10;5m", "\x1b[<0;40;12M", "\x1b[<32;45;14M", "\x1b[<0;45;14m",
		"\x1b[<64;30;10M", "\x1b[<65;30;10M", "\x1b[<2;20;8M", "\x1b[<0;2;1M", "\x1b[<0;2;1m",
		"\x1b[200~pasted\ntext\x1b[201~",
	}
	for seed := int64(1); seed <= 3; seed++ {
		t.Run("seed "+string(rune('0'+seed)), func(t *testing.T) {
			root := project(t, "mash", nil)
			s := startApp(t, root, 110, 30, nil)
			s.waitFor("hittable")
			r := rand.New(rand.NewSource(seed))
			for i := 0; i < 600; i++ {
				// ctrl+c quits; keep it out of the mash and test it at the end.
				_, _ = s.pty.WriteString(pieces[r.Intn(len(pieces))])
				if i%20 == 0 {
					time.Sleep(15 * time.Millisecond)
				}
				select {
				case err := <-s.done:
					s.mu.Lock()
					out := s.raw.String()
					s.mu.Unlock()
					t.Fatalf("the app exited mid-mash (key %d, err %v):\n%s", i, err, tail(out))
				default:
				}
			}
			time.Sleep(300 * time.Millisecond)
			s.quit()
		})
	}
}

// Edits save themselves: no ctrl+s, just typing, then the file on disk changes
// within moments, and the other view of a .hit shows the change.
func TestRealTerminalAutosave(t *testing.T) {
	root := project(t, "api", nil)
	hit := filepath.Join(root, "hittable", "users", "list.hit")
	s := startApp(t, root, 140, 40, nil)
	s.waitFor("hittable")

	s.keys("\x10") // ctrl+p: find file
	s.keys("list.hit")
	time.Sleep(300 * time.Millisecond)
	s.keys("\r")
	s.waitFor("<<BASE>>")
	s.keys("\x1b[F/9") // End, then type into the URL
	waitFile(t, hit, "<<BASE>>/users/9")
	s.keys("\x14") // ctrl+t: Text view shows the same request
	s.waitFor("users/9")

	notes := filepath.Join(root, "hittable", "notes.md")
	s.keys("\x10")
	s.keys("notes.md")
	time.Sleep(300 * time.Millisecond)
	s.keys("\r")
	s.waitFor("hello")
	s.keys("\x1b[Fsaved by itself")
	waitFile(t, notes, "saved by itself")
	s.quit()
}

func waitFile(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if b, _ := os.ReadFile(path); strings.Contains(string(b), want) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	b, _ := os.ReadFile(path)
	t.Fatalf("%s never got %q without a save; it holds:\n%s", filepath.Base(path), want, b)
}
