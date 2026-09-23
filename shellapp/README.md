# hittable.sh

A terminal API client. It opens a project directory, shows the whole tree, and runs the
`.hit` request files inside it. The file format is the same plain JSON the
[Hittable web app](../README.md) uses, so one folder opens in either.

```sh
hittable .            # current directory
hittable ~/code/app   # explicit path
```

Press `?` or `F1` inside the app for the keyboard reference.

---

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/Pantho-Haque/hittable/main/install.sh | sh
```

From source (Go 1.23+), from the repo root or from `shellapp/`:

```sh
make install
```

`make install` cross-compiles for the machine it runs on, copies the binary to
`~/.local/bin/hittable`, and overwrites any other `hittable` already on your `PATH` so a
stale copy can never shadow the new build. Make sure `~/.local/bin` is on your `PATH`:

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

The manual equivalent:

```sh
GOARCH=arm64 go build -o ~/.local/bin/hittable ./cmd/hittable   # Apple Silicon
GOARCH=amd64 go build -o ~/.local/bin/hittable ./cmd/hittable   # Intel Mac / Linux x86_64
```

`go install ./cmd/hittable` also works, but it writes to `~/go/bin` and builds for the
architecture of your Go toolchain — on an Apple Silicon Mac with an Intel Go install that
produces an x86_64 binary that runs under Rosetta. Check with `file "$(which hittable)"`.

---

## The `.hit` format

A request is one JSON file. Nothing else is stored about it.

```json
{
  "method": "GET",
  "url": "<<BASE_URL>>/users/3",
  "headers": { "Authorization": "Bearer <<AUTH_TOKEN>>" },
  "params": {},
  "body": "",
  "response": null
}
```

- `headers` and `params` are flat string→string objects, never arrays.
- `body` is a raw string, which may itself contain escaped JSON.
- `response` is `null` until the request has been sent at least once from this file. After
  a send it holds `data`, `status`, `statusText`, `ok`, `headers`, `cookies`, `durationMs`
  and `sizeBytes`.

`hittable/env.json` is a flat map, shared by every `.hit` file anywhere under `hittable/`:

```json
{
  "BASE_URL": "https://api.example.com",
  "AUTH_TOKEN": ""
}
```

`<<KEY>>` tokens in `url`, in header values, in param values and in `body` are resolved
against it **at send time only**. The templates stay in the file — the editor never shows
you, or writes back, an interpolated value.

---

## Commands

```sh
hittable [path]                # open a directory (never writes anything)
hittable init [path]           # create the hittable/ template
hittable -i collection.json    # import a Postman v2.1 or Insomnia export, then open
hittable -e postman            # export hittable/ (<folder>.postman_collection.json)
hittable -e insomnia           # export hittable/ (<folder>.insomnia.json)
hittable -e postman -o x.json  # choose the output file
hittable --icons emoji .       # emoji file icons instead of Nerd Font glyphs
hittable uninstall             # remove hittable from this machine
hittable -h                    # all options
```

| Flag | Meaning |
| :--- | :--- |
| `-i`, `--import <file>` | import a Postman v2.1 or Insomnia v4 export into `hittable/`, then open |
| `-e`, `--export [postman\|insomnia]` | export `hittable/` as a collection; defaults to `postman` |
| `-o`, `--out <file>` | output file for `--export` |
| `--icons <nerd\|emoji>` | file icon pack; defaults to `nerd` |

`init` creates `hittable/testcollection/test.hit` (a real, runnable request),
`hittable/notes/sample.md` and `hittable/env.json`. It refuses if `hittable/` already
exists. Opening a directory with plain `hittable [path]` never creates files.

Import puts the collection under `hittable/<Collection name>/` with folders as
directories, requests as `.hit` files, and variables merged into `hittable/env.json`
(existing keys are kept). Postman `{{var}}` and Insomnia `{{ _.var }}` become `<<var>>`;
scripts, unsupported auth types and cookie jars are dropped and reported.

`uninstall` refuses while another hittable is running. Otherwise it removes `~/.hittable`
(including anything `model enable` downloaded) and then deletes the binary from
`~/.local/bin`, `~/go/bin` and anywhere else on your `PATH`. Your project folders are left
untouched.

### `hittable model`

| Command | What it does |
| :--- | :--- |
| `model enable` | show the disk and memory cost, ask, then download the runtime and the model |
| `model status` | what is installed, how much disk, whether the server is running |
| `model disable` | stop the server and free the memory, keep the files |
| `model delete` | remove the model and runtime from this machine |

`model enable` takes `-y`/`--yes` to skip the prompt and `--dry-run` to print the plan and
stop. `model delete` also takes `-y`. See [AI commit messages](#ai-commit-messages).

---

## Features

**Explorer.** The whole working root as one tree, in real on-disk order, with per-type
icons. Single click opens a file or toggles a folder — no double click anywhere. Git
status colours every row and adds M/A/D/U badges, with a `●` on folders containing
changes. Directories load lazily on expand, so a repo with `node_modules` still starts
instantly. `x` or right-click opens a context menu (new file, new folder, rename, delete).
Drag the `│` separator to resize it.

**Request editor and response viewer.** `.hit` files open in Runner view: a method
dropdown, a URL bar showing the interpolated URL, and Params / Headers / Body tabs. `ctrl+r`
sends. The response panel shows status, duration and size, the body highlighted by Chroma,
a headers view (`h`), and a search overlay (`ctrl+f`) with match highlighting and
next/previous. `ctrl+y` copies the request as a `curl` command. `ctrl+t` flips to Text
view, which is the same file as raw JSON — one copy of the content in memory, so switching
is a transformation and never a reload.

**Autosave.** Every edit commits to the in-memory model immediately and is written to disk
on a short debounce. There is no save action; `ctrl+s` just flushes early. A file changed
outside the app — a git discard, a command in the integrated terminal, another editor — is
picked up and the open buffer reloads while keeping your cursor. Anything you have typed
but not yet flushed wins over the version on disk.

**Code editor.** Chroma syntax highlighting, a line-number gutter, block cursor,
click-to-position, drag selection, undo/redo (200 steps), find (`ctrl+f`), go-to-line
(`ctrl+g`), word jumps, and horizontal scroll with an `alt+z` word-wrap toggle.

Folding comes from indentation, the way VS Code folds a file with no folding provider: a
line is a header when the next non-blank line is indented further. `ctrl+o` folds the block
at the cursor, `alt+o` or the `[ ▾ Collapse ]` header button folds every block.

Autocompletion is **not** AI. Suggestions appear after two word characters, or on
`ctrl+space`, and are ranked in three tiers: request schema, language keywords, then
identifiers already present in the buffer. `.hit` files additionally complete their own
schema keys, the seven HTTP verbs on the `"method"` line, common header names inside
`"headers"`, and content types after `Content-Type`. Nothing is sent anywhere and no model
is involved.

**Markdown.** Open any `.md` file and press `ctrl+t` (or click `[ Text | Preview | Split ]`)
to cycle editor, rendered preview, and a split whose preview follows the cursor as you
type. Headings, lists, tables, task lists, links and highlighted code blocks render;
` ```mermaid ` fences become box-drawing diagrams (flowchart, sequence, ER).

