# Commit message generator for the shellapp TUI

> Status: shipped. Supersedes the heuristic-only draft of this document.
> `c` pre-fills a compose editor. With a model installed, it writes the subject
> and the body; without one, the mechanical draft stands alone.
>
> One thing here was wrong and is worth recording. This document originally had
> the model "rewrite the draft into prose". It does not, because that was tried
> and it invented details — a preview pane that did not exist, in every one of
> five runs. What ships instead is narrower: the model is shown a
> *specification* of the change (directories, each package's own doc comment,
> the exported symbols added — deliberately no line counts), and every claim it
> makes is checked back against that specification before it reaches the editor.
> See "Getting a valid message out of a 3B model" below.

## Context

The Git panel's commit flow (`c` in the Status section) opens an empty single-line
prompt (`gitpanel.go:814-815`, `:895-947`). Every commit message is typed from
scratch, even though git already knows exactly what changed.

This repo enforces conventional commits (`commitlint.config.js` at the **monorepo
root**, 11-type enum, lowercase subject, no trailing period, blank line before body)
and its history is consistent: `feat(shellapp): …`, `fix(shellapp): …`,
`feat(workspace): …`. That regularity makes the mechanical parts of a message —
type, scope, which files moved, how much — derivable from `git diff --cached` with
plain string work.

The earlier version of this document stopped there, and said so:

> *Explicit non-goal: generating the prose body. The real commit bodies in this repo
> explain why a change was made and no heuristic can produce that.*

That is the ceiling this revision removes. The bodies in this repo run to multiple
paragraphs — the v2.8.10 and v2.9 commits are 20+ lines each — and a string
heuristic will only ever emit `- ui/screens: view.go (+96 −31)`. Describing a change
needs a model.

**Goal:** press `c` and land in an editor already holding a well-formed conventional
commit — factual from the first frame, and improved by a local model if one is
installed, without ever asserting something the diff does not support.

## Two things that are easy to get wrong here

**The git root is the monorepo, not `shellapp/`.** `rev-parse --show-toplevel` is the
repository root and `--show-prefix` is `shellapp/`, so `git diff --cached` is
repo-wide: pressing `c` in this panel already commits web-app files. The digest ranks
`Repo.Prefix` files first and uses `shellapp` as the leading scope candidate, and the
learned history mixes `feat(workspace)` with `feat(shellapp)`.

**`commitlint.config.js` lives above the Go module.** The shellapp opens arbitrary
directories, so the rule set is a value struct with conventional-commit defaults —
never a path assumption. A repo whose history is not conventional gets a plain
imperative subject and no type picker.

## Flow

```
c → nothing staged? → "nothing staged", stop
  → type picker in the footer, inferred type preselected
  → ⏎ → compose editor opens NOW, pre-filled with the heuristic draft
         └─ model installed? generation streams over the draft
  → ctrl+s commits · esc cancels (draft kept for the next c)
```

The editor opens **before** the model answers, and that ordering is the whole design:

- A usable, commitlint-valid message exists within ~5ms, every time.
- `esc` mid-generation means one obvious thing — cancel the model, keep the buffer —
  so no new key vocabulary.
- With no model the flow is byte-for-byte identical; the editor simply never changes.
  Graceful degradation is not a code path, it is the absence of one.
- It hides the cold model load behind a screen the user is already reading.

### Why reuse `texteditor` instead of growing the prompt

The panel already runs an editor submode for "edit in preview": `Editing bool` +
`Editor *texteditor.TextEditor` (`gitpanel.go:135-139`), created at `:469-495`, keys
routed at `:643-656`, rendered over the detail pane at `:1325-1328`, mouse at
`:1029-1034` and `:1066-1071`. A `Composing` submode copies that shape almost line
for line and inherits undo/redo, selection, mouse, wrap, folding and autocompletion.

Note the panel has a **third** submode, `Resolving` (`:114`, `:639`, `:986`, `:1000`,
`:1015`). Every `if p.Editing` chain has a `Resolving` sibling, and `Composing` must
be threaded through all of them or `esc` and mouse clicks misroute.

No `gitx` commit change is needed: `Repo.Commit` passes the whole string as one `-m`
(`gitx.go:229-232`), which git already treats as subject + body.

## Architecture

```
internal/hithome     ~/.hittable layout. No logic.
internal/appconfig   config load + precedence.
internal/llm         stdlib HTTP client. Knows nothing about commits, git or provisioning.
internal/llmhost     downloads, verifies, installs and supervises llama-server.
internal/commitmsg   this feature: heuristic + digest + prompt + validator.
```

Strict one-way dependencies: `commitmsg → llm`, `llmhost → llm`. `llm` imports
nothing from the app, which is what makes it reusable for explain-a-diff,
explain-a-response, inline completion and codebase Q&A later.

`*gitx.Repo` satisfies `commitmsg.Runner` structurally, so there is no import cycle
and tests fake git in three lines:

```go
type Runner interface{ Run(args ...string) (string, error) }
```

### One heuristic, three roles

