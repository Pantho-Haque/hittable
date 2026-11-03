package extensions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma"
)

// ThemeChoice is one colour theme contributed by an installed extension.
type ThemeChoice struct {
	Label  string // as shown in the picker
	ExtID  string
	path   string // absolute path of the theme JSON
	UIType string // vs-dark, vs, hc-black, hc-light
}

// Themes lists colour themes from enabled extensions.
func Themes() []ThemeChoice {
	var out []ThemeChoice
	for _, e := range Enabled() {
		for _, t := range e.Manifest.Contributes.Themes {
			label := localized(t.Label, e.NLS)
			if label == "" {
				label = t.ID
			}
			out = append(out, ThemeChoice{Label: label, ExtID: e.ID, path: filepath.Join(e.Path(), t.Path), UIType: t.UITheme})
		}
	}
	return out
}

type themeFile struct {
	Include     string            `json:"include"`
	Colors      map[string]string `json:"colors"`
	TokenColors json.RawMessage   `json:"tokenColors"`
}

type tokenRule struct {
	Scope    any `json:"scope"`
	Settings struct {
		Foreground string `json:"foreground"`
		FontStyle  string `json:"fontStyle"`
	} `json:"settings"`
}

// loadTheme reads a theme and its include chain (parents first).
func loadTheme(path string, depth int) (map[string]string, []tokenRule, error) {
	if depth > 8 {
		return nil, nil, fmt.Errorf("theme include chain too deep at %s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var tf themeFile
	if err := json.Unmarshal(StripJSONC(b), &tf); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	colors := map[string]string{}
	var rules []tokenRule
	if tf.Include != "" {
		c, r, err := loadTheme(filepath.Join(filepath.Dir(path), tf.Include), depth+1)
		if err != nil {
			return nil, nil, err
		}
		colors, rules = c, r
	}
	for k, v := range tf.Colors {
		colors[k] = v
	}
	if len(tf.TokenColors) > 0 && tf.TokenColors[0] == '[' {
		var own []tokenRule
		if err := json.Unmarshal(tf.TokenColors, &own); err == nil {
			rules = append(rules, own...)
		}
	}
	// A tokenColors string names a .tmTheme (plist); those are rare in
	// current themes and are skipped, leaving the colours block.
	return colors, rules, nil
}

// tokenScopes maps each Chroma token type to the TextMate scopes that colour
// it, most specific first. Chroma (the editor's highlighter) and TextMate
// (what themes target) name things differently; this is the bridge.
var tokenScopes = []struct {
	tok    chroma.TokenType
	scopes []string
}{
	{chroma.Comment, []string{"comment"}},
	{chroma.CommentPreproc, []string{"meta.preprocessor", "keyword.control.directive", "comment"}},
	{chroma.Keyword, []string{"keyword.control", "keyword", "storage"}},
	{chroma.KeywordDeclaration, []string{"storage.type", "keyword"}},
	{chroma.KeywordNamespace, []string{"keyword.control.import", "keyword"}},
	{chroma.KeywordType, []string{"storage.type", "support.type", "entity.name.type"}},
	{chroma.KeywordConstant, []string{"constant.language", "keyword"}},
	{chroma.Name, []string{"variable", "source"}},
	{chroma.NameFunction, []string{"entity.name.function", "support.function"}},
	{chroma.NameClass, []string{"entity.name.type.class", "entity.name.class", "entity.name.type", "support.class"}},
	{chroma.NameBuiltin, []string{"support.function.builtin", "support.function", "support.type"}},
	{chroma.NameVariable, []string{"variable.other", "variable"}},
	{chroma.NameConstant, []string{"variable.other.constant", "constant.other", "constant"}},
	{chroma.NameTag, []string{"entity.name.tag"}},
	{chroma.NameAttribute, []string{"entity.other.attribute-name"}},
	{chroma.NameDecorator, []string{"entity.name.function.decorator", "meta.decorator"}},
	{chroma.NameTag, []string{"entity.name.tag"}},
	{chroma.NameOther, []string{"support.type.property-name", "variable.other.property", "variable"}},
	{chroma.LiteralString, []string{"string"}},
	{chroma.LiteralStringEscape, []string{"constant.character.escape", "string"}},
	{chroma.LiteralStringRegex, []string{"string.regexp", "string"}},
	{chroma.LiteralNumber, []string{"constant.numeric", "constant"}},
	{chroma.Operator, []string{"keyword.operator", "keyword"}},
	{chroma.Punctuation, []string{"punctuation"}},
	{chroma.GenericHeading, []string{"markup.heading", "entity.name.section"}},
	{chroma.GenericSubheading, []string{"markup.heading", "entity.name.section"}},
	{chroma.GenericInserted, []string{"markup.inserted"}},
	{chroma.GenericDeleted, []string{"markup.deleted"}},
	{chroma.GenericEmph, []string{"markup.italic"}},
	{chroma.GenericStrong, []string{"markup.bold"}},
}

// ChromaStyle converts a theme into a Chroma style for the editor. JSON keys
// get support.type.property-name, as VS Code colours them.
func (t ThemeChoice) ChromaStyle() (*chroma.Style, map[string]string, error) {
	colors, rules, err := loadTheme(t.path, 0)
	if err != nil {
		return nil, nil, err
	}
	entries := chroma.StyleEntries{}
	fg := colors["editor.foreground"]
	if bg := colors["editor.background"]; validHex(bg) {
		entry := "bg:" + hex6(bg)
		if validHex(fg) {
			entry += " " + hex6(fg)
		}
		entries[chroma.Background] = entry
	}
	if validHex(fg) {
		entries[chroma.Text] = hex6(fg)
	}
	for _, ts := range tokenScopes {
		for _, sc := range ts.scopes {
			if e, ok := matchRule(rules, sc); ok {
				entries[ts.tok] = e
				break
			}
		}
	}
	// JSON object keys are Chroma NameTag in the JSON lexer.
	if e, ok := matchRule(rules, "support.type.property-name.json"); ok {
		entries[chroma.NameTag] = e
	}
	st, err := chroma.NewStyle(t.Label, entries)
	return st, colors, err
}

// matchRule finds the most specific rule whose selector is a prefix of
// scope (TextMate's rule: "string" colours "string.quoted.double"). Only the
// last part of a descendant selector ("source.go keyword") is compared.
func matchRule(rules []tokenRule, scope string) (string, bool) {
	best, bestLen := "", -1
	for _, r := range rules {
		if r.Settings.Foreground == "" && r.Settings.FontStyle == "" {
			continue
		}
		for _, sel := range ruleScopes(r.Scope) {
			f := strings.Fields(sel)
			if len(f) == 0 {
				continue
			}
			s := f[len(f)-1]
			if (scope == s || strings.HasPrefix(scope, s+".")) && len(s) >= bestLen {
				best, bestLen = styleEntry(r), len(s)
			}
		}
	}
	return best, bestLen >= 0 && best != ""
}

func ruleScopes(v any) []string {
	switch x := v.(type) {
	case string:
		return strings.Split(x, ",")
	case []any:
		var out []string
		for _, s := range x {
			if str, ok := s.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

func styleEntry(r tokenRule) string {
	var parts []string
	if validHex(r.Settings.Foreground) {
		parts = append(parts, hex6(r.Settings.Foreground))
	}
	for _, f := range strings.Fields(r.Settings.FontStyle) {
		switch f {
		case "bold", "italic", "underline":
			parts = append(parts, f)
		}
	}
	return strings.Join(parts, " ")
}

func validHex(s string) bool {
	if !strings.HasPrefix(s, "#") {
		return false
	}
	n := len(s) - 1
	return n == 3 || n == 4 || n == 6 || n == 8
}

// hex6 drops an alpha channel; a terminal cannot blend.
func hex6(s string) string {
	h := strings.TrimPrefix(s, "#")
	switch len(h) {
	case 3, 4:
		return "#" + string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	case 8:
		return "#" + h[:6]
	}
	return "#" + h
}
