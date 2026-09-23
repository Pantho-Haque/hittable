package llmhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/hittable/shellapp/internal/hithome"
	"github.com/hittable/shellapp/internal/llm"
)

// Supervisor owns at most one llama-server. It starts one lazily, on the first
// request that needs it, and never at startup: a 3GB process and a two-second
// model load are not something an API client should pay for on launch.
//
// It is safe for concurrent use. Endpoint satisfies llm.Config.Endpoint, which
// is what makes lazy spawn fall out for free — the first generation is what
// triggers the server, and callers just see a slower first call.
type Supervisor struct {
	// startMu serialises the slow path (spawn and readiness polling) without
	// holding mu, so Close during a model load does not block behind it.
	startMu sync.Mutex

	mu     sync.Mutex
	cmd    *exec.Cmd
	wait   chan error
	log    *os.File
	pid    int
	port   int
	owned  bool // false for a server we adopted and must never kill
	closed bool
	last   time.Time
	starts []time.Time

	stopIdle chan struct{}

	// Seams. All zero values mean "production behaviour"; tests set them to
	// point at a helper process instead of a real llama-server.
	bin       string
	modelPath string
	preArgs   []string // inserted before spawnArgs, for the test helper
	extraEnv  []string
	client    *http.Client
	idle      time.Duration
	ready     time.Duration
	poll      time.Duration
	noAdopt   bool // never adopt a server on the default port
	now       func() time.Time
}

// Supervision constants.
const (
	// idleTimeout reclaims ~3GB when nobody is using the model. A server we
	// adopted from another process is never shut down on this timer.
	idleTimeout = 10 * time.Minute
	// readyTimeout bounds the cold model load. A 2GB GGUF off a cold cache on a
	// slow disk genuinely takes tens of seconds.
	readyTimeout = 120 * time.Second
	pollInterval = 200 * time.Millisecond

	healthTimeout = time.Second
	stopGrace     = 3 * time.Second

	// A server that keeps dying must not become a spawn loop.
	maxRestarts   = 3
	restartWindow = 5 * time.Minute

	// defaultLlamaPort is llama-server's own default. If the user is already
	// running one there, use it and download nothing.
	defaultLlamaPort = 8080

	// bindAttempts is how many times a spawn retries after the child exits
	// before it is ready — practically always a port that got taken between
	// picking it and binding it.
	bindAttempts = 3

	// logLimit ring-trims the server log so it cannot grow without bound.
	logLimit = 1 << 20
)

// NewSupervisor returns a supervisor that has not started anything and will not
// until Endpoint is called.
func NewSupervisor() *Supervisor {
	s := &Supervisor{}
	s.mu.Lock()
	s.defaults()
	s.mu.Unlock()
	return s
}

// defaults fills the zero seams. Callers must hold mu.
func (s *Supervisor) defaults() {
	if s.client == nil {
		s.client = &http.Client{Timeout: 2 * time.Second}
	}
	if s.idle == 0 {
		s.idle = idleTimeout
	}
	if s.ready == 0 {
		s.ready = readyTimeout
	}
	if s.poll == 0 {
		s.poll = pollInterval
	}
	if s.now == nil {
		s.now = time.Now
	}
}

func addr(port int) string { return "http://127.0.0.1:" + strconv.Itoa(port) }

// Endpoint makes sure a server is running and returns its base URL. It reuses
// one before it starts one, in three widening steps: the server this supervisor
// already started, a live server another hittable window recorded in the run
// file, and a llama-server the user is running themselves on the default port.
// Only the first of those is ever owned, and only an owned server is ever
// killed.
func (s *Supervisor) Endpoint(ctx context.Context) (string, error) {
	s.startMu.Lock()
	defer s.startMu.Unlock()

	s.mu.Lock()
	s.defaults()
	closed, port, pid := s.closed, s.port, s.pid
	client := s.client
	s.mu.Unlock()

	if closed {
		return "", errors.New("llmhost: supervisor is closed")
	}

	if port != 0 && (pid == 0 || processAlive(pid)) && health(ctx, client, port) {
		s.touch()
		return addr(port), nil
	}
	// The one we had is gone or has stopped answering. stop rather than forget:
	// a hung server that is still alive would otherwise sit there holding three
	// gigabytes while we started a second one beside it.
	if port != 0 {
		s.stop()
	}

	if rs, err := readRunState(); err == nil && rs.Port != 0 &&
		processAlive(rs.PID) && health(ctx, client, rs.Port) {
		s.adopt(rs.PID, rs.Port)
		return addr(rs.Port), nil
	}

	s.mu.Lock()
	noAdopt := s.noAdopt
	s.mu.Unlock()
	if !noAdopt && health(ctx, client, defaultLlamaPort) {
		s.adopt(0, defaultLlamaPort)
		return addr(defaultLlamaPort), nil
	}

	if err := s.spawn(ctx); err != nil {
		return "", err
	}
	s.mu.Lock()
	port = s.port
	s.mu.Unlock()
	return addr(port), nil
}

