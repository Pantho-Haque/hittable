package screens

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/extensions"
	"github.com/hittable/shellapp/internal/exthost"
	"github.com/hittable/shellapp/internal/hithome"
	"github.com/hittable/shellapp/internal/noderuntime"
	"github.com/hittable/shellapp/ui/components/texteditor"
)

// VS Code extensions in the terminal app. The extension host (Node) runs in
// the background; this file keeps it fed with the open documents and turns
// what it asks for into the TUI's own pieces: the palette for pickers and
// input boxes, the footer for status items and progress, the editor for
// diagnostics, completions and edits, and the browser for webview panels.

// Messages from the host's goroutines into the UI loop.
type (
	extNoteMsg struct {
		gen    int
		method string
		params json.RawMessage
	}
	extReqMsg struct {
		gen int
		req *exthost.Request
	}
	extExitMsg struct {
		gen    int
		err    error
		stderr string
	}
	extStartedMsg struct {
		gen  int
		host *exthost.Host
		web  *exthost.WebviewServer
		err  error
	}
	// extDoneMsg runs fn on the UI goroutine (the answer to a host call).
	extDoneMsg     struct{ fn func(m *MainScreen) }
	extProgressMsg struct {
		label       string
		line        string // a line of installer output (Node via a package manager)
		done, total int64
	}
	extRestartMsg struct{ gen int }
)

type extStatus struct {
	text, tooltip, command, align string
	args                          []any
	priority                      float64
	visible, err                  bool
}

type extDoc struct {
	version int
	text    string
}

type extDiag struct {
	owner    string
	line     int
	start    int // UTF-16, as sent
	end      int
	endLine  int
	severity int
	message  string
	source   string
}

type extState struct {
	host     *exthost.Host
	web      *exthost.WebviewServer
	gen      int
	starting bool
	stopped  string // why code extensions are not running ("" = running or none)
	crashes  []time.Time

	docs       map[string]*extDoc
	status     map[string]extStatus
	message    string
	messageTTL time.Time
	progress   map[string]string
	outputs    map[string]*strings.Builder
	diags      map[string][]extDiag // by path
	commands   map[string]string    // contributed command id -> title
	keybinds   []extensions.Keybinding
	views      map[string]bool // view id -> is a webview
	webviews   map[string]string
	contexts   map[string]any
	triggers   struct{ Completion, Signature map[string][]string }
	snippets   map[string][]texteditor.ExtItem
	notes      []string
	loadSeq    int
	uiQueue    []func()
	compSeq    int
	installing string
	installPct int
	nodeLine   string            // the Node install's latest step
	nodeErr    string            // why the last Node install failed
	activation map[string]string // extension id -> "activating", "active", "failed: …"
	installErr map[string]string // extension id -> why its last install failed
	settings   exthost.Settings
}

var errExtNotRunning = errors.New("no extension with code is running (install one with alt+p › Extensions: Install)")

// extInit prepares extension state and the editor hooks. Nothing is started
// here; Init returns the command that starts the host.
func (m *MainScreen) extInit() {
	m.Ext = &extState{
		docs: map[string]*extDoc{}, status: map[string]extStatus{}, progress: map[string]string{},
		outputs: map[string]*strings.Builder{}, diags: map[string][]extDiag{}, views: map[string]bool{},
		webviews: map[string]string{}, contexts: map[string]any{}, snippets: map[string][]texteditor.ExtItem{},
		activation: map[string]string{}, installErr: map[string]string{},
	}
	m.extReloadContributions()
	m.extApplyTheme()

	ed := m.TextEd
	ed.ExtComplete = func(path string, row, col int, trigger string) { m.extRequestCompletion(path, row, col, trigger) }
	ed.ExtItems = m.extSnippetItems
	ed.OnExtAccept = m.extAcceptCompletion
	ed.OnTyped = m.extTyped
}

// extReloadContributions re-reads what enabled extensions declare.
func (m *MainScreen) extReloadContributions() {
	x := m.Ext
	x.commands = map[string]string{}
	x.keybinds = nil
	x.snippets = map[string][]texteditor.ExtItem{}
	for _, e := range extensions.Enabled() {
		for _, c := range e.Manifest.Contributes.Commands {
			x.commands[c.Command] = c.CommandTitle(e.NLS)
		}
		x.keybinds = append(x.keybinds, e.Manifest.Contributes.Keybindings...)
		for id, vs := range e.Manifest.Contributes.Views {
			_ = id
			for _, v := range vs {
				x.views[v.ID] = v.Type == "webview"
			}
		}
	}
	extensions.ResetLanguageCache()
	x.settings = loadExtSettings(m.RootDir)
}

func needsHost(exts []extensions.Installed) []extensions.Installed {
	var out []extensions.Installed
	for _, e := range exts {
		if e.Manifest.HasCode() {
			out = append(out, e)
		}
	}
	return out
}

