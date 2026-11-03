package extensions

import (
	"strconv"
	"strings"
)

// semver is major.minor.patch; pre-release tags are ignored, as VS Code's own
// engine check ignores them.
type semver [3]int

func parseSemver(s string) (semver, bool) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "v"))
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	var v semver
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return v, false
	}
	for i, p := range parts {
		if p == "x" || p == "*" {
			v[i] = -1
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	for i := len(parts); i < 3; i++ {
		v[i] = -1
	}
	return v, true
}

func (a semver) cmp(b semver) int {
	for i := 0; i < 3; i++ {
		x, y := max(a[i], 0), max(b[i], 0)
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// EngineAllows reports whether an engines.vscode range accepts version.
// It covers the forms extensions actually publish: "*", "^1.80.0",
// ">=1.80.0", "~1.80.0", "1.80.x" and "1.80.0". An unparseable range is
// allowed, matching VS Code, which only refuses what it can tell is too new.
func EngineAllows(rng, version string) bool {
	rng = strings.TrimSpace(rng)
	v, ok := parseSemver(version)
	if !ok || rng == "" || rng == "*" {
		return true
	}
	for _, alt := range strings.Split(rng, "||") {
		if allowsAll(strings.Fields(alt), v) {
			return true
		}
	}
	return false
}

func allowsAll(terms []string, v semver) bool {
	for _, t := range terms {
		if !allows(t, v) {
			return false
		}
	}
	return true
}

func allows(t string, v semver) bool {
	op := ""
	for _, p := range []string{">=", "<=", ">", "<", "^", "~", "="} {
		if strings.HasPrefix(t, p) {
			op, t = p, t[len(p):]
			break
		}
	}
	b, ok := parseSemver(t)
	if !ok {
		return true
	}
	c := v.cmp(b)
	switch op {
	case ">=":
		return c >= 0
	case ">":
		return c > 0
	case "<=":
		return c <= 0
	case "<":
		return c < 0
	case "^":
		// ^1.80.0 means 1.80.0 up to (not including) 2.0.0. VS Code treats
		// ^0.x the same way rather than npm's tighter rule.
		return c >= 0 && v[0] == b[0]
	case "~":
		return c >= 0 && v[0] == b[0] && v[1] == b[1]
	}
	for i := 0; i < 3; i++ {
		if b[i] >= 0 && v[i] != b[i] {
			return false
		}
	}
	return true
}

// newer reports whether version a sorts after b.
func newer(a, b string) bool {
	x, _ := parseSemver(a)
	y, _ := parseSemver(b)
	return x.cmp(y) > 0
}
