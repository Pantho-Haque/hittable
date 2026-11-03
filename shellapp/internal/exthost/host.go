// Package exthost runs VS Code extensions in a Node.js child process (the
// "extension host") and speaks JSON-RPC to it over stdin/stdout, one JSON
// object per line. js/host.js, embedded here, implements the vscode module
// extensions import; everything it cannot do alone (messages, pickers,
// edits, diagnostics, webviews) comes back to Go as a request or
// notification for the UI to handle.
package exthost

import (
	"bufio"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hittable/shellapp/internal/extensions"
	"github.com/hittable/shellapp/internal/hithome"
)

//go:embed js/host.js
var hostJS []byte

// Settings is what workspace.getConfiguration reads: user settings from
// ~/.hittable/settings.json and workspace settings from .vscode/settings.json.
type Settings struct {
	User      map[string]any `json:"user"`
	Workspace map[string]any `json:"workspace"`
}

// Options configures one host process.
type Options struct {
	Node        string
	Root        string
	Extensions  []extensions.Installed
	Settings    Settings
	WebviewBase string
	MachineID   string

	// OnRequest receives requests from the host (a message box, a picker,
	// an edit). It runs on the reader goroutine, so it must hand the work
	// off (to the UI loop) and answer later with Request.Reply.
	OnRequest func(*Request)
	// OnNotify receives notifications, also on the reader goroutine.
	OnNotify func(method string, params json.RawMessage)
	// OnExit is called once when the process ends for any reason.
	OnExit func(err error, stderr string)
}

// Request is a call from the extension host awaiting an answer.
type Request struct {
	ID     json.RawMessage
	Method string
	Params json.RawMessage
	h      *Host
	once   sync.Once
}

// Reply answers the request; later calls are ignored.
func (r *Request) Reply(result any, err error) {
	r.once.Do(func() {
		msg := map[string]any{"id": r.ID}
		if err != nil {
			msg["error"] = map[string]string{"message": err.Error()}
		} else {
			msg["result"] = result
		}
		_ = r.h.write(msg)
	})
}

type response struct {
	result json.RawMessage
	err    error
}

// Host is a running extension host.
type Host struct {
	opts   Options
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	wmu    sync.Mutex
	nextID atomic.Int64

	pmu     sync.Mutex
	pending map[int64]chan response

	done    chan struct{}
	exitErr error
	stderr  *tail
	closing atomic.Bool
}

// ErrClosed is returned by calls on a host that has exited.
var ErrClosed = errors.New("the extension host is not running")

// scriptPath writes the embedded host once per content hash, so several
// hittable versions side by side never run each other's host.
func scriptPath() (string, error) {
	sum := sha256.Sum256(hostJS)
	dir := filepath.Join(hithome.ExtData(), "host")
	p := filepath.Join(dir, "host-"+hex.EncodeToString(sum[:6])+".js")
	if b, err := os.ReadFile(p); err == nil && string(b) == string(hostJS) {
		return p, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, hostJS, 0o644); err != nil {
		return "", err
	}
	return p, os.Rename(tmp, p)
}

type wireExt struct {
	ID       string            `json:"id"`
	Path     string            `json:"path"`
	Manifest json.RawMessage   `json:"manifest"`
	NLS      map[string]string `json:"nls"`
}

// Start launches the host and initializes it with the given extensions.
func Start(ctx context.Context, o Options) (*Host, error) {
	script, err := scriptPath()
	if err != nil {
		return nil, fmt.Errorf("preparing the extension host: %w", err)
	}
	// --max-old-space-size caps one runaway extension at 2 GB rather than
	// letting it take the machine.
	cmd := exec.Command(o.Node, "--max-old-space-size=2048", script)
	cmd.Dir = o.Root
	cmd.Env = append(os.Environ(), "ELECTRON_RUN_AS_NODE=1", "HITTABLE_EXTENSION_HOST=1", "VSCODE_NLS_CONFIG={\"locale\":\"en\"}")
	setProcAttrs(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	h := &Host{opts: o, cmd: cmd, stdin: stdin, pending: map[int64]chan response{}, done: make(chan struct{}), stderr: newTail(16 << 10)}
	logf := openLog()
	if logf != nil {
		cmd.Stderr = io.MultiWriter(h.stderr, logf)
	} else {
		cmd.Stderr = h.stderr
	}
	if err := cmd.Start(); err != nil {
		if logf != nil {
			logf.Close()
		}
		return nil, fmt.Errorf("starting Node (%s): %w", o.Node, err)
	}
	go h.read(stdout)
	go func() {
		err := cmd.Wait()
		if logf != nil {
			logf.Close()
		}
		h.exitErr = err
		close(h.done)
		h.failPending()
		if o.OnExit != nil {
			o.OnExit(err, h.stderr.String())
		}
	}()

	exts := make([]wireExt, 0, len(o.Extensions))
	for _, e := range o.Extensions {
		raw, err := os.ReadFile(filepath.Join(e.Path(), "package.json"))
		if err != nil {
			continue
		}
		exts = append(exts, wireExt{ID: e.ID, Path: e.Path(), Manifest: json.RawMessage(extensions.StripJSONC(raw)), NLS: e.NLS})
	}
	initCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	err = h.Call(initCtx, "initialize", map[string]any{
		"root": o.Root, "apiVersion": extensions.APIVersion, "webviewBase": o.WebviewBase,
		"extData": hithome.ExtData(), "machineId": o.MachineID, "settings": o.Settings, "extensions": exts,
	}, nil)
	if err != nil {
		h.Kill()
		return nil, fmt.Errorf("the extension host did not start: %w%s", err, stderrNote(h.stderr.String()))
	}
	return h, nil
}

func stderrNote(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) > 400 {
		s = "…" + s[len(s)-400:]
	}
	return "\n" + s
}

