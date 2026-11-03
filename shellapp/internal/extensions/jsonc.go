package extensions

import (
	"bytes"
	"encoding/json"
	"regexp"
)

// StripJSONC turns JSON-with-comments (VS Code's settings, themes, snippets)
// into plain JSON: // and /* */ comments become spaces and trailing commas
// before } or ] are dropped. Strings are copied untouched, escapes included.
func StripJSONC(in []byte) []byte {
	out := make([]byte, 0, len(in))
	// Skip a UTF-8 byte-order mark; encoding/json rejects it.
	if len(in) >= 3 && in[0] == 0xEF && in[1] == 0xBB && in[2] == 0xBF {
		in = in[3:]
	}
	for i := 0; i < len(in); i++ {
		c := in[i]
		switch {
		case c == '"':
			j := i + 1
			for j < len(in) && in[j] != '"' {
				if in[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(in) {
				j = len(in) - 1
			}
			out = append(out, in[i:j+1]...)
			i = j
		case c == '/' && i+1 < len(in) && in[i+1] == '/':
			for i < len(in) && in[i] != '\n' {
				i++
			}
			if i < len(in) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(in) && in[i+1] == '*':
			i += 2
			for i+1 < len(in) && !(in[i] == '*' && in[i+1] == '/') {
				if in[i] == '\n' {
					out = append(out, '\n')
				}
				i++
			}
			i++
		case c == ',':
			// Drop the comma when only whitespace/comments separate it from a closer.
			j := i + 1
			for j < len(in) {
				if in[j] == ' ' || in[j] == '\t' || in[j] == '\n' || in[j] == '\r' {
					j++
					continue
				}
				if in[j] == '/' && j+1 < len(in) && in[j+1] == '/' {
					for j < len(in) && in[j] != '\n' {
						j++
					}
					continue
				}
				if in[j] == '/' && j+1 < len(in) && in[j+1] == '*' {
					j += 2
					for j+1 < len(in) && !(in[j] == '*' && in[j+1] == '/') {
						j++
					}
					j += 2
					continue
				}
				break
			}
			if j < len(in) && (in[j] == '}' || in[j] == ']') {
				continue
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}

// SetJSONCValue sets (or with value nil removes) one top-level key in a
// JSON-with-comments document by editing its text, so comments and layout
// survive. It handles primitive values (strings, numbers, booleans, null);
// for anything else, or a key whose current value is an object or array,
// it reports false and the caller rewrites the file.
func SetJSONCValue(src []byte, key string, value any) ([]byte, bool) {
	var lit string
	if value != nil {
		switch value.(type) {
		case string, bool, float64, float32, int, int64:
		default:
			return nil, false
		}
		b, err := json.Marshal(value)
		if err != nil {
			return nil, false
		}
		lit = string(b)
	}
	qk, _ := json.Marshal(key)
	re := regexp.MustCompile(`(?m)^([ \t]*)` + regexp.QuoteMeta(string(qk)) + `[ \t]*:[ \t]*` +
		`("(?:[^"\\]|\\.)*"|-?[0-9][0-9.eE+-]*|true|false|null)([ \t]*,?)`)
	if loc := re.FindSubmatchIndex(src); loc != nil {
		if value == nil {
			// Drop the whole line, newline included.
			start, end := loc[0], loc[1]
			if end < len(src) && src[end] == '\n' {
				end++
			}
			return append(append([]byte{}, src[:start]...), src[end:]...), true
		}
		out := append([]byte{}, src[:loc[4]]...)
		out = append(out, lit...)
		return append(out, src[loc[5]:]...), true
	}
	if bytes.Contains(src, qk) {
		return nil, false // present with an object/array value, or nested
	}
	if value == nil {
		return src, true
	}
	open := firstBrace(src)
	if open < 0 {
		return []byte("{\n  " + string(qk) + ": " + lit + "\n}\n"), true
	}
	ins := "\n  " + string(qk) + ": " + lit + ","
	out := append([]byte{}, src[:open+1]...)
	out = append(out, ins...)
	return append(out, src[open+1:]...), true
}

// firstBrace is the offset of the first '{' outside comments, or -1.
func firstBrace(src []byte) int {
	for i := 0; i < len(src); i++ {
		switch {
		case src[i] == '{':
			return i
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '*':
			end := bytes.Index(src[i+2:], []byte("*/"))
			if end < 0 {
				return -1
			}
			i += end + 3
		}
	}
	return -1
}
