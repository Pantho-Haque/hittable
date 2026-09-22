# hittable.sh

Terminal API client for `.hit` request files. Launch it inside any project:

```sh
hittable .            # current directory
hittable ~/code/app   # explicit path
```

Press `?` inside the app for the keyboard reference.

## Commands

```sh
hittable init                  # create the hittable/ template (sample request, env.json, notes)
hittable -i collection.json    # import a Postman v2.1 or Insomnia export, then open
hittable -e postman            # export hittable/ as a Postman collection (<folder>.postman_collection.json)
hittable -e insomnia           # export hittable/ as an Insomnia collection (<folder>.insomnia.json)
hittable -e postman -o x.json  # choose the output file
hittable uninstall             # remove hittable from this machine
hittable -h                    # all options
```

Opening a directory never creates files. Import puts the collection under `hittable/<Collection name>/` with folders as directories, requests as `.hit` files, and variables merged into `hittable/env.json` (existing keys are kept). Postman `{{var}}` and Insomnia `{{ _.var }}` become `<<var>>`; scripts, unsupported auth types, and cookie jars are dropped and reported.

`hittable uninstall` refuses while another hittable is running; otherwise it deletes the binary from `~/.local/bin`, `~/go/bin`, and anywhere else on your PATH. Your project folders are left untouched.

## Install

Requires Go 1.23+.

```sh
make install        # from the repo root or from shellapp/
```

This builds a native binary for your machine and copies it to `~/.local/bin/hittable`. Make sure `~/.local/bin` is on your `PATH`:

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

## Reinstall after changes

Every time you edit the code, rebuild and replace the installed binary:

```sh
make install        # from the repo root or from shellapp/
```

`make install` also overwrites any other `hittable` found on your `PATH` (for example `~/go/bin/hittable`), so a stale copy can never shadow the new build. Open a new terminal tab, or run `hash -r`, if the shell still reports the old version.

Manual equivalent, if you prefer not to use make:

```sh
GOARCH=arm64 go build -o ~/.local/bin/hittable ./cmd/hittable   # Apple Silicon
GOARCH=amd64 go build -o ~/.local/bin/hittable ./cmd/hittable   # Intel Mac / Linux x86_64
```

### Why not `go install`?

`go install ./cmd/hittable` works, but it writes to `~/go/bin` and builds for the architecture of your Go toolchain. On an Apple Silicon Mac with the Intel build of Go, that produces an x86_64 binary that runs under Rosetta. `make install` always cross-compiles for the machine it runs on. Check what you have with:

```sh
file "$(which hittable)"
```

## Verify

```sh
which hittable      # ~/.local/bin/hittable
hittable --help
```

## Test

```sh
make test
```

## Uninstall

```sh
make uninstall
```

## Terminal setup

**Font.** File icons in the explorer are Nerd Font glyphs (the same icon pack VS Code icon themes use: Go gopher, TypeScript, JSON, Markdown, Docker, git, lock files, and about 450 more). They only render if your terminal font is a Nerd Font. Install one and select it in your terminal profile:

```sh
brew install --cask font-fira-code-nerd-font
```

Then pick **FiraCode Nerd Font** (iTerm2: Preferences → Profiles → Text; Terminal.app: Settings → Profiles → Text; Ghostty/Kitty: `font-family = FiraCode Nerd Font`).

If you see boxes or `?` where icons should be, either install the font or fall back to emoji icons:

```sh
hittable --icons emoji .
```

**Files changed outside the app.** A git discard, a command in the integrated terminal, or another editor writing the file are all picked up — the open buffer reloads (immediately for Git actions, within a couple of seconds otherwise) and keeps your cursor. Anything you have typed but not yet saved wins over the version on disk.\n\n**Mouse.** Requires a terminal that reports mouse events, which every modern terminal does. Everything clickable highlights under the cursor: navbar pills, tabs, mode toggles, the method badge and its dropdown, Send, the response mode and search icons, the terminal strip, and every row in the explorer, the Git panel (including its per-row stage and undo buttons) and the find palette.

## Layout of this repo

- `cmd/hittable` — entry point
- `internal/` — scaffolding, `.hit` schema, HTTP engine, document store
- `ui/` — Bubble Tea UI: explorer, request editor, response viewer, code editor, integrated terminal
- `CLAUDE.md` — full spec, keybinding table, and changelog

## Integrated terminal

