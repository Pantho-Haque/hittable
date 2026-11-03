package screens

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hittable/shellapp/internal/extensions"
	"github.com/hittable/shellapp/internal/hithome"
	"github.com/hittable/shellapp/internal/noderuntime"
	"github.com/hittable/shellapp/ui/components/palette"
	"github.com/hittable/shellapp/ui/theme"
)

type (
	paletteItem  = palette.Item
	paletteItems = palette.Items
)

// showItems opens a palette list. It is always a fresh list: the attach
// flow's flag must not route its pick.
func (m *MainScreen) showItems(cfg paletteItems) {
	m.attaching = false
	m.ShowHelp = false
	m.GitOpen = false
	m.Palette.ShowItems(cfg)
}

type extCommand struct {
	title string
	run   func(m *MainScreen) tea.Cmd
}

// builtinCommands are hittable's own entries in the command palette.
func builtinCommands() []extCommand {
	return []extCommand{
		{"Extensions: Open Extensions Panel  (alt+x)", func(m *MainScreen) tea.Cmd { m.openExtensions(""); return nil }},
		{"Extensions: Install from Open VSX…", func(m *MainScreen) tea.Cmd { m.openExtensions(""); return nil }},
		{"Extensions: Install from VSIX File…", (*MainScreen).cmdInstallVSIX},
		{"Extensions: Show Installed Extensions", func(m *MainScreen) tea.Cmd { m.openExtensions("@installed"); return nil }},
		{"Extensions: Manage Installed (list)…", (*MainScreen).cmdInstalled},
		{"Extensions: Restart Extension Host", func(m *MainScreen) tea.Cmd { m.Ext.crashes = nil; return m.extRestart() }},
		{"Extensions: Trust This Folder", (*MainScreen).cmdTrust},
		{"Extensions: Install Node.js Runtime", func(m *MainScreen) tea.Cmd { return m.cmdInstallNode(nil) }},
		{"Extensions: Show Log", func(m *MainScreen) tea.Cmd { m.showOutput("Extension Host"); return nil }},
		{"Preferences: Color Theme…", (*MainScreen).cmdTheme},
		{"Preferences: Open User Settings (JSON)", func(m *MainScreen) tea.Cmd { m.openSettingsFile(); return nil }},
		{"Output: Show Output Channel…", (*MainScreen).cmdOutput},
		{"Problems: Show All", (*MainScreen).cmdProblems},
		{"Notifications: Show Recent", (*MainScreen).cmdNotifications},
		{"View: Open View…", (*MainScreen).cmdViews},
		{"View: Reopen Extension Panel in Browser…", (*MainScreen).cmdPanels},
		{"Format Document  (alt+shift+f)", func(m *MainScreen) tea.Cmd { m.extFormat(); return nil }},
		{"Go to Definition  (F12)", func(m *MainScreen) tea.Cmd { m.extLocations("definition"); return nil }},
		{"Go to Type Definition", func(m *MainScreen) tea.Cmd { m.extLocations("typeDefinition"); return nil }},
		{"Go to Implementations", func(m *MainScreen) tea.Cmd { m.extLocations("implementation"); return nil }},
		{"Go to References  (shift+F12)", func(m *MainScreen) tea.Cmd { m.extLocations("references"); return nil }},
		{"Go to Symbol in File…", func(m *MainScreen) tea.Cmd { m.extSymbols(); return nil }},
		{"Rename Symbol  (F2)", func(m *MainScreen) tea.Cmd { m.extRename(); return nil }},
		{"Quick Fix…  (alt+.)", func(m *MainScreen) tea.Cmd { m.extCodeActions(); return nil }},
		{"Show Hover  (alt+h)", func(m *MainScreen) tea.Cmd { m.extHover(); return nil }},
		{"Go to Next Problem  (F8)", func(m *MainScreen) tea.Cmd { m.extNextProblem(); return nil }},
	}
}

// openCommandPalette lists hittable's commands and every command enabled
// extensions contribute (alt+p).
func (m *MainScreen) openCommandPalette() {
	if m.SingleFile && m.Ext == nil {
		return
	}
	type row struct {
		label, detail string
		run           func() tea.Cmd
	}
	var rows []row
	for _, c := range builtinCommands() {
		c := c
		rows = append(rows, row{label: c.title, run: func() tea.Cmd { return c.run(m) }})
	}
	var ids []string
	for id := range m.Ext.commands {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return m.Ext.commands[ids[i]] < m.Ext.commands[ids[j]] })
	for _, id := range ids {
		id := id
		rows = append(rows, row{label: m.Ext.commands[id], detail: id, run: func() tea.Cmd { m.extExecute(id, nil); return nil }})
	}
	items := make([]paletteItem, len(rows))
	for i, r := range rows {
		items[i] = paletteItem{Label: r.label, Detail: r.detail}
	}
	m.showItems(paletteItems{Title: "Command Palette", Placeholder: "type a command", Items: items,
		OnPick: func(i int, _ string, _ []int) {
			if i >= 0 {
				m.pendingCmd = rows[i].run()
			}
		},
	})
}

