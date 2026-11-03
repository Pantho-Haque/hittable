package extensions

import (
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// builtinLanguages are the ids VS Code itself knows by extension, so a
// document gets the languageId extensions register providers for (a YAML
// extension's selector says "yaml", not ".yml").
var builtinLanguages = map[string]string{
	".go": "go", ".json": "json", ".jsonc": "jsonc", ".hit": "json", ".yaml": "yaml", ".yml": "yaml",
	".py": "python", ".js": "javascript", ".mjs": "javascript", ".cjs": "javascript",
	".jsx": "javascriptreact", ".ts": "typescript", ".mts": "typescript", ".cts": "typescript",
	".tsx": "typescriptreact", ".md": "markdown", ".markdown": "markdown", ".rs": "rust",
	".html": "html", ".htm": "html", ".css": "css", ".scss": "scss", ".less": "less",
	".xml": "xml", ".svg": "xml", ".sql": "sql", ".sh": "shellscript", ".bash": "shellscript",
	".zsh": "shellscript", ".toml": "toml", ".ini": "ini", ".java": "java", ".c": "c", ".h": "c",
	".cpp": "cpp", ".cc": "cpp", ".hpp": "cpp", ".cs": "csharp", ".php": "php", ".rb": "ruby",
	".lua": "lua", ".swift": "swift", ".kt": "kotlin", ".dart": "dart", ".vue": "vue",
	".svelte": "svelte", ".graphql": "graphql", ".gql": "graphql", ".proto": "proto3",
	".tf": "terraform", ".r": "r", ".ex": "elixir", ".exs": "elixir", ".erl": "erlang",
	".scala": "scala", ".zig": "zig", ".nix": "nix", ".txt": "plaintext", ".log": "log",
}

var builtinFilenames = map[string]string{
	"dockerfile": "dockerfile", "makefile": "makefile", "go.mod": "go.mod", "go.sum": "go.sum",
	"settings.json": "jsonc", "tsconfig.json": "jsonc", "jsconfig.json": "jsonc",
	".gitignore": "ignore", "cargo.lock": "toml",
}

var langCache struct {
	sync.Mutex
	at    time.Time
	ext   map[string]string
	names map[string]string
}

// LanguageID is the VS Code language id for a file path. Languages that
// enabled extensions contribute win over the built-in table, as in VS Code.
func LanguageID(path string) string {
	ext, names := contributedLanguages()
	base := strings.ToLower(filepath.Base(path))
	if id, ok := names[base]; ok {
		return id
	}
	if id, ok := builtinFilenames[base]; ok {
		return id
	}
	// Longest matching suffix first: ".d.ts" before ".ts".
	best := ""
	for suffix := range ext {
		if strings.HasSuffix(base, suffix) && len(suffix) > len(best) {
			best = suffix
		}
	}
	if best != "" {
		return ext[best]
	}
	if id, ok := builtinLanguages[strings.ToLower(filepath.Ext(base))]; ok {
		return id
	}
	return "plaintext"
}

func contributedLanguages() (map[string]string, map[string]string) {
	langCache.Lock()
	defer langCache.Unlock()
	if langCache.ext != nil && time.Since(langCache.at) < 5*time.Second {
		return langCache.ext, langCache.names
	}
	ext, names := map[string]string{}, map[string]string{}
	for _, e := range Enabled() {
		for _, l := range e.Manifest.Contributes.Languages {
			for _, x := range l.Extensions {
				ext[strings.ToLower(x)] = l.ID
			}
			for _, f := range l.Filenames {
				names[strings.ToLower(f)] = l.ID
			}
		}
	}
	langCache.ext, langCache.names, langCache.at = ext, names, time.Now()
	return ext, names
}

// ResetLanguageCache forgets contributed languages after an install.
func ResetLanguageCache() {
	langCache.Lock()
	langCache.ext = nil
	langCache.Unlock()
}