// Running reports the current server, if any.
func (s *Supervisor) Running() (pid, port int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.port == 0 {
		return 0, 0, false
	}
	return s.pid, s.port, true
}

func (s *Supervisor) touch() {
	s.mu.Lock()
	s.last = s.now()
	s.mu.Unlock()
}

func (s *Supervisor) adopt(pid, port int) {
	s.mu.Lock()
	s.pid, s.port, s.owned, s.cmd, s.wait = pid, port, false, nil, nil
	s.last = s.now()
	s.mu.Unlock()
}

// spawn picks a port, starts the server and waits for it to answer /health.
func (s *Supervisor) spawn(ctx context.Context) error {
	bin, model, err := s.paths()
	if err != nil {
		return err
	}

	s.mu.Lock()
	now := s.now()
	kept := s.starts[:0]
	for _, t := range s.starts {
		if now.Sub(t) < restartWindow {
			kept = append(kept, t)
		}
	}
	s.starts = kept
	if len(s.starts) >= maxRestarts {
		s.mu.Unlock()
		return fmt.Errorf("%w: %d starts in %s — llama-server is not staying up, see %s",
			ErrRestartBudget, maxRestarts, restartWindow, logPath())
	}
	s.starts = append(s.starts, now)
	preArgs, extraEnv, ready, poll, client := s.preArgs, s.extraEnv, s.ready, s.poll, s.client
	s.mu.Unlock()

	var last error
	for attempt := 0; attempt < bindAttempts; attempt++ {
		port, err := freePort()
		if err != nil {
			last = err
			continue
		}

		dir := filepath.Dir(bin)
		args := append(append([]string{}, preArgs...), spawnArgs(model, port, runtime.GOOS, runtime.GOARCH, runtime.NumCPU())...)
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		cmd.Env = append(libraryEnv(os.Environ(), dir), extraEnv...)

		logf := openLog()
		if logf != nil {
			cmd.Stdout, cmd.Stderr = logf, logf
		}
		if err := cmd.Start(); err != nil {
			closeLog(logf)
			last = err
			continue
		}

		// Buffered, then closed: the exit is reported once to whoever is
		// waiting for readiness, and every later reader — terminate, Close —
		// sees a closed channel and returns at once instead of sitting through
		// the grace period for a process that is already gone.
		waitCh := make(chan error, 1)
		go func() {
			waitCh <- cmd.Wait()
			close(waitCh)
		}()

		err = awaitReady(ctx, client, port, ready, poll, waitCh)
		if err == nil {
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				terminate(cmd, waitCh)
				closeLog(logf)
				return errors.New("llmhost: supervisor is closed")
			}
			s.cmd, s.wait, s.log = cmd, waitCh, logf
			s.pid, s.port, s.owned = cmd.Process.Pid, port, true
			s.last = s.now()
			s.mu.Unlock()

			writeRunState(runState{
				PID: cmd.Process.Pid, Port: port, Model: model,
				Binary: bin, Started: time.Now(),
			})
			s.startIdleWatch()
			return nil
		}

		terminate(cmd, waitCh)
		closeLog(logf)
		last = err

		// Only an early exit is worth retrying: that is what losing a race for
		// the port looks like. A readiness timeout means the model is not
		// loading, and three more two-minute waits would not change that.
		if !errors.Is(err, errChildExited) {
			return err
		}
	}
	return fmt.Errorf("llmhost: starting llama-server: %w (log: %s)", last, logPath())
}

