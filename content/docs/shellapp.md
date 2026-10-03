---
title: Terminal app
description: hittable.sh is the terminal client. It opens a project directory, runs the .hit files inside it, and brings an editor, a Git panel and a real shell along. This page installs it and gets you to your first request.
---

## What it is

`hittable` is a terminal UI written in Go. It is **directory-mode only**: launching the binary *is* opening a directory. There is no global store, no account, no mode to pick. Whatever folder you point it at is the workspace, and the `.hit` files and `hittable/env.json` inside it are exactly the files the [web app](/docs/web-app) reads and writes. One folder opens in either tool.

Around the request runner it carries the tools you would otherwise leave the terminal for:

- a code editor with syntax highlighting, folding, local autocompletion, find and go-to-line
- a markdown preview with Mermaid diagrams rendered as box-drawing
- fuzzy file find and live grep
- a Git panel with staging by line, split diffs you can edit in place, a commit-message drafter, blame, branches and stashes
- an integrated shell on a real PTY

What it deliberately does **not** do: AI code completion in the editor (suggestions come from the buffer, the language and the `.hit` schema), commit bodies that explain *why* (the diff does not know), an integrated terminal on native Windows (it needs a PTY; use WSL), and GitLens-style commit graphs, worktrees or interactive rebase.

## Install

The one-liner fetches the latest release for your OS and CPU, verifies its SHA-256 against the published `checksums.txt`, and puts the binary in `~/.local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/Pantho-Haque/hittable/main/install.sh | sh
```

It needs `curl` or `wget`, and `tar` (or `unzip` on Windows). On macOS it strips the quarantine attribute and ad-hoc signs the binary. On WSL it installs the Linux build. It refuses 32-bit systems. If `~/.local/bin` is not on your `PATH`, it prints the line to add for zsh, bash or fish.

Two environment variables steer it:

| Variable | Effect |
| :--- | :--- |
| `HITTABLE_VERSION` | release tag to install, e.g. `shellapp-v1.2.0` (default: the latest release) |
| `HITTABLE_INSTALL_DIR` | where to put the binary (default: `~/.local/bin`) |

```sh
HITTABLE_VERSION=shellapp-v1.2.0 sh -c "$(curl -fsSL https://raw.githubusercontent.com/Pantho-Haque/hittable/main/install.sh)"
HITTABLE_INSTALL_DIR=/usr/local/bin sh -c "$(curl -fsSL https://raw.githubusercontent.com/Pantho-Haque/hittable/main/install.sh)"
```

> Releases of the terminal app are tagged `shellapp-v*`. The browser extension has its own `v*` tags in the same repository, so pin with `HITTABLE_VERSION` when "latest" is not the one you want.

### Prebuilt platforms

Every release ships static binaries (`CGO_ENABLED=0`, so one Linux build runs on any distribution) for:

| OS | Architectures | Notes |
| :--- | :--- | :--- |
| macOS | arm64, amd64 | |
| Linux | amd64, arm64 | |
| Windows | amd64, arm64 | everything works except the integrated terminal |

Verify a download by hand with `sha256sum -c checksums.txt --ignore-missing`.

### From source

You need Go 1.25 or newer.

```sh
git clone https://github.com/Pantho-Haque/hittable
cd hittable
make install
```

`make install` builds for the machine it runs on, copies the binary to `~/.local/bin/hittable`, and also overwrites any other `hittable` on your `PATH` so a stale copy can never shadow the new build. The other targets:

```sh
make build      # native binary in ./shellapp/hittable
make test       # go vet ./... && go test ./...
make uninstall  # remove ~/.local/bin/hittable and ~/go/bin/hittable
```

`go install ./cmd/hittable` works too, but it writes to `~/go/bin` and builds for your Go toolchain's architecture. On an Apple Silicon Mac with an Intel Go install that gives you an x86_64 binary running under Rosetta. Check with `file "$(which hittable)"`.

### Verify

```sh
which hittable      # ~/.local/bin/hittable
hittable version    # hittable shellapp-v1.2.0 darwin/arm64
hittable --help
```

A source build reports `dev` as its version.

## First run

```sh
mkdir demo && cd demo
hittable init       # creates hittable/ with a runnable request, env.json and a notes file
hittable            # opens the current directory
```

`init` scaffolds this tree and refuses to touch an existing `hittable/` folder:

