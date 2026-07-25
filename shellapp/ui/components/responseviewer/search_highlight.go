package responseviewer

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/hittable/shellapp/ui/theme"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type matchPos struct {
	lineIdx  int
	startCol int
	length   int
}

func (r *ResponseViewer) findAllMatches(rawLines []string) []matchPos {
	var matches []matchPos
	if r.SearchQuery == "" {
		return matches
	}
	queryRunes := []rune(strings.ToLower(r.SearchQuery))
	queryLen := len(queryRunes)
	for lineIdx, line := range rawLines {
		runes := []rune(strings.ToLower(line))
		offset := 0
		for {
			idx := runeSearch(runes[offset:], queryRunes)
			if idx < 0 {
				break
			}
			matches = append(matches, matchPos{
				lineIdx:  lineIdx,
				startCol: offset + idx,
				length:   queryLen,
			})
			offset += idx + 1
		}
	}
	return matches
}

func runeSearch(haystack, needle []rune) int {
	if len(needle) == 0 {
		return -1
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func (r *ResponseViewer) applySearchHighlight(ansiLine string, rawLine string, lineMatches []matchPos) string {
	if len(lineMatches) == 0 {
		return ansiLine
	}

	visualMatches := make([]struct{ start, end int }, 0, len(lineMatches))
	for _, m := range lineMatches {
		visualMatches = append(visualMatches, struct{ start, end int }{m.startCol, m.startCol + m.length})
	}

	result := ""
	ansiIdx := 0
	visualCol := 0
	matchIdx := 0

	for ansiIdx < len(ansiLine) {
		if ansiIdx < len(ansiLine) && ansiLine[ansiIdx] == '\x1b' {
			end := ansiIdx + 1
			for end < len(ansiLine) && ansiLine[end] != 'm' {
				end++
			}
			if end < len(ansiLine) {
				end++
			}
			result += ansiLine[ansiIdx:end]
			ansiIdx = end
			continue
		}

		for matchIdx < len(visualMatches) && visualMatches[matchIdx].end <= visualCol {
			matchIdx++
		}

		if matchIdx < len(visualMatches) && visualCol >= visualMatches[matchIdx].start && visualCol < visualMatches[matchIdx].end {
			inMatch := true
			chars := ""
			for inMatch && ansiIdx < len(ansiLine) && ansiLine[ansiIdx] != '\x1b' {
				r, size := utf8.DecodeRuneInString(ansiLine[ansiIdx:])
				chars += string(r)
				ansiIdx += size
				visualCol++
				if visualCol >= visualMatches[matchIdx].end {
					inMatch = false
				}
			}
			result += theme.SearchHighlightStyle.Render(chars)
		} else {
			if ansiIdx < len(ansiLine) {
				r, size := utf8.DecodeRuneInString(ansiLine[ansiIdx:])
				result += string(r)
				ansiIdx += size
				visualCol++
			}
		}
	}

	return result
}