**Find and grep.** `ctrl+p`, `/` in the explorer, or the `Find` button opens a fuzzy file
finder that skips `node_modules`, `.git`, `.next`, `dist`, `build`, `vendor` and `target`.
`alt+f` opens live grep: type two or more characters and `path:line` results stream in
(ripgrep when installed, then git grep, then a Go walk). `⏎` opens the file at that line,
`tab` switches modes.

**Integrated terminal.** `ctrl+j`, or click the `▸ TERMINAL` strip. `$SHELL -l` on a real
PTY, so full-screen programs work. While it has focus every key goes to the shell;
`ctrl+b` returns to the explorer. 5000 lines of scrollback (wheel, or
`shift+↑/↓`/`shift+pgup/pgdown`). Drag to select and `ctrl+c` to copy — with nothing
selected `ctrl+c` still interrupts the shell, as in VS Code. Multi-line pastes go through
as one bracketed block rather than being executed line by line.

**Git.** `ctrl+g`, `alt+g`, `F5`, `g` in the explorer, or the `⎇ Git` button. The panel
takes the full window width; the file tree steps aside and comes back on `esc`. Five
sections:

| Section | What it holds |
| :--- | :--- |
| Status | *Staged Changes* and *Changes* groups, per-row `[ + ]` / `[ − ]` / `[ ⟲ ]` buttons, and the selected file's diff below |
| Commits | repo history, or `f` for the open file's history; `/` searches message, author or hash |
| Branches | `⏎` checkout, `n` new, `d` delete |
| Stashes | `⏎` pop, `s` stash, `d` drop |
| Blame | every line with author, age and commit; `b` turns on inline blame in the editor gutter |

