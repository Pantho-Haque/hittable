---
title: VS Code extensions in the terminal app
description: Install extensions from Open VSX and use them in the terminal editor — language servers, formatters, themes, snippets, commands — with Node.js downloaded only when an extension needs it.
---

The terminal app runs VS Code extensions. Language extensions give the editor real
diagnostics, completions, hover, go-to-definition, rename, quick fixes and formatting;
themes recolour it; snippets show up in the suggestion list; extension commands appear in
the command palette.

## Installing

Press `alt+x` (`⌥x`), or click **Extensions** in the top bar, for the Extensions panel,
laid out like VS Code's:

- **Left:** a search box over collapsible sections — **Installed** and **Popular on Open
  VSX**, or your search results — each extension with its icon, description, publisher,
  installs and rating. `⏎` on a section header (or a click) folds it. Type `@installed` to
  filter what you have.
- **Right:** the selected extension's page: its icon, publisher, installs, rating and
  licence, the **Install**, **Uninstall**, **Enable / Disable** and **Update** buttons, a
  **Setup** checklist (downloaded and verified → Node.js → folder trusted → extension host
  → active), and three tabs — **Details** (its README as a Markdown preview), **Features**
  (the commands, settings, languages, themes and keys it adds) and **Changelog**.

The pane under the mouse takes the keyboard, and the wheel scrolls it (one extension per
notch in the list), so nothing needs a click first.

Icons are real images in terminals that speak the kitty graphics protocol (kitty,
Ghostty). Elsewhere, and inside tmux, they are drawn with coloured half blocks, which
needs true colour. `HITTABLE_IMAGES=blocks` or `=kitty` overrides the choice.

`⏎` installs the selected extension, `tab` moves to its page, `1` `2` `3` switch tabs, and
`i` `u` `e` `U` install, uninstall, enable/disable and update. Everything loads in the
background with a spinner, so the panel never goes blank. Every step an extension still
needs (Node.js, trusting the folder) is on its Setup checklist with the button that does
it, and the checklist follows each step live until the extension is **Active**.

From the shell:

```sh
hittable ext search yaml
hittable ext install redhat.vscode-yaml        # also: publisher.name@1.2.3, or a .vsix file
hittable ext list
hittable ext disable | enable | uninstall <id>
```

Extensions come from [Open VSX](https://open-vsx.org), the open registry VSCodium,
Gitpod and Theia use. Microsoft's marketplace terms only allow it in Microsoft's own
products, and Microsoft-licensed extensions (Pylance, C/C++, Remote, Live Share, Copilot,
C# Dev Kit) are refused even from a `.vsix` file, because their licences only allow
Microsoft's VS Code.

Every download is checked against the registry's SHA-256 and its Ed25519 signature. The
installer picks the newest stable version whose `engines.vscode` range accepts the API the
app provides, and the build for your platform when an extension ships per-platform builds
(rust-analyzer does). Dependencies and extension packs are installed with it.

## Node.js

Themes, snippets and language definitions need nothing else. Extensions with code run in
Node.js, in a background process called the extension host, and it is the Node already
on your machine:

- hittable looks in this order and takes the first that is Node 20 or newer:
  `HITTABLE_NODE`, `node` on your `PATH`, then Homebrew, nvm, fnm, Volta, asdf, mise and
  the standard install locations, newest version first.
- If there is none (or only an older one), the extension's page says so and offers
  **Install Node.js**. It installs Node for your whole account, not privately for
  hittable: with Homebrew when you have it (`brew install node`, or `brew upgrade node`
  for an old one), with winget on Windows, otherwise the official LTS build (checked
  against its SHA-256) in `~/.local/lib/nodejs` with `node`, `npm` and `npx` linked into
  `~/.local/bin`. It never asks for sudo, and each line of the installer's output shows
  on the page while it runs.
- `hittable ext node status` shows which Node runs extensions; `install` does the same
  install from the shell; `remove` removes only what hittable installed itself.

## Trust

Extension code runs with your user's permissions, as in VS Code. It only runs in folders
you trust: the app asks the first time, or run `hittable ext trust [folder]`. Themes and
snippets work everywhere.

## Keys

| Key | Action |
| :--- | :--- |
| `alt+x` | Extensions panel |
| `alt+p` | command palette |
| `alt+h` | hover information |
| `F12` / `shift+F12` | go to definition / references |
| `F2` | rename symbol |
| `alt+.` | quick fixes and refactorings |
| `alt+shift+f` | format document |
| `F8` | next problem in the file |

Completions from extensions merge into the editor's suggestion list as you type, and after
the trigger characters a language server declares. Signature help shows in the status row
while you type a call. Errors and warnings are underlined, the cursor line's message is in
the status row, and the footer counts them (**Problems: Show All** lists them). Extension
status bar items and progress appear on the right of the footer.

An extension's own key bindings work when a terminal can send the key (alt+letter,
ctrl+alt+letter, function keys) and hittable does not already use it.

## Settings and themes

Extensions read settings from `~/.hittable/settings.json` and the project's
`.vscode/settings.json`, both in VS Code's format. **Preferences: Open User Settings
(JSON)** opens the first; `ctrl+s` applies changes. **Preferences: Color Theme…** switches
the editor's syntax colours to any installed theme.

## Panels, views and output

- **Webview panels** (an extension's HTML page: a preview, a dashboard) open in your
  browser, served from `127.0.0.1` with a random token in every URL. Over SSH without a
  display, the app shows the URL instead. **View: Reopen Extension Panel in Browser…**
  brings a closed tab back.
- **Tree views** are browsed in the palette with **View: Open View…**.
- **Output channels** open as log files with **Output: Show Output Channel…**;
  **Extensions: Show Log** is the extension host's own log.

## What does not work

Debuggers, notebooks, custom editors, file icon themes (the terminal's font decides the
icons) and extensions that need proposed VS Code APIs. Calling an API the app does not
provide fails with a message in the log naming it. If the extension host crashes, it is
restarted; after three crashes in a minute it stays stopped until **Extensions: Restart
Extension Host**.