// extExecute runs an extension command, starting the host first when it is
// not running.
func (m *MainScreen) extExecute(id string, args []any) {
	if m.Ext.host == nil {
		m.extWhyStopped()
		return
	}
	if args == nil {
		args = []any{}
	}
	m.extGo("command/execute", map[string]any{"id": id, "args": args}, 10*time.Minute, func(m *MainScreen, _ json.RawMessage, err error) {
		if err != nil {
			m.extNote("error", id+": "+err.Error())
		}
	})
}

// extWhyStopped explains why extension code is not running, and offers the fix.
func (m *MainScreen) extWhyStopped() {
	switch m.Ext.stopped {
	case "untrusted":
		m.extAsk(func() {
			m.showItems(paletteItems{Title: "Extensions run code with your permissions. Trust " + filepath.Base(m.RootDir) + "?",
				Items: []paletteItem{{Label: "Trust this folder and start extensions"}, {Label: "Not now"}},
				OnPick: func(i int, _ string, _ []int) {
					if i == 0 {
						m.pendingCmd = m.cmdTrust()
					}
					m.extNextPrompt()
				},
				OnCancel: m.extNextPrompt,
			})
		})
	case "node":
		m.pendingCmd = m.cmdInstallNode(nil)
	case "crashed", "failed":
		m.extNote("error", "Extensions are stopped after errors. alt+p › Extensions: Show Log, then Restart Extension Host.")
	case "":
		if m.Ext.starting {
			m.StatusBar = "Extensions are starting…"
		} else {
			m.extNote("info", errExtNotRunning.Error())
		}
	}
}

// extBuiltinCommand answers commands.executeCommand for ids no extension
// registered: the VS Code workbench commands that map onto this app.
func (m *MainScreen) extBuiltinCommand(id string, args []any) (any, error) {
	switch id {
	case "workbench.action.openSettings", "workbench.action.openSettingsJson", "workbench.action.openGlobalSettings":
		m.openSettingsFile()
	case "workbench.action.files.save":
		m.saveAndEnqueue()
		m.WriteQueue.FlushNow()
	case "workbench.action.files.saveAll":
		m.saveAndEnqueue()
		m.WriteQueue.FlushNow()
	case "workbench.action.closeActiveEditor":
		m.closeFile()
	case "workbench.action.quickOpen":
		m.openPalette(palette.ModeFiles)
	case "workbench.action.showCommands":
		m.openCommandPalette()
	case "workbench.action.terminal.new", "workbench.action.terminal.focus", "workbench.action.terminal.toggleTerminal":
		if !m.Term.Open {
			m.toggleTerminal()
		}
	case "workbench.view.explorer", "workbench.files.action.focusFilesExplorer":
		m.ExplorerHidden = false
		m.setExplorerFocused(true)
	case "workbench.view.scm":
		m.toggleGit()
	case "editor.action.formatDocument":
		m.extFormat()
	case "editor.action.showHover":
		m.extHover()
	case "editor.action.rename":
		m.extRename()
	case "editor.action.quickFix":
		m.extCodeActions()
	case "editor.action.marker.next", "editor.action.marker.nextInFiles":
		m.extNextProblem()
	case "workbench.actions.view.problems", "workbench.action.problems.focus":
		m.cmdProblems()
	case "workbench.action.output.toggleOutput":
		m.cmdOutput()
	case "editor.action.showReferences", "editor.action.goToLocations", "editor.action.peekLocations":
		// (uri, position, locations, …): show the locations to pick from.
		if len(args) >= 3 {
			b, _ := json.Marshal(args[2])
			var locs []struct {
				URI   any                     `json:"uri"`
				Range struct{ Start wirePos } `json:"range"`
			}
			_ = json.Unmarshal(b, &locs)
			var out []extLocation
			for _, l := range locs {
				out = append(out, extLocation{Path: uriPath(l.URI), Line: l.Range.Start.Line, Char: l.Range.Start.Character})
			}
			m.extPickLocation("References", out)
		}
	case "workbench.action.reloadWindow":
		m.pendingCmd = m.extRestart()
	default:
		return nil, fmt.Errorf("command '%s' not found", id)
	}
	return nil, nil
}

func uriPath(v any) string {
	switch u := v.(type) {
	case string:
		return strings.TrimPrefix(u, "file://")
	case map[string]any:
		if p, ok := u["fsPath"].(string); ok {
			return p
		}
		if p, ok := u["$uri"].(string); ok {
			return p
		}
		if p, ok := u["path"].(string); ok {
			return p
		}
	}
	return ""
}

// ---------------------------------------------------------------- install

