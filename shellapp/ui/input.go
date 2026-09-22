package ui

import (
	"io"
	"os"
)

// Bubble Tea reads input in 256-byte chunks. Its parser carries a partial
// *key* sequence over to the next read, but its mouse branch does not: an
// incomplete `ESC [ < b ; x ; y M` falls through to key parsing, which emits
// the leftover as plain runes. A fast scroll fills the read buffer mid-report
// and the tail gets typed into whatever has focus — literal "[<66;51;23M" in
// the editor or the shell, and an undo history flooded with it.
//
// Holding the partial report back until the rest arrives is enough; the
// remaining bytes are always in the same input burst.

// MouseSafeInput wraps the terminal so Bubble Tea never sees half a mouse
// report. It embeds the file so it still satisfies the term.File /
// cancelreader.File assertions that raw mode and cancellation depend on.
func MouseSafeInput(f *os.File) *mouseSafeFile { //nolint:revive // must keep File's methods
	return &mouseSafeFile{File: f}
}

type mouseSafeFile struct {
	*os.File
	held []byte
}

func (t *mouseSafeFile) Read(p []byte) (int, error) {
	return readMouseSafe(t.File, &t.held, p)
}

// MouseSafeReader is MouseSafeInput for a plain reader (the e2e harness feeds
// the program through a pipe, which is not an *os.File).
type MouseSafeReader struct {
	R    io.Reader
	held []byte
}

func (t *MouseSafeReader) Read(p []byte) (int, error) { return readMouseSafe(t.R, &t.held, p) }

func readMouseSafe(r io.Reader, held *[]byte, p []byte) (int, error) {
	// Whatever was held back goes first; the bytes completing it are right
	// behind it in the OS buffer. held is only ever a few bytes, so it always
	// fits in Bubble Tea's 256-byte buffer.
	n := copy(p, *held)
	*held = (*held)[:0]

	m, err := r.Read(p[n:])
	n += m
	if cut := partialMouseTail(p[:n]); cut > 0 {
		*held = append(*held, p[n-cut:n]...)
		n -= cut
	}
	return n, err
}

// partialMouseTail returns the length of an incomplete mouse report at the end
// of b, or 0 if b ends on a message boundary. A bare trailing "ESC" or "ESC ["
// is left alone: Bubble Tea already carries those over itself, and holding
// them would strand a real Escape or arrow key.
func partialMouseTail(b []byte) int {
	const maxReport = 24 // no mouse report is longer than this
	start := max(len(b)-maxReport, 0)
	for i := len(b) - 1; i >= start; i-- {
		if b[i] != 0x1b || i+1 >= len(b) || b[i+1] != '[' || i+2 >= len(b) {
			continue
		}
		tail := b[i:]
		switch tail[2] {
		case 'M': // X10: ESC [ M Cb Cx Cy
			if len(tail) < 6 {
				return len(tail)
			}
		case '<': // SGR: ESC [ < b ; x ; y (M|m)
			for _, c := range tail[3:] {
				if c == 'M' || c == 'm' {
					return 0 // complete
				}
				if c != ';' && (c < '0' || c > '9') {
					return 0 // not a mouse report after all
				}
			}
			return len(tail)
		}
		return 0
	}
	return 0
}
