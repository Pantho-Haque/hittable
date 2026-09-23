package llmhost

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hittable/shellapp/internal/hithome"
)

// The supervisor tests never spawn a real llama-server. The "binary" is this
// test binary re-executed into TestLlamaServerStub, which speaks just enough of
// the protocol — /health and one chat completion — to exercise readiness
// polling, adoption, idle shutdown and the smoke test.

const (
	stubEnv   = "LLMHOST_STUB"
	stubMode  = "LLMHOST_STUB_MODE"
	stubDelay = "LLMHOST_STUB_DELAY"
	stubArgs  = "LLMHOST_STUB_ARGS"
)

// TestLlamaServerStub is not a test. It is the fake server the tests below
// spawn; in a normal run the environment variable is unset and it returns
// immediately.
func TestLlamaServerStub(t *testing.T) {
	if os.Getenv(stubEnv) != "1" {
		return
	}
	stubServer()
}

func stubServer() {
	if path := os.Getenv(stubArgs); path != "" {
		_ = os.WriteFile(path, []byte(strings.Join(os.Args, "\n")), 0o644)
	}
	if os.Getenv(stubMode) == "exit" {
		os.Exit(3)
	}

	delay, _ := time.ParseDuration(os.Getenv(stubDelay))
	start := time.Now()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		// llama-server answers 503 while the model is still loading, which is
		// exactly the state the readiness poll has to sit through.
		if time.Since(start) < delay {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message":       map[string]any{"content": stubReply},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{"prompt_tokens": 12, "completion_tokens": 10},
		})
	})

	srv := &http.Server{Addr: "127.0.0.1:" + argAfter(os.Args, "--port"), Handler: mux}
	if err := srv.ListenAndServe(); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

const stubReply = "It records why a change was made."

func argAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// stubSupervisor is a supervisor wired to the stub instead of a real runtime.
func stubSupervisor(t *testing.T, env ...string) *Supervisor {
	t.Helper()
	dir := home(t)

	model := filepath.Join(dir, "weights.gguf")
	if err := os.WriteFile(model, []byte("pretend weights"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewSupervisor()
	s.noAdopt = true // never touch whatever is on :8080 on the dev machine
	s.bin = os.Args[0]
	s.modelPath = model
	s.preArgs = []string{"-test.run=TestLlamaServerStub", "--"}
	s.extraEnv = append([]string{stubEnv + "=1"}, env...)
	s.ready = 20 * time.Second
	s.poll = 20 * time.Millisecond
	t.Cleanup(s.Close)
	return s
}

func TestSupervisorStartsNothingUntilAsked(t *testing.T) {
	home(t)
	s := NewSupervisor()
	defer s.Close()

	if _, _, ok := s.Running(); ok {
		t.Fatal("NewSupervisor started a server")
	}
	if _, err := os.Stat(hithome.RunFile()); err == nil {
		t.Fatal("NewSupervisor wrote a run file")
	}
}

func TestSpawnArgs(t *testing.T) {
	tests := []struct {
		name         string
		goos, goarch string
		ncpu         int
		wantThreads  string
		wantGPU      bool
	}{
		{"apple silicon", "darwin", "arm64", 10, "5", true},
		{"intel mac", "darwin", "amd64", 8, "4", false},
		{"linux server", "linux", "amd64", 64, "32", false},
		{"linux arm", "linux", "arm64", 4, "2", false},
		{"windows", "windows", "amd64", 16, "8", false},
		{"single core floors at two threads", "linux", "amd64", 1, "2", false},
		{"dual core floors at two threads", "linux", "amd64", 2, "2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := spawnArgs("/m/weights.gguf", 51234, tt.goos, tt.goarch, tt.ncpu)

			for flag, want := range map[string]string{
				"--model":    "/m/weights.gguf",
				"--host":     "127.0.0.1",
				"--port":     "51234",
				"--ctx-size": strconv.Itoa(ContextSize),
				"--threads":  tt.wantThreads,
				"--parallel": "2",
			} {
				if got := argAfter(args, flag); got != want {
					t.Errorf("%s = %q, want %q", flag, got, want)
				}
			}
			for _, want := range []string{"--cont-batching", "--jinja", "--no-webui"} {
				if !hasArg(args, want) {
					t.Errorf("%s missing from %v", want, args)
				}
			}
			if got := hasArg(args, "--n-gpu-layers"); got != tt.wantGPU {
				t.Errorf("--n-gpu-layers present = %v, want %v", got, tt.wantGPU)
			}
			// The context ceiling is what the prompt budget is sized against;
			// letting it drift silently truncates prompts.
			if ContextSize != 8192 {
				t.Errorf("ContextSize = %d, want 8192", ContextSize)
			}
		})
	}
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestFreePortReturnsDistinctPorts(t *testing.T) {
	seen := map[int]bool{}
	for i := 0; i < 8; i++ {
		p, err := freePort()
		if err != nil {
			t.Fatal(err)
		}
		if p <= 0 || p > 65535 {
			t.Fatalf("freePort() = %d", p)
		}
		if p == defaultLlamaPort {
			t.Fatalf("freePort() returned llama-server's default port %d", p)
		}
		if seen[p] {
			t.Fatalf("freePort() returned %d twice", p)
		}
		seen[p] = true
	}
}

func TestEndpointSpawnsAndWaitsForHealth(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "args")
	// 300ms of 503s: readiness has to be polled, not assumed.
	s := stubSupervisor(t, stubDelay+"=300ms", stubArgs+"="+recorded)

	start := time.Now()
	endpoint, err := s.Endpoint(context.Background())
	if err != nil {
		t.Fatalf("Endpoint: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 300*time.Millisecond {
		t.Errorf("Endpoint returned after %s; it did not wait for /health", elapsed)
	}

	pid, port, ok := s.Running()
	if !ok || pid <= 0 || port <= 0 {
		t.Fatalf("Running() = %d, %d, %v", pid, port, ok)
	}
	if endpoint != addr(port) {
		t.Errorf("endpoint = %q, want %q", endpoint, addr(port))
	}

	// The run file is what lets a second hittable window share this process.
	rs, err := readRunState()
	if err != nil {
		t.Fatalf("run file: %v", err)
	}
	if rs.PID != pid || rs.Port != port {
		t.Errorf("run file = %+v, want pid %d port %d", rs, pid, port)
	}

	args, err := os.ReadFile(recorded)
	if err != nil {
		t.Fatalf("the stub recorded no arguments: %v", err)
	}
	if !strings.Contains(string(args), "--ctx-size") {
		t.Errorf("spawned without --ctx-size: %s", args)
	}

	// A second call reuses the same process rather than starting another.
	again, err := s.Endpoint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if again != endpoint {
		t.Errorf("second Endpoint = %q, want the same %q", again, endpoint)
	}
	if pid2, _, _ := s.Running(); pid2 != pid {
		t.Errorf("a second Endpoint spawned a new process (%d then %d)", pid, pid2)
	}
}

func TestEndpointAdoptsALiveRunFile(t *testing.T) {
	home(t)

	// Stand in for another hittable window's server. This process is alive by
	// definition, which is what the adoption check looks for.
	srv := healthyServer(t)
	writeRunState(runState{PID: os.Getpid(), Port: portOf(t, srv), Model: "x", Binary: "y", Started: time.Now()})

	s := NewSupervisor()
	s.noAdopt = true
	s.bin = filepath.Join(t.TempDir(), "does-not-exist")

	got, err := s.Endpoint(context.Background())
	if err != nil {
		t.Fatalf("Endpoint: %v", err)
	}
	if want := addr(portOf(t, srv)); got != want {
		t.Fatalf("endpoint = %q, want the adopted %q", got, want)
	}

	s.mu.Lock()
	owned := s.owned
	s.mu.Unlock()
	if owned {
		t.Error("an adopted server was marked owned; Close would kill somebody else's process")
	}

	// Closing must leave the adopted server, and its run file, alone.
	s.Close()
	if _, err := os.Stat(hithome.RunFile()); err != nil {
		t.Errorf("Close removed the run file of a server it does not own: %v", err)
	}
	if !health(context.Background(), srv.Client(), portOf(t, srv)) {
		t.Error("Close stopped a server it does not own")
	}
}

func TestEndpointIgnoresADeadRunFile(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "args")
	s := stubSupervisor(t, stubArgs+"="+recorded)

	// A pid that is not running, which is what a crashed server leaves behind.
	writeRunState(runState{PID: 0x7FFFFFF, Port: 59999, Model: "x", Binary: "y", Started: time.Now()})

	if _, err := s.Endpoint(context.Background()); err != nil {
		t.Fatalf("Endpoint: %v", err)
	}
	if _, port, _ := s.Running(); port == 59999 {
		t.Fatal("a stale run file was adopted")
	}
	if _, err := os.Stat(recorded); err != nil {
		t.Errorf("no server was spawned: %v", err)
	}
}

func TestRestartBudgetCapsSpawning(t *testing.T) {
	s := stubSupervisor(t, stubMode+"=exit")
	s.ready = 2 * time.Second

	for i := 0; i < maxRestarts; i++ {
		_, err := s.Endpoint(context.Background())
		if err == nil {
			t.Fatalf("attempt %d unexpectedly succeeded", i+1)
		}
		if errors.Is(err, ErrRestartBudget) {
			t.Fatalf("attempt %d hit the budget early; %d are allowed", i+1, maxRestarts)
		}
	}

	_, err := s.Endpoint(context.Background())
	if !errors.Is(err, ErrRestartBudget) {
		t.Fatalf("err = %v, want ErrRestartBudget after %d restarts", err, maxRestarts)
	}

	// The window slides: once the recorded starts age out, spawning resumes.
	s.mu.Lock()
	s.now = func() time.Time { return time.Now().Add(restartWindow + time.Minute) }
	s.mu.Unlock()
	if _, err := s.Endpoint(context.Background()); errors.Is(err, ErrRestartBudget) {
		t.Error("the restart window did not slide")
	}
}

func TestReadinessTimeoutDoesNotRetry(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "args")
	// Never becomes healthy. Retrying three times would cost three timeouts
	// and could not change the answer.
	s := stubSupervisor(t, stubDelay+"=1h", stubArgs+"="+recorded)
	s.ready = 300 * time.Millisecond

	start := time.Now()
	_, err := s.Endpoint(context.Background())
	if err == nil {
		t.Fatal("Endpoint succeeded against a server that never reported healthy")
	}
	if !strings.Contains(err.Error(), "did not become ready") {
		t.Errorf("err = %v, want a readiness timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %s — the readiness timeout was retried", elapsed)
	}
	if _, _, ok := s.Running(); ok {
		t.Error("a server that never became ready was left registered")
	}
}