func (m *MainScreen) cmdInstall() tea.Cmd {
	m.extAsk(func() {
		m.showItems(paletteItems{Title: "Search Open VSX", Placeholder: "e.g. yaml, prettier, rust", Input: true,
			OnPick: func(_ int, q string, _ []int) {
				q = strings.TrimSpace(q)
				if q == "" {
					m.extNextPrompt()
					return
				}
				m.StatusBar = "Searching Open VSX for “" + q + "”…"
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					res, err := extensions.Search(ctx, q, 40)
					send(extDoneMsg{fn: func(m *MainScreen) { m.showSearchResults(q, res, err) }})
				}()
				m.extNextPrompt()
			},
			OnCancel: m.extNextPrompt,
		})
	})
	return nil
}

func (m *MainScreen) showSearchResults(q string, res []extensions.Summary, err error) {
	if err != nil {
		m.extNote("error", err.Error())
		return
	}
	if len(res) == 0 {
		m.extNote("info", "No extensions on Open VSX match “"+q+"”.")
		return
	}
	items := make([]paletteItem, len(res))
	for i, r := range res {
		d := r.ID + " · " + humanCount(r.Downloads) + " installs"
		if r.Verified {
			d += " · ✓ verified publisher"
		}
		if r.Deprecated {
			d += " · deprecated"
		}
		if _, ok := extensions.Find(r.ID); ok {
			d += " · installed"
		}
		label := r.DisplayName
		if label == "" {
			label = r.ID
		}
		if r.Description != "" {
			d += " — " + r.Description
		}
		items[i] = paletteItem{Label: label, Detail: d}
	}
	m.extAsk(func() {
		m.showItems(paletteItems{Title: "Install an extension (" + q + ")", Items: items,
			OnPick: func(i int, _ string, _ []int) {
				if i >= 0 {
					m.pendingCmd = m.extInstall(res[i].ID)
				}
				m.extNextPrompt()
			},
			OnCancel: m.extNextPrompt,
		})
	})
}

func (m *MainScreen) cmdInstallVSIX() tea.Cmd {
	m.extAsk(func() {
		m.showItems(paletteItems{Title: "Install from a .vsix file", Placeholder: "path (relative to " + filepath.Base(m.RootDir) + ", absolute, or ~/)", Input: true,
			OnPick: func(_ int, p string, _ []int) {
				p = strings.TrimSpace(p)
				if p != "" {
					if !filepath.IsAbs(p) && !strings.HasPrefix(p, "~/") {
						p = filepath.Join(m.RootDir, p)
					}
					m.pendingCmd = m.extInstall(p)
				}
				m.extNextPrompt()
			},
			OnCancel: m.extNextPrompt,
		})
	})
	return nil
}

// extInstall installs in the background with progress in the footer, then
// offers Node.js if the extension has code and Node is missing, and
// restarts the host so the extension loads.
func (m *MainScreen) extInstall(spec string) tea.Cmd {
	if m.Ext.installing != "" {
		m.extNote("warning", "An install is already running.")
		return nil
	}
	m.Ext.installing = "⬇ " + filepath.Base(spec)
	m.Ext.installPct = -1
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		last := time.Time{}
		res, err := extensions.Install(ctx, spec, func(label string, done, total int64) {
			if time.Since(last) > 150*time.Millisecond {
				last = time.Now()
				send(extProgressMsg{label: label, done: done, total: total})
			}
		})
		return extDoneMsg{fn: func(m *MainScreen) { m.extInstalled(res, err) }}
	}
}

func (m *MainScreen) extInstalled(res extensions.Result, err error) {
	m.Ext.installing = ""
	if err != nil {
		m.extNote("error", "Install failed: "+err.Error())
		return
	}
	e := res.Installed
	msg := "Installed " + e.Manifest.Label() + " " + e.Version
	if len(res.Deps) > 0 {
		msg += " (with " + strings.Join(res.Deps, ", ") + ")"
	}
	if !e.Signed && e.Source == "openvsx" {
		msg += " · not signed"
	}
	for _, w := range res.Warnings {
		m.extAppendOutput("Extension Host", "[install] "+w+"\n")
	}
	if len(res.Warnings) > 0 {
		msg += " · " + res.Warnings[0]
	}
	m.extNote("info", msg)
	m.extReloadContributions()
	m.ExtPanel.Reload()
	if m.ExtOpen {
		// The panel shows the next step (Set Color Theme, Download Node.js,
		// Trust this folder) as buttons on the extension's page.
		if e.Manifest.HasCode() && noderuntime.Installed() && extensions.Trusted(m.RootDir) {
			m.pendingCmd = m.extRestart()
		} else if e.Manifest.HasCode() && !extensions.Trusted(m.RootDir) {
			m.Ext.stopped = "untrusted"
		} else if e.Manifest.HasCode() {
			m.Ext.stopped = "node"
		}
		return
	}
	if len(e.Manifest.Contributes.Themes) > 0 && !e.Manifest.HasCode() {
		m.pendingCmd = m.cmdTheme()
		return
	}
	if !e.Manifest.HasCode() {
		return
	}
	if !noderuntime.Installed() {
		m.pendingCmd = m.cmdInstallNode(func(m *MainScreen) tea.Cmd { return m.extAfterInstall() })
		return
	}
	m.pendingCmd = m.extAfterInstall()
}