Click the `▸ TERMINAL` strip under the main pane, or press `ctrl+j` (VS Code's panel toggle; `ctrl+`` also works in terminals that send it). While the panel has focus every key goes to the shell; `ctrl+b` returns to the explorer. Drag over the output to select it and `ctrl+c` to copy — with nothing selected `ctrl+c` still interrupts the shell, as in VS Code.

## Markdown

Open any `.md` file and press `ctrl+t` (or click `[ Text | Preview | Split ]` in the header) to switch between the editor, a rendered preview, and a split view whose preview updates as you type. The preview renders headings, lists, tables, task lists, links, and highlighted code blocks; ```mermaid fences become box-drawing diagrams (flowcharts, sequence, and ER diagrams).

## Find files and text

- `ctrl+p` (or `/` in the explorer, or the `Find` button) opens the fuzzy file finder. Type part of a name, `⏎` opens.
- `alt+f` opens live grep: type at least two characters and matching `path:line` results stream in (uses ripgrep when installed). `⏎` opens the file at that line. `tab` switches between the two modes.

## cmd+z / cmd+c on macOS

`ctrl+z` undoes and `ctrl+c` copies a selection in every editor and in the Git diff pane. The cmd key combinations never reach a terminal program: the terminal app handles `cmd+c` (it copies the terminal's own selection, which is empty while hittable owns the mouse) and `cmd+z` itself. To use them anyway, map them in your terminal to send the control bytes. In iTerm2: Settings → Keys → Key Bindings → add `⌘C` → "Send Hex Codes" `0x03` and `⌘Z` → `0x1a`. Ghostty: `keybind = super+c=text:\x03` and `keybind = super+z=text:\x1a`.

## Scrolling

Every pane has a vertical scrollbar on its right edge. Code files do not wrap: `shift+wheel` (or a horizontal wheel/trackpad swipe) scrolls sideways, and `alt+z` (or ⌥z on macOS, which sends `Ω`) toggles word wrap, as in VS Code. Markdown files wrap by default. Functions and blocks fold by indentation: click the `▾` in the gutter, or press `ctrl+o` to collapse the block around the cursor.

## Git

Click `⎇ Git` in the top bar (or press `F5`, `alt+g`, or `g` in the explorer). The panel takes the full window width — the file tree steps aside while it is up and comes back when you close it with `esc`, `ctrl+b`, or the same button. It has five sections:

- **Status** — two collapsible groups, *Staged Changes* and *Changes*. Click `[ + ]` / `[ − ]` on a row to stage or unstage it, or the `[ + stage all ]` / `[ − unstage all ]` button on the group header. Unstaged rows also carry `[ ⟲ ]`, and the *Changes* header `[ ⟲ undo all ]`, to throw the edits away (confirmed first; `d` and `D` do the same). The selected file's diff shows below; `v` (or the `[inline split]` toggle) switches to side-by-side. Added and removed lines carry a full-width green / red tint so a change is easy to follow across the pane. Diff lines wrap to the pane in both modes; `z` (or the `[wrap]` toggle) turns wrapping off and clips them instead, as VS Code's word wrap does. In split view, drag the `│` divider to give one side more room. `e` edits the working copy right in the preview (unstaged files only), `d` discard, `c` commit, `S` stash, `p` push, `P` pull, `f` fetch, `⏎` open in the main editor.
- **Commits** — repo history, or `f` for the active file's history; `/` searches message, author, or hash. The selected commit's full diff shows below.
- **Branches** — `⏎` checkout, `n` new, `d` delete.
- **Stashes** — `⏎` pop, `s` stash, `d` drop.
- **Blame** — every line of the open file with author, age, and commit; `b` turns on inline blame in the editor gutter (with current-line blame in the status row), `⏎` jumps to the line.

The explorer colours changed files and shows M/A/D/U badges; the top bar shows the branch and the number of changed files, plus a sync button (VS Code's default: pull then push, or publish when the branch has no upstream).

**Merge conflicts.** When a merge, rebase, or cherry-pick stops on conflicts, Status shows a `⚠ merge in progress` group with the conflicted files. `⏎` (or the `resolve` button) opens the resolver: each block is highlighted, `c` accepts current, `i` incoming, `b` both, `n`/`p` move between blocks, `a` marks the file resolved, `o` opens it in the editor. The group header commits the merge once everything is resolved, or aborts it.

**Sidebar.** `alt+b` or the `☰` button hides the file explorer; `ctrl+b` brings it back.

**Copying text.** Drag to select in any editor, in the Git diff pane, or in the integrated terminal, then `ctrl+c` (it only quits when nothing is selected). To use your terminal's own selection instead, hold the key your terminal reserves for it while dragging: Option in iTerm2, Fn in Terminal.app, Shift in most Linux terminals.

## Troubleshooting keys

If a shortcut does nothing, find out what your terminal actually sends:

```sh
HITTABLE_KEYLOG=/tmp/hittable-keys.log hittable .
# press the chord, quit, then:
cat /tmp/hittable-keys.log
```