func TestIdleShutdownReclaimsTheProcess(t *testing.T) {
	s := stubSupervisor(t)
	s.idle = 200 * time.Millisecond

	if _, err := s.Endpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	pid, _, _ := s.Running()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, ok := s.Running(); !ok {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, _, ok := s.Running(); ok {
		t.Fatal("the idle timer never fired")
	}
	waitGone(t, pid)
	if _, err := os.Stat(hithome.RunFile()); err == nil {
		t.Error("the run file outlived the process it points at")
	}
}

func TestCloseTerminatesAnOwnedServer(t *testing.T) {
	s := stubSupervisor(t)
	if _, err := s.Endpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	pid, _, _ := s.Running()

	s.Close()
	waitGone(t, pid)
	if _, _, ok := s.Running(); ok {
		t.Error("Running() still reports a server after Close")
	}
	// Close twice is what quitting during a failed shutdown looks like.
	s.Close()

	if _, err := s.Endpoint(context.Background()); err == nil {
		t.Error("a closed supervisor spawned a new server")
	}
}

func TestSmokeTestRunsOneCompletion(t *testing.T) {
	s := stubSupervisor(t)

	out, tps, err := smoke(context.Background(), s)
	if err != nil {
		t.Fatalf("smoke: %v", err)
	}
	if out != stubReply {
		t.Errorf("out = %q, want %q", out, stubReply)
	}
	if tps <= 0 {
		t.Errorf("tokens/sec = %v, want a positive rate", tps)
	}
}

func TestEndpointReportsNothingInstalled(t *testing.T) {
	home(t)
	s := NewSupervisor()
	s.noAdopt = true
	defer s.Close()

	_, err := s.Endpoint(context.Background())
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("err = %v, want ErrNotInstalled", err)
	}
	if !strings.Contains(err.Error(), "hittable model enable") {
		t.Errorf("err = %v, want it to say how to fix this", err)
	}
}

// --- helpers ----------------------------------------------------------------

// healthyServer stands in for a llama-server another process is running.
func healthyServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func portOf(t *testing.T, s *httptest.Server) int {
	t.Helper()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func waitGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("process %d is still running", pid)
}

// Stop is the "give me my memory back" half of the split: it must leave the
// model on disk, so turning the feature back on costs nothing. A user who
// wants their RAM should not have to pay a two-gigabyte download for it.
func TestStopLeavesTheModelOnDisk(t *testing.T) {
	t.Setenv(hithome.EnvHome, t.TempDir())
	if err := hithome.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	model := ModelPath()
	if err := os.MkdirAll(filepath.Dir(model), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(model, []byte("gguf"), 0o644); err != nil {
		t.Fatal(err)
	}

	running, ram, err := Stop()
	if err != nil {
		t.Fatalf("Stop with no server: %v", err)
	}
	if running || ram != 0 {
		t.Errorf("Stop reported running=%v ram=%d with no server", running, ram)
	}
	if _, err := os.Stat(model); err != nil {
		t.Errorf("Stop deleted the model: %v", err)
	}
}

// A run file left behind by a crash names a pid that is gone. Stop must clear
// it rather than report a server that is not there, or status lies forever.
func TestStopClearsAStaleRunFile(t *testing.T) {
	t.Setenv(hithome.EnvHome, t.TempDir())
	if err := hithome.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	// A pid that cannot be alive.
	if err := os.WriteFile(hithome.RunFile(),
		[]byte(`{"pid":999999,"port":1,"model":"x","binary":"y","started":"2020-01-01T00:00:00Z"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	running, _, err := Stop()
	if err != nil {
		t.Fatal(err)
	}
	if running {
		t.Error("reported a dead pid as running")
	}
	if _, err := os.Stat(hithome.RunFile()); !os.IsNotExist(err) {
		t.Error("stale run file was not cleared")
	}
}