// paths resolves the binary and the weights, honouring the test seams.
func (s *Supervisor) paths() (bin, model string, err error) {
	s.mu.Lock()
	bin, model = s.bin, s.modelPath
	s.mu.Unlock()
	if bin == "" {
		bin = ServerPath()
	}
	if model == "" {
		model = ModelPath()
	}
	if fi, statErr := os.Stat(bin); statErr != nil || fi.IsDir() {
		return "", "", fmt.Errorf("%w: no runtime at %s — run `hittable model enable`", ErrNotInstalled, bin)
	}
	if fi, statErr := os.Stat(model); statErr != nil || fi.IsDir() {
		return "", "", fmt.Errorf("%w: no model at %s — run `hittable model enable`", ErrNotInstalled, model)
	}
	return bin, model, nil
}

// spawnArgs is the entire command line, as a pure function of its inputs, so it
// can be asserted as a table across platforms without spawning anything.
//
// --ctx-size is set explicitly because the server default is what silently
// truncates a long prompt; --parallel with --cont-batching lets the editor's
// completion request overtake a running commit-message stream; --no-webui drops
// a static file server nobody here will ever open.
func spawnArgs(model string, port int, goos, goarch string, ncpu int) []string {
	threads := ncpu / 2
	if threads < 2 {
		threads = 2
	}
	args := []string{
		"--model", model,
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"--ctx-size", strconv.Itoa(ContextSize),
		"--threads", strconv.Itoa(threads),
		"--parallel", "2",
		"--cont-batching",
		"--jinja",
		"--no-webui",
	}
	// Apple silicon has a unified-memory Metal backend, so every layer belongs
	// on the GPU. Elsewhere the prebuilt archives are CPU-only builds and the
	// flag would either be ignored or fail.
	if goos == "darwin" && goarch == "arm64" {
		args = append(args, "--n-gpu-layers", "999")
	}
	return args
}

// freePort asks the kernel for an unused loopback port, then releases it. There
// is an unavoidable window between the release and llama-server's bind, which
// is why spawn retries. Hard-coding 8080 instead would silently collide with a
// server the user started themselves, which is the worse failure.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// errChildExited distinguishes "it died" from "it never got ready".
var errChildExited = errors.New("llama-server exited before it was ready")

func awaitReady(ctx context.Context, client *http.Client, port int, limit, poll time.Duration, exited <-chan error) error {
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	tick := time.NewTicker(poll)
	defer tick.Stop()

	for {
		if health(ctx, client, port) {
			return nil
		}
		select {
		case err := <-exited:
			if err != nil {
				return fmt.Errorf("%w: %v", errChildExited, err)
			}
			return errChildExited
		case <-tick.C:
		case <-deadline.C:
			return fmt.Errorf("llama-server did not become ready within %s (log: %s)", limit, logPath())
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// health is llama-server's own readiness signal: 200 once the model is loaded,
// 503 while it is still loading.
func health(ctx context.Context, client *http.Client, port int) bool {
	c, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(c, http.MethodGet, addr(port)+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<12))
	return resp.StatusCode == http.StatusOK
}

// startIdleWatch reclaims the memory when nobody has asked for anything in a
// while. It only ever stops a server this supervisor started.
func (s *Supervisor) startIdleWatch() {
	s.mu.Lock()
	if s.stopIdle != nil {
		s.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	s.stopIdle = stop
	every := s.idle / 4
	s.mu.Unlock()

	if every < 10*time.Millisecond {
		every = 10 * time.Millisecond
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				s.mu.Lock()
				idleFor := s.now().Sub(s.last)
				expired := s.owned && s.port != 0 && idleFor >= s.idle
				s.mu.Unlock()
				if expired {
					s.stop()
					return
				}
			}
		}
	}()
}

// Close stops the server if this supervisor started it, and does nothing to one
// it merely adopted — three hittable windows share one process, and the first
// to quit must not take it away from the other two.
func (s *Supervisor) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.stop()
}

func (s *Supervisor) stop() {
	s.mu.Lock()
	cmd, wait, owned, logf := s.cmd, s.wait, s.owned, s.log
	stop := s.stopIdle
	s.cmd, s.wait, s.log, s.pid, s.port, s.owned, s.stopIdle = nil, nil, nil, 0, 0, false, nil
	s.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	if owned && cmd != nil {
		terminate(cmd, wait)
		_ = os.Remove(hithome.RunFile())
	}
	closeLog(logf)
}

