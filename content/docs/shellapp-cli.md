---
title: CLI reference
description: Every command, flag and environment variable the hittable binary understands.
---

## Commands

```sh
hittable [path]                # open a directory (never writes anything)
hittable init [path]           # create the hittable/ template
hittable -i collection.json    # import a Postman v2.1 or Insomnia export, then open
hittable -e postman            # export hittable/ as <folder>.postman_collection.json
hittable -e insomnia           # export hittable/ as <folder>.insomnia.json
hittable -e postman -o x.json  # choose the output file
hittable --icons emoji .       # emoji file icons instead of Nerd Font glyphs

hittable model enable          # download a local model for commit messages (asks first)
hittable model status          # what is installed, how much disk, is it running
hittable model disable         # stop the server, free the memory, keep the files
hittable model delete          # remove the model and runtime, reclaim the disk

hittable update                # update to the latest release (verified download)
hittable version               # hittable <version> <os>/<arch>
hittable uninstall             # remove hittable, its data and the model
hittable -h                    # all options
```

`hittable` takes at most one positional argument. An explicit path is made absolute and must be an existing directory; without one the current directory is used.

## Root flags

| Flag | Meaning |
| :--- | :--- |
| `-i`, `--import <file>` | import a Postman v2.1 or Insomnia v4 export into `hittable/`, then open the app. Prints a report: requests, folders, variables, and anything dropped |
| `-e`, `--export [postman\|insomnia]` | export `hittable/` as a collection; a bare `-e` means `postman`. Writes `<folder>.postman_collection.json` or `<folder>.insomnia.json` next to `hittable/` and prints `exported N requests → <file>` |
| `-o`, `--out <file>` | output file for `--export` |
| `--icons <nerd\|emoji>` | file icon pack; defaults to `nerd` |
| `--version` | same as `hittable version` |

Export without a `hittable/` folder fails with a pointer to `hittable init` or an import.

## `hittable init [path]`

Creates `hittable/testcollection/test.hit` (a real, runnable request), `hittable/notes/sample.md` and `hittable/env.json`. If `hittable/` already exists it says so and exits 0 without touching anything. See [Project format](/docs/project-format) for the files it writes.

## `hittable model …`

| Command | What it does | Flags |
| :--- | :--- | :--- |
| `model enable` | show the disk and memory cost, ask, then download the runtime and the model into `~/.hittable` | `-y`/`--yes` skip the prompt · `--dry-run` show what would be downloaded and stop |
| `model status` | runtime, model, disk use, whether the server is running, and the AI setting | |
| `model disable` | stop the server and free about 3.2 GB of memory; the files stay | |
| `model delete` | remove the model and runtime from disk | `-y`/`--yes` |

Nothing is downloaded until you run `model enable`, and a non-interactive stdin answers "no", so a script piping into it never starts a 2 GB download by accident. Details on what gets installed and how it runs are on the [Commit messages with a local model](/docs/shellapp-ai) page.

## `hittable update`

| Flag | Meaning |
| :--- | :--- |
| `-y`, `--yes` | skip the confirmation prompt |
| `--check` | report whether an update exists, install nothing |
| `--tag <tag>` | install this exact release tag instead of the latest |

The download is verified against the release's `checksums.txt`; a release that publishes no checksums is refused. A source build (`version dev`) asks before replacing itself with a release. A running instance keeps the old binary until it exits. <kbd>ctrl+c</kbd> mid-update leaves everything unchanged.

## `hittable uninstall`

Refuses while another `hittable` process is running. Removes, in this order: `~/.hittable` (or `$HITTABLE_HOME`), `~/.config/hittable`, `~/.local/share/hittable`, `~/.cache/hittable` and their XDG equivalents, reporting the space freed; then the binary itself, every `hittable` on your `PATH`, `~/.local/bin/hittable` and `~/go/bin/hittable`. Project folders are never touched.

## Environment variables

| Variable | Effect |
| :--- | :--- |
| `HITTABLE_AI=0` | turn commit-message drafting with a model off. `1`/`true`/`on`/`yes` and `0`/`false`/`off`/`no` are accepted |
| `HITTABLE_AI_ENDPOINT` | base URL of an OpenAI-compatible server you already run; hittable will never spawn or download one |
| `HITTABLE_AI_MODEL` | model identifier to request from that endpoint |
| `HITTABLE_HOME` | relocate `~/.hittable` |
| `HITTABLE_KEYLOG=<path>` | log the key names your terminal actually delivers |
| `HITTABLE_PTYLOG=<path>` | record the raw byte stream a program in the integrated terminal writes, before the emulator sees it |
| `HITTABLE_VERSION`, `HITTABLE_INSTALL_DIR` | installer only: release tag and target directory for `install.sh` |

## Configuration file

`~/.hittable/config.json` is created by `hittable model enable|disable`. An optional `<root>/hittable/config.json` overrides it per project and can be committed. Precedence, highest first: environment → project file → home file → defaults, merged field by field.

```json
{
  "ai": {
    "enabled": true,
    "endpoint": "",
    "model": "qwen2.5-coder-3b-instruct-q4_k_m",
    "timeoutMs": 60000,
    "completionTimeoutMs": 800,
    "maxPromptBytes": 12000,
    "idleMinutes": 10,
    "temperature": 0.2
  }
}
```

A missing, unreadable or malformed file is never fatal: you get the defaults and a warning in the status line. `ai.enabled` means "use a model if one is installed", not a claim that one is. An empty `endpoint` means hittable supervises its own `llama-server`; a non-empty one means "use this server, never spawn, never download".

## Exit codes and output

The TUI runs in the alternate screen with full mouse tracking. CLI subcommands (`init`, `-e`, `model`, `update`, `uninstall`) print plain text and exit non-zero on failure, so they are safe in scripts.
