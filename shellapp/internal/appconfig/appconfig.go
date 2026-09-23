// Package appconfig loads the app's configuration from ~/.hittable/config.json,
// an optional per-project <root>/hittable/config.json and the environment, and
// resolves the precedence between them. Load never fails: a missing, unreadable
// or malformed file yields a fully-defaulted struct plus warning strings the
// caller can show in the status line, because a broken config file must never
// prevent the app from opening.
//
// Non-goals: this package does not watch files, does not cache, holds no global
// state, and knows nothing about what the values mean. A caller may layer its
// own source above the environment; nothing here reads command-line flags.
// Save is the one write path and exists only so that
// `hittable model enable|disable` can flip ai.enabled.
package appconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hittable/shellapp/internal/hithome"
)

// Environment variables, all higher precedence than any file.
const (
	EnvAI         = "HITTABLE_AI"          // "0"/"1" (also false/true, off/on, no/yes)
	EnvAIEndpoint = "HITTABLE_AI_ENDPOINT" // base URL of an already-running server
	EnvAIModel    = "HITTABLE_AI_MODEL"    // model identifier to request
)

// AI holds everything about local inference. Enabled means "use a model if one
// is installed" — it is not a claim that one is. Endpoint "" means "supervise
// our own server"; a non-empty endpoint means "use this one, never spawn and
// never download".
type AI struct {
	Enabled             bool    `json:"enabled"`
	Endpoint            string  `json:"endpoint"`
	Model               string  `json:"model"`
	TimeoutMs           int     `json:"timeoutMs"`
	CompletionTimeoutMs int     `json:"completionTimeoutMs"`
	MaxPromptBytes      int     `json:"maxPromptBytes"`
	IdleMinutes         int     `json:"idleMinutes"`
	Temperature         float64 `json:"temperature"`
}

// Config is the whole config file.
type Config struct {
	AI AI `json:"ai"`
}

// Defaults is the configuration of an installation with no config file at all.
func Defaults() Config {
	return Config{AI: AI{
		Enabled:             true,
		Endpoint:            "",
		Model:               "qwen2.5-coder-3b-instruct-q4_k_m",
		TimeoutMs:           60000,
		CompletionTimeoutMs: 800,
		MaxPromptBytes:      12000,
		IdleMinutes:         10,
		Temperature:         0.2,
	}}
}

// FileName is the config file's base name, in both locations.
const FileName = "config.json"

// ProjectPath is the per-project config file for a working root, which sits
// beside the collection in <root>/hittable/ so it can be committed.
func ProjectPath(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	return filepath.Join(root, "hittable", FileName)
}

// Load resolves the configuration for a working root. Precedence, highest
// first: environment, <root>/hittable/config.json, ~/.hittable/config.json,
// Defaults. Layers are merged field by field, so a file that sets only
// "ai.model" leaves every other field to the layer below it.
//
// The second return value is a list of non-fatal warnings — an unreadable file,
// invalid JSON, an out-of-range number. It is never an error: whatever went
// wrong, the returned Config is usable.
func Load(root string) (Config, []string) {
	c := Defaults()
	var warns []string

	// Lowest file layer first so the project file overrides the user's.
	for _, path := range []string{hithome.ConfigPath(), ProjectPath(root)} {
		if path == "" {
			continue
		}
		warns = append(warns, applyFile(&c, path)...)
	}
	warns = append(warns, applyEnv(&c)...)
	warns = append(warns, sanitize(&c)...)
	return c, warns
}

// applyFile merges one config file over c. A file that does not exist is not a
// warning; anything else is. A malformed file leaves c untouched rather than
// half-applied, so one bad layer cannot corrupt a good one.
func applyFile(c *Config, path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return []string{fmt.Sprintf("config: %s could not be read (%v); using defaults", short(path), err)}
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}

	merged := *c
	if err := json.Unmarshal(data, &merged); err != nil {
		return []string{fmt.Sprintf("config: %s is not valid JSON (%v); using defaults", short(path), err)}
	}
	*c = merged
	return nil
}

// applyEnv overlays the environment, the highest precedence this package knows.
func applyEnv(c *Config) []string {
	var warns []string

	if raw, ok := os.LookupEnv(EnvAI); ok {
		if v := strings.TrimSpace(raw); v != "" {
			switch strings.ToLower(v) {
			case "1", "true", "on", "yes":
				c.AI.Enabled = true
			case "0", "false", "off", "no":
				c.AI.Enabled = false
			default:
				warns = append(warns, fmt.Sprintf("config: %s=%q is not 0 or 1; ignored", EnvAI, raw))
			}
		}
	}
	if v := strings.TrimSpace(os.Getenv(EnvAIEndpoint)); v != "" {
		c.AI.Endpoint = v
	}
	if v := strings.TrimSpace(os.Getenv(EnvAIModel)); v != "" {
		c.AI.Model = v
	}
	return warns
}

// sanitize replaces values that cannot work with the default for that field.
// A zero timeout is not a fast timeout, it is a client that never talks to
// anything, and that failure is invisible at the call site.
func sanitize(c *Config) []string {
	d := Defaults().AI
	var warns []string

	ints := []struct {
		name string
		p    *int
		def  int
	}{
		{"timeoutMs", &c.AI.TimeoutMs, d.TimeoutMs},
		{"completionTimeoutMs", &c.AI.CompletionTimeoutMs, d.CompletionTimeoutMs},
		{"maxPromptBytes", &c.AI.MaxPromptBytes, d.MaxPromptBytes},
		{"idleMinutes", &c.AI.IdleMinutes, d.IdleMinutes},
	}
	for _, f := range ints {
		if *f.p <= 0 {
			warns = append(warns, fmt.Sprintf("config: ai.%s must be positive; using %d", f.name, f.def))
			*f.p = f.def
		}
	}
	if c.AI.Temperature < 0 || c.AI.Temperature > 2 {
		warns = append(warns, fmt.Sprintf("config: ai.temperature must be between 0 and 2; using %g", d.Temperature))
		c.AI.Temperature = d.Temperature
	}
	c.AI.Endpoint = strings.TrimSpace(c.AI.Endpoint)
	c.AI.Model = strings.TrimSpace(c.AI.Model)
	return warns
}

// Save writes c to ~/.hittable/config.json atomically: a temp file in the same
// directory, then a rename, so a crash mid-write cannot leave a truncated file
// that the next Load would report as malformed.
func Save(c Config) error {
	if err := hithome.EnsureDirs(); err != nil {
		return err
	}
	path := hithome.ConfigPath()

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // no-op once the rename succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// short trims a path to its last two elements for warning messages, which are
// shown in a one-line status bar.
func short(path string) string {
	dir, file := filepath.Split(path)
	parent := filepath.Base(filepath.Clean(dir))
	if parent == "." || parent == string(filepath.Separator) {
		return file
	}
	return filepath.Join(parent, file)
}
