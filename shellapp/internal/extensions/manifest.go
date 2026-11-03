// Package extensions installs and describes VS Code extensions for the
// terminal app: the Open VSX registry client, .vsix install and removal, the
// on-disk index, and the declarative parts of a manifest Go uses directly
// (colour themes, snippets, language ids). Running extension code is
// internal/exthost's job.
package extensions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// APIVersion is the VS Code version hittable reports to extensions
// (vscode.version) and checks engines.vscode against. It tracks the current
// VS Code release, as other VS Code-compatible editors do: engines.vscode is
// the newest API an extension *may* use, and an API the host lacks fails
// with a logged warning rather than at install. Bump it with VS Code
// (https://update.code.visualstudio.com/api/releases/stable).
const APIVersion = "1.144.0"

// Manifest is the subset of an extension's package.json the app reads.
type Manifest struct {
	Name          string            `json:"name"`
	Publisher     string            `json:"publisher"`
	Version       string            `json:"version"`
	DisplayName   string            `json:"displayName"`
	Description   string            `json:"description"`
	Icon          string            `json:"icon"` // path of a PNG inside the extension
	Engines       map[string]string `json:"engines"`
	Main          string            `json:"main"`
	Browser       string            `json:"browser"`
	Activation    []string          `json:"activationEvents"`
	Dependencies  []string          `json:"extensionDependencies"`
	Pack          []string          `json:"extensionPack"`
	APIProposals  []string          `json:"enabledApiProposals"`
	Contributes   Contributes       `json:"contributes"`
	ExtensionKind []string          `json:"extensionKind"`
}

// Contributes is the declarative part of a manifest.
type Contributes struct {
	Commands      []Command         `json:"commands"`
	Themes        []ThemeContrib    `json:"themes"`
	Snippets      []SnippetContrib  `json:"snippets"`
	Languages     []LanguageContrib `json:"languages"`
	Grammars      []json.RawMessage `json:"grammars"`
	Keybindings   []Keybinding      `json:"keybindings"`
	Views         map[string][]View `json:"views"`
	Configuration json.RawMessage   `json:"configuration"`
	IconThemes    []json.RawMessage `json:"iconThemes"`
	Debuggers     []json.RawMessage `json:"debuggers"`
	Notebooks     []json.RawMessage `json:"notebooks"`
	CustomEditors []json.RawMessage `json:"customEditors"`
}

type Command struct {
	Command  string `json:"command"`
	Title    any    `json:"title"`    // a string, or {value, original} when localized
	Category any    `json:"category"` // same
}

type ThemeContrib struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	UITheme string `json:"uiTheme"`
	Path    string `json:"path"`
}

type SnippetContrib struct {
	Language string `json:"language"`
	Path     string `json:"path"`
}

type LanguageContrib struct {
	ID         string   `json:"id"`
	Extensions []string `json:"extensions"`
	Filenames  []string `json:"filenames"`
	Aliases    []string `json:"aliases"`
}

type Keybinding struct {
	Command string `json:"command"`
	Key     string `json:"key"`
	Mac     string `json:"mac"`
	Linux   string `json:"linux"`
	Win     string `json:"win"`
	When    string `json:"when"`
	Args    any    `json:"args"`
}

type View struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "webview" for webview views, else a tree
}

// ID is publisher.name, lowercased as VS Code compares ids.
func (m Manifest) ID() string { return strings.ToLower(m.Publisher + "." + m.Name) }

// Label is the display name, falling back to the id.
func (m Manifest) Label() string {
	if m.DisplayName != "" && !strings.HasPrefix(m.DisplayName, "%") {
		return m.DisplayName
	}
	return m.ID()
}

// HasCode reports whether running the extension needs Node: it has a main
// entry point. Themes, snippets and language definitions need nothing.
func (m Manifest) HasCode() bool { return m.Main != "" }

// Unsupported lists what the extension contributes that the terminal app
// cannot show, so the user hears it before installing rather than after.
func (m Manifest) Unsupported() []string {
	var out []string
	c := m.Contributes
	if m.Main == "" && m.Browser != "" {
		out = append(out, "its code, which is built only for browser editors (vscode.dev)")
	}
	if len(c.Debuggers) > 0 {
		out = append(out, "debuggers")
	}
	if len(c.Notebooks) > 0 {
		out = append(out, "notebooks")
	}
	if len(c.CustomEditors) > 0 {
		out = append(out, "custom editors")
	}
	if len(c.IconThemes) > 0 {
		out = append(out, "file icon themes (the terminal's font decides icons)")
	}
	return out
}

// CommandTitle renders a contributed command as "Category: Title".
func (c Command) CommandTitle(nls map[string]string) string {
	t := localized(c.Title, nls)
	if cat := localized(c.Category, nls); cat != "" {
		return cat + ": " + t
	}
	if t == "" {
		return c.Command
	}
	return t
}

func localized(v any, nls map[string]string) string {
	var s string
	switch x := v.(type) {
	case string:
		s = x
	case map[string]any:
		s, _ = x["value"].(string)
	}
	if strings.HasPrefix(s, "%") && strings.HasSuffix(s, "%") {
		if t, ok := nls[strings.Trim(s, "%")]; ok {
			return t
		}
	}
	return s
}

// ReadManifest parses dir/package.json, substituting %key% strings from
// package.nls.json where the manifest is localized.
func ReadManifest(dir string) (Manifest, map[string]string, error) {
	var m Manifest
	b, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return m, nil, err
	}
	if err := json.Unmarshal(StripJSONC(b), &m); err != nil {
		return m, nil, fmt.Errorf("package.json: %w", err)
	}
	if m.Name == "" || m.Publisher == "" {
		return m, nil, fmt.Errorf("package.json has no name or publisher")
	}
	nls := map[string]string{}
	if b, err := os.ReadFile(filepath.Join(dir, "package.nls.json")); err == nil {
		var raw map[string]any
		if json.Unmarshal(StripJSONC(b), &raw) == nil {
			for k, v := range raw {
				switch x := v.(type) {
				case string:
					nls[k] = x
				case map[string]any:
					if s, ok := x["message"].(string); ok {
						nls[k] = s
					}
				}
			}
		}
	}
	if strings.HasPrefix(m.DisplayName, "%") {
		m.DisplayName = localized(m.DisplayName, nls)
	}
	return m, nls, nil
}
