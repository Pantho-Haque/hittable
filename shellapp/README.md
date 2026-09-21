# hittable.sh

Terminal API client for `.hit` request files. Launch it inside any project:

```sh
hittable .            # current directory
hittable ~/code/app   # explicit path
```

On first launch it creates `hittable/` with a sample request, `env.json`, and a notes file. Press `?` inside the app for the keyboard reference.

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

**Mouse.** Requires a terminal that reports mouse events, which every modern terminal does.

## Layout of this repo

- `cmd/hittable` — entry point
- `internal/` — scaffolding, `.hit` schema, HTTP engine, document store
- `ui/` — Bubble Tea UI: explorer, request editor, response viewer, code editor, integrated terminal
- `CLAUDE.md` — full spec, keybinding table, and changelog

## Integrated terminal

Click the `▸ TERMINAL` strip under the main pane, or press `ctrl+j` (VS Code's panel toggle; `ctrl+`` also works in terminals that send it). While the panel has focus every key goes to the shell; `ctrl+b` returns to the explorer.

## Find files and text

- `ctrl+p` (or `/` in the explorer, or the `Find` button) opens the fuzzy file finder. Type part of a name, `⏎` opens.
- `alt+f` opens live grep: type at least two characters and matching `path:line` results stream in (uses ripgrep when installed). `⏎` opens the file at that line. `tab` switches between the two modes.

## Undo on macOS

`ctrl+z` undoes in every editor. `cmd+z` is swallowed by the terminal app itself and never reaches programs; to use it, map it in your terminal to send `0x1a` (iTerm2: Settings → Keys → Key Bindings → `⌘Z` → Send Hex Codes `0x1a`).

## Git

Click `⎇ Git` in the top bar (or press `F5`, `alt+g`, or `g` in the explorer). The panel has five sections:

- **Status** — two collapsible groups, *Staged Changes* and *Changes*. Click `[ + ]` / `[ − ]` on a row to stage or unstage it, or the `[ + stage all ]` / `[ − unstage all ]` button on the group header. The selected file's diff shows below; `v` (or the `[inline split]` toggle) switches to side-by-side. `e` edits the working copy right in the preview (unstaged files only), `d` discard, `c` commit, `S` stash, `p` push, `P` pull, `f` fetch, `⏎` open in the main editor.
- **Commits** — repo history, or `f` for the active file's history; `/` searches message, author, or hash. The selected commit's full diff shows below.
- **Branches** — `⏎` checkout, `n` new, `d` delete.
- **Stashes** — `⏎` pop, `s` stash, `d` drop.
- **Blame** — every line of the open file with author, age, and commit; `b` turns on inline blame in the editor gutter (with current-line blame in the status row), `⏎` jumps to the line.

The explorer colours changed files and shows M/A/D/U badges; the top bar shows the branch, the number of changed files, and ahead/behind counts.

## Troubleshooting keys

If a shortcut does nothing, find out what your terminal actually sends:

```sh
HITTABLE_KEYLOG=/tmp/hittable-keys.log hittable .
# press the chord, quit, then:
cat /tmp/hittable-keys.log
```