// extStartCmd starts the host when an enabled extension has code, the
// folder is trusted and Node is available; otherwise it records why not.
func (m *MainScreen) extStartCmd() tea.Cmd {
	x := m.Ext
	if x == nil || x.host != nil || x.starting {
		return nil
	}
	all := extensions.Enabled()
	coded := needsHost(all)
	if len(coded) == 0 {
		x.stopped = ""
		return nil
	}
	if !extensions.Trusted(m.RootDir) {
		x.stopped = "untrusted"
		return nil
	}
	node, err := noderuntime.Path()
	if err != nil {
		x.stopped = "node"
		if !errors.Is(err, noderuntime.ErrNotInstalled) && !errors.Is(err, noderuntime.ErrTooOld) {
			m.extNote("error", err.Error())
		}
		return nil
	}
	x.starting = true
	x.stopped = ""
	x.gen++
	gen := x.gen
	root := m.RootDir
	settings := x.settings
	return func() tea.Msg {
		web, err := exthost.StartWebviewServer()
		if err != nil {
			return extStartedMsg{gen: gen, err: err}
		}
		h, err := exthost.Start(context.Background(), exthost.Options{
			Node: node, Root: root, Extensions: all, Settings: settings,
			WebviewBase: web.Base(), MachineID: machineID(),
			OnRequest: func(r *exthost.Request) { send(extReqMsg{gen: gen, req: r}) },
			OnNotify: func(method string, p json.RawMessage) {
				send(extNoteMsg{gen: gen, method: method, params: p})
			},
			OnExit: func(err error, stderr string) { send(extExitMsg{gen: gen, err: err, stderr: stderr}) },
		})
		if err != nil {
			web.Close()
			return extStartedMsg{gen: gen, err: err}
		}
		web.OnMessage = func(handle string, msg json.RawMessage) {
			_ = h.Notify("webview/message", map[string]any{"handle": handle, "message": msg})
		}
		return extStartedMsg{gen: gen, host: h, web: web}
	}
}

func machineID() string {
	host, _ := os.Hostname()
	u, _ := user.Current()
	name := ""
	if u != nil {
		name = u.Username
	}
	sum := sha256.Sum256([]byte("hittable:" + host + ":" + name))
	return hex.EncodeToString(sum[:])
}

// CloseExtensions stops the host and the webview server (on quit).
func (m *MainScreen) CloseExtensions() {
	if m.Ext == nil {
		return
	}
	if m.Ext.host != nil {
		m.Ext.host.Close()
		m.Ext.host = nil
	}
	if m.Ext.web != nil {
		m.Ext.web.Close()
		m.Ext.web = nil
	}
}

// extRestart replaces the host (after an install, uninstall or settings
// change that needs one). Open documents are replayed into the new one.
func (m *MainScreen) extRestart() tea.Cmd {
	x := m.Ext
	old, oldWeb := x.host, x.web
	x.host, x.web, x.starting = nil, nil, false
	x.gen++ // anything the old host still sends is ignored
	x.status = map[string]extStatus{}
	x.activation = map[string]string{}
	x.progress = map[string]string{}
	x.diags = map[string][]extDiag{}
	m.extRefreshMarks("")
	go func() {
		if old != nil {
			old.Close()
		}
		if oldWeb != nil {
			oldWeb.Close()
		}
	}()
	m.extReloadContributions()
	return m.extStartCmd()
}

// extUpdate handles every extension message.
func (m *MainScreen) extUpdate(msg tea.Msg) tea.Cmd {
	x := m.Ext
	switch msg := msg.(type) {
	case extStartedMsg:
		if msg.gen != x.gen {
			if msg.host != nil {
				go msg.host.Close()
			}
			if msg.web != nil {
				msg.web.Close()
			}
			return nil
		}
		x.starting = false
		if msg.err != nil {
			x.stopped = "failed"
			m.extNote("error", "Extensions could not start: "+msg.err.Error())
			return nil
		}
		x.host, x.web = msg.host, msg.web
		m.extReplayDocs()
	case extExitMsg:
		if msg.gen != x.gen || x.host == nil {
			return nil
		}
		x.host, x.starting = nil, false
		if x.web != nil {
			x.web.Close()
			x.web = nil
		}
		now := time.Now()
		recent := x.crashes[:0]
		for _, t := range x.crashes {
			if now.Sub(t) < time.Minute {
				recent = append(recent, t)
			}
		}
		x.crashes = append(recent, now)
		m.extAppendOutput("Extension Host", fmt.Sprintf("\n[host exited: %v]\n%s\n", msg.err, msg.stderr))
		if len(x.crashes) >= 3 {
			x.stopped = "crashed"
			m.extNote("error", "The extension host crashed 3 times in a minute and was stopped. See alt+p › Extensions: Show Log, then Restart Extension Host.")
			return nil
		}
		m.extNote("warning", "The extension host stopped unexpectedly; restarting it.")
		gen := x.gen
		return tea.Tick(time.Second, func(time.Time) tea.Msg { return extRestartMsg{gen: gen} })
	case extRestartMsg:
		if msg.gen == x.gen && x.host == nil {
			return m.extRestart()
		}
	case extNoteMsg:
		if msg.gen == x.gen {
			return m.extNotification(msg.method, msg.params)
		}
	case extReqMsg:
		if msg.gen != x.gen {
			msg.req.Reply(nil, exthost.ErrClosed)
			return nil
		}
		return m.extRequest(msg.req)
	case extDoneMsg:
		msg.fn(m)
	case extProgressMsg:
		if msg.line != "" {
			x.nodeLine = msg.line
			m.extAppendOutput("Node.js install", msg.line+"\n")
			return nil
		}
		if msg.total > 0 {
			x.installPct = int(msg.done * 100 / msg.total)
			x.installing = fmt.Sprintf("⬇ %s %d%%", msg.label, x.installPct)
		} else {
			x.installing = fmt.Sprintf("⬇ %s %s", msg.label, humanSize(int(msg.done)))
		}
	}
	return nil
}

