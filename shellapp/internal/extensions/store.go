package extensions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hittable/shellapp/internal/hithome"
	"github.com/hittable/shellapp/internal/llmhost"
)

// Installed is one entry of ~/.hittable/extensions/extensions.json.
type Installed struct {
	ID             string    `json:"id"`
	Version        string    `json:"version"`
	Dir            string    `json:"dir"` // folder name under Extensions()
	Enabled        bool      `json:"enabled"`
	Source         string    `json:"source"` // "openvsx" or "vsix"
	TargetPlatform string    `json:"targetPlatform,omitempty"`
	Signed         bool      `json:"signed"`
	InstalledAt    time.Time `json:"installedAt"`

	Manifest Manifest          `json:"-"`
	NLS      map[string]string `json:"-"`
}

// Path is the extension's root folder (where package.json is).
func (e Installed) Path() string { return filepath.Join(hithome.Extensions(), e.Dir) }

type index struct {
	Extensions []Installed `json:"extensions"`
	// Trusted lists project roots the user allowed extension code to run in
	// (VS Code's workspace trust). Themes and snippets work everywhere.
	Trusted []string `json:"trusted"`
}

func indexPath() string { return filepath.Join(hithome.Extensions(), "extensions.json") }

func readIndex() (index, error) {
	var ix index
	b, err := os.ReadFile(indexPath())
	if errors.Is(err, os.ErrNotExist) {
		return ix, nil
	}
	if err != nil {
		return ix, err
	}
	if err := json.Unmarshal(b, &ix); err != nil {
		return ix, fmt.Errorf("%s is damaged: %w", indexPath(), err)
	}
	return ix, nil
}

// writeIndex replaces the file atomically, so a crash mid-write never
// leaves a half-written index that would hide every installed extension.
func writeIndex(ix index) error {
	if err := os.MkdirAll(hithome.Extensions(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(ix, "", "  ")
	if err != nil {
		return err
	}
	tmp := indexPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, indexPath())
}

// List returns installed extensions with their manifests, sorted by id. An
// entry whose folder has gone missing or whose manifest no longer parses is
// skipped rather than failing the whole list.
func List() ([]Installed, error) {
	ix, err := readIndex()
	if err != nil {
		return nil, err
	}
	var out []Installed
	for _, e := range ix.Extensions {
		m, nls, err := ReadManifest(e.Path())
		if err != nil {
			continue
		}
		e.Manifest, e.NLS = m, nls
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Enabled is List filtered to enabled extensions.
func Enabled() []Installed {
	all, _ := List()
	var out []Installed
	for _, e := range all {
		if e.Enabled {
			out = append(out, e)
		}
	}
	return out
}

// Find returns the installed extension with id (case-insensitive).
func Find(id string) (Installed, bool) {
	all, _ := List()
	for _, e := range all {
		if e.ID == strings.ToLower(id) {
			return e, true
		}
	}
	return Installed{}, false
}

// SetEnabled turns an installed extension on or off.
func SetEnabled(id string, on bool) error {
	unlock, err := lock()
	if err != nil {
		return err
	}
	defer unlock()
	ix, err := readIndex()
	if err != nil {
		return err
	}
	for i := range ix.Extensions {
		if ix.Extensions[i].ID == strings.ToLower(id) {
			ix.Extensions[i].Enabled = on
			return writeIndex(ix)
		}
	}
	return fmt.Errorf("%s is not installed", id)
}

// Uninstall removes an extension. It refuses while another installed
// extension declares it as a dependency, naming that extension.
func Uninstall(id string) error {
	id = strings.ToLower(id)
	unlock, err := lock()
	if err != nil {
		return err
	}
	defer unlock()
	ix, err := readIndex()
	if err != nil {
		return err
	}
	all, _ := List()
	for _, e := range all {
		for _, d := range e.Manifest.Dependencies {
			if strings.ToLower(d) == id {
				return fmt.Errorf("%s needs %s; uninstall it first", e.ID, id)
			}
		}
	}
	kept := ix.Extensions[:0]
	var gone *Installed
	for _, e := range ix.Extensions {
		if e.ID == id {
			e := e
			gone = &e
			continue
		}
		kept = append(kept, e)
	}
	if gone == nil {
		return fmt.Errorf("%s is not installed", id)
	}
	ix.Extensions = kept
	if err := writeIndex(ix); err != nil {
		return err
	}
	return os.RemoveAll(gone.Path())
}

// Trusted reports whether extension code may run in root.
func Trusted(root string) bool {
	ix, _ := readIndex()
	root = canonical(root)
	for _, t := range ix.Trusted {
		if t == root || strings.HasPrefix(root, t+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// Trust records root as trusted.
func Trust(root string) error {
	unlock, err := lock()
	if err != nil {
		return err
	}
	defer unlock()
	ix, err := readIndex()
	if err != nil {
		return err
	}
	root = canonical(root)
	for _, t := range ix.Trusted {
		if t == root {
			return nil
		}
	}
	ix.Trusted = append(ix.Trusted, root)
	return writeIndex(ix)
}

// canonical is the absolute path with symlinks resolved, so /var/x and
// /private/var/x (macOS) compare equal. For a path that does not exist yet,
// the nearest existing parent is resolved and the rest re-attached.
func canonical(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	p = filepath.Clean(p)
	rest := ""
	for dir := p; ; {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(r, rest)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return p
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
}

// lock serialises installs across hittable windows and CLI runs with an
// O_EXCL lock file holding the owner's pid. A lock whose owner is no longer
// running (it crashed or was killed mid-install) is taken over at once; one
// older than ten minutes is too, in case the pid was reused.
func lock() (func(), error) {
	if err := os.MkdirAll(hithome.Extensions(), 0o755); err != nil {
		return nil, err
	}
	p := filepath.Join(hithome.Extensions(), ".lock")
	deadline := time.Now().Add(2 * time.Minute)
	for {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			fmt.Fprint(f, os.Getpid())
			f.Close()
			return func() { _ = os.Remove(p) }, nil
		}
		if fi, serr := os.Stat(p); serr == nil {
			b, _ := os.ReadFile(p)
			pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
			if (pid > 0 && pid != os.Getpid() && !llmhost.ProcessAlive(pid)) || time.Since(fi.ModTime()) > 10*time.Minute {
				_ = os.Remove(p)
				continue
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("another hittable is installing extensions (lock %s); try again when it finishes", p)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// LocalDoc reads an installed extension's README.md or CHANGELOG.md (kind
// "readme" / "changelog"), whatever the file's capitalisation. "" if absent.
func LocalDoc(e Installed, kind string) string {
	entries, err := os.ReadDir(e.Path())
	if err != nil {
		return ""
	}
	for _, d := range entries {
		n := strings.ToLower(d.Name())
		if !d.IsDir() && (n == kind+".md" || n == kind || n == kind+".markdown") {
			b, _ := os.ReadFile(filepath.Join(e.Path(), d.Name()))
			return string(b)
		}
	}
	return ""
}