The diff pane does inline or side-by-side (`v`), wraps or clips (`z`), and can ignore
whitespace (`w`). Added and removed lines carry a full-width tint. `e` opens the working
copy in the editor right inside the preview (unstaged files only). A merge, rebase or
cherry-pick that stops on conflicts shows a `⚠ merge in progress` group with a resolver:
`c` accept current, `i` incoming, `b` both, `n`/`p` move between blocks, `a` mark resolved.

Git state is polled every two seconds off the UI goroutine, so changes made in the
integrated terminal or outside the app show up without a refresh.

---

## AI commit messages

Opt-in. Nothing is downloaded, and no model process exists, until you run
`hittable model enable`. That command prints exactly what it will cost in disk and memory,
asks for confirmation, and does nothing if you say no or if stdin is not a terminal.

```
$ hittable model enable

  Download
    llama.cpp b11120 · macos-arm64                      11 MB
    qwen2.5-coder 3B instruct · Q4_K_M                2.10 GB
                                               ──────────
    total                                            2.12 GB

  Disk
    installs to ~/.hittable                          2.13 GB
    free now 245.1 GB, after                          243.0 GB

  Memory
    about 3.20 GB resident while generating
    this machine has 16.00 GB
    the server stops on its own after 10 idle minutes

  Continue? [y/N]
```

It fetches a pinned llama.cpp `llama-server` build and
`qwen2.5-coder-3b-instruct-q4_k_m.gguf` into `~/.hittable`, verifies SHA256 before
installing anything, installs atomically, ad-hoc signs the binary on macOS, and finishes
with a smoke test — spawn, `/health`, one 16-token completion — so a missing dylib or a
corrupt GGUF surfaces then rather than silently inside the TUI a week later.

Everything after that runs on your machine. The server listens on localhost, there is no
network traffic once the download is done, and no part of your diff leaves the machine.

### Using it

Press `c` in the Git panel. If the repo's history is conventional, a type picker appears in
the footer with the likely type already selected; `⏎` accepts it. A full editor then opens
over the detail pane, already holding a valid draft — it is a `texteditor`, so it arrives
with undo/redo, selection, mouse, folding and autocompletion.

| Key | |
| :--- | :--- |
| `ctrl+s` | commit |
| `ctrl+r` | redraft |
| `esc` | cancel; the draft is kept for the next `c` |

The editor opens before the model answers. A commitlint-valid message is in the buffer
within a few milliseconds every time, and generation streams over it. If you start typing,
your text is never overwritten.

### What it will and will not say

**It says what changed, not why.** The why is not in the diff. It is the one part of a
commit message a model cannot recover, and it stays yours to write.

The model is never handed a raw diff and asked for prose. It is given a *specification*
built from the staged changes: the directories that changed, each package's own doc
comment (written by you, in this very change, so it is semantic and true by construction),
and the exported symbols the change adds. No file counts and no line counts — given a
number a 3B model repeats the number instead of describing behaviour.

It answers with a subject line and bullets, and both are checked back against that
specification before you see them. A bullet naming a path or a symbol that is not in the
staged change is dropped; a bullet that attributes a symbol to the wrong directory is
dropped. If the subject itself names something invented, the whole answer is discarded. If
too few bullets survive, the body falls back to the mechanical file summary. Decoding is
also grammar-constrained by llama-server, which makes the conventional-commit shape, the
lowercase subject, the missing trailing period and the 72-character cap structurally
impossible to violate.

This is deliberate. An earlier version let the model write free prose from the diff and it
invented details — plausible file names, functions that did not exist. Grounding every
claim against the change is what makes the output safe to commit.

The footer tells you which path the draft took, so you always know what you are reading:
`reformatted` when something was repaired or dropped, `model output rejected · heuristic
draft` when nothing survived.

### Without a model

Everything works. `c` opens the same editor with the same flow; you get a mechanical draft
derived from the staged diff — a conventional-commit subject and one bullet per directory
with file names and `(+n −m)`. There is no separate code path for this: the draft is the
same object the model is asked to improve on, so "no model installed" is the absence of a
step, not a fallback branch.

### Turning it off

```sh
hittable model disable   # stops the server, frees ~3.2 GB of RAM, keeps the files
hittable model delete    # removes them and reclaims ~2.1 GB of disk
```