// extGo runs a host call off the UI loop and hands the answer back to it.
func (m *MainScreen) extGo(method string, params any, timeout time.Duration, done func(m *MainScreen, raw json.RawMessage, err error)) {
	h := m.Ext.host
	if h == nil {
		done(m, nil, errExtNotRunning)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		var raw json.RawMessage
		err := h.Call(ctx, method, params, &raw)
		send(extDoneMsg{fn: func(m *MainScreen) { done(m, raw, err) }})
	}()
}

// ---------------------------------------------------------------- documents

// extDocText is the text the host last saw for path ("" if unknown).
func (m *MainScreen) extDocText(path string) string {
	if d := m.Ext.docs[path]; d != nil {
		return d.text
	}
	if doc := m.Store.Get(path); doc != nil {
		return doc.GetContent()
	}
	b, _ := os.ReadFile(path)
	return string(b)
}

// extOpenActive tells the host which file is in front, opening it there
// first. Called (deferred) from openFileRaw.
func (m *MainScreen) extOpenActive() {
	if m.Ext == nil || m.ActiveFile == "" {
		return
	}
	doc := m.Store.Get(m.ActiveFile)
	if doc == nil || doc.Kind == document.KindBinary {
		return
	}
	text := doc.GetContent()
	if m.ViewMode == ViewText {
		text = m.TextEd.GetContent()
	}
	m.extOpen(m.ActiveFile, text)
	m.extSendActive()
}

func (m *MainScreen) extOpen(path, text string) {
	x := m.Ext
	if _, ok := x.docs[path]; ok {
		m.extChanged(path, text)
		return
	}
	x.docs[path] = &extDoc{version: 1, text: text}
	if x.host != nil {
		_ = x.host.Notify("doc/open", map[string]any{"path": path, "languageId": extensions.LanguageID(path), "version": 1, "text": text})
	}
}

// extChanged forwards an edit. Cheap when nothing changed, so it can run
// after every key.
func (m *MainScreen) extChanged(path, text string) {
	if m.Ext == nil {
		return
	}
	d := m.Ext.docs[path]
	if d == nil || d.text == text {
		return
	}
	d.version++
	d.text = text
	if m.Ext.host != nil {
		_ = m.Ext.host.Notify("doc/change", map[string]any{"path": path, "version": d.version, "text": text})
	}
}

// extSyncActive sends the active editor's text and cursor after a key.
func (m *MainScreen) extSyncActive() {
	if m.Ext == nil || m.ActiveFile == "" || m.ViewMode != ViewText || m.TextEd.CurrentPath() != m.ActiveFile {
		return
	}
	m.extChanged(m.ActiveFile, m.TextEd.GetContent())
}

func (m *MainScreen) extSendActive() {
	if m.Ext.host == nil || m.ActiveFile == "" {
		return
	}
	row, col := m.TextEd.Cursor()
	text := m.extDocText(m.ActiveFile)
	c16 := exthost.ToUTF16(lineOf(text, row), col)
	p := map[string]int{"line": row, "character": c16}
	_ = m.Ext.host.Notify("editor/active", map[string]any{"path": m.ActiveFile, "selections": []any{map[string]any{"anchor": p, "active": p}}})
}

// extSaved fires onDidSaveTextDocument, and reloads settings when a
// settings file was saved.
func (m *MainScreen) extSaved(path string) {
	if m.Ext == nil {
		return
	}
	if path == hithome.SettingsPath() || path == filepath.Join(m.RootDir, ".vscode", "settings.json") {
		m.Ext.settings = loadExtSettings(m.RootDir)
		m.extApplyTheme()
		if m.Ext.host != nil {
			_ = m.Ext.host.Notify("config/changed", map[string]any{"settings": m.Ext.settings})
		}
	}
	if m.Ext.host != nil && m.Ext.docs[path] != nil {
		_ = m.Ext.host.Notify("doc/save", map[string]any{"path": path})
	}
}

func (m *MainScreen) extClosed(path string) {
	if m.Ext == nil {
		return
	}
	for p := range m.Ext.docs {
		if p == path || strings.HasPrefix(p, path+string(os.PathSeparator)) {
			delete(m.Ext.docs, p)
			if m.Ext.host != nil {
				_ = m.Ext.host.Notify("doc/close", map[string]any{"path": p})
			}
		}
	}
}

func (m *MainScreen) extReplayDocs() {
	x := m.Ext
	paths := make([]string, 0, len(x.docs))
	for p := range x.docs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		d := x.docs[p]
		_ = x.host.Notify("doc/open", map[string]any{"path": p, "languageId": extensions.LanguageID(p), "version": d.version, "text": d.text})
	}
	if m.ActiveFile != "" && x.docs[m.ActiveFile] == nil {
		m.extOpenActive()
		return
	}
	m.extSendActive()
}