func openLog() *os.File {
	_ = os.MkdirAll(hithome.Logs(), 0o755)
	f, err := os.OpenFile(filepath.Join(hithome.Logs(), "exthost.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	if fi, err := f.Stat(); err == nil && fi.Size() > 5<<20 {
		_ = f.Truncate(0)
	}
	fmt.Fprintf(f, "\n--- extension host started %s ---\n", time.Now().Format(time.RFC3339))
	return f
}

type wireMsg struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (h *Host) read(r io.Reader) {
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 1 {
			h.dispatch(line)
		}
		if err != nil {
			return
		}
	}
}

func (h *Host) dispatch(line []byte) {
	var m wireMsg
	if err := json.Unmarshal(line, &m); err != nil {
		return
	}
	hasID := len(m.ID) > 0 && string(m.ID) != "null"
	switch {
	case m.Method != "" && hasID:
		req := &Request{ID: m.ID, Method: m.Method, Params: m.Params, h: h}
		if h.opts.OnRequest == nil {
			req.Reply(nil, fmt.Errorf("%s is not supported", m.Method))
			return
		}
		h.opts.OnRequest(req)
	case m.Method != "":
		if h.opts.OnNotify != nil {
			h.opts.OnNotify(m.Method, m.Params)
		}
	case hasID:
		var id int64
		if json.Unmarshal(m.ID, &id) != nil {
			return
		}
		h.pmu.Lock()
		ch := h.pending[id]
		delete(h.pending, id)
		h.pmu.Unlock()
		if ch == nil {
			return
		}
		if m.Error != nil {
			ch <- response{err: errors.New(m.Error.Message)}
		} else {
			ch <- response{result: m.Result}
		}
	}
}

func (h *Host) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	h.wmu.Lock()
	defer h.wmu.Unlock()
	select {
	case <-h.done:
		return ErrClosed
	default:
	}
	_, err = h.stdin.Write(b)
	return err
}

// Call sends a request and decodes the result into out (which may be nil).
func (h *Host) Call(ctx context.Context, method string, params, out any) error {
	id := h.nextID.Add(1)
	ch := make(chan response, 1)
	h.pmu.Lock()
	h.pending[id] = ch
	h.pmu.Unlock()
	if err := h.write(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		h.pmu.Lock()
		delete(h.pending, id)
		h.pmu.Unlock()
		return err
	}
	select {
	case r := <-ch:
		if r.err != nil {
			return r.err
		}
		if out != nil && len(r.result) > 0 {
			return json.Unmarshal(r.result, out)
		}
		return nil
	case <-ctx.Done():
		h.pmu.Lock()
		delete(h.pending, id)
		h.pmu.Unlock()
		return ctx.Err()
	case <-h.done:
		return ErrClosed
	}
}

// Notify sends a notification (no answer expected).
func (h *Host) Notify(method string, params any) error {
	return h.write(map[string]any{"method": method, "params": params})
}

func (h *Host) failPending() {
	h.pmu.Lock()
	defer h.pmu.Unlock()
	for id, ch := range h.pending {
		ch <- response{err: ErrClosed}
		delete(h.pending, id)
	}
}

// Done is closed when the process has exited.
func (h *Host) Done() <-chan struct{} { return h.done }

// Running reports whether the process is still alive.
func (h *Host) Running() bool {
	select {
	case <-h.done:
		return false
	default:
		return true
	}
}

// Closing reports whether Close was called (an exit is then expected).
func (h *Host) Closing() bool { return h.closing.Load() }

// Close asks extensions to deactivate, then ends the process and anything it
// started (language servers), waiting at most a few seconds in total.
func (h *Host) Close() {
	if !h.closing.CompareAndSwap(false, true) {
		return
	}
	if h.Running() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = h.Call(ctx, "shutdown", nil, nil)
		cancel()
		_ = h.stdin.Close()
		select {
		case <-h.done:
		case <-time.After(2 * time.Second):
		}
	}
	killTree(h.cmd)
}

// Kill ends the process tree at once.
func (h *Host) Kill() {
	h.closing.Store(true)
	_ = h.stdin.Close()
	killTree(h.cmd)
}

// Pid is the host's process id.
func (h *Host) Pid() int {
	if h.cmd.Process == nil {
		return 0
	}
	return h.cmd.Process.Pid
}

// tail keeps the last n bytes written, for error messages.
type tail struct {
	mu  sync.Mutex
	buf []byte
	n   int
}

func newTail(n int) *tail { return &tail{n: n} }

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.n {
		t.buf = t.buf[len(t.buf)-t.n:]
	}
	t.mu.Unlock()
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
