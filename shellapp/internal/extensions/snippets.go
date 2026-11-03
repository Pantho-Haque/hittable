package extensions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Snippet is one entry from a contributed snippets file.
type Snippet struct {
	Prefix      string
	Body        string // VS Code snippet syntax; ExpandSnippet turns it into text
	Description string
}

type snippetJSON struct {
	Prefix      any    `json:"prefix"`
	Body        any    `json:"body"`
	Description string `json:"description"`
}

// Snippets returns the snippets enabled extensions contribute for a
// language id. Files are re-read on each call; callers cache per language.
func Snippets(lang string) []Snippet {
	var out []Snippet
	for _, e := range Enabled() {
		for _, s := range e.Manifest.Contributes.Snippets {
			if s.Language != lang {
				continue
			}
			b, err := os.ReadFile(filepath.Join(e.Path(), s.Path))
			if err != nil {
				continue
			}
			var raw map[string]snippetJSON
			if json.Unmarshal(StripJSONC(b), &raw) != nil {
				continue
			}
			for name, sj := range raw {
				body := joinAny(sj.Body, "\n")
				desc := sj.Description
				if desc == "" {
					desc = name
				}
				for _, p := range listAny(sj.Prefix) {
					out = append(out, Snippet{Prefix: p, Body: body, Description: desc})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Prefix < out[j].Prefix })
	return out
}

func listAny(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
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

func joinAny(v any, sep string) string { return strings.Join(listAny(v), sep) }

// ExpandSnippet resolves VS Code snippet syntax into plain text and the
// rune offset where the cursor should land: the first tabstop ($1), else
// $0, else the end. Placeholders keep their default text (${1:name} →
// "name"), choices their first option, and variables their default or
// nothing ($TM_FILENAME has no meaning here).
func ExpandSnippet(body string) (string, int) {
	var out []rune
	first, zero := -1, -1
	r := []rune(body)
	var walk func(i int, stop rune) int
	walk = func(i int, stop rune) int {
		for i < len(r) {
			c := r[i]
			if stop != 0 && c == stop {
				return i
			}
			if c == '\\' && i+1 < len(r) && strings.ContainsRune(`$}\,|`, r[i+1]) {
				out = append(out, r[i+1])
				i += 2
				continue
			}
			if c != '$' || i+1 >= len(r) {
				out = append(out, c)
				i++
				continue
			}
			j := i + 1
			mark := func(n int) {
				if n == 0 && zero < 0 {
					zero = len(out)
				}
				if n == 1 && first < 0 {
					first = len(out)
				}
			}
			switch {
			case isDigit(r[j]): // $1
				n, k := readInt(r, j)
				mark(n)
				i = k
			case r[j] == '{':
				k := j + 1
				if k < len(r) && isDigit(r[k]) {
					n, e := readInt(r, k)
					mark(n)
					switch {
					case e < len(r) && r[e] == ':': // ${1:default}
						i = walk(e+1, '}') + 1
					case e < len(r) && r[e] == '|': // ${1|a,b|}
						end := e + 1
						for end < len(r) && r[end] != '|' && r[end] != ',' {
							out = append(out, r[end])
							end++
						}
						for end < len(r) && r[end] != '}' {
							end++
						}
						i = end + 1
					default: // ${1}
						for e < len(r) && r[e] != '}' {
							e++
						}
						i = e + 1
					}
				} else { // ${VAR} or ${VAR:default}
					for k < len(r) && (isWord(r[k])) {
						k++
					}
					if k < len(r) && r[k] == ':' {
						i = walk(k+1, '}') + 1
					} else {
						for k < len(r) && r[k] != '}' {
							k++
						}
						i = k + 1
					}
				}
			case isWord(r[j]): // $VAR
				k := j
				for k < len(r) && isWord(r[k]) {
					k++
				}
				i = k
			default:
				out = append(out, c)
				i++
			}
		}
		return i
	}
	walk(0, 0)
	cur := len(out)
	if first >= 0 {
		cur = first
	} else if zero >= 0 {
		cur = zero
	}
	return string(out), cur
}

func isDigit(c rune) bool { return c >= '0' && c <= '9' }
func isWord(c rune) bool  { return c == '_' || isDigit(c) || (c|0x20 >= 'a' && c|0x20 <= 'z') }

func readInt(r []rune, i int) (int, int) {
	n := 0
	for i < len(r) && isDigit(r[i]) {
		n = n*10 + int(r[i]-'0')
		i++
	}
	return n, i
}