func lineOf(text string, row int) string {
	for i := 0; i < row; i++ {
		j := strings.IndexByte(text, '\n')
		if j < 0 {
			return ""
		}
		text = text[j+1:]
	}
	if j := strings.IndexByte(text, '\n'); j >= 0 {
		text = text[:j]
	}
	return strings.TrimSuffix(text, "\r")
}

// ---------------------------------------------------------------- settings

// loadExtSettings reads user settings (~/.hittable/settings.json) and the
// project's .vscode/settings.json, both VS Code's JSON-with-comments.
func loadExtSettings(root string) exthost.Settings {
	read := func(p string) map[string]any {
		b, err := os.ReadFile(p)
		if err != nil {
			return map[string]any{}
		}
		var v map[string]any
		if json.Unmarshal(extensions.StripJSONC(b), &v) != nil || v == nil {
			return map[string]any{}
		}
		return v
	}
	return exthost.Settings{User: read(hithome.SettingsPath()), Workspace: read(filepath.Join(root, ".vscode", "settings.json"))}
}

// writeSetting updates one key, editing the file's text in place so its
// comments and layout survive (extensions.SetJSONCValue). A value that
// cannot be edited in place (an object, a list) rewrites the file, except
// the project's .vscode/settings.json when it has comments: that value goes
// to user settings rather than erasing someone's notes.
func (m *MainScreen) writeSetting(key string, value any, target string) error {
	p := hithome.SettingsPath()
	if target == "workspace" {
		p = filepath.Join(m.RootDir, ".vscode", "settings.json")
	}
	src, _ := os.ReadFile(p)
	if len(src) == 0 {
		src = []byte("{\n}\n")
	}
	out, ok := extensions.SetJSONCValue(src, key, value)
	if !ok {
		hasComments := string(extensions.StripJSONC(src)) != string(src)
		if hasComments && target == "workspace" {
			m.extNote("warning", "Saved "+key+" to user settings: rewriting .vscode/settings.json would remove its comments.")
			return m.writeSetting(key, value, "user")
		}
		cur := map[string]any{}
		_ = json.Unmarshal(extensions.StripJSONC(src), &cur)
		if cur == nil {
			cur = map[string]any{}
		}
		if value == nil {
			delete(cur, key)
		} else {
			cur[key] = value
		}
		b, err := json.MarshalIndent(cur, "", "  ")
		if err != nil {
			return err
		}
		out = append(b, '\n')
		if hasComments {
			m.extNote("warning", "Rewrote "+filepath.Base(p)+" to save "+key+"; its comments could not be kept.")
		}
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, out, 0o644); err != nil {
		return err
	}
	m.Ext.settings = loadExtSettings(m.RootDir)
	if doc := m.Store.Get(p); doc != nil {
		m.adoptDiskContent(doc, out)
	}
	return nil
}

// extApplyTheme sets the editor colours from workbench.colorTheme.
func (m *MainScreen) extApplyTheme() {
	name, _ := m.Ext.settings.User["workbench.colorTheme"].(string)
	if ws, ok := m.Ext.settings.Workspace["workbench.colorTheme"].(string); ok {
		name = ws
	}
	if name == "" {
		texteditor.SetSyntaxStyle(nil)
		return
	}
	for _, t := range extensions.Themes() {
		if t.Label == name {
			st, colors, err := t.ChromaStyle()
			if err != nil {
				m.extNote("error", "Theme "+name+": "+err.Error())
				return
			}
			texteditor.SetSyntaxStyle(st)
			if m.Ext.web != nil {
				m.Ext.web.SetColors(webviewColors(colors))
			}
			return
		}
	}
	texteditor.SetSyntaxStyle(nil)
}

func webviewColors(colors map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range colors {
		if strings.HasPrefix(v, "#") {
			out["--vscode-"+strings.ReplaceAll(k, ".", "-")] = v
		}
	}
	return out
}

// ---------------------------------------------------------------- messages to the user

// extNote shows a message in the footer and keeps it in the notification
// list (alt+p › Notifications).
func (m *MainScreen) extNote(severity, text string) {
	icon := map[string]string{"error": "✖ ", "warning": "⚠ "}[severity]
	m.StatusBar = icon + text
	x := m.Ext
	x.notes = append(x.notes, time.Now().Format("15:04")+"  "+icon+text)
	if len(x.notes) > 50 {
		x.notes = x.notes[len(x.notes)-50:]
	}
}

// extAsk shows a palette list for the host (or for a built-in command),
// queued behind any list already open so two prompts never fight.
func (m *MainScreen) extAsk(show func()) {
	if m.Palette.ItemsOpen() && !m.Palette.Loading() {
		m.Ext.uiQueue = append(m.Ext.uiQueue, show)
		return
	}
	show()
}

