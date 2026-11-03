package extensions

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
