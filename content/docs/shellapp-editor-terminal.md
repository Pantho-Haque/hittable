---
title: Editor, terminal, find
description: The code editor, the markdown preview, the fuzzy finder and live grep, and the integrated shell.
---

## Code editor

Every text surface in the app is the same editor: `.hit` files in Text view, the Params / Headers / Body tabs, markdown, the Git panel's edit mode and the commit-message composer. It gives you:

- **Syntax highlighting** by Chroma, picked from the file name, so any language Chroma knows renders coloured. `.hit` files are lexed as JSON. Highlighting is cached per line, so rendering stays proportional to the visible rows.
- **A gutter** with line numbers, fold arrows, and the git blame annotation when you turn it on from the Git panel.
- **Undo and redo** (200 steps), click-to-position, drag selection, double-click to select a word, <kbd>ctrl+a</kbd>.
- **Find** (<kbd>ctrl+f</kbd>, <kbd>⏎</kbd>/<kbd>F3</kbd> for next) and **go to line** (<kbd>ctrl+g</kbd>).
- **Word wrap** with <kbd>alt+z</kbd>, or horizontal scrolling with <kbd>shift+wheel</kbd> when wrap is off. The status row shows `⟷ col n` when lines are cut off.
- **Autosave.** Every edit lands in the in-memory model immediately and is written on a short debounce. <kbd>ctrl+s</kbd> flushes early. A file changed outside the app reloads in place while keeping your cursor; anything you typed but have not yet flushed wins over the disk.

### Folding

Folding comes from indentation, the way VS Code folds a file that has no folding provider: a line is a header when the next non-blank line is indented further. <kbd>ctrl+o</kbd> (or a click on `▾`/`▸`) folds the block at the cursor. <kbd>alt+o</kbd> or the `[ ▾ Collapse ]` button in the header folds every block, nested ones included, and expands them all again when any is collapsed.

### Completion

Suggestions appear after two word characters, or on <kbd>ctrl+space</kbd>. They are computed locally and ranked in three tiers: request schema, language keywords, identifiers already in the buffer. Keyword sets exist for Go, Python, JavaScript, TypeScript, Rust, JSON and YAML. `.hit` files additionally complete their own keys (`method`, `url`, `headers`, `params`, `body`, `response`), the HTTP verbs on the `"method"` line, common header names inside `"headers"`, and content types after `Content-Type`. No model is involved and nothing leaves the machine. Files over 512 KB skip the identifier scan.

### Params, Headers, Body

The Runner tabs are editors too. Params and Headers are edited as raw JSON objects with a fixed status row that shows `⚠ invalid JSON` while you are mid-edit; the last valid value is kept and invalid JSON is never written to disk. <kbd>ctrl+l</kbd> pretty-prints the body.

## Markdown

Open any `.md` file and press <kbd>ctrl+t</kbd> (or click `[ Text | Preview | Split ]`) to cycle the editor, a rendered preview, and a split whose preview follows your cursor as you type. The mode is remembered across files.

The preview renders headings, lists, tables, task lists, links and highlighted code blocks. ```` ```mermaid ```` fences become box-drawing diagrams: flowcharts (`graph TD` / `LR`), sequence and ER diagrams. A diagram that cannot be rendered falls back to its source block. Output is cached per content and width, so scrolling is free.

## Find and live grep

<kbd>ctrl+p</kbd>, <kbd>/</kbd> in the explorer, or the `Find` button opens a fuzzy file finder that skips `node_modules`, `.git`, `.next`, `dist`, `build`, `vendor`, `target` and `.cache`.

<kbd>alt+f</kbd> (or <kbd>tab</kbd> inside the palette) switches to live grep: type two or more characters and `path:line` results stream in as they are found. It uses ripgrep when installed, then `git grep`, then a plain Go walk. <kbd>⏎</kbd> opens the file at that line. Stale queries are dropped, so typing fast never shows results for a prefix you already left.

## Integrated terminal

<kbd>ctrl+j</kbd>, or a click on the `▸ TERMINAL` strip under the main pane, opens your `$SHELL -l` on a real pseudo-terminal. Full-screen programs work: `vim`, `htop`, `lazygit`, Ink-based tools such as Claude Code all render correctly, because the emulator answers the cursor-position and colour queries they send. 16, 256 and true colour plus bold, italic, underline and reverse are rendered.

While the panel has focus every key goes to the shell. <kbd>ctrl+b</kbd> returns to the explorer. Scrollback is 5000 lines (wheel, or <kbd>shift+↑↓</kbd>, <kbd>shift+pgup/pgdn</kbd>); the strip shows `↑ scrollback n/m` while you are up, and any key returns to the bottom. Drag to select output; the selection is copied the moment you release, and <kbd>ctrl+c</kbd> copies it too. With nothing selected, <kbd>ctrl+c</kbd> still interrupts the shell, as in VS Code. <kbd>ctrl+v</kbd> pastes; multi-line pastes go through as one bracketed block rather than being executed line by line. Drag the strip to resize the panel. If the shell exits, any key restarts it.

The terminal is not available on native Windows, which has no PTY the way the app needs one. Everything else works there; WSL gives you the full app.

If a program misrenders inside the panel, `HITTABLE_PTYLOG=<path>` records the raw byte stream it writes before the emulator sees it, and `HITTABLE_KEYLOG=<path>` records what your terminal delivers for each key. Those two logs are the first thing to attach to a bug report.

## Response viewer

Status, duration and size sit above a body highlighted by Chroma (bodies over 256 KB render plain). <kbd>h</kbd> flips to the response headers. <kbd>ctrl+f</kbd> opens a search overlay with match highlighting and next/previous. <kbd>ctrl+y</kbd> copies the body, or the request as a `curl` command when the Runner has focus. The response is also written into the `.hit` file's `response` field, so the last result travels with the request and shows up in the web app too.
