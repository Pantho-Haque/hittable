---
title: Keyboard and mouse
description: The complete key reference for the terminal app, organised by where you are. Press ? inside the app for the same tables.
---

All chords use <kbd>ctrl</kbd> for cross-platform reach. A terminal cannot deliver <kbd>cmd</kbd>, cannot tell <kbd>ctrl+shift+c</kbd> from <kbd>ctrl+c</kbd>, and sends a plain return for <kbd>ctrl+enter</kbd>, so those never appear here. On macOS, <kbd>⌥</kbd> chords arrive as composed characters and are accepted as such: <kbd>⌥z</kbd>, <kbd>⌥o</kbd>, <kbd>⌥g</kbd>, <kbd>⌥f</kbd>, <kbd>⌥b</kbd> all work.

## Global

| Key | Action |
| :--- | :--- |
| `ctrl+b` | toggle explorer / main pane focus |
| `alt+b` | hide / show the sidebar |
| `ctrl+r`, `ctrl+⏎` | send the open request |
| `ctrl+s` | save now (autosave is always on) |
| `ctrl+t` | Runner ⇄ Text view for `.hit`; Text → Preview → Split for `.md` |
| `ctrl+y` | copy as curl (Runner) / copy response body (Response) |
| `ctrl+p`, `alt+f` | find file · live grep |
| `ctrl+j`, `` ctrl+` `` | toggle the integrated terminal (`ctrl+j` is the reliable one) |
| `ctrl+g`, `alt+g`, `F5` | toggle the Git panel (`g` in the explorer, or click `⎇ Git`) |
| `esc` | close file / dismiss |
| `?`, `F1` | help overlay (scrolls with `↑↓`, `pgup/pgdn`, `g/G`, wheel) |
| `ctrl+c` | quit (copies a selection first, if there is one) |
| drag borders | resize sidebar · terminal · editor/response · markdown split |
| drag any text | select a block of the screen, copied on release |

## Explorer

| Key | Action |
| :--- | :--- |
| `↑↓`, `j k` | move |
| `⏎`, `l`, `h` | open · expand · collapse (or jump to parent) |
| `g`, `G`, `home`, `end` | top / bottom |
| `pgup`/`ctrl+u`, `pgdn`/`ctrl+d` | page |
| `x`, right-click | context menu: new file, new folder, rename, delete |
| `ctrl+n`, `ctrl+f` | new file · new folder |
| `ctrl+e`, `ctrl+d` | rename · delete (`y`/`n` confirms) |
| `r` | refresh the tree |
| `/` | find file by name |
| click | single click opens a file or toggles a folder; there is no double-click anywhere |

## Runner view

| Key | Action |
| :--- | :--- |
| `tab`, `shift+tab` | URL bar → tabs → editor → response |
| `⏎` on the URL | method picker (`←/→`, first letter, `⏎`) |
| `ctrl+←/→` | cycle the method |
| `←/→`, `1/2/3` | switch Params / Headers / Body when the tab bar has focus |
| `alt+1/2/3` | switch tabs from inside the editor |
| `ctrl+l` | format the body as JSON |
| `ctrl+f` | search the response; `⏎`/`n` next, `shift+⏎`/`N` previous |
| `h` | response headers ⇄ body |
| `g`, `G`, `pgup`, `pgdn` | scroll the response |
| click `▶ Send` | send |

## Editor

Every editor is the same component: the Text view, the Params / Headers / Body tabs, markdown files, the Git panel's edit mode and the commit-message composer.

| Key | Action |
| :--- | :--- |
| `ctrl+z`, `ctrl+y` | undo · redo (200 steps) |
| `ctrl+c`, `ctrl+x`, `ctrl+v` | copy · cut · paste (`ctrl+c` quits only when nothing is selected) |
| `ctrl+a` | select all |
| drag, `shift+←→↑↓`, double-click | select · select a word |
| `ctrl+f`, `⏎`, `F3` | find · next match |
| `ctrl+g` | go to line |
| `ctrl+o`, click `▾`/`▸` | fold / unfold the block at the cursor |
| `alt+o`, click `[ ▾ Collapse ]` | collapse every block, or expand them all when any is collapsed |
| 2 typed chars, `ctrl+space` | suggestions; `↑↓`/`ctrl+p/n` select, `⇥`/`⏎` accept, `esc` close |
| `alt+z` | word wrap on / off |
| `shift+wheel`, wheel left/right | scroll sideways with wrap off |
| `ctrl+←/→`, `alt+←/→` | jump by word |
| `tab`, `shift+tab` | indent two spaces · leave the editor |

## Markdown

| Key | Action |
| :--- | :--- |
| `ctrl+t`, click `[ Text \| Preview \| Split ]` | Text → Preview → Split (the choice is remembered across files) |
| `tab` | editor ⇄ preview in split view |
| `j k`, `↑↓`, `pgup/pgdn`, `g/G`, wheel | scroll the preview |

## Find palette

| Key | Action |
| :--- | :--- |
| `ctrl+p`, `/` in the explorer, click `Find` | fuzzy file finder (skips `node_modules`, `.git`, `.next`, `dist`, `build`, `vendor`, `target`, `.cache`) |
| `alt+f` | live grep: two or more characters, `path:line` results stream in (ripgrep → git grep → Go walk) |
| `tab` | switch between the two |
| `↑↓`, `⏎` | move · open the file (at that line for grep) |

## Terminal panel

| Key | Action |
| :--- | :--- |
| `ctrl+j`, click the `▸ TERMINAL` strip | toggle / focus |
| `ctrl+b` | back to the explorer; every other key goes to the shell |
| `ctrl+c` | copy the selection, else interrupt the shell |
| drag | select output text, copied on release |
| wheel, `shift+↑↓`, `shift+pgup/pgdn` | scroll back (5000 lines); any key returns to the bottom |
| `ctrl+v` | paste (multi-line pastes arrive as one bracketed block) |
| drag the strip | resize the panel |
| any key after exit | restart the shell |

## Git panel

| Key | Action |
| :--- | :--- |
| `1-5`, `h`/`l`, `←/→`, click | Status · Commits · Branches · Stashes · Blame |
| `tab`, click | file list ⇄ diff; `esc` back to the list |
| `/` | filter files · search commits |
| drag `│`, drag `─` | resize list / diff columns · list / graph rows |

### Status

| Key | Action |
| :--- | :--- |
| `+`/`s`, `−`/`u` | stage · unstage the file (or click `[ + ]` / `[ − ]`) |
| `a`, `A` | stage all · unstage all |
| `d`, `D` | discard the file · undo all changes (`y`/`n` confirms) |
| `e` | edit the working copy in the preview pane |
| `v`, `z`, `w` | inline ⇄ split · wrap lines · ignore whitespace |
| `c`, `S` | commit (type picker, then the message editor) · stash |
| `p`, `P`, `f`, `y` | push · pull · fetch · sync |
| `⏎` | open the file · collapse a group |

### Diff pane

| Key | Action |
| :--- | :--- |
| `j k`, `shift+↑↓` | move the cursor · select lines |
| `s`, `u`, `d` | stage · unstage-or-revert · revert the selected lines or the cursor's hunk |
| click `[ + stage ]`, `[ − unstage ]`, `[ ⟲ revert ]` | the same, from the hunk header |
| click the right side of a split diff | edit that line in place; `⟲` and `+` in the gutter revert or stage a hunk; `esc` returns |
| `⏎` on a file header (commits) | fold / unfold that file |
| `ctrl+c` | copy the selected lines |

### Commit message editor

| Key | Action |
| :--- | :--- |
| `←/→`, first letter, `⏎`, `esc` | type picker (conventional repos only) |
| `ctrl+s` | commit |
| `ctrl+r` | redraft with the model, when one is installed |
| `esc` | cancel; the draft is kept for the next `c` |

### Commits · Branches · Stashes · Blame

| Section | Keys |
| :--- | :--- |
| Commits | `⏎`/`jk` browse · `J/K` scroll the diff · `f` file ↔ repo history · `/` search · `y` copy hash · click the graph to select a commit |
| Branches | `⏎` checkout · `n` new · `d` delete · `f` fetch |
| Stashes | `⏎` pop · `s` stash · `d` drop |
| Blame | `⏎` open the editor at that line · `tab`/click to move through lines · `b` inline blame in the editor gutter |

### Merge conflicts

| Key | Action |
| :--- | :--- |
| `c`/`1`, `i`/`2`, `b`/`3` | accept current · incoming · both |
| `n`/`↓`, `p`/`↑` | next · previous conflict block |
| `J`, `K` | scroll |
| `o` | open the file in the editor |
| `a`, `⏎` | mark resolved (refuses while blocks remain) |
| `esc` | leave the resolver |

## Mouse

| Gesture | Where |
| :--- | :--- |
| click | files, folders, tabs, toggles, the method badge, the URL cursor, the editor cursor, the response, every button in the Git panel |
| wheel | one row per event, everywhere |
| drag over text | select and copy on release, anywhere; editors, the terminal and the Git diff keep their own selections |
| drag a splitter | sidebar `│`, the `▸ TERMINAL` strip, the editor/response border, the markdown seam, the Git panel's `│` and `─` |
| hover a splitter | it lights up; terminals that honour OSC 22 also show a resize pointer |
