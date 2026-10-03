---
title: Git panel
description: Status and staging by file, hunk or line; split diffs you can edit in place; commits as a per-file accordion; blame; branches, stashes, sync and conflict resolution.
---

Open it with <kbd>ctrl+g</kbd>, <kbd>alt+g</kbd>, <kbd>F5</kbd>, <kbd>g</kbd> in the explorer, or the `⎇ Git` button. The panel takes the main pane and keeps the file tree beside it. It has two columns like the VS Code source control view: the section list on the left with a clickable commit graph under it, and a detail pane on the right at full height. <kbd>esc</kbd> closes it.

Git state is polled every two seconds off the UI thread, so a `git checkout` in the integrated terminal or a commit from another window shows up without a refresh. The explorer carries the same information: `M` `A` `D` `U` `!` badges on files and a `●` on folders that contain changes.

## Status

Two collapsible groups, **Staged Changes** and **Changes** (untracked included). Each header has a `[ + stage all ]` or `[ − unstage all ]` button; each row has `[ + ]` / `[ − ]` and `[ ⟲ ]`. One click, or <kbd>s</kbd> / <kbd>u</kbd>, moves the file. <kbd>d</kbd> discards a file and <kbd>D</kbd> undoes everything, both behind a `y/n` confirmation. <kbd>/</kbd> filters the list.

The selected file's diff fills the detail pane:

- <kbd>v</kbd> flips between inline and side-by-side. <kbd>z</kbd> wraps or clips long lines. <kbd>w</kbd> runs the diff with `-w` to hide whitespace-only changes.
- Added and removed lines carry a full-width tint. Tabs are expanded and trailing spaces shown as `·`, so a re-indent is not an invisible all-red, all-green diff.
- Drag over lines to select them; <kbd>ctrl+c</kbd> copies.

### Staging by hunk or line

<kbd>tab</kbd> moves the keyboard into the diff. <kbd>j</kbd> <kbd>k</kbd> move a cursor, <kbd>shift+↑↓</kbd> select rows, and then:

| Key / button | Effect |
| :--- | :--- |
| `s`, `[ + stage ]` | stage the selected lines, or the cursor's hunk |
| `u`, `[ − unstage ]` | unstage (on a staged file) |
| `u`, `d`, `[ ⟲ revert ]` | revert the lines in the working copy (confirmed) |

Partial patches are built the way `git add -p` does it and applied with `git apply --recount`. Untracked files and `-w` diffs are excluded because neither patch applies cleanly. The diff cursor stays where it was after an operation.

### Editing inside the diff

Two ways to fix something without leaving the panel:

- <kbd>e</kbd> on an unstaged, non-conflicting file replaces the diff with the code editor on the working copy. It is the same editor as everywhere else, so undo, selection, folding and completion all work. Edits autosave through the same document store the main editor uses, so an open tab stays in sync. <kbd>esc</kbd> returns to the diff.
- In a **split** diff, click the right (new) side. The editor takes the right column at that line while the index side stays on the left. The two are diffed live as you type: removed lines show red on the left, paired with the editor rows that replaced them; added lines are tinted green in the editor; a pure deletion folds its lines onto the row after it. Between the columns each hunk gets `⟲` (put the index lines back, undoable with <kbd>ctrl+z</kbd>) and `+` (stage that hunk straight from the buffer with `git apply --cached`, without leaving the editor). Wrap follows the panel's `[wrap]` toggle and <kbd>⌥z</kbd>; wheel left/right scroll both columns together.

## Commits

The list shows the repository history, or the open file's history after <kbd>f</kbd>. <kbd>/</kbd> searches message, author or hash; <kbd>y</kbd> copies the hash.

A commit opens as an **accordion**: the message first, then one header row per file with `+n −m` counts. Click a header, or press <kbd>⏎</kbd> on it after <kbd>tab</kbd>, to fold that file's diff; folds are remembered per path. The commit graph under the file list (`git log --graph --oneline`) is clickable too: the row highlights, its commit fills the detail pane, and the list takes over again when you move the cursor. Scrolling to the end of the graph loads more history.

## Blame

The left column groups the commits that wrote the file, newest first, with line counts. The right column is the annotated source: hash, author, age and line number in front of every line, the selected commit's lines marked. The sidebar stays beside the panel here, and clicking a file in it re-blames that file without leaving the tab. <kbd>⏎</kbd> opens the editor at the selected commit's first line (or the cursor line after <kbd>tab</kbd>). <kbd>b</kbd> toggles a GitLens-style gutter annotation in the editor and a current-line blame note in its status row.

## Branches, stashes, sync

| Section | Actions |
| :--- | :--- |
| Branches | `⏎` checkout · `n` new branch · `d` delete · `f` fetch (`--prune`) |
| Stashes | `⏎` pop · `s` stash (optional message) · `d` drop; the detail pane shows the stash |
| Sync | `p` push · `P` pull (`--ff-only`) · `f` fetch · `y` sync, which mirrors VS Code: pull then push, or publish the branch to `origin` when it has no upstream |

Push and pull show a live spinner in the panel header and in the top bar's sync pill. Every git invocation is serialised, status is polled with `--no-optional-locks`, and transient `index.lock` collisions are retried.

## Merge conflicts

A merge, rebase or cherry-pick that stops on conflicts adds a `⚠ merge in progress` group with `[ ✕ abort ]` and a `[ resolve ]` button per conflicted file. The resolver shows each `<<<<<<<` block:

| Key | Action |
| :--- | :--- |
| `c`/`1`, `i`/`2`, `b`/`3` | accept current · incoming · both |
| `n`, `p` | next · previous block |
| `o` | open the file in the editor instead |
| `a`, `⏎` | mark resolved (runs `git add`; refuses while blocks remain) |
| `esc` | back to the list |

When every file is resolved the group offers to continue, which runs the matching `rebase --continue`, `cherry-pick --continue` or `commit --no-edit`.

## Committing

Press <kbd>c</kbd>. If the repository's history follows conventional commits, a type picker appears in the footer with the likely type preselected; <kbd>⏎</kbd> accepts it. A full editor then opens over the detail pane already holding a valid draft: a conventional subject under 72 characters and one bullet per directory with file names and `(+n −m)`. <kbd>ctrl+s</kbd> commits, <kbd>esc</kbd> cancels and keeps the draft for the next <kbd>c</kbd>.

With a local model installed the same editor fills in before the model answers, and the model's rewrite streams over the draft, checked claim by claim against the staged changes. If you start typing, your text is never overwritten. <kbd>ctrl+r</kbd> asks for another draft. How that works, and how to turn it on or off, is on the [next page](/docs/shellapp-ai).

The rules are those of `@commitlint/config-conventional`: the eleven types (`feat fix docs style refactor perf test build ci chore revert`), lowercase type, no trailing period, a blank line before the body. In a repository without conventional history the drafter uses plain imperative subjects and never invents a type or scope.
