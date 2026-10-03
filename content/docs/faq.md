---
title: FAQ and troubleshooting
description: Short answers to the questions that come up, and the first thing to try when something misbehaves.
---

## General

**Is there an account or a cloud?**
No. The web app keeps data in your browser or in a folder you pick; the terminal app works on the directory you launch it in. There is no telemetry.

**Which parts are shared between the web app and the terminal app?**
Directory mode in the browser and the terminal app read and write the same files: `.hit` requests, `hittable/env.json`, markdown notes. Local mode in the browser stores curl strings in `localStorage` and is not shared. Export a Local-mode collection as Postman or Insomnia and import it with `hittable -i` to move it to a folder.

**Why `<<KEY>>` and not `{{key}}`?**
So pasted curl commands and imported Postman or Insomnia collections can carry their own syntax without ambiguity. Both are translated on import and export.

**Are the docs versioned?**
No. These pages describe `main`. GitHub releases carry the notes for each version.

## Web app

**The Directory button says my browser is not supported.**
Directory mode needs the File System Access API, which Chromium browsers (Chrome, Edge, Opera) have and Safari and Firefox do not. Local mode works everywhere.

**Sending to localhost fails with "Install the Hittable Extension".**
A server-side proxy cannot reach your machine. Install the [browser extension](/docs/browser-extension) and reload. If you self-host, add your origin to the extension's manifest first. Use `127.0.0.1` rather than `0.0.0.0`.

**Send is disabled.**
One of the Params, Body or Headers tabs has invalid JSON. The banner above the editor says where.

**The folder scan is slow.**
The whole folder you picked is walked, including `node_modules`. Pick the project folder, not your home directory.

**My data disappeared.**
In Local mode everything is in `localStorage`; clearing site data removes it, and a full quota makes writes fail silently. Export collections you care about, or use Directory mode, where the files are yours.

**Where is the Env button?**
Inside a collection in the sidebar (the variable icon), and in the collapsed sidebar's tool rail.

## Terminal app

**Icons show as boxes or `?`.**
Install a Nerd Font and select it in your terminal profile, or run `hittable --icons emoji .`.

**A shortcut does nothing.**
Record what your terminal sends: `HITTABLE_KEYLOG=/tmp/keys.log hittable .`, press the key, quit, read the log. Some chords cannot arrive in a terminal at all: `ctrl+shift+…` is indistinguishable from `ctrl+…`, `ctrl+enter` from `enter`, and `cmd` never reaches a program. The bindings avoid those; `ctrl+r` sends, `ctrl+j` toggles the terminal, `ctrl+y` copies as curl.

**`cmd+c` does not copy.**
Map it in your terminal to send `0x03` (iTerm2: Keys → Key Bindings → Send Hex Codes; Ghostty: `keybind = super+c=text:\x03`). Likewise `super+z=text:\x1a` for undo.

**The integrated terminal is missing on Windows.**
It needs a PTY, which native Windows does not provide the way the app uses it. Everything else works; WSL gives you the full app.

**A program renders wrongly inside the integrated terminal.**
Capture its output with `HITTABLE_PTYLOG=/tmp/pty.log hittable .` and attach the log to an issue.

**`hittable update` installed something unexpected.**
Releases of the terminal app are tagged `shellapp-v*`, and the repository's "latest" release may be a browser-extension tag. Pin with `hittable update --tag shellapp-vX.Y.Z` or `HITTABLE_VERSION=shellapp-vX.Y.Z` for the installer.

**Does the Git panel send my code anywhere?**
No. Without a model it drafts commit messages from the staged diff locally. With `hittable model enable` a model runs on your machine, on localhost, and files that look like secrets are never packed into a prompt. `HITTABLE_AI=0` turns the model off entirely.

**How much does the local model cost?**
About 2.1 GB of disk and 3.2 GB of memory while generating. `hittable model disable` frees the memory and keeps the files; `hittable model delete` removes them.

**Where does the terminal app keep its state?**
`~/.hittable` (or `$HITTABLE_HOME`): the runtime, model, logs and `config.json`. `hittable uninstall` removes it along with the binary and leaves your project folders alone.

## Still stuck

Open an issue on [GitHub](https://github.com/Pantho-Haque/hittable/issues) with the app, the platform and, for the terminal app, the emulator and the relevant log.
