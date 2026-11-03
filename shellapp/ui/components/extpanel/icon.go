package extpanel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"image"
	_ "image/gif" // icon decoders
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/hittable/shellapp/internal/extensions"
	"github.com/hittable/shellapp/internal/hithome"
)

// Extension icons are drawn with half blocks: each cell is two pixels, the
// top one the foreground of "▀" and the bottom one its background, so a
// 6×3-cell icon is a 6×6-pixel picture. Images are fetched once, kept in
// ~/.hittable/extdata/icons, decoded once and rendered once per size and
// background, so drawing a frame costs a map lookup.

type icons struct {
	mu       sync.Mutex
	img      map[string]image.Image
	failed   map[string]bool
	loading  map[string]bool
	rendered map[string][]string
	slots    chan struct{} // at most a few downloads at once

	// kitty graphics (see kitty.go)
	mode  graphicsMode
	kitty map[string]*kimg // by icon key (id|cols|rows)
	frame []*kimg          // images the frame being drawn shows
	epoch int              // bumped when the terminal may have dropped images
	salt  uint32
	next  uint32
}

func newIcons() *icons {
	return &icons{img: map[string]image.Image{}, failed: map[string]bool{}, loading: map[string]bool{},
		rendered: map[string][]string{}, slots: make(chan struct{}, 4),
		mode: detectGraphics(), kitty: map[string]*kimg{}, salt: sessionSalt(), epoch: 1}
}

// panelBg is what a transparent pixel is blended against when the cell
// background is the terminal's own (unknown) colour.
const panelBg = "#282a36"

// icon returns rows lines, each cols cells wide, for an extension. bg is
// the row's background ("" = the terminal's). Until the picture has loaded
// (or when there is none) it is the initials tile.
func (p *Panel) icon(id, src string, cols, rows int, bg string) []string {
	ic := p.icons
	ic.mu.Lock()
	img, have := ic.img[id]
	failed, busy := ic.failed[id], ic.loading[id]
	key := fmt.Sprintf("%s|%d|%d|%s", id, cols, rows, bg)
	if have {
		if r, ok := ic.rendered[key]; ok {
			ic.mu.Unlock()
			return r
		}
	}
	ic.mu.Unlock()
	if !have {
		if !failed && !busy && src != "" {
			p.loadIcon(id, src)
		}
		return tileLines(id, cols, rows, bg)
	}
	if ic.mode == gKitty {
		return ic.kittyIcon(key, img, cols, rows, bg)
	}
	out := halfBlocks(img, cols, rows, bg)
	ic.mu.Lock()
	ic.rendered[key] = out
	ic.mu.Unlock()
	return out
}

// iconSource is a local file path (installed) or a URL (registry).
func (p *Panel) iconSource(e entry) string {
	if ie, ok := p.installedEntry(e.id); ok && ie.Manifest.Icon != "" {
		return filepath.Join(ie.Path(), filepath.FromSlash(ie.Manifest.Icon))
	}
	if pg := p.pages[e.id]; pg != nil && pg.detail != nil && pg.detail.Icon != "" {
		return pg.detail.Icon
	}
	return e.icon
}

func (p *Panel) loadIcon(id, src string) {
	ic := p.icons
	ic.mu.Lock()
	ic.loading[id] = true
	ic.mu.Unlock()
	go func() {
		ic.slots <- struct{}{}
		defer func() { <-ic.slots }()
		img, err := fetchIcon(src)
		ic.mu.Lock()
		delete(ic.loading, id)
		if err != nil {
			ic.failed[id] = true
		} else {
			ic.img[id] = img
		}
		ic.mu.Unlock()
		if err == nil {
			p.deliver(func(*Panel) {}) // redraw with the picture
		}
	}()
}

func fetchIcon(src string) (image.Image, error) {
	var data []byte
	var err error
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		data, err = os.ReadFile(src)
	} else {
		sum := sha256.Sum256([]byte(src))
		cache := filepath.Join(hithome.ExtData(), "icons", hex.EncodeToString(sum[:12]))
		if data, err = os.ReadFile(cache); err != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if data, err = extensions.FetchBytes(ctx, src); err == nil {
				_ = os.MkdirAll(filepath.Dir(cache), 0o755)
				_ = os.WriteFile(cache, data, 0o644)
			}
		}
	}
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data)) // SVG icons fail here and keep the tile
	return img, err
}