func (m *MainScreen) extAfterInstall() tea.Cmd {
	if !extensions.Trusted(m.RootDir) {
		m.Ext.stopped = "untrusted"
		m.extWhyStopped()
		return nil
	}
	return m.extRestart()
}

// cmdInstallNode asks before installing Node globally (Homebrew, winget or
// the official build in ~/.local), then runs then. A Node already on the
// machine is simply used.
func (m *MainScreen) cmdInstallNode(then func(m *MainScreen) tea.Cmd) tea.Cmd {
	noderuntime.Forget()
	if f, err := noderuntime.Find(); err == nil {
		m.StatusBar = "Extensions run with Node " + f.Version + " (" + f.Source + ": " + f.Path + ")"
		if then != nil {
			return then(m)
		}
		return m.extStartCmd()
	}
	plan, err := noderuntime.PlanInstall()
	if err != nil {
		m.extNote("error", err.Error())
		return nil
	}
	title := "Extensions with code need Node.js, and none is installed on this machine"
	if _, err := noderuntime.Find(); errors.Is(err, noderuntime.ErrTooOld) {
		title = err.Error()
	}
	m.extAsk(func() {
		m.showItems(paletteItems{Title: title,
			Items: []paletteItem{{Label: plan.Summary, Detail: plan.Command}, {Label: "Not now", Detail: "or set HITTABLE_NODE=/path/to/node"}},
			OnPick: func(i int, _ string, _ []int) {
				if i == 0 {
					m.pendingCmd = m.extInstallNodeNow(then)
				}
				m.extNextPrompt()
			},
			OnCancel: m.extNextPrompt,
		})
	})
	return nil
}

// extInstallNodeNow installs Node globally in the background. Each line of
// package-manager output, or the download's progress, is shown as it comes.
func (m *MainScreen) extInstallNodeNow(then func(m *MainScreen) tea.Cmd) tea.Cmd {
	x := m.Ext
	if x.installing != "" {
		return nil
	}
	plan, err := noderuntime.PlanInstall()
	if err != nil {
		x.nodeErr = err.Error()
		m.extNote("error", err.Error())
		return nil
	}
	x.installing, x.installPct, x.nodeLine, x.nodeErr = "⬇ Node.js", -1, plan.Summary, ""
	delete(x.outputs, "Node.js install")
	m.extAppendOutput("Node.js install", "$ "+plan.Summary+"\n")
	if plan.Command != "" {
		m.extAppendOutput("Node.js install", "$ "+plan.Command+"\n")
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		last := time.Time{}
		f, err := noderuntime.InstallGlobal(ctx, func(line string, done, total int64) {
			if line == "" && time.Since(last) < 150*time.Millisecond {
				return
			}
			last = time.Now()
			send(extProgressMsg{label: "Node.js", line: line, done: done, total: total})
		})
		return extDoneMsg{fn: func(m *MainScreen) {
			m.Ext.installing = ""
			if err != nil {
				m.Ext.nodeErr = err.Error()
				m.extAppendOutput("Node.js install", "\n✖ "+err.Error()+"\n")
				m.extNote("error", "Node.js install failed: "+err.Error()+" (its full output: Log on the extension's page)")
				return
			}
			m.Ext.nodeLine = ""
			m.extNote("info", "Node "+f.Version+" installed ("+f.Path+"); extensions use it now.")
			if then != nil {
				m.pendingCmd = then(m)
			} else {
				m.pendingCmd = m.extRestart()
			}
		}}
	}
}

func (m *MainScreen) cmdTrust() tea.Cmd {
	if err := extensions.Trust(m.RootDir); err != nil {
		m.extNote("error", err.Error())
		return nil
	}
	m.extNote("info", "Trusted "+m.RootDir+"; extensions can run here.")
	m.Ext.stopped = ""
	if !noderuntime.Installed() && len(needsHost(extensions.Enabled())) > 0 {
		if m.ExtOpen {
			m.Ext.stopped = "node" // the panel offers the download as a button
			return nil
		}
		return m.cmdInstallNode(nil)
	}
	return m.extStartCmd()
}

// cmdInstalled lists installed extensions; picking one offers its actions.
func (m *MainScreen) cmdInstalled() tea.Cmd {
	all, err := extensions.List()
	if err != nil {
		m.extNote("error", err.Error())
		return nil
	}
	if len(all) == 0 {
		m.extNote("info", "No extensions installed. alt+p › Extensions: Install from Open VSX…")
		return nil
	}
	items := make([]paletteItem, len(all))
	for i, e := range all {
		state := "enabled"
		if !e.Enabled {
			state = "disabled"
		}
		kind := "declarative"
		if e.Manifest.HasCode() {
			kind = "runs code"
		}
		items[i] = paletteItem{Label: e.Manifest.Label(), Detail: fmt.Sprintf("%s %s · %s · %s", e.ID, e.Version, state, kind)}
	}
	m.extAsk(func() {
		m.showItems(paletteItems{Title: "Installed extensions", Items: items,
			OnPick: func(i int, _ string, _ []int) {
				if i >= 0 {
					m.extActions(all[i])
				}
				m.extNextPrompt()
			},
			OnCancel: m.extNextPrompt,
		})
	})
	return nil
}

