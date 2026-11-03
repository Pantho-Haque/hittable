package extpanel

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestRealOpenVSX renders the panel from the real registry (network), with
// real icons and a real README. HITTABLE_EXT_E2E=1 runs it.
func TestRealOpenVSX(t *testing.T) {
	if os.Getenv("HITTABLE_EXT_E2E") == "" {
		t.Skip("set HITTABLE_EXT_E2E=1 to render the panel from open-vsx.org")
	}
	p, ch := newPanel(t)
	p.SetSize(150, 44)
	p.Open()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case m := <-ch:
			if pm, ok := m.(Msg); ok {
				p.Apply(pm)
			}
		case <-time.After(200 * time.Millisecond):
		}
		pg := p.pages[p.sel]
		p.icons.mu.Lock()
		icons := len(p.icons.img)
		p.icons.mu.Unlock()
		if len(p.list) > 5 && pg != nil && pg.readmeDone && icons >= 5 {
			break
		}
		p.View() // asks for the icons of visible rows
	}
	p.icons.mu.Lock()
	n, failed := len(p.icons.img), len(p.icons.failed)
	p.icons.mu.Unlock()
	if n == 0 {
		t.Fatalf("no icon loaded (%d failed)", failed)
	}
	v := p.View()
	if !strings.Contains(v, "▀") {
		t.Error("no half-block icon pixels in the frame")
	}
	t.Logf("icons loaded: %d, failed: %d\n%s", n, failed, plain(v))
}
