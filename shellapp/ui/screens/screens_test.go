package screens

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/hittable/shellapp/ui/components/requesteditor"
)

func TestTabClickChangesTab(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(80, 24)

	hitFile := filepath.Join(hittableDir, "test.hit")
	os.WriteFile(hitFile, []byte(`{"method":"GET","url":"http://test.com","headers":{},"params":{},"body":"","response":null}`), 0o644)
	m.openFileRaw(hitFile)

	if m.ActiveTab != requesteditor.TabParams {
		t.Fatalf("expected TabParams, got %d", m.ActiveTab)
	}

	_, cmd := m.Update(tabClickMsg{tab: requesteditor.TabHeaders})
	if cmd != nil {
		cmd()
	}

	if m.ActiveTab != requesteditor.TabHeaders {
		t.Errorf("expected TabHeaders after click, got %d", m.ActiveTab)
	}
}

func TestToggleViewMsg(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(80, 24)

	hitFile := filepath.Join(hittableDir, "test.hit")
	os.WriteFile(hitFile, []byte(`{"method":"GET","url":"http://test.com","headers":{},"params":{},"body":"","response":null}`), 0o644)
	m.openFileRaw(hitFile)

	if m.ViewMode != ViewRunner {
		t.Fatalf("expected ViewRunner, got %d", m.ViewMode)
	}

	_, _ = m.Update(toggleViewMsg{mode: ViewText})
	if m.ViewMode != ViewText {
		t.Errorf("expected ViewText after toggle, got %d", m.ViewMode)
	}

	_, _ = m.Update(toggleViewMsg{mode: ViewRunner})
	if m.ViewMode != ViewRunner {
		t.Errorf("expected ViewRunner after second toggle, got %d", m.ViewMode)
	}
}

func TestEscClosesFile(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(80, 24)

	hitFile := filepath.Join(hittableDir, "test.hit")
	os.WriteFile(hitFile, []byte(`{"method":"GET","url":"http://test.com","headers":{},"params":{},"body":"","response":null}`), 0o644)
	m.openFileRaw(hitFile)

	if m.ActiveFile == "" {
		t.Fatal("expected file to be open")
	}

	m.ExplorerFocused = false
	m.Focus = FocusURLBar

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})

	if m.ActiveFile != "" {
		t.Errorf("expected file closed after Esc, still open: %s", m.ActiveFile)
	}
	if !m.ExplorerFocused {
		t.Error("expected explorer focused after Esc")
	}
}

func TestMethodSelectChangesMethod(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(80, 24)

	hitFile := filepath.Join(hittableDir, "test.hit")
	os.WriteFile(hitFile, []byte(`{"method":"GET","url":"http://test.com","headers":{},"params":{},"body":"","response":null}`), 0o644)
	m.openFileRaw(hitFile)

	if m.URLBar.Method != "GET" {
		t.Fatalf("expected initial method GET, got %s", m.URLBar.Method)
	}

	_, cmd := m.Update(methodSelectMsg{idx: 1, method: "POST"})
	if cmd != nil {
		cmd()
	}

	if m.URLBar.Method != "POST" {
		t.Errorf("expected method POST after select, got %s", m.URLBar.Method)
	}
	if m.URLBar.DropdownOpen {
		t.Error("expected dropdown closed after select")
	}
	if m.Focus != FocusURLBar {
		t.Errorf("expected FocusURLBar after select, got %d", m.Focus)
	}
}

func TestSearchIconClick(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(80, 24)

	hitFile := filepath.Join(hittableDir, "test.hit")
	os.WriteFile(hitFile, []byte(`{"method":"GET","url":"http://test.com","headers":{},"params":{},"body":"","response":null}`), 0o644)
	m.openFileRaw(hitFile)

	m.Response.SetResponse(200, "OK", 100, 50, true, `{"key":"value"}`)

	if m.Response.SearchOpen {
		t.Fatal("search should not be open initially")
	}

	_, cmd := m.Update(searchIconClickMsg{})
	if cmd != nil {
		cmd()
	}

	if !m.Response.SearchOpen {
		t.Error("expected search to open after icon click")
	}
	if m.Focus != FocusResponse {
		t.Errorf("expected FocusResponse, got %d", m.Focus)
	}
}

func TestDebounceSaveMsg(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(80, 24)

	hitFile := filepath.Join(hittableDir, "test.hit")
	os.WriteFile(hitFile, []byte(`{"method":"GET","url":"http://test.com","headers":{},"params":{},"body":"","response":null}`), 0o644)
	m.openFileRaw(hitFile)

	_, _ = m.Update(debounceSaveMsg{})

	if m.StatusBar == "" {
		t.Log("debounce save executed without error")
	}
}

func TestParamsJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(80, 24)

	hitFile := filepath.Join(hittableDir, "test.hit")
	os.WriteFile(hitFile, []byte(`{"method":"GET","url":"http://test.com","headers":{},"params":{},"body":"","response":null}`), 0o644)
	m.openFileRaw(hitFile)

	m.Params.SetContent(map[string]string{"key": "value"})
	content := m.Params.GetContent()
	if content["key"] != "value" {
		t.Errorf("expected key=value, got %v", content)
	}

	m.Params.SetContent(map[string]string{})
	if !m.Params.InvalidJSON {
		t.Log("empty JSON object is valid")
	}
}

func TestHeadersJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(80, 24)

	hitFile := filepath.Join(hittableDir, "test.hit")
	os.WriteFile(hitFile, []byte(`{"method":"GET","url":"http://test.com","headers":{},"params":{},"body":"","response":null}`), 0o644)
	m.openFileRaw(hitFile)

	m.Headers.SetContent(map[string]string{"Content-Type": "application/json"})
	content := m.Headers.GetContent()
	if content["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type header, got %v", content)
	}
}