func (m *MainScreen) extActions(e extensions.Installed) {
	toggle := "Disable"
	if !e.Enabled {
		toggle = "Enable"
	}
	c := e.Manifest.Contributes
	var have []string
	if n := len(c.Commands); n > 0 {
		have = append(have, fmt.Sprintf("%d commands", n))
	}
	if n := len(c.Themes); n > 0 {
		have = append(have, fmt.Sprintf("%d themes", n))
	}
	if n := len(c.Snippets); n > 0 {
		have = append(have, "snippets")
	}
	if n := len(c.Languages); n > 0 {
		have = append(have, fmt.Sprintf("%d languages", n))
	}
	details := strings.Join(have, ", ")
	if u := e.Manifest.Unsupported(); len(u) > 0 {
		details += " · not in the terminal: " + strings.Join(u, ", ")
	}
	actions := []paletteItem{{Label: toggle}, {Label: "Uninstall"}, {Label: "Update to the newest compatible version"}, {Label: "Details", Detail: details}}
	m.extAsk(func() {
		m.showItems(paletteItems{Title: e.Manifest.Label() + " " + e.Version, Items: actions,
			OnPick: func(i int, _ string, _ []int) {
				switch i {
				case 0:
					if err := extensions.SetEnabled(e.ID, !e.Enabled); err != nil {
						m.extNote("error", err.Error())
					} else {
						m.extNote("info", toggle+"d "+e.Manifest.Label())
						m.pendingCmd = m.extRestart()
					}
				case 1:
					if err := extensions.Uninstall(e.ID); err != nil {
						m.extNote("error", err.Error())
					} else {
						m.extNote("info", "Uninstalled "+e.Manifest.Label())
						m.pendingCmd = m.extRestart()
					}
				case 2:
					spec := e.ID
					if e.Source == "vsix" {
						m.extNote("warning", e.ID+" was installed from a file; install the new .vsix to update it.")
						break
					}
					if err := extensions.Uninstall(e.ID); err != nil {
						m.extNote("error", err.Error())
						break
					}
					m.pendingCmd = m.extInstall(spec)
				case 3:
					m.extNote("info", e.ID+": "+details)
				}
				m.extNextPrompt()
			},
			OnCancel: m.extNextPrompt,
		})
	})
}

// ---------------------------------------------------------------- themes, settings, output

func (m *MainScreen) cmdTheme() tea.Cmd {
	themes := extensions.Themes()
	items := []paletteItem{{Label: "Dracula (built-in)"}}
	for _, t := range themes {
		items = append(items, paletteItem{Label: t.Label, Detail: t.ExtID})
	}
	m.extAsk(func() {
		m.showItems(paletteItems{Title: "Color theme", Items: items,
			OnPick: func(i int, _ string, _ []int) {
				var v any
				if i > 0 {
					v = themes[i-1].Label
				}
				if err := m.writeSetting("workbench.colorTheme", v, "user"); err != nil {
					m.extNote("error", err.Error())
				}
				m.extApplyTheme()
				if m.Ext.host != nil {
					_ = m.Ext.host.Notify("config/changed", map[string]any{"settings": m.Ext.settings})
				}
				m.extNextPrompt()
			},
			OnCancel: m.extNextPrompt,
		})
	})
	return nil
}

func (m *MainScreen) openSettingsFile() {
	p := hithome.SettingsPath()
	if _, err := os.Stat(p); err != nil {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("{\n}\n"), 0o644)
	}
	m.openFileRaw(p)
	m.StatusBar = "User settings · extensions read them through workspace.getConfiguration · ctrl+s applies"
}

func (m *MainScreen) cmdOutput() tea.Cmd {
	var names []string
	for n := range m.Ext.outputs {
		names = append(names, n)
	}
	if len(names) == 0 {
		m.extNote("info", "No extension has written any output yet.")
		return nil
	}
	sort.Strings(names)
	items := make([]paletteItem, len(names))
	for i, n := range names {
		items[i] = paletteItem{Label: n, Detail: humanSize(m.Ext.outputs[n].Len())}
	}
	m.extAsk(func() {
		m.showItems(paletteItems{Title: "Output channels", Items: items,
			OnPick: func(i int, _ string, _ []int) {
				if i >= 0 {
					m.showOutput(names[i])
				}
				m.extNextPrompt()
			},
			OnCancel: m.extNextPrompt,
		})
	})
	return nil
}