These are separate verbs on purpose. Conflating them would charge a two-gigabyte download
to anyone who only wanted their memory back — after `disable`, `model enable` turns it
straight back on with nothing to fetch.

The server also stops on its own after ten idle minutes, and it is pre-warmed when the Git
panel opens, which hides the ~2.8 s cold start.

### Privacy

Files that look like secrets — `.env*`, `*.pem`, `*.key`, `id_rsa*`, `*credential*` — are
listed by name in the digest and their contents are never packed into a prompt. A canary
test drives a real repository end to end to prove it. The traffic is localhost-only today,
but this is the exact path that would ship to a remote provider the day anyone added one.

### Speed

On an Apple M2 the model generates at roughly 38 tokens/sec. A warm commit message takes
about 3–5 seconds. A cold start adds around 2.8 s, which the pre-warm on Git-panel open
normally hides.

---

## Configuration

`~/.hittable/config.json`, created by `hittable model enable|disable`:

```json
{
  "ai": {
    "enabled": true,
    "endpoint": "",
    "model": "qwen2.5-coder-3b-instruct-q4_k_m",
    "timeoutMs": 60000,
    "completionTimeoutMs": 800,
    "maxPromptBytes": 12000,
    "idleMinutes": 10,
    "temperature": 0.2
  }
}
```

An optional `<root>/hittable/config.json` overrides it per project and can be committed.
A missing, unreadable or malformed file is never fatal — you get defaults plus a warning in
the status line.

Environment variables override both:

| Variable | Effect |
| :--- | :--- |
| `HITTABLE_AI=0` | turn AI drafting off (`1`/`true`/`on`/`yes` and `0`/`false`/`off`/`no` are accepted) |
| `HITTABLE_AI_ENDPOINT` | base URL of a server you already run; hittable will never spawn or download one |
| `HITTABLE_AI_MODEL` | model identifier to request from that endpoint |
| `HITTABLE_HOME` | relocate `~/.hittable` |
| `HITTABLE_KEYLOG=<path>` | log the key names your terminal actually delivers |

`ai.enabled` means "use a model if one is installed" — it is not a claim that one is.

---

## Keybindings

| Action                | Binding         | Context              |
|-----------------------|-----------------|----------------------|
| Quit                  | `ctrl+c`        | Global               |
| Help                  | `?` / `F1`; `↑↓`/wheel scroll it | Explorer / Global (full keyboard reference) |
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
| Editor: fold          | `ctrl+o`, click `▾`/`▸` | Any editor (folds the innermost block at the cursor) |
| Editor: fold all      | `alt+o` (`⌥o` on macOS, which sends `ø`), click `[ ▾ Collapse ]` / `[ ▸ Expand ]` in the header | Text view (collapses every block, or expands them all when any is collapsed) |
| Editor: suggestions   | 2 typed chars or `ctrl+space`; `↑↓`/`ctrl+p/n` select, `⇥`/`⏎` accept, `esc` close | Any editor (buffer identifiers + language keywords; `.hit` files also get schema keys, HTTP verbs and header names) |
| Editor: wrap / h-scroll | `alt+z` (`⌥z` on macOS, which sends `Ω`), `shift+wheel`, wheel-left/right | Any editor |
| Editor: select        | drag, `shift+←→↑↓`, double-click word, `ctrl+a` | Any editor |
| Editor: copy/cut/paste| `ctrl+c` / `ctrl+x` / `ctrl+v` | Any editor (ctrl+c quits only without a selection) |
| Send (mouse)          | click `▶ Send`  | URL bar              |
| Terminal toggle/focus | `ctrl+j`, `` ctrl+` ``, click strip | Global (`ctrl+j` is the reliable one; `` ctrl+` `` only in terminals that emit NUL for it) |
| Terminal leave        | `ctrl+b`        | Terminal focused (all other keys go to the shell) |
| Terminal select/copy  | drag, `ctrl+c`  | Terminal focused (ctrl+c interrupts the shell only with no selection) |
| Git panel             | `ctrl+g` (outside editors), `alt+g`, `F5`, `g` in explorer, click `⎇ Git` | Global |
| Git: sections         | `1-5`, `tab`, click | Git panel: Status · Commits · Branches · Stashes · Blame |
| Git: diff layout      | drag `│`, `z`/`alt+z`/`⌥z`, `v`, `w` | resize split columns · wrap lines · inline⇄split · ignore whitespace |
| Git: status           | `+`/`s` `−`/`u` `a` `A` `e` `v` `z` `d` `D` `c` `S` `p` `P` `f` `⏎` `/` | stage · unstage · stage all · unstage all · edit in preview · inline⇄split · wrap lines · discard · undo all · commit · stash · push · pull · fetch · open/collapse · filter |
| Git: commit type      | `←/→`, first letter, `⏎`, `esc` | Type picker (conventional repos only) |
| Git: compose message  | `ctrl+s` commit · `ctrl+r` redraft · `esc` cancel | Message editor (full text editor: undo, selection, folding) |
| Markdown view         | `ctrl+t` cycles Text → Preview → Split; click `[ Text \| Preview \| Split ]` | .md file open |
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