// extLoading shows a spinner in the palette while a list is fetched, so the
// screen never flashes back to the editor between the request and the
// answer. The returned func reports whether the answer is still wanted (esc
// cancels); extLoaded closes the spinner when the answer is not a list.
func (m *MainScreen) extLoading(title, what string) func() bool {
	x := m.Ext
	x.loadSeq++
	seq := x.loadSeq
	if m.Palette.ItemsOpen() && !m.Palette.Loading() {
		return func() bool { return seq == x.loadSeq } // a prompt is up; leave it
	}
	cancelled := false
	m.showItems(paletteItems{Title: title, Placeholder: what, Loading: true, OnCancel: func() { cancelled = true }})
	return func() bool { return !cancelled && seq == x.loadSeq }
}

func (m *MainScreen) extLoaded() {
	if m.Palette.Loading() {
		m.Palette.Close()
	}
}

// extNextPrompt shows the next queued prompt, if any; called after a list
// closes.
func (m *MainScreen) extNextPrompt() {
	x := m.Ext
	if m.Palette.ItemsOpen() || len(x.uiQueue) == 0 {
		return
	}
	next := x.uiQueue[0]
	x.uiQueue = x.uiQueue[1:]
	next()
}

func (m *MainScreen) extAppendOutput(channel, text string) {
	b := m.Ext.outputs[channel]
	if b == nil {
		b = &strings.Builder{}
		m.Ext.outputs[channel] = b
	}
	b.WriteString(text)
	if b.Len() > 2<<20 { // keep the last ~1 MB
		s := b.String()
		b.Reset()
		b.WriteString(s[len(s)-(1<<20):])
	}
}

// ---------------------------------------------------------------- notifications

func (m *MainScreen) extNotification(method string, raw json.RawMessage) tea.Cmd {
	x := m.Ext
	switch method {
	case "log":
		var p struct{ Level, Text string }
		_ = json.Unmarshal(raw, &p)
		m.extAppendOutput("Extension Host", fmt.Sprintf("[%s] %s\n", p.Level, p.Text))
	case "ext/activating":
		var p struct{ ID string }
		_ = json.Unmarshal(raw, &p)
		x.activation[p.ID] = "activating"
	case "ext/activated":
		var p struct{ ID string }
		_ = json.Unmarshal(raw, &p)
		x.activation[p.ID] = "active"
	case "ext/failed":
		var p struct{ ID, Error string }
		_ = json.Unmarshal(raw, &p)
		x.activation[p.ID] = "failed: " + p.Error
		m.extNote("error", p.ID+" failed to start: "+p.Error+" (alt+p › Extensions: Show Log)")
	case "output/append":
		var p struct{ Channel, Text string }
		_ = json.Unmarshal(raw, &p)
		m.extAppendOutput(p.Channel, p.Text)
	case "output/clear":
		var p struct{ Channel string }
		_ = json.Unmarshal(raw, &p)
		delete(x.outputs, p.Channel)
	case "output/show":
		var p struct{ Channel string }
		_ = json.Unmarshal(raw, &p)
		m.StatusBar = "Output from " + p.Channel + " · alt+p › Output: Show"
	case "statusBar/set":
		var p struct {
			Key, Text, Tooltip, Command, Alignment string
			Args                                   []any
			Visible, Error                         bool
			Priority                               float64
		}
		_ = json.Unmarshal(raw, &p)
		if !p.Visible || strings.TrimSpace(p.Text) == "" {
			delete(x.status, p.Key)
		} else {
			x.status[p.Key] = extStatus{text: p.Text, tooltip: p.Tooltip, command: p.Command, args: p.Args, align: p.Alignment, priority: p.Priority, visible: true, err: p.Error}
		}
	case "statusBar/message":
		var p struct {
			Text    string
			Timeout int
		}
		_ = json.Unmarshal(raw, &p)
		x.message = p.Text
		x.messageTTL = time.Time{}
		if p.Timeout > 0 {
			x.messageTTL = time.Now().Add(time.Duration(p.Timeout) * time.Millisecond)
		}
	case "progress/start", "progress/report":
		var p struct{ Key, Title, Message string }
		_ = json.Unmarshal(raw, &p)
		cur := x.progress[p.Key]
		if p.Title != "" {
			cur = p.Title
		}
		if p.Message != "" {
			title, _, _ := strings.Cut(cur, ": ")
			cur = strings.TrimPrefix(title+": "+p.Message, ": ")
		}
		x.progress[p.Key] = cur
	case "progress/end":
		var p struct{ Key string }
		_ = json.Unmarshal(raw, &p)
		delete(x.progress, p.Key)
	case "diagnostics/set":
		var p struct {
			Batch []struct {
				Owner, Path string
				Diagnostics []struct {
					Range                 wireRange
					Severity              int
					Message, Source, Code string
				}
			}
		}
		_ = json.Unmarshal(raw, &p)
		touched := map[string]bool{}
		for _, b := range p.Batch {
			kept := x.diags[b.Path][:0]
			for _, d := range x.diags[b.Path] {
				if d.owner != b.Owner {
					kept = append(kept, d)
				}
			}
			for _, d := range b.Diagnostics {
				msg := d.Message
				if d.Source != "" {
					msg += " (" + d.Source + ")"
				}
				kept = append(kept, extDiag{owner: b.Owner, line: d.Range.Start.Line, start: d.Range.Start.Character,
					endLine: d.Range.End.Line, end: d.Range.End.Character, severity: d.Severity, message: msg, source: d.Source})
			}
			if len(kept) == 0 {
				delete(x.diags, b.Path)
			} else {
				x.diags[b.Path] = kept
			}
			touched[b.Path] = true
		}
		for p := range touched {
			m.extRefreshMarks(p)
		}
	case "commands/registered":
		// Shown in the palette only when contributed, as in VS Code.
	case "views/registered":
		var p struct {
			ID      string
			Webview bool
		}
		_ = json.Unmarshal(raw, &p)
		if _, ok := x.views[p.ID]; !ok || p.Webview {
			x.views[p.ID] = p.Webview
		}
	case "languages/triggers":
		_ = json.Unmarshal(raw, &x.triggers)
	case "webview/create":
		var p struct {
			Handle, ViewType, Title, ExtensionID string
			Roots                                []string
		}
		_ = json.Unmarshal(raw, &p)
		if x.web == nil {
			return nil
		}
		x.web.Create(p.Handle, p.Title, p.Roots)
		x.webviews[p.Handle] = p.Title
		m.extOpenWebview(p.Handle)
	case "webview/html":
		var p struct{ Handle, HTML string }
		_ = json.Unmarshal(raw, &p)
		if x.web != nil {
			x.web.SetHTML(p.Handle, p.HTML)
		}
	case "webview/post":
		var p struct {
			Handle  string
			Message json.RawMessage
		}
		_ = json.Unmarshal(raw, &p)
		if x.web != nil {
			x.web.Post(p.Handle, p.Message)
		}
	case "webview/title":
		var p struct{ Handle, Title string }
		_ = json.Unmarshal(raw, &p)
		x.webviews[p.Handle] = p.Title
		if x.web != nil {
			x.web.SetTitle(p.Handle, p.Title)
		}
	case "webview/reveal":
		var p struct{ Handle string }
		_ = json.Unmarshal(raw, &p)
		m.extOpenWebview(p.Handle)
	case "webview/dispose":
		var p struct{ Handle string }
		_ = json.Unmarshal(raw, &p)
		delete(x.webviews, p.Handle)
		if x.web != nil {
			x.web.Dispose(p.Handle)
		}
	case "terminal/create", "terminal/show":
		if !m.Term.Open {
			m.toggleTerminal()
		}
	case "terminal/send":
		var p struct{ Text string }
		_ = json.Unmarshal(raw, &p)
		if !m.Term.Open {
			m.toggleTerminal()
		}
		m.Term.Write([]byte(strings.ReplaceAll(p.Text, "\n", "\r")))
	case "editor/reveal":
		var p struct {
			Path  string
			Range wireRange
		}
		_ = json.Unmarshal(raw, &p)
		if p.Path == m.ActiveFile {
			m.TextEd.GotoLine(p.Range.Start.Line)
		}
	case "fs/changed":
		_ = m.Explorer.RebuildTree()
		m.reloadExternalEdits()
	case "context/set":
		var p struct {
			Key   string
			Value any
		}
		_ = json.Unmarshal(raw, &p)
		x.contexts[p.Key] = p.Value
	case "host/restart":
		return m.extRestart()
	}
	return nil
}