// showOutput writes a channel to a log file and opens it in the editor.
func (m *MainScreen) showOutput(name string) {
	b := m.Ext.outputs[name]
	text := ""
	if b != nil {
		text = b.String()
	}
	if name == "Extension Host" {
		if log, err := os.ReadFile(filepath.Join(hithome.Logs(), "exthost.log")); err == nil && len(text) == 0 {
			text = string(log)
		}
	}
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r < ' ' {
			return '_'
		}
		return r
	}, name)
	p := filepath.Join(hithome.ExtData(), "output", safe+".log")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		m.extNote("error", err.Error())
		return
	}
	if doc := m.Store.Get(p); doc != nil {
		m.adoptDiskContent(doc, []byte(text))
	}
	m.openFileRaw(p)
	m.TextEd.GotoLine(strings.Count(text, "\n"))
}

func (m *MainScreen) cmdNotifications() tea.Cmd {
	if len(m.Ext.notes) == 0 {
		m.StatusBar = "No notifications."
		return nil
	}
	items := make([]paletteItem, len(m.Ext.notes))
	for i := range m.Ext.notes {
		items[i] = paletteItem{Label: m.Ext.notes[len(m.Ext.notes)-1-i]}
	}
	m.extAsk(func() {
		m.showItems(paletteItems{Title: "Recent notifications", Items: items,
			OnPick: func(int, string, []int) { m.extNextPrompt() }, OnCancel: m.extNextPrompt})
	})
	return nil
}

func (m *MainScreen) cmdPanels() tea.Cmd {
	var handles []string
	for h := range m.Ext.webviews {
		handles = append(handles, h)
	}
	if len(handles) == 0 {
		m.extNote("info", "No extension panel is open.")
		return nil
	}
	sort.Strings(handles)
	items := make([]paletteItem, len(handles))
	for i, h := range handles {
		items[i] = paletteItem{Label: m.Ext.webviews[h], Detail: "⏎ open in browser"}
	}
	m.extAsk(func() {
		m.showItems(paletteItems{Title: "Extension panels", Items: items,
			OnPick: func(i int, _ string, _ []int) {
				if i >= 0 {
					m.extOpenWebview(handles[i])
				}
				m.extNextPrompt()
			},
			OnCancel: m.extNextPrompt,
		})
	})
	return nil
}

// cmdViews lists contributed views. Tree views are browsed in the palette
// (⏎ on a folder opens it, on a leaf runs its command); webview views open
// in the browser.
func (m *MainScreen) cmdViews() tea.Cmd {
	var ids []string
	for id := range m.Ext.views {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		m.extNote("info", "No installed extension contributes a view.")
		return nil
	}
	sort.Strings(ids)
	names := map[string]string{}
	for _, e := range extensions.Enabled() {
		for _, vs := range e.Manifest.Contributes.Views {
			for _, v := range vs {
				names[v.ID] = v.Name
			}
		}
	}
	items := make([]paletteItem, len(ids))
	for i, id := range ids {
		label := names[id]
		if label == "" || strings.HasPrefix(label, "%") {
			label = id
		}
		d := "tree"
		if m.Ext.views[id] {
			d = "panel (opens in browser)"
		}
		items[i] = paletteItem{Label: label, Detail: id + " · " + d}
	}
	m.extAsk(func() {
		m.showItems(paletteItems{Title: "Open a view", Items: items,
			OnPick: func(i int, _ string, _ []int) {
				if i >= 0 {
					if m.Ext.views[ids[i]] {
						m.extGo("webview/openView", map[string]any{"viewId": ids[i]}, 30*time.Second, func(m *MainScreen, raw json.RawMessage, err error) {
							var r struct{ Error string }
							_ = json.Unmarshal(raw, &r)
							if err == nil && r.Error != "" {
								err = fmt.Errorf("%s", r.Error)
							}
							if err != nil {
								m.extNote("error", err.Error())
							}
						})
					} else {
						m.extTree(ids[i], "", items[i].Label)
					}
				}
				m.extNextPrompt()
			},
			OnCancel: m.extNextPrompt,
		})
	})
	return nil
}