// terminate asks politely, then insists.
func terminate(cmd *exec.Cmd, wait <-chan error) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	if wait == nil {
		time.Sleep(50 * time.Millisecond)
		_ = cmd.Process.Kill()
		return
	}
	select {
	case <-wait:
		return
	case <-time.After(stopGrace):
	}
	_ = cmd.Process.Kill()
	select {
	case <-wait:
	case <-time.After(stopGrace):
	}
}

// runState is the descriptor other hittable windows read to find a live server.
type runState struct {
	PID     int       `json:"pid"`
	Port    int       `json:"port"`
	Model   string    `json:"model"`
	Binary  string    `json:"binary"`
	Started time.Time `json:"started"`
}

func readRunState() (runState, error) {
	var rs runState
	b, err := os.ReadFile(hithome.RunFile())
	if err != nil {
		return rs, err
	}
	if err := json.Unmarshal(b, &rs); err != nil {
		return rs, err
	}
	if rs.PID <= 0 || rs.Port <= 0 {
		return rs, errors.New("llmhost: incomplete run file")
	}
	return rs, nil
}

func writeRunState(rs runState) {
	b, err := json.MarshalIndent(rs, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(hithome.Run(), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(hithome.RunFile(), append(b, '\n'), 0o644)
}

// stopRunFileServer terminates whatever the run file points at, for Remove and
// for `uninstall`. It only ever touches a process this app recorded, so a
// llama-server the user runs themselves is never killed.
func stopRunFileServer() {
	rs, err := readRunState()
	if err != nil || !processAlive(rs.PID) {
		_ = os.Remove(hithome.RunFile())
		return
	}
	p, err := os.FindProcess(rs.PID)
	if err == nil {
		_ = p.Signal(syscall.SIGTERM)
		for waited := time.Duration(0); waited < stopGrace; waited += 100 * time.Millisecond {
			if !processAlive(rs.PID) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if processAlive(rs.PID) {
			_ = p.Kill()
		}
	}
	_ = os.Remove(hithome.RunFile())
}

func logPath() string { return filepath.Join(hithome.Logs(), "llama-server.log") }

// openLog returns the append-mode server log, ring-trimmed so it cannot grow
// without bound. Trimming on open rather than on write keeps the hot path free
// of any size accounting.
func openLog() *os.File {
	if err := os.MkdirAll(hithome.Logs(), 0o755); err != nil {
		return nil
	}
	p := logPath()
	if fi, err := os.Stat(p); err == nil && fi.Size() > logLimit {
		if b, err := os.ReadFile(p); err == nil {
			_ = os.WriteFile(p, b[len(b)-logLimit/2:], 0o644)
		}
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	return f
}

func closeLog(f *os.File) {
	if f != nil {
		_ = f.Close()
	}
}

// SmokeTest is the difference between "the files are on disk" and "this works".
// It spawns a server, waits for /health, asks for one short completion and
// shuts down again, leaving nothing running. An unsigned binary, a missing
// dylib, a corrupt GGUF or a CPU without the instructions the build needs all
// surface here, at install time with a real error, rather than silently inside
// the TUI three days later.
func SmokeTest(ctx context.Context) (out string, tokPerSec float64, err error) {
	s := NewSupervisor()
	// Prove *this* install runs. Adopting whatever happens to be on 8080 would
	// make the smoke test pass without the new files ever being executed.
	s.noAdopt = true
	defer s.Close()
	return smoke(ctx, s)
}

func smoke(ctx context.Context, s *Supervisor) (string, float64, error) {
	// Start the server first so the model load, which can be seconds, is not
	// counted against the generation rate reported to the user.
	if _, err := s.Endpoint(ctx); err != nil {
		return "", 0, err
	}

	client := llm.New(llm.Config{
		Endpoint: s.Endpoint,
		Timeout:  2 * time.Minute,
	})

	start := time.Now()
	resp, err := client.Complete(ctx, llm.Request{
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: "In one short sentence, say what a git commit message is for.",
		}},
		MaxTokens:   48,
		Temperature: 0,
	})
	if err != nil {
		return "", 0, err
	}
	elapsed := time.Since(start)
	if resp.Total > 0 {
		elapsed = resp.Total
	}

	tokens := resp.CompletionTokens
	if tokens == 0 {
		// A server that reports no usage still produced text; roughly four
		// characters to the token is close enough for a one-line report.
		tokens = len(resp.Text) / 4
	}
	var rate float64
	if secs := elapsed.Seconds(); secs > 0 {
		rate = float64(tokens) / secs
	}
	return resp.Text, rate, nil
}