func (m *MainScreen) extOpenWebview(handle string) {
	x := m.Ext
	if x.web == nil {
		return
	}
	u := x.web.URL(handle)
	title := x.webviews[handle]
	if exthost.OpenBrowser(u) {
		m.StatusBar = "Opened “" + title + "” in your browser (extension panels can't draw in a terminal)"
	} else {
		m.extNote("info", "Open “"+title+"” in a browser: "+u)
	}
}

// extRefreshMarks converts stored diagnostics (UTF-16 columns) into editor
// marks for path, or for every path when path is "".
func (m *MainScreen) extRefreshMarks(path string) {
	if path == "" {
		for _, p := range m.Store.Paths() {
			m.TextEd.SetMarks(p, nil)
		}
		for p := range m.Ext.diags {
			m.extRefreshMarks(p)
		}
		return
	}
	text := m.extDocText(path)
	var marks []texteditor.Mark
	for _, d := range m.Ext.diags[path] {
		line := lineOf(text, d.line)
		start := exthost.FromUTF16(line, d.start)
		end := len([]rune(line))
		if d.endLine == d.line {
			end = exthost.FromUTF16(line, d.end)
		}
		marks = append(marks, texteditor.Mark{Line: d.line, Start: start, End: end, Severity: d.severity, Message: d.message})
	}
	m.TextEd.SetMarks(path, marks)
}

// ---------------------------------------------------------------- requests from the host

type wirePos struct{ Line, Character int }
type wireRange struct{ Start, End wirePos }