func (m *MainScreen) extTree(viewID, handle, title string) {
	wanted := m.extLoading(title, "loading")
	m.extGo("tree/children", map[string]any{"viewId": viewID, "handle": handle}, 30*time.Second, func(m *MainScreen, raw json.RawMessage, err error) {
		if !wanted() {
			return
		}
		var r struct {
			Error string
			Items []struct {
				Handle, Label, Description, Tooltip, Path string
				Collapsible                               bool
				Command                                   *struct {
					Command string
					Args    []any
				}
			}
		}
		_ = json.Unmarshal(raw, &r)
		if err == nil && r.Error != "" {
			err = fmt.Errorf("%s", r.Error)
		}
		if err != nil {
			m.extLoaded()
			m.extNote("error", err.Error())
			return
		}
		if len(r.Items) == 0 {
			m.extLoaded()
			m.StatusBar = title + " is empty."
			return
		}
		items := make([]paletteItem, len(r.Items))
		for i, it := range r.Items {
			label := it.Label
			if it.Collapsible {
				label = "▸ " + label
			}
			items[i] = paletteItem{Label: label, Detail: strings.TrimSpace(it.Description + " " + it.Tooltip)}
		}
		m.extAsk(func() {
			m.showItems(paletteItems{Title: title, Items: items,
				OnPick: func(i int, _ string, _ []int) {
					if i >= 0 {
						it := r.Items[i]
						switch {
						case it.Collapsible:
							m.extTree(viewID, it.Handle, title+" › "+it.Label)
						case it.Command != nil:
							m.extGo("tree/command", map[string]any{"viewId": viewID, "handle": it.Handle}, 10*time.Minute, func(m *MainScreen, _ json.RawMessage, err error) {
								if err != nil {
									m.extNote("error", err.Error())
								}
							})
						case it.Path != "":
							m.openFileRaw(it.Path)
						}
					}
					m.extNextPrompt()
				},
				OnCancel: m.extNextPrompt,
			})
		})
	})
}

// ---------------------------------------------------------------- footer

// extFooter is the right-hand side of the footer: extension status items,
// progress, install progress, and the host's state.
func (m *MainScreen) extFooter() string {
	x := m.Ext
	if x == nil {
		return ""
	}
	var parts []string
	if x.installing != "" {
		parts = append(parts, theme.WordmarkStyle.Render(x.installing))
	}
	for _, p := range sortedValues(x.progress) {
		parts = append(parts, "◌ "+p)
	}
	if x.message != "" && (x.messageTTL.IsZero() || time.Now().Before(x.messageTTL)) {
		parts = append(parts, x.message)
	}
	items := make([]extStatus, 0, len(x.status))
	for _, s := range x.status {
		items = append(items, s)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].align != items[j].align {
			return items[i].align == "left"
		}
		if items[i].priority != items[j].priority {
			return items[i].priority > items[j].priority
		}
		return items[i].text < items[j].text
	})
	for _, s := range items {
		t := s.text
		if s.err {
			t = lipgloss.NewStyle().Foreground(theme.ErrorColor).Render(t)
		}
		parts = append(parts, t)
	}
	if n, w := m.problemCounts(); n+w > 0 {
		parts = append(parts, fmt.Sprintf("✖ %d ⚠ %d", n, w))
	}
	switch x.stopped {
	case "untrusted":
		parts = append(parts, "⏸ extensions: folder not trusted (alt+p)")
	case "node":
		parts = append(parts, "⏸ extensions need Node.js (alt+p)")
	case "crashed", "failed":
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.ErrorColor).Render("✖ extensions stopped"))
	}
	if x.starting {
		parts = append(parts, "◌ starting extensions")
	}
	return strings.Join(parts, "  ")
}

func sortedValues(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if m[k] != "" {
			out = append(out, m[k])
		}
	}
	return out
}

func (m *MainScreen) problemCounts() (errs, warns int) {
	for _, ds := range m.Ext.diags {
		for _, d := range ds {
			switch d.severity {
			case 0:
				errs++
			case 1:
				warns++
			}
		}
	}
	return
}

func humanCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.0fK", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

// ---------------------------------------------------------------- keybindings

// extKeybinding finds a contributed keybinding for key in the editor. Only
// keys a terminal can deliver are bound (alt+letter, ctrl+alt+letter,
// function keys), never one hittable already uses, and the binding's
// `when` clause must hold.
func (m *MainScreen) extKeybinding(key string) (string, any, bool) {
	if reservedKeys[key] {
		return "", nil, false
	}
	lang := ""
	if m.ActiveFile != "" {
		lang = extensions.LanguageID(m.ActiveFile)
	}
	for _, kb := range m.Ext.keybinds {
		k := kb.Key
		switch runtime.GOOS {
		case "darwin":
			if kb.Mac != "" {
				k = kb.Mac
			}
		case "linux":
			if kb.Linux != "" {
				k = kb.Linux
			}
		case "windows":
			if kb.Win != "" {
				k = kb.Win
			}
		}
		if teaKey(k) == key && m.whenHolds(kb.When, lang) {
			return kb.Command, kb.Args, true
		}
	}
	return "", nil, false
}

// keys the app owns in the editor (see §9 of CLAUDE.md).
var reservedKeys = map[string]bool{
	"alt+p": true, "alt+h": true, "alt+.": true, "alt+F": true, "alt+shift+f": true, "f2": true, "f8": true, "f12": true, "shift+f12": true,
	"alt+z": true, "alt+o": true, "alt+f": true, "alt+g": true, "alt+b": true, "alt+t": true, "alt+a": true, "alt+1": true, "alt+2": true, "alt+3": true,
	"f1": true, "f3": true, "f5": true,
}

