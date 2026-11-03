---
title: FAQ and troubleshooting
description: Short answers to the questions that come up, and the first thing to try when something misbehaves.
---

## General

### Is there an account or a cloud?

There is no Hittable account and no database. In the web app you either sign in with GitHub and work on your own repository, or open a local folder with no account at all. The terminal app works on the directory you launch it in. There is no telemetry.

### Which parts are shared between the web app and the terminal app?

All of them. Both read and write the same files: `.hit` requests, `hittable/env.json`, `hittable/env.local.json` and markdown notes. A GitHub workspace shares them through the repository; a local folder shares them on disk.

### Why `<<KEY>>` and not `{{key}}`?

So pasted curl commands and imported Postman or Insomnia collections can carry their own syntax without ambiguity. The terminal app translates both on import and export.

### Are the docs versioned?

No. These pages describe `main`. GitHub releases carry the notes for each version.

## Web app

### Sign in with GitHub is missing or says it is not configured

The instance has no GitHub OAuth app set up. Sign-in needs `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` and `SESSION_SECRET` on the server; see [Self-hosting](/docs/self-hosting#github-sign-in). **Open a local folder** works without them.

### Organisation repositories are missing

Organisations restrict OAuth apps by default. Open the app's entry in GitHub under [Settings → Applications](https://github.com/settings/applications) and request or grant access for the organisation. An organisation owner may have to approve it.

### A repository opens read-only

You have no push permission, the repository is archived, or it is empty (add a first commit on GitHub). You can still send requests; you cannot edit or commit.

### Where are my uncommitted edits?

In a working copy saved in this browser's IndexedDB, per repository and branch. They survive a reload but not clearing site data, and they do not follow you to another browser or machine. Commit to keep them.

### What does the GitHub permission allow, and how do I revoke it?

`repo` lets the app list your repositories, read their files and commit your changes; `read:user` gives your name and avatar. The token stays on the server in an encrypted, `HttpOnly` cookie and is never visible to page JavaScript. Signing out revokes the grant. You can also revoke it in GitHub under [Settings → Applications](https://github.com/settings/applications).

### Open a local folder is not available in Safari or Firefox

It needs the File System Access API, which Chrome and Edge have and Safari and Firefox do not. Use a Chromium browser, or sign in with GitHub, which works in any modern browser.

### Where did Local mode go?

The old browser-storage mode was removed in v3. If the app finds old Local-mode data in this browser, a one-time banner should offer to download it as a Postman collection, which the terminal app can import with `hittable -i`. Dismissing it forgets the data.

### Sending to localhost fails

A server-side proxy cannot reach your machine. Install the [browser extension](/docs/browser-extension) and reload. If you self-host, add your origin to the extension's manifest first. Use `127.0.0.1` or `localhost` rather than `0.0.0.0`.

### Send is disabled

One of the Params, Body or Headers tabs has invalid JSON. The banner above the editor says where.

### The folder scan is slow

The whole folder you picked is walked, including `node_modules`. Pick the project folder, not your home directory.

## Terminal app

### Icons show as boxes or `?`

Install a Nerd Font and select it in your terminal profile, or run `hittable --icons emoji .`.

### A shortcut does nothing

Record what your terminal sends: `HITTABLE_KEYLOG=/tmp/keys.log hittable .`, press the key, quit, read the log. Some chords cannot arrive in a terminal at all: `ctrl+shift+…` is indistinguishable from `ctrl+…`, `ctrl+enter` from `enter`, and `cmd` never reaches a program. The bindings avoid those; `ctrl+r` sends, `ctrl+j` toggles the terminal, `ctrl+y` copies as curl.

### `cmd+c` does not copy

Map it in your terminal to send `0x03` (iTerm2: Keys → Key Bindings → Send Hex Codes; Ghostty: `keybind = super+c=text:\x03`). Likewise `super+z=text:\x1a` for undo.

### The integrated terminal is missing on Windows

It needs a PTY, which native Windows does not provide the way the app uses it. Everything else works; WSL gives you the full app.

### A program renders wrongly inside the integrated terminal

Capture its output with `HITTABLE_PTYLOG=/tmp/pty.log hittable .` and attach the log to an issue.

### `hittable update` installed something unexpected

Terminal-app releases are tagged `sh-v*` and browser-extension releases `ext-v*`, in the same repository; `hittable update` and the installer only pick `sh-v*` tags. To pin a version, use `hittable update --tag sh-vX.Y.Z`, or `HITTABLE_VERSION=sh-vX.Y.Z` for the installer.

### Does the Git panel send my code anywhere?

No. Without a model it drafts commit messages from the staged diff locally. With `hittable model enable` a model runs on your machine, on localhost, and files that look like secrets are never packed into a prompt. `HITTABLE_AI=0` turns the model off entirely.

### How much does the local model cost?

About 2.1 GB of disk and 3.2 GB of memory while generating. `hittable model disable` frees the memory and keeps the files; `hittable model delete` removes them.

### Can I use VS Code extensions?

In the terminal app, yes: the Extensions panel (<kbd>alt+x</kbd>) installs them from Open VSX, the open registry VSCodium uses. Language servers, formatters, themes, snippets and commands work; extension panels made of HTML open in your browser. Debuggers, notebooks and custom editors do not, and Microsoft-licensed extensions (Pylance, C/C++, Remote, Copilot) are refused because their licences allow only Microsoft's VS Code. Details on the [extensions page](/docs/shellapp-extensions).

### An extension says it needs Node.js

Extensions with code run in the Node.js on your machine (20 or newer). If there is none, the extension's page in the panel has an **Install Node.js** button that installs it for your account with Homebrew, winget, or the official build in `~/.local`; or run `hittable ext node install`. To use a particular Node, set `HITTABLE_NODE=/path/to/node`.

### Where does the terminal app keep its state?

`~/.hittable` (or `$HITTABLE_HOME`): the runtime, model, logs, `config.json`, `settings.json`, installed extensions and their data. `hittable uninstall` removes it along with the binary and leaves your project folders alone.

## Still stuck

Open an issue on [GitHub](https://github.com/Pantho-Haque/hittable/issues) with the app, the platform and, for the terminal app, the emulator and the relevant log.
