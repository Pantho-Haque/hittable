# hittable.sh — Full Requirements & Architecture Spec

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
- **Vim editor**: `github.com/kujtimiihoxha/vimtea` for modal text editing
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
| Toggle Explorer       | `ctrl+b`        | Global               |
| Send Request          | `ctrl+r`        | Runner mode, main pane |
| Save                  | `ctrl+s`        | Global               |
| Toggle Runner/Text    | `ctrl+t`        | .hit file open       |
| Search Response       | `ctrl+f`        | Main pane            |
| Close File            | `esc`           | Main pane (not in vim insert/visual mode) |
| Next Tab Pane         | `tab`           | Main pane            |
| Prev Tab Pane         | `shift+tab`     | Main pane            |
| Move Up               | `↑` / `k`       | Explorer             |
| Move Down             | `↓` / `j`       | Explorer             |
| Open / Expand         | `enter`         | Explorer             |
| Context Menu          | `x`             | Explorer             |
| New File              | `ctrl+n`        | Explorer             |
| New Folder            | `ctrl+shift+n`  | Explorer             |
| Rename                | `ctrl+e`        | Explorer             |
| Delete                | `ctrl+d`        | Explorer             |
| Copy as Curl          | `ctrl+shift+c`  | Runner mode          |
| Help                  | `?`             | Global               |
| Select Method         | `enter`         | URL bar focused      |
| Method Dropdown Up    | `↑` / `k`       | Method dropdown open |
| Method Dropdown Down  | `↓` / `j`       | Method dropdown open |
| Search Next           | `enter` / `n`   | Response search open |
| Search Prev           | `shift+enter`   | Response search open |
| Delete Confirm        | `y` / `n`       | Delete confirmation  |
| Rename Confirm        | `enter`         | Inline rename        |
| Add Confirm           | `enter`         | Inline add file/folder |
| Click Tab             | `mouse left`    | Tab bar (Params/Headers/Body) |
| Click Toggle          | `mouse left`    | Runner/Text toggle   |
| Click Method Badge    | `mouse left`    | Method dropdown badge |
| Click Search Icon     | `mouse left`    | Response panel header |

---

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