// teaKey converts a VS Code key ("ctrl+alt+k", "f6", "alt+shift+x") to the
// string Bubble Tea reports, or "" for one a terminal cannot deliver
// (chords, cmd/meta, ctrl+shift).
func teaKey(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	if k == "" || strings.Contains(k, " ") || strings.Contains(k, "cmd") || strings.Contains(k, "meta") || strings.Contains(k, "win+") {
		return ""
	}
	parts := strings.Split(k, "+")
	base := parts[len(parts)-1]
	mods := map[string]bool{}
	for _, p := range parts[:len(parts)-1] {
		mods[p] = true
	}
	if mods["ctrl"] && mods["shift"] {
		return ""
	}
	if strings.HasPrefix(base, "f") && len(base) > 1 && base[1] >= '0' && base[1] <= '9' {
		if len(mods) > 0 {
			return ""
		}
		return base
	}
	if len([]rune(base)) != 1 {
		return ""
	}
	switch {
	case mods["alt"] && mods["ctrl"]:
		return "ctrl+alt+" + base
	case mods["alt"] && mods["shift"]:
		return "alt+" + strings.ToUpper(base)
	case mods["alt"]:
		return "alt+" + base
	}
	return "" // plain ctrl+x keys all belong to the app or the editor
}

// whenHolds evaluates the parts of a `when` clause the app knows:
// editorTextFocus, editorLangId/resourceLangId == x, resourceExtname,
// editorHasSelection, isMac/isLinux/isWindows, and context keys extensions
// set with setContext. Anything unknown is false, so a binding meant for a
// state the terminal never has does not fire.
func (m *MainScreen) whenHolds(when, lang string) bool {
	when = strings.TrimSpace(when)
	if when == "" {
		return true
	}
	for _, alt := range strings.Split(when, "||") {
		all := true
		for _, term := range strings.Split(alt, "&&") {
			if !m.whenTerm(strings.TrimSpace(term), lang) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func (m *MainScreen) whenTerm(t, lang string) bool {
	neg := strings.HasPrefix(t, "!")
	t = strings.TrimSpace(strings.TrimPrefix(t, "!"))
	val := func(key string) any {
		switch key {
		case "editorTextFocus", "editorFocus", "textInputFocus":
			return m.Focus == FocusTextEditor && !m.ExplorerFocused
		case "editorLangId", "resourceLangId":
			return lang
		case "resourceExtname":
			return filepath.Ext(m.ActiveFile)
		case "editorHasSelection":
			return m.TextEd.HasSelection()
		case "isMac":
			return runtime.GOOS == "darwin"
		case "isLinux":
			return runtime.GOOS == "linux"
		case "isWindows":
			return runtime.GOOS == "windows"
		case "true":
			return true
		case "false":
			return false
		}
		if v, ok := m.Ext.contexts[key]; ok {
			return v
		}
		return nil
	}
	var res bool
	switch {
	case strings.Contains(t, "!="):
		k, v, _ := strings.Cut(t, "!=")
		res = fmt.Sprint(val(strings.TrimSpace(k))) != strings.Trim(strings.TrimSpace(v), "'\"")
	case strings.Contains(t, "=="):
		k, v, _ := strings.Cut(t, "==")
		res = fmt.Sprint(val(strings.TrimSpace(k))) == strings.Trim(strings.TrimSpace(v), "'\"")
	default:
		v := val(t)
		switch x := v.(type) {
		case bool:
			res = x
		case string:
			res = x != ""
		case nil:
			res = false
		default:
			res = true
		}
	}
	return res != neg
}

// ---------------------------------------------------------------- the Extensions panel

// toggleExtensions opens or closes the Extensions panel (alt+x, the navbar).
func (m *MainScreen) toggleExtensions() {
	if m.ExtOpen {
		m.ExtOpen = false
		if m.ActiveFile != "" {
			m.focusCurrent()
		} else {
			m.setExplorerFocused(true)
		}
		return
	}
	m.openExtensions(m.ExtPanel.Query)
}

// openExtensions shows the panel with a search already typed ("" for the
// installed + popular lists, "@installed" for only what is installed).
func (m *MainScreen) openExtensions(query string) {
	m.closePanels()
	m.ExtOpen = true
	m.blurAll()
	m.setExplorerFocused(false)
	m.wireExtPanel()
	m.ExtPanel.Retransmit()
	m.ExtPanel.FocusDetail = false
	if query != m.ExtPanel.Query {
		m.ExtPanel.Query = query
		m.pendingCmd = tea.Batch(m.pendingCmd, m.ExtPanel.SetQuery(query))
	}
	m.ExtPanel.Open()
}

// wireExtPanel connects the panel's buttons to the same install, enable and
// uninstall paths the command palette uses.
func (m *MainScreen) wireExtPanel() {
	p := m.ExtPanel
	if p.Actions.State != nil {
		return
	}
	p.Actions = extpanelActions(m)
}
