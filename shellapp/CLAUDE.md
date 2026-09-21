[<64;61;32M[<64;61;32M[<65;46;31M# hittable.sh — Full Requirements & Architecture Spec

This is the complete, self-contained source of truth for `hittable.sh`, a terminal-based
(TUI) API client written in Go. Read this doc top to bottom before writing any code — it
supersedes any prior draft. An agent picking this up cold should be able to build the
entire app from this file alone.

---

## 1. What this app is

`hittable.sh` is a **directory-mode-only** terminal API client, in the spirit of
`posting.sh`, built for testing API contracts from the command line. There is no global
store, no "modes to switch between," and no concept of opening the app without a working
directory. Launching the binary **is** opening a directory — full stop.

```
cd myproject
hittable          # opens Directory Mode against the current directory
# or
hittable ~/code/myproject   # opens Directory Mode against an explicit path
```

That's the entire invocation surface. There is no `hittable open <file>`, no separate
"global mode," no alternate storage backend to pick between. One mode, one behavior.

### 1.1 File-format parity with the web app

`hittable.sh` and the existing browser-based Hittable web app must be able to open the
**same project folder** interchangeably. Every file this app creates or edits — `.hit`
files and `env.json` — uses plain JSON (`encoding/json`), matching the web app's format
exactly, field for field. This is a hard constraint, not a preference.

---

## 2. Directory scaffolding

On launch, resolve the working root (CWD or the given path) and check it for a `hittable/`
subfolder:

- **Present** → use it as-is, no changes.
- **Absent** → create it, idempotently, with this exact default structure:

```
<root>/
└── hittable/
    ├── testcollection/
    │   └── test.hit
    ├── notes/
    │   └── sample.md
    └── env.json
```

**`test.hit`** — a real, immediately-runnable request (not an empty stub):

```json
{
  "method": "GET",
  "url": "https://jsonplaceholder.typicode.com/posts/1",
  "headers": {
    "Content-Type": "application/json"
  },
  "params": {},
  "body": "",
  "response": null
}
```

**`env.json`** — flat key/value JSON, nothing nested:

```json
{
  "BASE_URL": "https://jsonplaceholder.typicode.com",
  "AUTH_TOKEN": ""
}
```

**`notes/sample.md`**:

```markdown
# Notes

Write your notes here.
```

Scaffolding must never overwrite an existing `hittable/` folder or any file inside it —
it only fills in what's missing on a genuinely fresh directory.

---

## 3. `.hit` file schema (canonical, shared with the web app)

```json
{
  "method": "PATCH",
  "url": "https://jsonplaceholder.typicode.com/posts/3",
  "headers": {
    "Content-Type": "application/json"
  },
  "params": {},
  "body": "{\n\t\"title\": \"Partially Updated\"\n}",
  "response": {
    "data": {
      "userId": 1,
      "id": 3,
      "title": "Partially Updated23",
      "body": "et iusto sed quo iure\nvoluptatem occaecati omnis eligendi aut ad\nvoluptatem doloribus vel accusantium quis pariatur\nmolestiae porro eius odio et labore et velit aut"
    },
    "status": 200,
    "statusText": "OK",
    "ok": true,
    "headers": {
      "content-type": "application/json; charset=utf-8",
      "cache-control": "no-cache"
    },
    "cookies": {},
    "durationMs": 546,
    "sizeBytes": 226
  }
}
```

Rules:
- `headers` and `params` are flat JSON objects (string → string), never arrays.
- `body` is a raw string — may itself contain escaped JSON, or be empty.
- `response` is `null` until the request has been sent at least once from this file.
- `<<KEY>>` tokens anywhere in `url`, `headers` values, `params` values, or `body` resolve
  against `env.json`. `env.json` is global: any `.hit` file anywhere under `hittable/`,
  in any nested folder, can reference any key in it.
- `env.json` itself is just a flat `map[string]string` — no schema beyond that.

---

## 4. Layout & navigation

### 4.1 Screen layout

```
┌ /myproject ──────────────────┬───────────────────────────────────────────────────────┐
│ 📁 myproject                 │  PATCH ▾   https://jsonplaceholder.typicode.com/posts/3│
│  ▾ 📁 hittable                │ ┌────────┬─────────┬──────┐        [ Runner | Text ]  │
│    ▾ 📁 testcollection        │ │ Params │ Headers │ Body │             (⌘⏎ Send)     │
│      ⚡ test.hit  ◀ active    │ └────────┴─────────┴──────┘                           │
│    ▸ 📁 notes                 │  Content-Type: application/json                        │
│    🔧 env.json                │                                                        │
│  ▸ 📁 src                     │ ────────────────────────────────────────────────────── │
│  📄 go.mod                    │  Response · 200 OK · 546ms · 226B                      │
│  📄 README.md                 │  {                                                     │
│                                │    "userId": 1,                                       │
│                                │    "id": 3,                                           │
│                                │    "title": "Partially Updated"                       │
│                                │  }                                                    │
├────────────────────────────────┴──────────────────────────────────────────────────────┤
│ ctrl+B focus explorer · ↑↓ navigate · ⏎ open/expand · tab switch · ctrl+⏎ send       │
└─────────────────────────────────────────────────────────────────────────────────────────┘
```

- **Explorer header**: `/<root folder name>` — a literal leading slash followed by the
  name of the directory the app was launched against (e.g. `/myproject`). Not the word
  "Explorer" anywhere.
- **Explorer body**: the entire working root, recursively, exactly like a VS Code file
  tree — folders and files interleaved per their actual on-disk order, distinct icons for
  `.hit` / `.md` / `.json` / generic files, `📂`/`📁` chevrons on folders to show
  expanded/collapsed state. File icons: `⚡` for `.hit`, `📝` for `.md`, `🔧` for
  `env.json`, `📋` for `.json`, `📄` for generic.
- **Main pane**: file path breadcrumb → URL bar (method dropdown + interpolated URL) →
  Params/Headers/Body tab bar with a Runner/Text toggle in the corner (`.hit` files only)
  → response panel (status/timing/size + Chroma-highlighted JSON body with search).
- **Footer**: live key hints for whatever's focused.

### 4.2 Navigation model

- **Mouse**: clicking any file in the Explorer selects **and opens** it in one action
  (single click, no double-click, no delay). Clicking a folder toggles its expand/collapse
  state. Bubble Tea's mouse-all-motion mode is enabled app-wide so this works natively in
  the terminal. Hover feedback highlights the element under the cursor.
- **Draggable separator**: the `│` between Explorer and main pane is clickable and
  draggable — mouse-down + drag resizes the Explorer width; release commits. Width is
  clamped between 15 and `terminal_width - 20`.
- **`Ctrl+B`** toggles focus to the Explorer pane. Pressing it again while the Explorer
  is already focused toggles back to the last-focused main pane element — sidebar focus is
  a toggle, not a one-way jump.
- **While the Explorer is focused**: `↑`/`↓` (or `j`/`k` as a vim-style alias) move the
  selection cursor up/down through the currently-visible (i.e. respecting collapsed
  folders) flattened list of nodes.
- **`Enter` on a folder**: toggles it open/closed, accordion-style — expanding reveals its
  children inline in the tree (pushing everything below it down), collapsing hides them.
  Does not move focus out of the Explorer.
- **`Enter` on a file**: opens it in the main pane. Plain files open in the generic text
  editor. `.hit` files open in **Runner view** by default (URL bar + tabs + response
  panel), with the `[ Runner | Text ]` toggle available to switch to raw JSON.
- **`Tab`** cycles focus between the Explorer and the main pane's sub-regions (URL bar →
  tab bar → body/response) when the main pane has focus.
- **`x`** opens a context menu on the currently cursor-highlighted node (New File, New
  Folder, Rename, Delete). Also accessible via right-click on the node.
- **`Ctrl+T`** toggles between Runner and Text view for `.hit` files.
- **`Ctrl+F`** opens search in the response panel.
- **`Enter` on URL bar** opens the method dropdown (GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS).
- There is no double-click and no double-press anywhere in the open/expand flow, on mouse
  or keyboard.

---

## 5. Autosave & state consistency

Every edit — in Runner view or Text view, on a `.hit` file, `env.json`, or any plain file
— commits to the in-memory model immediately and is persisted to disk without an explicit
save action, and without ever losing or resetting content when switching files or view
modes. This is architected in from the start:

- **One `DocumentModel` per open file path**, created the first time a path is opened in
  the session and kept in an in-memory `Store` for the rest of the session. Re-opening an
  already-open path reuses its model — it is **never** re-read from disk mid-session.
- A single `document.Update(path, fn)` function is the only way any view mutates a
  document's content: lock → apply `fn` → increment a `generation` counter → unlock →
  enqueue a debounced (~150–300ms), per-path serialized disk write carrying that
  generation and a content snapshot.
- At flush time, if a newer generation exists for that path than the one this write is
  carrying, the write is dropped — a fresher one is already queued behind it. This is what
  prevents a slow/stale write from ever clobbering a newer edit.
- **Runner view and Text view for `.hit` files both operate on the same underlying
  struct** — Text view renders a live `json.MarshalIndent` of it and parses keystrokes back
  in when valid; Runner view patches individual fields directly. There is exactly one copy
  of a file's content in memory at any time; switching view modes via `Ctrl+T` is a
  content transformation, never a reload.
- `env.json` follows the identical `DocumentModel`/write-queue pattern — no special-casing.
- The Explorer's tree structure (names/paths/kind/expanded-state) is entirely separate
  from `DocumentModel` and never holds file content.

---

## 6. Tech stack

- **Go 1.23+**
- **TUI**: `github.com/charmbracelet/bubbletea` (with `tea.WithMouseAllMotion()`
  enabled for true hover tracking), `github.com/charmbracelet/bubbles` (textinput, textarea,
  viewport, table), `github.com/charmbracelet/lipgloss`
- **Mouse hit-testing**: `github.com/lrstanley/bubblezone` for precise zone-based click detection
- **Syntax highlighting**: `github.com/alecthomas/chroma` for JSON body/response
- **Code editor**: `bubbles/textarea` as the buffer + a custom Chroma (Dracula) renderer
- **File icons**: `github.com/epilande/go-devicons` for Nerd Font file type icons
- **CLI entry**: `github.com/spf13/cobra` — a single root command; no subcommands needed
  beyond accepting an optional path argument
- **Storage format**: standard library `encoding/json` only — no YAML anywhere in this
  app
- **File watching** (optional, for detecting external edits made outside the app while
  it's running): `github.com/fsnotify/fsnotify`
- **Clipboard** (copy request as curl): `github.com/atotto/clipboard`

## 7. Package layout

```
cmd/hittable/
└── main.go                    # entrypoint; resolves root (CWD or arg), launches TUI

internal/
├── rootdir/
│   └── rootdir.go              # resolves & validates the working root path
├── scaffold/
│   ├── scaffold.go             # idempotent hittable/ + default file creation
│   └── scaffold_test.go        # idempotency + existing-dir tests
├── explorer/
│   └── tree.go                 # FileNode tree, VisibleNodes, Icon (go-devicons),
│                                # FindNode, RemoveNode, AddChild, Indent
├── hitfile/
│   ├── schema.go                # Go struct for .hit, json tags matching §3 exactly
│   ├── schema_test.go           # round-trip marshal/unmarshal test
│   ├── interpolate.go           # <<KEY>> resolution against env.json
│   └── request.go               # .hit -> net/http request, ExecuteAndCapture, AsCurl
├── envfile/
│   ├── envfile.go               # env.json load/save, flat map[string]string,
│   │                            # Interpolate, LoadContent, SaveContent
│   └── envfile_test.go          # interpolation, load/save, missing-file tests
├── document/
│   ├── model.go                  # DocumentModel + generation counter + ViewMode
│   ├── store.go                  # in-memory Map[path]*DocumentModel + Delete
│   ├── writequeue.go             # per-path debounced, generation-guarded disk writer
│   └── document_test.go          # write-queue race, concurrency, thread-safety tests
└── httpclient/
    └── client.go                  # executes a request, captures timing + response shape

ui/
├── theme/
│   └── theme.go                 # dark theme, cyan accents, all Lip Gloss styles
├── keymap/
│   └── keymap.go                 # centralized keymap: every action defined once
├── components/
│   ├── explorer/
│   │   ├── model.go              # ExplorerComponent struct, New, SetSize, RebuildTree
│   │   ├── update.go             # Update, keyMap, keyboard navigation
│   │   ├── view.go               # View rendering, context menu overlay
│   │   ├── hover.go              # Click, Hover, scroll helpers
│   │   └── contextmenu.go        # Context menu state, rename, add, delete
│   ├── requesteditor/
│   │   ├── urlbar.go              # method dropdown + URL input
│   │   ├── paramstab.go           # JSON params editor with validation
│   │   ├── headerstab.go          # JSON headers editor with validation
│   │   ├── bodytab.go             # raw body textarea
│   │   └── tabs_test.go           # JSON round-trip, invalid indicator, pretty-print tests
│   ├── responseviewer/
│   │   ├── model.go               # ResponseViewer struct, New, SetSize
│   │   ├── search.go              # Search overlay, doSearch, Next/PrevMatch
│   │   └── view.go                # View rendering, Chroma highlighting, search icon
│   └── texteditor/
│       └── texteditor.go          # vimtea-based editor with status bar
├── screens/
│   ├── model.go                   # MainScreen struct, NewMainScreen, SetSize
│   ├── update.go                  # Update, handleKey, handleMouse, focus routing
│   ├── view.go                    # View, renderRunnerView, renderTabs, renderFooter
│   ├── helpers.go                 # Interpolation, file ops, save/send logic
│   ├── screens_test.go            # Tab click, toggle, Esc, method select, search icon tests
│   └── persist_test.go            # Live persistence tests for params/headers/body
└── app.go                          # root Bubble Tea model, bubblezone manager
```

## 8. Build plan (phases)

**Phase 0 — Root resolution & CLI**
`cmd/hittable/main.go` resolves the working root from `os.Args` (optional path) or CWD,
validates it's a real directory, hands off to `scaffold` then launches the TUI. Uses
`tea.WithMouseAllMotion()` for true hover tracking.

**Phase 1 — Scaffolding**
`scaffold.Ensure(root)`: idempotent creation of `hittable/`, `testcollection/test.hit`,
`notes/sample.md`, `env.json` with the exact default content in §2. Unit tests: running
twice in a row produces no changes on the second run; running against a root that already
has a `hittable/` folder never touches existing files.

**Phase 2 — Data layer**
`hitfile`: struct + JSON marshal/unmarshal matching §3 exactly; round-trip test against
the sample payload (unmarshal → remarshal → structurally equal). `envfile`: flat map
load/save + `<<KEY>>` interpolation, unit tested against `url`, `headers`, `params`, and
`body` fields.

**Phase 3 — HTTP engine**
Execute a resolved request via `net/http`, capture timing (`durationMs`) via
`net/http/httptrace`, compute `sizeBytes`, and populate the exact `response` shape from
§3 (`data`, `status`, `statusText`, `ok`, `headers`, `cookies`, `durationMs`,
`sizeBytes`).

**Phase 4 — Document model & write-through**
Implement `document.Store`, `DocumentModel`, and the generation-guarded write queue from
§5. Explicit race test: fire two rapid `document.Update` calls on the same path before
the first debounce flushes; assert only the final content ever reaches disk, and that no
intermediate write occurs.

**Phase 5 — Explorer**
Whole-root recursive tree with real on-disk ordering, expand/collapse state per folder,
distinct icons per file kind, `/<root folder name>` header (literal leading slash, no
"Explorer:" label). Mouse click selects-and-opens a file or toggles a folder in one
action. Arrow-key (and `j`/`k`) navigation through the flattened visible-node list when
focused. `Enter` on a folder = accordion toggle, no focus change. `Enter` on a file =
open in main pane. `Ctrl+B` toggles focus into/out of the Explorer. Hover feedback on
all items. Context menu via `x` key or right-click.

**Phase 6 — Request editor + response panel**
URL bar with method dropdown (Enter to open, arrows to navigate, Enter to select, Esc to
close) and live `<<KEY>>` interpolation. Params/Headers/Body tabs. Send action (`Ctrl+Enter`).
Response panel with Chroma JSON highlighting and search overlay (`Ctrl+F`).

**Phase 7 — Generic text editor**
Plain-text buffer for any non-`.hit` file, same write-through autosave via
`document.Update`.

**Phase 8 — Polish & QA gate**
- `go build ./...` and `go test ./...` pass.
- All 11 maintenance items verified (see §9).

---

## 9. Keybinding reference (current, canonical)

All bindings use `Ctrl+<key>` for cross-platform compatibility. macOS terminals that
support the Kitty keyboard protocol may additionally receive `Cmd+<key>`, but only
`Ctrl+<key>` is guaranteed to work everywhere. Bubbletea v1.3.10 does not support the
Kitty keyboard protocol, so `Cmd+Enter` is not available — `Ctrl+R` is the guaranteed
send binding.

| Action                | Binding         | Context              |
|-----------------------|-----------------|----------------------|
| Quit                  | `ctrl+c`        | Global               |
| Help                  | `?` / `F1`      | Explorer / Global    |
| Toggle Explorer       | `ctrl+b`        | Global               |
| Send Request          | `ctrl+r` / `ctrl+⏎` | Global (.hit open) |
| Save now              | `ctrl+s`        | Global (autosave is always on) |
| Toggle Runner/Text    | `ctrl+t`        | Global (.hit open)   |
| Copy as curl / body   | `ctrl+y`        | Runner / Response    |
| Close File            | `esc`           | Main pane            |
| Next / Prev pane      | `tab` / `shift+tab` | Main pane        |
| Move Up / Down        | `↑`/`k`, `↓`/`j` | Explorer            |
| Top / Bottom          | `g` / `G`, `home` / `end` | Explorer, Response |
| Page Up / Down        | `pgup`/`ctrl+u`, `pgdown`/`ctrl+d` | Explorer, Response |
| Open / Expand         | `enter` / `l`   | Explorer             |
| Collapse / Parent     | `h`             | Explorer             |
| Context Menu          | `x` / right-click | Explorer           |
| New File / Folder     | `ctrl+n` / `ctrl+f` | Explorer         |
| Rename / Delete       | `ctrl+e` / `ctrl+d` | Explorer         |
| Refresh tree          | `r`             | Explorer             |
| Method picker         | `enter`         | URL bar focused      |
| Cycle method          | `ctrl+←` / `ctrl+→` | URL bar focused  |
| Pick method           | `←/→`, first letter, `enter` | Picker open |
| Switch tab            | `←/→`, `1/2/3`  | Tab bar focused      |
| Switch tab            | `alt+1/2/3`     | Editor focused       |
| Format body JSON      | `ctrl+l`        | Body tab focused     |
| Search Response       | `ctrl+f`        | Runner view          |
| Search Next / Prev    | `enter`/`n`, `shift+enter`/`N` | Search |
| Response headers      | `h`             | Response focused     |
| Delete Confirm        | `y` / `n`       | Delete confirmation  |
| Editor: undo / redo   | `ctrl+z` / `ctrl+y` | Text editor      |
| Editor: find / next   | `ctrl+f` / `⏎`, `F3` | Text editor     |
| Editor: go to line    | `ctrl+g`        | Text editor          |
| Editor: word jump     | `ctrl+←/→`, `alt+←/→` | Text editor    |
| Editor: indent        | `tab` (2 spaces) | Text editor         |
| Editor: leave         | `shift+tab`     | Text editor → Explorer |
| Editor: select        | drag, `shift+←→↑↓`, double-click word, `ctrl+a` | Any editor |
| Editor: copy/cut/paste| `ctrl+c` / `ctrl+x` / `ctrl+v` | Any editor (ctrl+c quits only without a selection) |
| Send (mouse)          | click `▶ Send`  | URL bar              |
| Terminal toggle/focus | `ctrl+j`, `ctrl+\``, click strip | Global (`ctrl+j` is the reliable one; ctrl+` only in terminals that emit NUL for it) |
| Terminal leave        | `ctrl+b`        | Terminal focused (all other keys, incl. ctrl+c, go to the shell) |
| Git panel             | `ctrl+g` (outside editors), `alt+g`, `F5`, `g` in explorer, click `⎇ Git` | Global |
| Git: sections         | `1-5`, `tab`, click | Git panel: Status · Commits · Branches · Stashes · Blame |
| Git: status           | `+`/`s` `−`/`u` `a` `A` `e` `v` `d` `c` `S` `p` `P` `f` `⏎` `/` | stage · unstage · stage all · unstage all · edit in preview · inline⇄split · discard · commit · stash · push · pull · fetch · open/collapse · filter |
| Markdown view         | `ctrl+t` cycles Text → Preview → Split; click `[ Text | Preview | Split ]` | .md file open |
| Preview scroll        | `jk` `↑↓` `pgup/pgdown` `g/G`, wheel | Preview focused (tab ⇄ editor in split) |
| Find file             | `ctrl+p`, `/` in explorer, click `Find` | Global (fuzzy, skips node_modules etc.) |
| Live grep             | `alt+f`, `tab` inside the palette | Global (ripgrep → git grep → walk) |
| Git: commits          | `f` `/` `y` `J/K` | file↔repo history · search · hash · scroll diff |
| Git: branches         | `⏎` `n` `d` `f` | checkout · new · delete · fetch |
| Git: stashes          | `⏎` `s` `d`     | pop · stash · drop |
| Git: blame            | `⏎` `b`         | go to line · toggle inline blame in editor |
| Click                 | mouse left      | Everything: files, folders, tabs, toggle, method, URL cursor, editor cursor, response, body/headers label |
| Scroll                | wheel           | Explorer, editor, response |
| Resize explorer       | drag `│`        | Separator            |

## 10. Changelog

### v1.0 — Initial build (all phases 0–8)
- Built from scratch: CLI, scaffolding, data layer, HTTP engine, document model,
  explorer, request editor, response viewer, text editor.
- `go build`, `go test`, `go vet` all pass.
- Scaffold idempotency verified.
- `.hit` JSON round-trip verified.
- Write-queue race condition tested.

### v1.1 — Maintenance & upgrade pass (11 items)
1. **Enter key fix**: cursor always initialized to valid index; single cursor state for
   both keyboard and mouse.
2. **Mouse click accuracy**: single `y - 1 + scrollStart` mapping for click, hover, and
   renderer. Draggable `│` separator for explorer width.
3. **Hover feedback**: switched to `tea.WithMouseAllMotion()` for true hover tracking.
   `HoverStyle` background on hovered items.
4. **Improved icons**: `📂`/`📁` folders, `⚡` .hit files, `📝` .md, `🔧` env.json,
   `📋` .json, `📄` generic.
5. **Context menu**: `x` key or right-click → New File, New Folder, Rename, Delete.
   Inline confirmations. New .hit files get default scaffold shape.
6. **File path breadcrumb**: `BreadcrumbStyle` shows relative path at top of Runner and
   Text mode.
7. **Runner ↔ Text toggle**: `Ctrl+T` flips `ViewMode`. Content serialized/deserialized
   on mode switch.
8. **Centralized keymap**: all bindings in `ui/keymap/keymap.go`. Send is `Ctrl+Enter`.
   Footer shows `ctrl+` notation.
9. **Response search**: `Ctrl+F` opens overlay. Live match highlighting. Next/prev
   navigation. Match counter.
10. **Method dropdown**: `Enter` on URL bar opens GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS.
    Arrow keys navigate, Enter selects, Esc closes.
11. **Mouse support audit**: all clickable elements support mouse. Hover styling via
    shared `HoverStyle`.

### v1.2 — Restructure & feature pass
1. **Module restructure**: all files over 200 lines split by responsibility into
   `<name>_model.go`, `<name>_update.go`, `<name>_view.go`, `<name>_helpers.go`.
   Explorer split into model/update/view/hover/contextmenu. Response viewer split into
   model/search/view.
2. **Text editor**: replaced bubbles/textarea with vimtea (Vim-style modal editor).
   Status bar shows mode (NORMAL/INSERT/VISUAL). App-level shortcuts (ctrl+b, ctrl+t,
   ctrl+s, ctrl+r, ctrl+f) pass through in normal mode but not in insert mode.
3. **Text editor scroll indicator**: "Ln X/Y" position indicator at bottom of editor.
4. **Explorer icons via go-devicons**: Nerd Font glyphs for file types with fallback to
   emoji. Custom icons for `.hit` (⚡) and `env.json` (🔧). Folder open/closed
   chevrons (📂/📁) preserved.
5. **Right-click context menu**: `tea.MouseRight` triggers context menu at click position.
   Same menu as `x` key — New File, New Folder, Rename, Delete.
6. **Send binding**: both `ctrl+enter` and `ctrl+r` trigger send. Footer updated to show
   `ctrl+r send`.

### v1.3 — Mouse control & Esc behavior
1. **Hit region system**: new `ui/hitregion/` package with shared registry for clickable
   UI regions. Components register bounding boxes during View(), main screen dispatches
   mouse clicks during Update().
2. **Clickable tab bar**: Params/Headers/Body tabs now respond to mouse clicks with hover
   highlighting.
3. **Clickable Runner/Text toggle**: click "Runner" or "Text" to switch directly (not just
   toggle). Active mode highlighted, hover feedback on both.
4. **Clickable method badge**: click the method badge (e.g. "GET ▾") to open the method
   dropdown, same as Enter on URL bar.
5. **Hover highlighting**: all clickable elements show HoverStyle on mouse hover.
6. **Esc to close file**: Esc closes the open file and returns focus to Explorer. Respects
   vimtea modal state — Esc in INSERT/VISUAL mode only drops to NORMAL, doesn't close file.
   Esc also closes context menus, dropdowns, search overlays before closing files (innermost
   layer first).
7. **Cmd+Enter note**: Bubbletea v1.3.10 has no Kitty keyboard protocol support, so
   Cmd+Enter is not distinguishable from Enter. Ctrl+R remains the guaranteed send binding.
   Footer and spec doc updated to reflect this.

### v1.4 — Search fix, method dropdown, live edits, search icon
1. **Search highlighting fix**: Rewrote search highlight compositing to find all matches
   in raw text first (line, col, length), then apply highlights at correct visual
   positions in ANSI-escaped output. Prevents offset mismatches between Chroma escape
   codes and raw text coordinates.
2. **Method dropdown clickable**: Each method option (GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS)
   is now registered as a hitregion. Clicking a method selects it and closes the dropdown.
   Dropdown regions are deregistered when closed.
3. **Live document updates**: Params/Headers/Body tab edits now trigger debounced
   `saveAndEnqueue` on every keystroke, ensuring the DocumentModel stays in sync without
   requiring Ctrl+R. 300ms debounce prevents excessive disk writes.
4. **Response search icon**: 🔍 glyph rendered in the response panel header row, opening
   the same search overlay as Ctrl+F.
5. **Text editor improvements**: Added `GetCursorRow()` helper. vimtea mode tracking
   (InInsert/InVisual) preserved for Esc precedence logic.
6. **Tests added**: methodSelectMsg, searchIconClickMsg, debounceSaveMsg, search highlight
   correctness (verifies "ve" matches "veniam" not "title").

### v1.5 — BubbleZone, JSON params/headers, persistence fix
1. **BubbleZone migration**: Replaced hand-rolled `ui/hitregion/` registry with
   `github.com/lrstanley/bubblezone`. Each component marks its rendered output with
   `z.Mark(id, content)` (instance method, not global). Mouse click/hover detection uses
   `z.Get(id).InBounds(msg)`. Deleted the old hitregion package entirely. Fixed initial
   panic caused by calling global `zone.Mark` instead of the manager instance.
2. **Params/Headers JSON format**: Tabs now edit content as raw JSON (`{"key": "value"}`)
   instead of key=value lines. Added `InvalidJSON` indicator (⚠ invalid JSON) shown when
   the buffer contains invalid JSON. Pretty-printed on load. No validation-blocking on
   keystrokes — invalid JSON is kept in the buffer but not written to DocumentModel.
3. **Persistence fix**: `saveAndEnqueue` now reads directly from textarea components'
   `GetContent()` methods on every debounced save. Params/Headers `GetContent()` returns
   `nil` when JSON is invalid (preventing broken writes). Body tab passes through raw
   content. Integration tests verify disk content matches what was typed.
4. **Tests added**: JSON round-trip, invalid JSON indicator, pretty-print validation,
   live persist for params/headers/body (type → save → read disk → assert).

### v1.6 — Bug-fix pass
1. **Templates preserved**: `<<KEY>>` values are no longer interpolated into the
   editor widgets and written back to disk; resolution happens only at send time.
   `.hit` files are written without HTML escaping (`<<` stays `<<`, not `\u003c`).
2. **No lost edits**: the outgoing file is committed before any file switch, close,
   delete, rename, or quit. Re-opening a path reuses the in-memory document instead
   of re-reading disk. Pending writes are cancelled on delete / flushed before rename.
3. **Explorer**: directories load lazily on expand (startup no longer walks
   `node_modules`/`.git`); folders toggle on single click; rename actually renames
   and "new folder" makes a directory (both used to create an empty file); `x` /
   right-click open a working context menu; rows are width-clamped.
4. **Mouse**: separator drag works; clicking the URL bar / editor / response focuses
   it and drops explorer focus; wheel scrolls the response and text editor.
5. **Layout**: frame is exactly terminal height at any size; footer truncates.
6. **Status bar**: send errors, "Saved", copy-as-curl, and file-op failures are now
   shown in the footer. `ctrl+y` copies the curl command to the clipboard.
7. **env.json** is edited as raw JSON (no lossy `KEY=value` transform); `EnvData`
   refreshes on save.
8. Query params are URL-encoded; response for a file that was switched away from is
   dropped instead of written into the wrong file.

### v1.7 — Production polish
1. **Performance**: response highlighting is computed once per response (was on
   every frame, including every mouse move); bodies over 256KB render plain so a
   1MB response lands in ~30ms. Frame render is ~1ms.
2. **Navigation**: `ctrl+r/s/t/y` and `F1` work from any focus. `alt+1/2/3` jump to
   Params/Headers/Body; `ctrl+←/→` cycle the method; `g/G`, `pgup/pgdown` in the
   explorer and response; `h` toggles response headers; `r` refreshes the tree;
   `?`/`F1` open a keyboard reference overlay. Footer hints are per-focus.
3. **Visuals**: method badge coloured per verb; status coloured 2xx/3xx/4xx+;
   response shows size, scroll percentage and body/headers mode; the method picker
   renders inline (no layout shift); the tab bar shows a focus marker and a ⚠ on
   tabs with invalid JSON; explorer cursor is bright when focused and dim otherwise;
   folders/.hit files are tinted; text editor has line numbers, cursor-line
   highlight and a Ln/Col indicator; a spinner shows while sending.
4. **Editing**: Params/Headers keep a fixed status row (invalid JSON never changes
   the layout, and the last valid value is kept); `ctrl+l` formats the body; the
   text editor scrolls with the mouse wheel; the URL bar accepts long URLs.
5. **Sending**: env.json is re-read before each send if not open in-app; URLs
   without a scheme get `http://`; double-send is guarded; duration includes the
   body read; non-JSON responses (HTML, text) display verbatim; the search overlay
   scrolls to the current match.

### v1.8 — VS Code-style editor, Dracula theme, floating menu
1. **Context menu** is a floating box anchored under the cursor row (above it near
   the bottom), composited over the tree instead of pushing rows down. Mouse hover
   moves the highlight; click selects; click elsewhere closes.
2. **Code editor** (`ui/components/texteditor`): bubbles/textarea remains the editing
   buffer, but the view is rendered by the package: Chroma syntax highlighting by
   file extension (`.hit` as JSON) with the Dracula style, line-number gutter with
   active-line emphasis, horizontal scroll instead of soft wrap, block cursor,
   click-to-position, wheel scroll, undo/redo (`ctrl+z`/`ctrl+y`, 200 steps), find
   (`ctrl+f`, `⏎` next, `F3`), go-to-line (`ctrl+g`), `tab` inserts two spaces,
   `ctrl+←/→` word jumps, status row `Ln, Col · language`. Per-line highlight cache
   keeps rendering O(visible lines).
3. **Dracula palette** across the app (`ui/theme`), response highlighting uses
   Chroma's `dracula` style.
4. Explorer hides `.git`, `.svn`, `.hg`, `.DS_Store` like VS Code's `files.exclude`.
5. Fira Code is a terminal font setting, not something the app can pick; set it in
   the terminal profile (see README note in v1.8 delivery).

### v1.9 — Mouse everywhere
1. **Runner tabs** (Params / Headers / Body) are now instances of the code editor:
   click-to-position, drag selection, wheel scroll, JSON highlighting, undo/redo,
   find, go-to-line. Validity hint lives in the editor's status row.
2. **Selection**: drag, shift+arrows, double-click word, `ctrl+a`; typing replaces
   the selection (one undo step); `ctrl+c` copies when a selection exists (otherwise
   quits, as before), `ctrl+x` cuts, `ctrl+v` pastes.
3. **URL bar**: clicking places the input cursor; a `▶ Send` button sends.
4. **Response**: the `body` / `headers` label is clickable.
5. Drag detection: bubbletea reports a drag as a left-button event with a motion
   action; the screen now distinguishes press vs. drag, which also makes the
   explorer separator drag work in real terminals.

### v2.0 — Integrated terminal + zone fix
1. **Root-cause fix for main-pane mouse**: `ui.App.Update` returned the inner
   `MainScreen` as the model, so bubbletea bypassed `App.View` (and its bubblezone
   `Scan`) after the first message — no main-pane zone ever registered in the real
   program. The App now returns itself. The explorer's hand-rolled hit-testing was a
   workaround for this same bug.
2. **End-to-end harness** (`ui/e2e`): runs the real program with piped input and a
   vt10x emulator on its output, injecting SGR mouse sequences and asserting on the
   rendered screen. Covers explorer clicks, editor click-to-position, tab clicks,
   and the terminal panel.
3. **Integrated terminal** (`ui/components/terminal`): `$SHELL -l` on a PTY
   (`creack/pty`), parsed by `hinshun/vt10x`, rendered with 16/256/true-colour and
   bold/italic/underline/reverse. A one-row `▸ TERMINAL` strip sits under the main
   pane; click it (or press ctrl+`) to open + focus, click again while focused to
   hide, click the panel to focus it. While focused every key goes to the shell
   (`ctrl+c` included); `ctrl+b` returns to the explorer; clicking the editor leaves.
   The panel takes a third of the height; the main pane shrinks accordingly. The
   shell is killed on quit; if it exits, any key restarts it. `ctrl+v` pastes.

### v2.1 — Icon pack
1. Explorer icons come from `go-devicons` (nvim-web-devicons port): Nerd Font glyph
   + colour per file name/extension, folder glyphs closed/open, `.hit` = bolt (cyan),
   `env.json` = cog (orange). Highlighted rows render as one styled run so the row
   background stays unbroken; plain rows colour the icon per type.
2. `--icons emoji` restores the emoji set for terminals without a Nerd Font.
   README documents installing FiraCode Nerd Font.

### v2.2 — Brand mark
1. `ui/theme/brand.go`: the web app's logo (rounded dark tile, bold italic "H") as a
   block-art tile, the `H I T T A B L E` wordmark in brand cyan `#00e5cc`, and the
   tagline. Shown centred on the welcome screen (wordmark-only on tiny terminals)
   and as the help overlay title.
2. `.hit` files use a one-cell "H" chip (white on the tile colour) as their icon in
   nerd mode, matching the web app's file identity.

### v2.2.1
- `ctrl+j` toggles the terminal (VS Code panel binding) — ctrl+` is not delivered
  by every macOS terminal. `HITTABLE_KEYLOG=<path>` logs delivered key names.

### v2.3 — Git (GitLens-style)
1. **Top navbar** (row 0): brand, root name, `⎇ Git`, `▤ Terminal`, `? Help` buttons
   and a branch badge (`⎇ main ●3 ↑1 ↓0`). Explorer and main column shift down one row.
2. **`internal/gitx`**: git CLI wrapper (status v2 with branch/ahead/behind, stage,
   unstage, discard, commit, push/pull/fetch, diff, show, log, blame porcelain,
   branches, checkout/create/delete, stashes push/pop/drop/show). Handles roots that
   are sub-directories and macOS symlinked paths via `--show-prefix`.
3. **`ui/components/gitpanel`**: main-pane panel with Status / Commits (repo or
   file history, search) / Branches / Stashes / Blame lists plus a detail pane
   (coloured unified diff, commit, stash, or the commit behind a blamed line).
   Prompts for commit message, branch name, stash message, filter; y/n confirms for
   discard, delete, drop, pop. Network ops run async with a busy indicator. Mouse:
   tabs, rows (click selects, click again acts), wheel on list or detail.
4. **Explorer decorations**: VS Code-style status colours and M/A/D/U/! badges on
   files, `●` on folders containing changes. Refreshed on open, save (rate-limited),
   explorer `r`, and after every git action.
5. **Blame in the editor**: `b` in the Blame section toggles a GitLens-style gutter
   annotation (author, age, summary) per line and a current-line blame note in the
   status row. `⏎` on a blame row jumps the editor to that line.
6. Not included (GitLens features without a terminal equivalent here): commit graph,
   worktrees, GitHub/GitLab integrations, interactive rebase editor, line history,
   compare refs, launchpad.

### v2.4 — Navbar, status groups, split diff, edit-in-preview, find/grep
1. **Navbar** restyled: brand block `H HITTABLE › project`, pill buttons (Git · Find ·
   Terminal · Help) with hover and active states, branch pill on the right with
   changed-file count and ahead/behind. Nerd Font icons in nerd mode, text fallback.
2. **Status section**: two collapsible groups — `▾ Staged Changes` and `▾ Changes`
   (untracked included). Each header has a right-hand `[ + stage all ]` /
   `[ − unstage all ]` button; every file row has `[ + ]` / `[ − ]`. One click (or
   `+`/`−`/`s`/`u`) moves the file; `⏎` on a header collapses it.
3. **Inline ⇄ side-by-side diff**: `v` or the `[inline split]` toggle in the panel
   header. Split view pairs removed/added runs with old/new line numbers.
4. **Edit in preview**: `e` on an unstaged, non-conflicting file replaces the diff
   pane with the code editor on the working copy; edits autosave through the
   document store (so an open editor tab stays in sync); `esc` returns to the diff.
   Staged or conflicted files refuse, matching the request.
5. **Find palette** (`ui/components/palette`): `ctrl+p` / `/` fuzzy file finder over
   the tree (skips node_modules, .git, .next, dist, build, vendor, target); `alt+f`
   live grep (ripgrep, then git grep, then a Go walk) with `path:line` results;
   `⏎` opens the file at the line; `tab` flips modes; results are async and stale
   queries are dropped.
6. Undo: verified through the real-terminal harness that `ctrl+z` (0x1a) undoes.
   `cmd+z` never reaches a terminal program on macOS unless the terminal maps it
   (iTerm2: Keys → Key Bindings → map ⌘Z to "Send Hex Codes: 0x1a").

### v2.5 — Terminal scrollback & paste, realtime git
1. **Scrollback**: the emulator only holds the visible screen, so rows are now
   captured as they scroll off (output is fed line by line; a screen shift is
   detected by comparing the new grid with the previous one, excluding the cursor
   row; alternate-screen apps are never captured). Wheel over the panel or
   `shift+↑/↓` / `shift+pgup/pgdown` scroll back (5000 lines); any key returns.
   The strip shows `↑ scrollback n/m` while scrolled.
2. **Paste**: writes to the PTY go through a dedicated writer goroutine (a large
   paste used to block the UI on the tty input buffer). Pasted text (terminal
   bracketed paste or `ctrl+v`) is sent as one bracketed block when the shell has
   enabled mode 2004 (zsh/bash do), so multi-line pastes are inserted, not executed
   line by line; newlines are normalised to CR.
3. **Realtime git**: a 2s tick runs `git status` off the UI goroutine; when the
   fingerprint changes the explorer badges, tree, top-bar badge, blame overlay and
   the open Git panel refresh. Covers changes made in the integrated terminal or
   outside the app.

### v2.7 — Markdown preview
1. **`ui/components/mdpreview`**: glamour (Dracula style, word-wrapped to the pane)
   renders headings, emphasis, lists, task lists, tables, block quotes, links and
   fenced code with syntax highlighting. ```mermaid fences are converted to
   box-drawing diagrams with `mermaid-ascii` (flowchart/graph TD·LR, sequence and
   ER diagrams); unsupported or broken diagrams fall back to the source block and a
   renderer panic is recovered. Output is cached per (content, width).
2. **Modes** for `.md` files: Text (editor), Preview (rendered, scrollable), Split
   (editor left, live preview right that follows the cursor while typing). `ctrl+t`
   cycles; the header toggle is clickable; the choice is remembered across files.
3. Dependencies added: `charmbracelet/glamour`, `AlexanderGrooff/mermaid-ascii`
   (only its `pkg/render` + `pkg/diagram` packages are compiled in).
