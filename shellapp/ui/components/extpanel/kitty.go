package extpanel

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strings"
	"time"
)

// Sharp icons through the kitty graphics protocol (kitty, Ghostty).
//
// Each icon is sent to the terminal once, as a PNG with a virtual placement
// sized in cells (a=T, U=1), and then drawn with Unicode placeholders: the
// character U+10EEEE, with combining marks naming the row and column of the
// image the cell shows, in a foreground colour that encodes the image id.
// To the layout these are ordinary one-cell characters, so borders, padding,
// scrolling and the renderer's line diff all keep working; the terminal
// swaps in the picture at full resolution.
//
// The upload rides at the start of a frame (an APC sequence is zero width)
// for a second after it is first needed, so a frame the renderer drops never
// loses it; the renderer's line diff stops it being written twice.
// HITTABLE_IMAGES=blocks forces the half-block fallback, =kitty forces this.

type graphicsMode int

const (
	gBlocks graphicsMode = iota
	gKitty
)

func detectGraphics() graphicsMode {
	switch strings.ToLower(os.Getenv("HITTABLE_IMAGES")) {
	case "kitty":
		return gKitty
	case "blocks", "off", "none":
		return gBlocks
	}
	// tmux and screen swallow APC sequences unless passthrough is set up.
	if os.Getenv("TMUX") != "" || strings.HasPrefix(os.Getenv("TERM"), "screen") {
		return gBlocks
	}
	if os.Getenv("TERM_PROGRAM") == "ghostty" || os.Getenv("GHOSTTY_RESOURCES_DIR") != "" ||
		os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("TERM") == "xterm-kitty" {
		return gKitty
	}
	return gBlocks
}

func sessionSalt() uint32 {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return binary.LittleEndian.Uint32(b[:])
}

type kimg struct {
	id        uint32 // 24-bit, carried in the placeholder's foreground colour
	chunks    []string
	sentEpoch int
	until     time.Time
}

// placeholder row/column marks, from kitty's rowcolumn-diacritics table.
var diacritics = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F,
	0x0346, 0x034A, 0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357,
	0x035B, 0x0363, 0x0364, 0x0365, 0x0366, 0x0367, 0x0368, 0x0369,
}

const placeholder = '\U0010EEEE'

// kittyIcon returns placeholder lines for img at cols×rows, registering the
// upload on first use. Called with ic.mu unlocked.
func (ic *icons) kittyIcon(key string, img image.Image, cols, rows int, bg string) []string {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	k := ic.kitty[key]
	if k == nil {
		ic.next++
		id := (ic.salt + ic.next*7919) & 0xFFFFFF
		if id == 0 {
			id = 1
		}
		k = &kimg{id: id, chunks: uploadChunks(id, img, cols, rows)}
		ic.kitty[key] = k
	}
	ic.frame = append(ic.frame, k)
	r, g, b := (k.id>>16)&0xFF, (k.id>>8)&0xFF, k.id&0xFF
	fg := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
	bgSeq := ""
	if bg != "" {
		br, bgg, bb := parseHex(bg)
		bgSeq = fmt.Sprintf("\x1b[48;2;%d;%d;%dm", int(br*255+0.5), int(bgg*255+0.5), int(bb*255+0.5))
	}
	out := make([]string, rows)
	for y := 0; y < rows && y < len(diacritics); y++ {
		var sb strings.Builder
		sb.WriteString(fg + bgSeq)
		for x := 0; x < cols && x < len(diacritics); x++ {
			sb.WriteRune(placeholder)
			sb.WriteRune(diacritics[y])
			sb.WriteRune(diacritics[x])
		}
		sb.WriteString("\x1b[39;49m")
		out[y] = sb.String()
	}
	return out
}

// uploadChunks encodes img as PNG (shrunk to at most 128 px; the terminal
// scales it to the cells) and splits the transmission into the 4 KB
// chunks the protocol allows. q=2 keeps the terminal from answering, which
// would otherwise arrive as keystrokes.
func uploadChunks(id uint32, img image.Image, cols, rows int) []string {
	var buf bytes.Buffer
	_ = png.Encode(&buf, shrink(img, 128))
	data := base64.StdEncoding.EncodeToString(buf.Bytes())
	var out []string
	for i := 0; i < len(data); i += 4096 {
		end := min(i+4096, len(data))
		more := 0
		if end < len(data) {
			more = 1
		}
		if i == 0 {
			out = append(out, fmt.Sprintf("\x1b_Ga=T,U=1,f=100,t=d,i=%d,c=%d,r=%d,q=2,m=%d;%s\x1b\\", id, cols, rows, more, data[i:end]))
		} else {
			out = append(out, fmt.Sprintf("\x1b_Gm=%d;%s\x1b\\", more, data[i:end]))
		}
	}
	return out
}

// shrink box-filters img down to at most n pixels on its longer side.
func shrink(img image.Image, n int) image.Image {
	b := img.Bounds()
	if b.Dx() <= n && b.Dy() <= n {
		return img
	}
	w, h := n, n
	if b.Dx() > b.Dy() {
		h = max(n*b.Dy()/b.Dx(), 1)
	} else {
		w = max(n*b.Dx()/b.Dy(), 1)
	}
	px := sample(img, w, h)
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := px[y][x]
			if c.a <= 0 {
				continue
			}
			out.SetNRGBA(x, y, color.NRGBA{uint8(c.r / c.a * 255), uint8(c.g / c.a * 255), uint8(c.b / c.a * 255), uint8(c.a * 255)})
		}
	}
	return out
}

// uploads returns the transmissions the frame just drawn needs (each image
// for a second after it first shows in this epoch) and resets the frame.
func (ic *icons) uploads() string {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	now := time.Now()
	var sb strings.Builder
	seen := map[uint32]bool{}
	for _, k := range ic.frame {
		if seen[k.id] {
			continue
		}
		seen[k.id] = true
		if k.sentEpoch != ic.epoch {
			k.sentEpoch, k.until = ic.epoch, now.Add(time.Second)
		}
		if now.Before(k.until) {
			for _, c := range k.chunks {
				sb.WriteString(c)
			}
		}
	}
	ic.frame = ic.frame[:0]
	return sb.String()
}

// Retransmit sends every image again the next time it is drawn, for when
// the terminal may have dropped them (a resize repaints the screen).
func (p *Panel) Retransmit() {
	p.icons.mu.Lock()
	p.icons.epoch++
	p.icons.mu.Unlock()
}

// DeleteImages frees the terminal memory the icons use; call after the
// program has stopped drawing.
func (p *Panel) DeleteImages(w io.Writer) {
	p.icons.mu.Lock()
	defer p.icons.mu.Unlock()
	for _, k := range p.icons.kitty {
		fmt.Fprintf(w, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", k.id)
	}
}
