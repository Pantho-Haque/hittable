package exthost

import "unicode/utf16"

// The VS Code API counts characters in UTF-16 code units (JavaScript string
// indexes); the editor counts runes. These convert a column on one line.

// ToUTF16 converts a rune column in line to a UTF-16 column.
func ToUTF16(line string, runeCol int) int {
	n, i := 0, 0
	for _, r := range line {
		if i >= runeCol {
			break
		}
		n += utf16.RuneLen(r)
		i++
	}
	return n
}

// FromUTF16 converts a UTF-16 column in line to a rune column. A column in
// the middle of a surrogate pair maps to the rune that contains it.
func FromUTF16(line string, col int) int {
	n, i := 0, 0
	for _, r := range line {
		if n >= col {
			return i
		}
		n += utf16.RuneLen(r)
		i++
	}
	return i
}