func (m *MainScreen) extRequest(r *exthost.Request) tea.Cmd {
	switch r.Method {
	case "window/showMessage":
		var p struct {
			Severity, Message, Detail string
			Modal                     bool
			Items                     []string
		}
		_ = json.Unmarshal(r.Params, &p)
		text := p.Message
		if p.Detail != "" {
			text += " — " + p.Detail
		}
		if len(p.Items) == 0 && !p.Modal {
			m.extNote(p.Severity, text)
			r.Reply(nil, nil)
			return nil
		}
		items := make([]paletteItem, len(p.Items))
		for i, s := range p.Items {
			items[i] = paletteItem{Label: s}
		}
		if len(items) == 0 {
			items = []paletteItem{{Label: "OK"}}
		}
		m.extNote(p.Severity, text)
		m.extAsk(func() {
			m.showItems(paletteItems{Title: shortTitle(text), Items: items,
				OnPick: func(i int, _ string, _ []int) {
					if len(p.Items) == 0 {
						i = -1
					}
					r.Reply(i, nil)
					m.extNextPrompt()
				},
				OnCancel: func() { r.Reply(-1, nil); m.extNextPrompt() },
			})
		})
	case "window/showQuickPick":
		var p struct {
			Items []struct {
				Label, Description, Detail string
				Picked, Separator          bool
			}
			PlaceHolder, Title string
			CanPickMany        bool
		}
		_ = json.Unmarshal(r.Params, &p)
		var items []paletteItem
		var index []int
		for i, it := range p.Items {
			if it.Separator {
				continue
			}
			d := it.Description
			if it.Detail != "" {
				d = strings.TrimSpace(d + "  " + it.Detail)
			}
			items = append(items, paletteItem{Label: it.Label, Detail: d, Picked: it.Picked})
			index = append(index, i)
		}
		title := p.Title
		if title == "" {
			title = p.PlaceHolder
		}
		m.extAsk(func() {
			m.showItems(paletteItems{Title: shortTitle(title), Placeholder: p.PlaceHolder, Items: items, Multi: p.CanPickMany,
				OnPick: func(i int, _ string, picked []int) {
					if p.CanPickMany {
						out := make([]int, 0, len(picked))
						for _, k := range picked {
							out = append(out, index[k])
						}
						r.Reply(out, nil)
					} else if i >= 0 {
						r.Reply(index[i], nil)
					} else {
						r.Reply(nil, nil)
					}
					m.extNextPrompt()
				},
				OnCancel: func() { r.Reply(nil, nil); m.extNextPrompt() },
			})
		})
	case "window/showInputBox":
		var p struct {
			Prompt, PlaceHolder, Value, Title string
			Password                          bool
		}
		_ = json.Unmarshal(r.Params, &p)
		title := p.Title
		if title == "" {
			title = p.Prompt
		}
		m.extAsk(func() {
			m.showItems(paletteItems{Title: shortTitle(title), Placeholder: p.PlaceHolder, Value: p.Value, Input: true, Password: p.Password,
				OnPick:   func(_ int, text string, _ []int) { r.Reply(text, nil); m.extNextPrompt() },
				OnCancel: func() { r.Reply(nil, nil); m.extNextPrompt() },
			})
		})
	case "window/showTextDocument":
		var p struct {
			Path      string
			Selection *wireRange
		}
		_ = json.Unmarshal(r.Params, &p)
		if p.Selection != nil {
			m.extJump(p.Path, p.Selection.Start.Line, p.Selection.Start.Character)
		} else {
			m.openFileRaw(p.Path)
		}
		r.Reply(true, nil)
	case "workspace/applyEdit":
		var p struct {
			Edits []wireEdit
			Files []wireFileOp
		}
		_ = json.Unmarshal(r.Params, &p)
		r.Reply(m.extApplyEdits(p.Edits, p.Files), nil)
	case "workspace/save":
		var p struct{ Path string }
		_ = json.Unmarshal(r.Params, &p)
		if p.Path == m.ActiveFile {
			m.saveAndEnqueue()
		}
		m.WriteQueue.FlushNow()
		r.Reply(true, nil)
	case "workspace/saveAll":
		m.saveAndEnqueue()
		m.WriteQueue.FlushNow()
		r.Reply(true, nil)
	case "languages/idFor":
		var p struct{ Path string }
		_ = json.Unmarshal(r.Params, &p)
		r.Reply(extensions.LanguageID(p.Path), nil)
	case "config/update":
		var p struct {
			Key, Target string
			Value       any
		}
		_ = json.Unmarshal(r.Params, &p)
		err := m.writeSetting(p.Key, p.Value, p.Target)
		if err == nil && m.Ext.host != nil {
			_ = m.Ext.host.Notify("config/changed", map[string]any{"settings": m.Ext.settings})
		}
		r.Reply(nil, err)
	case "clipboard/read":
		s, err := clipboard.ReadAll()
		r.Reply(s, err)
	case "clipboard/write":
		var p struct{ Text string }
		_ = json.Unmarshal(r.Params, &p)
		r.Reply(nil, clipboard.WriteAll(p.Text))
	case "env/openExternal":
		var p struct{ URI string }
		_ = json.Unmarshal(r.Params, &p)
		if strings.HasPrefix(p.URI, "file://") {
			m.openFileRaw(strings.TrimPrefix(p.URI, "file://"))
		} else if !exthost.OpenBrowser(p.URI) {
			m.extNote("info", "Open in a browser: "+p.URI)
		}
		r.Reply(true, nil)
	case "commands/execute":
		var p struct {
			ID   string
			Args []any
		}
		_ = json.Unmarshal(r.Params, &p)
		res, err := m.extBuiltinCommand(p.ID, p.Args)
		r.Reply(res, err)
		return nil
	default:
		r.Reply(nil, fmt.Errorf("%s is not supported by hittable", r.Method))
	}
	return nil
}

