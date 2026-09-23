// Package hithome resolves the app's state directory, ~/.hittable, and the
// fixed layout inside it. It exists so that no other package ever writes a
// literal "~/.hittable": one root means "delete everything" is one RemoveAll,
// and $HITTABLE_HOME lets every test point the whole tree at a t.TempDir().
//
// Non-goals: this package holds no state, reads no files, parses nothing and
// never touches the network. The only I/O it performs is the MkdirAll in
// EnsureDirs. What lives under these paths is entirely the caller's business.
package hithome

import (
	"os"
	"path/filepath"
	"strings"
)

// EnvHome is the environment variable that overrides the root directory.
const EnvHome = "HITTABLE_HOME"

// Dir is the app state root: $HITTABLE_HOME when set, else ~/.hittable. If the
// user's home directory cannot be determined the relative ".hittable" is
// returned rather than an error — callers are path builders, not error
// handlers, and a failing MkdirAll later reports the real problem.
func Dir() string {
	if v := strings.TrimSpace(os.Getenv(EnvHome)); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".hittable"
	}
	return filepath.Join(home, ".hittable")
}

// Runtime holds immutable, version-tagged runtime installs plus the "current"
// file naming the active tag.
func Runtime() string { return filepath.Join(Dir(), "runtime") }

// RuntimeTag is the directory for one runtime version.
func RuntimeTag(tag string) string { return filepath.Join(Runtime(), tag) }

// RuntimeCurrent is the text file holding the active runtime tag.
func RuntimeCurrent() string { return filepath.Join(Runtime(), "current") }

// Models holds downloaded model weights.
func Models() string { return filepath.Join(Dir(), "models") }

// Model is the path of one model file by name.
func Model(name string) string { return filepath.Join(Models(), name) }

// Run holds runtime state about live processes (pid, port, model).
func Run() string { return filepath.Join(Dir(), "run") }

// RunFile is the descriptor of a supervised server process.
func RunFile() string { return filepath.Join(Run(), "llama.json") }

// Tmp holds in-progress downloads (<name>.part) and nothing else, so it can be
// swept at the start of any run.
func Tmp() string { return filepath.Join(Dir(), "tmp") }

// Logs holds process logs.
func Logs() string { return filepath.Join(Dir(), "logs") }

// ConfigPath is the app config file.
func ConfigPath() string { return filepath.Join(Dir(), "config.json") }

// EnsureDirs creates the root and every sub-directory of the layout. It is
// idempotent and safe to call from several processes at once.
func EnsureDirs() error {
	for _, d := range []string{Dir(), Runtime(), Models(), Run(), Tmp(), Logs()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