`Digest` is the shared intermediate, so the heuristic and the model consume
byte-identical input and the "fallback" is genuinely the same code path:

| File | Role |
|---|---|
| `collect.go` | `Collect(Runner) (*Digest, error)` |
| `digest.go` | **(b)** prompt input — `Pack(budget int) string` |
| `heuristic.go` | **(a)** fallback — `Generate(*Digest, Options) Message` |
| `validate.go` | **(c)** validator/repairer — `Validate`, `Repair`, `Grammar` |
| `prompt.go` | `BuildRequest(*Digest, Options) llm.Request` |
| `draft.go` | `Draft`, `DraftStream` |

`Message.Source` (`SourceHeuristic` / `SourceModel` / `SourceRepaired`) lets the
footer label the draft honestly. A user told the model was skipped will trust the one
time it wasn't.

## Generation rules (heuristic)

**Type** — preselects the picker, never silently final: all `*_test.go`/`*.spec.*` →
`test`; all `*.md`/`docs/` → `docs`; all `*.css`/`*.scss` → `style`; all
`go.mod`/`go.sum`/`package.json`/lockfiles/Dockerfile/Makefile → `build`; all
`.github/workflows` → `ci`; branch prefixed `fix/`, `bugfix/`, `hotfix/` → `fix`; any
added source file → `feat`; deletions only → `refactor`; otherwise `feat`.

**Scope** — most meaningful segment of the common path prefix, preferring one that
appears in the learned history scope counts, so `shellapp`, `workspace`, `texteditor`
win over invented names. Omitted when there is no clear candidate.

**Subject** — first rule that fires: renames → `rename x to y`; single added file →
`add <basename>`; deletions only → `remove <names>`; new exported symbols in added
lines → `add fold all and unfold all`; tests only → `add tests for <scope>`; fallback
→ `update <scope>`. Symbols are split on camelCase and lowercased (`FoldAll` →
`fold all`). Header capped at 72, trailing period stripped.

**Body** — one bullet per directory: basenames (cap 4, then `…`) and `(+n −m)`.

## Diff packing

Budget 12 000 bytes (≈3-4k tokens) against the `--ctx-size 8192` the sidecar is
launched with.

**Survival order**: branch + the full staged file list with status letters and
`+n −m` (from numstat, not `--stat`, whose column padding wastes budget) → per-file
hunks ranked by `added+removed`, demoted for lockfiles and generated output,
promoted for files declaring new exported symbols, promoted under `Repo.Prefix` →
per file, first 3 hunks, ≤40 lines each split head/tail so both ends survive.

**Drop ladder** when over budget: context lines first (`--unified=0`) → trailing
hunks → whole low-rank files collapsed to one line → finally the file list alone,
which always fits. `Digest.Truncated` records this so the prompt says "the diff was
truncated, describe what you can see" rather than letting the model invent.

**Hard caps before packing**: 200 files, 1MB raw diff, any single file over 64KB →
counts only, binaries (`-\t-` in numstat) → `binary`, never read. **Secrets**
(`.env*`, `*.pem`, `*.key`, `id_rsa*`, `*credential*`) are listed by name and their
contents are never packed. The traffic is localhost-only today, but this is the exact
path that would ship to a remote provider the day anyone adds one.

## Getting a valid message out of a 3B model

Three mechanisms, strongest first.

**1. Grammar-constrained decoding.** `Grammar(rules, pickedType)` emits GBNF that
llama-server enforces *during sampling*:

```gbnf
root    ::= header "\n\n" body
header  ::= type scope? "!"? ": " subject
type    ::= "feat"                        # narrowed to the type the user picked
scope   ::= "(" [a-z0-9][a-z0-9-]* ")"
subject ::= [a-z] [^\n.]{3,68}
```

That makes `type-enum`, `type-case`, `subject-case`, `subject-full-stop`,
`body-leading-blank` and the 72-char cap **structurally unviolatable** — five of the
eight commitlint error rules. A small model's failure modes are overwhelmingly
formatting, not content, so this converts the hardest problem into a non-problem. It
is the concrete reason the sidecar is llama-server rather than a chat-only endpoint.

**2. A short prompt with the heuristic as an anchor.** The user has already picked
the type, so the model is never asked for it. It is handed the packed digest and the
heuristic subject and told to improve on it — a 3B is far better at improving a
candidate than inventing one, and the worst realistic outcome becomes "the heuristic
subject, reworded". Few-shot: three entries from the repo's own `git log` that pass
`Validate`, deduped by type, bodies truncated to three lines. They teach register and
shape, not content.

**3. Validate → repair → fallback**, because a grammar cannot enforce semantics.
Strip fences and `Commit message:` preambles; map type synonyms; **lowercase only the
first character** — commitlint rejects sentence/start/pascal/upper as a *class*, so
`add FoldAll to the editor` is legal and blanket-lowercasing would destroy
identifiers; strip a trailing `.`; truncate at a word boundary ≤72; force one blank
line before the body; reject bodies containing `As an AI` or that merely restate the
subject. Then: mechanical repair → one re-ask at `temperature 0` (skipped while
streaming, where the user is already reading) → heuristic, with the footer saying
`heuristic draft · model output rejected`.