func shortTitle(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if r := []rune(s); len(r) > 90 {
		return string(r[:89]) + "…"
	}
	return s
}

// ---------------------------------------------------------------- edits

type wireEdit struct {
	Path    string
	Range   wireRange
	NewText string
	Snippet bool
}

type wireFileOp struct {
	Op, Path, To                                 string
	Overwrite, IgnoreIfExists, IgnoreIfNotExists bool
	Recursive                                    bool
	Contents                                     string // base64
}

// extApplyEdits applies a workspace edit: file operations first, then text
// edits grouped by file, each file's edits applied from the end so earlier
// ranges stay valid. Open files change through the document store (so the
// write queue persists them and undo covers them); closed files are written.
func (m *MainScreen) extApplyEdits(edits []wireEdit, files []wireFileOp) bool {
	ok := true
	for _, f := range files {
		switch f.Op {
		case "create":
			if _, err := os.Stat(f.Path); err == nil && !f.Overwrite {
				if !f.IgnoreIfExists {
					ok = false
				}
				continue
			}
			data, _ := base64.StdEncoding.DecodeString(f.Contents)
			_ = os.MkdirAll(filepath.Dir(f.Path), 0o755)
			if os.WriteFile(f.Path, data, 0o644) != nil {
				ok = false
			}
		case "delete":
			if _, err := os.Stat(f.Path); err != nil {
				ok = ok && f.IgnoreIfNotExists
				continue
			}
			m.applyDelete(f.Path)
		case "rename":
			m.applyRename(f.Path, f.To)
		}
	}
	if len(files) > 0 {
		_ = m.Explorer.RebuildTree()
	}

	byPath := map[string][]wireEdit{}
	var order []string
	for _, e := range edits {
		if _, seen := byPath[e.Path]; !seen {
			order = append(order, e.Path)
		}
		byPath[e.Path] = append(byPath[e.Path], e)
	}
	for _, path := range order {
		text := m.extDocText(path)
		if m.ActiveFile == path && m.ViewMode == ViewText {
			text = m.TextEd.GetContent()
		}
		out, row, col := applyTextEdits(text, byPath[path])
		if out == text {
			continue
		}
		if err := m.extSetText(path, out, row, col); err != nil {
			m.extNote("error", "Could not apply an extension edit to "+filepath.Base(path)+": "+err.Error())
			ok = false
		}
	}
	return ok
}

// extSetText puts new content into a file the way an edit in the app would.
func (m *MainScreen) extSetText(path, content string, row, col int) error {
	doc := m.Store.Get(path)
	if doc == nil {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
		m.extChanged(path, content)
		return nil
	}
	if path == m.ActiveFile && m.ViewMode == ViewText {
		m.TextEd.ApplyContent(path, content, row, col)
		m.saveAndEnqueue()
	} else {
		if path == m.ActiveFile && doc.Kind == document.KindHit {
			// Refreshes the Runner fields (and cancels any queued write,
			// which the Update below replaces).
			m.adoptDiskContent(doc, []byte(content))
		}
		doc.Lock()
		doc.SetContent(content)
		gen := doc.IncGeneration()
		doc.SetLastFlushed(gen)
		doc.Unlock()
		m.WriteQueue.Update(path, content, gen)
		m.TextEd.ApplyContent(path, content, -1, 0)
	}
	m.extChanged(path, content)
	m.extRefreshMarks(path)
	return nil
}

// applyTextEdits applies UTF-16-ranged edits to text. The returned row/col
// is where a snippet's first tabstop landed (-1 when no edit was a snippet).
func applyTextEdits(text string, edits []wireEdit) (string, int, int) {
	lines := strings.Split(text, "\n")
	offset := func(p wirePos) int {
		if p.Line >= len(lines) {
			return len(text)
		}
		off := 0
		for i := 0; i < p.Line; i++ {
			off += len(lines[i]) + 1
		}
		line := strings.TrimSuffix(lines[p.Line], "\r")
		col := exthost.FromUTF16(line, p.Character)
		r := []rune(line)
		if col > len(r) {
			col = len(r)
		}
		return off + len(string(r[:col]))
	}
	type span struct {
		s, e    int
		text    string
		snippet bool
	}
	spans := make([]span, 0, len(edits))
	for _, e := range edits {
		s, end := offset(e.Range.Start), offset(e.Range.End)
		if end < s {
			s, end = end, s
		}
		spans = append(spans, span{s, end, e.NewText, e.Snippet})
	}
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].s > spans[j].s })
	cursor := -1
	for _, sp := range spans {
		ins := sp.text
		if sp.snippet {
			var cur int
			ins, cur = extensions.ExpandSnippet(sp.text)
			cursor = sp.s + len(string([]rune(ins)[:cur]))
		} else if cursor >= sp.s {
			cursor += len(ins) - (sp.e - sp.s)
		}
		text = text[:sp.s] + ins + text[sp.e:]
	}
	if cursor < 0 {
		return text, -1, 0
	}
	before := text[:min(cursor, len(text))]
	row := strings.Count(before, "\n")
	col := len([]rune(before[strings.LastIndexByte(before, '\n')+1:]))
	return text, row, col
}