```
demo/
└── hittable/
    ├── testcollection/
    │   └── test.hit        GET https://jsonplaceholder.typicode.com/posts/1
    ├── notes/
    │   └── sample.md
    └── env.json            { "BASE_URL": "...", "AUTH_TOKEN": "" }
```

Inside the app:

1. Move with <kbd>↑</kbd> <kbd>↓</kbd> (or <kbd>j</kbd> <kbd>k</kbd>) in the explorer and press <kbd>⏎</kbd> on `test.hit`. It opens in the **Runner view**: method, URL, Params / Headers / Body tabs, response panel.
2. Press <kbd>ctrl+r</kbd> to send. Status, duration, size and the highlighted body appear below. The response is also written into the file's `response` field.
3. Press <kbd>ctrl+t</kbd> to see the same file as raw JSON in the **Text view**, and again to go back. It is one copy of the content in memory, so the switch is a transformation, never a reload.
4. Press <kbd>?</kbd> for the complete key reference, <kbd>ctrl+j</kbd> for a shell, <kbd>ctrl+g</kbd> for Git.

Plain `hittable [path]` never creates or modifies files on its own. Edits autosave on a short debounce; <kbd>ctrl+s</kbd> only flushes early. A file changed outside the app, by a git checkout or a command in the integrated terminal, is picked up and the open buffer reloads while keeping your cursor.

## Already have a Postman or Insomnia collection?

```sh
hittable -i collection.json
```

Imports it under `hittable/<Collection name>/`, folders as directories, requests as `.hit` files, variables merged into `env.json`, then opens the app. See the [CLI reference](/docs/shellapp-cli) and [Import and export](/docs/import-export).

## Terminal setup

**Font.** File icons are Nerd Font glyphs, the icon pack VS Code themes use. Install one and select it in your terminal profile:

```sh
brew install --cask font-fira-code-nerd-font
```

iTerm2: Preferences → Profiles → Text. Terminal.app: Settings → Profiles → Text. Ghostty and Kitty: `font-family = FiraCode Nerd Font`. If you see boxes or `?` where icons should be, either install the font or run `hittable --icons emoji .`.

**Mouse.** Every modern terminal reports mouse events, and everything clickable highlights under the cursor. Splitters light up on hover, and terminals that honour OSC 22 (kitty, Ghostty, WezTerm, foot, xterm) show a resize pointer too.

**cmd+c / cmd+z on macOS** never reach a terminal program. <kbd>ctrl+c</kbd> copies a selection and <kbd>ctrl+z</kbd> undoes everywhere in the app. To keep your habits, map the cmd keys to the control bytes:

- iTerm2: Settings → Keys → Key Bindings → `⌘C` → Send Hex Codes `0x03`, `⌘Z` → `0x1a`
- Ghostty: `keybind = super+c=text:\x03` and `keybind = super+z=text:\x1a`

**Selecting with the terminal's own selection** instead of the app's: hold the modifier your terminal reserves for it while dragging (Option in iTerm2, Fn in Terminal.app, Shift in most Linux terminals).

**A shortcut that does nothing.** Find out what your terminal actually sends:

```sh
HITTABLE_KEYLOG=/tmp/hittable-keys.log hittable .
# press the chord, quit, then
cat /tmp/hittable-keys.log
```

Some chords cannot arrive at all: a terminal sends the same byte for <kbd>ctrl+c</kbd> and <kbd>ctrl+shift+c</kbd>, and a plain CR for <kbd>ctrl+enter</kbd>. That is why the bindings are <kbd>ctrl+r</kbd> to send, <kbd>ctrl+y</kbd> to copy as curl and <kbd>ctrl+f</kbd> for a new folder. Option on macOS sends a composed character, which is handled: <kbd>⌥z</kbd> arrives as `Ω` and <kbd>⌥o</kbd> as `ø`.

## Updating and removing

```sh
hittable update            # download the latest release, verify SHA-256, replace this binary
hittable update --check    # only report whether a newer release exists
hittable update --tag shellapp-v1.2.0
hittable uninstall         # remove the binary, ~/.hittable and any downloaded model
```

`update` checks the install directory is writable before downloading, verifies the checksum, swaps the binary atomically, and runs the new binary's `version` to prove it works. It never touches `~/.hittable`, so a model you enabled stays enabled. `uninstall` refuses while another `hittable` is running, removes data first and binaries last, and leaves your project folders alone.