type rgba struct{ r, g, b, a float64 }

// sample averages the source pixels under each target pixel (a box
// filter), so a 128×128 icon shrinks to 6×6 without aliasing.
func sample(img image.Image, w, h int) [][]rgba {
	b := img.Bounds()
	out := make([][]rgba, h)
	for y := 0; y < h; y++ {
		out[y] = make([]rgba, w)
		y0 := b.Min.Y + y*b.Dy()/h
		y1 := max(b.Min.Y+(y+1)*b.Dy()/h, y0+1)
		for x := 0; x < w; x++ {
			x0 := b.Min.X + x*b.Dx()/w
			x1 := max(b.Min.X+(x+1)*b.Dx()/w, x0+1)
			var s rgba
			n := 0.0
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					r, g, bb, a := img.At(sx, sy).RGBA() // premultiplied, 16-bit
					s.r += float64(r)
					s.g += float64(g)
					s.b += float64(bb)
					s.a += float64(a)
					n++
				}
			}
			out[y][x] = rgba{s.r / n / 65535, s.g / n / 65535, s.b / n / 65535, s.a / n / 65535}
		}
	}
	return out
}

func parseHex(h string) (float64, float64, float64) {
	var r, g, b int
	fmt.Sscanf(strings.TrimPrefix(h, "#"), "%02x%02x%02x", &r, &g, &b)
	return float64(r) / 255, float64(g) / 255, float64(b) / 255
}

// over composites a premultiplied pixel on a background colour.
func over(c rgba, bg string) string {
	br, bgc, bb := parseHex(bg)
	r := c.r + br*(1-c.a)
	g := c.g + bgc*(1-c.a)
	b := c.b + bb*(1-c.a)
	return fmt.Sprintf("#%02x%02x%02x", int(r*255+0.5), int(g*255+0.5), int(b*255+0.5))
}

func halfBlocks(img image.Image, cols, rows int, bg string) []string {
	px := sample(img, cols, rows*2)
	blend := bg
	if blend == "" {
		blend = panelBg
	}
	out := make([]string, rows)
	for r := 0; r < rows; r++ {
		var sb strings.Builder
		for c := 0; c < cols; c++ {
			top, bot := px[2*r][c], px[2*r+1][c]
			st := lipgloss.NewStyle()
			switch {
			case bg == "" && top.a < 0.5 && bot.a < 0.5:
				sb.WriteString(" ")
				continue
			case bg == "" && bot.a < 0.5:
				sb.WriteString(st.Foreground(lipgloss.Color(over(top, blend))).Render("▀"))
				continue
			case bg == "" && top.a < 0.5:
				sb.WriteString(st.Foreground(lipgloss.Color(over(bot, blend))).Render("▄"))
				continue
			}
			sb.WriteString(st.Foreground(lipgloss.Color(over(top, blend))).Background(lipgloss.Color(over(bot, blend))).Render("▀"))
		}
		out[r] = sb.String()
	}
	return out
}

// tileLines is the stand-in icon: the extension's initials on a colour
// picked from its id, the same size as the picture it replaces.
func tileLines(id string, cols, rows int, bg string) []string {
	h := fnv.New32a()
	h.Write([]byte(id))
	colors := []string{"#6272a4", "#bd93f9", "#ff79c6", "#8be9fd", "#50fa7b", "#ffb86c", "#ff5555", "#f1fa8c"}
	c := colors[h.Sum32()%uint32(len(colors))]
	name := id
	if i := strings.IndexByte(id, '.'); i >= 0 {
		name = id[i+1:]
	}
	name = strings.TrimPrefix(strings.TrimPrefix(name, "vscode-"), "theme-")
	letters := strings.ToUpper(string([]rune(name)[:min(2, len([]rune(name)))]))
	st := lipgloss.NewStyle().Background(lipgloss.Color(c)).Foreground(lipgloss.Color("#282a36")).Bold(true)
	out := make([]string, rows)
	for r := range out {
		text := strings.Repeat(" ", cols)
		if r == rows/2 {
			pad := (cols - len([]rune(letters))) / 2
			text = strings.Repeat(" ", pad) + letters + strings.Repeat(" ", cols-pad-len([]rune(letters)))
		}
		out[r] = st.Render(text)
	}
	_ = bg
	return out
}