### Grounding: the check that replaced the phrase filter

The first attempt at keeping the model honest was a list of forbidden phrases —
`easy to use`, `intuitive`, `allows users to`. It does not work, and the reason
is worth stating: a phrase list filters *wording*, and the problem is *truth*.
Five runs on the same diff all invented a preview pane; four were rejected for
how they were worded and the fifth passed saying exactly the same false thing in
different words.

What works is checking each claim against the specification the model was given:

- a path a bullet names must be one the change touched;
- an identifier must appear in the spec's vocabulary;
- a symbol named beside a directory must belong to that directory.

Two refinements came out of real use, both of which were rejecting correct
bullets. Anything quoted from the spec is grounded by definition — purpose lines
are the author's prose and contain paths like `~/.hittable/config.json` that are
not part of the change. And a word preceded by an article is prose, not an
identifier: "which closes the AI" is English, even though `AI` is also a type.

Bullets are dropped individually rather than discarding the body. One wrong
claim in six used to throw away five correct ones and leave the user watching
the model's work be replaced by the file list. If fewer than half survive, the
mechanical body is the honest answer.

## Streaming into the editor

A 3B emits 30-80 tok/s; one redraw per token would be 80 full frames a second. The
stream is coalesced exactly the way `terminal.wake()` (`terminal.go:171-178`) already
does it: `CompareAndSwap(false,true)` → 8ms sleep → one `Send`. Chunks are dropped if
the sequence number is stale (the `palette.go:201-203` supersede rule) or if the user
has started typing — **typing is never overwritten**, the same precedence CLAUDE.md §5
states for the document store.

`p.Busy = "drafting…"` is the entire spinner change: `m.busy()`
(`ui/screens/update.go:49`) already reads `Git.Busy != ""`, the tick starts itself,
and `ui/screens/view.go:18` feeds the frame in. Because `Composing` paints the editor
over the *detail pane* while the footer paints the spinner, both show at once.

Two details that bite otherwise: `SetContent` resets the cursor, so it is moved back
to end-of-buffer each update; and the virtual path is `COMMIT_EDITMSG`, with no
extension, so Chroma picks no lexer.

## Installing the model

`hittable model enable` downloads a pinned `llama-server` build and
`qwen2.5-coder-3b-instruct-q4_k_m.gguf` into `~/.hittable`, after showing what it will
cost in disk and RAM and asking for confirmation. It verifies SHA256 before installing
anything, installs atomically so an interrupt can never leave a half-state, ad-hoc
signs the binary on macOS, and finishes with a smoke test — spawn, `/health`, one
16-token completion — so a missing dylib or corrupt GGUF surfaces at install time
rather than silently inside the TUI days later.

`hittable model disable` stops the server and releases its ~2.4GB of memory,
leaving the model on disk so re-enabling costs nothing. `hittable model delete`
removes it from the machine entirely. `hittable uninstall`
additionally clears every directory any version of the app could have created.

Enabling from the **integrated terminal takes effect without restarting the TUI**: the
existing 2s git-status tick also stats the model file and the config mtime, and flips
the panel's drafter live on a false→true transition. Same mtime/size trick
`reloadExternalEdits` already uses for open documents (v2.8.6).

## Bounds

Generation runs through `p.async` (`gitpanel.go:870-878`), never `p.run` — `p.run` is
synchronous on the UI goroutine and would freeze the app. The collect-and-pack step
stays synchronous because it is only git plumbing. If nothing is staged, the footer
says `nothing staged` and the picker never opens.

## Verification

`make test` must pass on a machine with nothing downloaded, no llama-server and no
network — that is the default state for every user and for CI. Tests set
`HITTABLE_HOME` to a temp dir and `HITTABLE_AI=0`.

- `validate_test.go` is the highest-value table: one row per commitlint rule plus the
  realistic small-model failures (fenced output, `Here is the commit message:`,
  `Feat: Add Fold All.`, a 140-char subject, an invented type, `As an AI language
  model`), and a row asserting `add FoldAll to the editor` survives repair unchanged.
- `digest_test.go`: a 500-file digest packs under budget, the drop ladder order,
  lockfile and binary demotion, secrets excluded by name, `Prefix` files first.
- `collect_test.go`: a real temp repo, mirroring `newRepo(t)` at
  `internal/gitx/gitx_test.go:10-31`.
- `internal/llm`: `httptest` only — an SSE frame deliberately split across two writes
  (the v2.8.3 failure mode), a line over 1MB proving the `Scanner.Buffer` bump,
  mid-stream cancel returning `ctx.Err()` with partial text preserved.
- Panel: `TestGitPanelComposeCommit` modelled on `TestGitPanel`
  (`ui/screens/regress_test.go:255-320`) with AI off, plus streaming, stale-sequence,
  never-overwrite-typing, no-network-at-startup, and model-appears-without-restart.
  Re-run the v2.8.7 frame-size regression with the panel in compose mode — the footer
  is one truncated row and the type picker has to fit at 50 columns.