---

## Terminal setup

**Font.** File icons are Nerd Font glyphs — the same icon pack VS Code icon themes use.
They only render if your terminal font is a Nerd Font:

```sh
brew install --cask font-fira-code-nerd-font
```

Then pick **FiraCode Nerd Font** (iTerm2: Preferences → Profiles → Text; Terminal.app:
Settings → Profiles → Text; Ghostty/Kitty: `font-family = FiraCode Nerd Font`). If you see
boxes or `?` where icons should be, either install the font or run
`hittable --icons emoji .`.

**Mouse.** Requires a terminal that reports mouse events, which every modern terminal does.
Everything clickable highlights under the cursor: navbar pills, tabs, mode toggles, the
method badge and its dropdown, Send, the response mode and search icons, the terminal
strip, and every row in the explorer, the Git panel (including its per-row stage and undo
buttons) and the find palette.

**`cmd+c` / `cmd+z` on macOS.** `ctrl+z` undoes and `ctrl+c` copies a selection in every
editor and in the Git diff pane. The cmd combinations never reach a terminal program — the
terminal app handles them itself. To use them anyway, map them to send the control bytes.
iTerm2: Settings → Keys → Key Bindings → `⌘C` → "Send Hex Codes" `0x03`, `⌘Z` → `0x1a`.
Ghostty: `keybind = super+c=text:\x03` and `keybind = super+z=text:\x1a`.

**Selecting with the terminal's own selection** instead of the app's: hold the key your
terminal reserves for it while dragging — Option in iTerm2, Fn in Terminal.app, Shift in
most Linux terminals.

**A shortcut that does nothing.** Find out what your terminal actually sends:

```sh
HITTABLE_KEYLOG=/tmp/hittable-keys.log hittable .
# press the chord, quit, then:
cat /tmp/hittable-keys.log
```

---

## Building and testing

```sh
make build      # native binary in ./hittable
make install    # build, then install to ~/.local/bin and every hittable on PATH
make test       # go vet ./... && go test ./...
make uninstall  # remove the installed binary
```

`make test` passes on a machine with nothing downloaded, no llama-server and no network —
that is the default state for every user and for CI. Tests point `HITTABLE_HOME` at a temp
directory and set `HITTABLE_AI=0`.

Verify an install with:

```sh
which hittable      # ~/.local/bin/hittable
hittable --help
file "$(which hittable)"   # confirm the architecture
```

---

## What it does not do

- **No AI code completion in the editor.** The suggestions are buffer identifiers, language
  keywords and the `.hit` schema, computed locally. This is a decision, not a gap: an
  editor that guesses at your request bodies is worse than one that completes what is
  already in front of you.
- **The commit body never explains why.** It describes what changed, grounded in the staged
  diff. The reasoning is not in the diff and is not invented.
- **No integrated terminal on native Windows.** It needs a PTY, which Windows does not
  provide the way `creack/pty` uses it. Everything else works there; use WSL if you want the
  terminal panel.
- **No GitLens commit graph, worktrees, interactive rebase, forge integration, or
  compare-refs.** Those have no good terminal equivalent here.

---

## Layout of this repo

- `cmd/hittable` — entry point, CLI commands
- `internal/` — `.hit` schema, env interpolation, HTTP engine, document store, git wrapper,
  collection import/export, commit-message drafting, model host
- `ui/` — Bubble Tea UI: explorer, request editor, response viewer, code editor, markdown
  preview, find palette, integrated terminal, Git panel
- `CLAUDE.md` — full spec, keybinding table and changelog
- `docs/commit-message-generator-plan.md` — design notes for the commit-message feature
