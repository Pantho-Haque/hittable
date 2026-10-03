---
title: Commit messages with a local model
description: Optional, local, and verified. A small model running on your machine rewrites the mechanical commit draft; every claim it makes is checked against the staged diff before you see it.
---

## What you get without it

Everything. Pressing <kbd>c</kbd> in the Git panel opens the message editor with a draft derived from the staged diff: a conventional-commit subject and one bullet per changed directory with file names and `(+n −m)`. It is valid the moment it appears. There is no separate code path for "no model": the draft is the same object the model is later asked to improve on.

## Turning it on

```sh
hittable model enable
```

Nothing is downloaded until you run this. It prints exactly what it will cost, asks, and does nothing if you say no or if stdin is not a terminal:

```
  Download
    llama.cpp b11120 · macos-arm64                      11 MB
    qwen2.5-coder 3B instruct · Q4_K_M                2.10 GB
                                               ──────────
    total                                            2.12 GB

  Disk
    installs to ~/.hittable                          2.13 GB
    free now 245.1 GB, after                          243.0 GB

  Memory
    about 3.20 GB resident while generating
    this machine has 16.00 GB
    the server stops on its own after 10 idle minutes

  Continue? [y/N]
```

What it installs, pinned to exact versions and SHA-256 sums:

| Item | Value |
| :--- | :--- |
| Runtime | `llama-server` from llama.cpp release `b11120`, the prebuilt archive for your OS and CPU |
| Model | `qwen2.5-coder-3b-instruct-q4_k_m.gguf`, 2.10 GB, fetched from a pinned Hugging Face commit |
| Context | 8192 tokens |
| Memory | about 3.2 GB resident while generating |
| Location | `~/.hittable` (or `$HITTABLE_HOME`) |

Supported platforms are those llama.cpp publishes prebuilt binaries for: macOS arm64 and x64, Linux x64 and arm64, Windows x64 and arm64. On Apple Silicon the model runs on the GPU through Metal; the other prebuilt archives are CPU-only.

The install verifies checksums before writing anything, installs atomically, ad-hoc signs the binaries on macOS, and finishes with a smoke test: spawn the server, wait for `/health`, run one 16-token completion. A missing dylib or a corrupt download surfaces then, not a week later inside the TUI. An interrupted download resumes on the next `model enable`.

### Requirements

- Free disk of at least 1.3 × the install size, checked before downloading.
- 4 GiB of memory is the hard floor. Under 8 GiB you get a warning that generating is likely to swap.
- On an Apple M2 the model generates at roughly 38 tokens/second. A warm commit message takes 3 to 5 seconds. A cold start adds about 2.8 seconds, which the pre-warm on opening the Git panel normally hides.

## Using it

Press <kbd>c</kbd> as before. The editor opens with the mechanical draft immediately; the model's rewrite streams over it. The footer tells you where the text came from: `heuristic`, `model`, or `model (repaired)` when something was fixed or dropped. If nothing the model said survived verification, you keep the mechanical draft and the footer says so.

| Key | Action |
| :--- | :--- |
| `ctrl+s` | commit |
| `ctrl+r` | redraft with the model |
| `esc` | cancel generation, or close the editor and keep the draft |

Once you type into the draft it is yours; a redraft will not overwrite it.

## What the model is and is not allowed to do

- **It says what changed, not why.** The reasoning is not in the diff and is not invented.
- **It never sees a raw diff.** It is handed a specification built from the staged changes: the directories that changed, each package's own doc comment, and the exported symbols the change adds. No file counts and no line counts, because a 3B model given a number repeats the number instead of describing behaviour.
- **Every claim is checked.** A bullet naming a path or symbol that is not in the staged change is dropped. A bullet attributing a symbol to the wrong directory is dropped. If the subject names something invented, the whole answer is discarded. If too few bullets survive, the body falls back to the mechanical summary.
- **The shape cannot be wrong.** Decoding is grammar-constrained by `llama-server`, so the conventional-commit form, the lowercase subject, the missing trailing period and the 72-character cap are structurally impossible to violate.
- **Secrets stay out.** Files that look like credentials (`.env*`, `*.pem`, `*.key`, `id_rsa*`, `*credential*`) are listed by name only; their contents are never packed into a prompt. A test drives a real repository end to end to prove it.

The server listens on localhost. Once the download is done there is no network traffic, and no part of your diff leaves the machine.

## How it runs

The supervisor starts at most one `llama-server`, lazily, on the first request that needs it, and never at launch. Before spawning it reuses, in order: a server it started itself, a live server another hittable window recorded in `~/.hittable/run/llama.json`, or a `llama-server` you run yourself on port 8080. Only the first is ever owned, and only an owned server is ever stopped, so three windows share one process and the first to quit does not take it away from the others.

It asks the kernel for a free loopback port, starts with `--ctx-size 8192 --parallel 2 --cont-batching --jinja --no-webui` (plus `--n-gpu-layers 999` on Apple Silicon), waits for `/health`, and stops itself after ten idle minutes. A server that keeps dying is not restarted more than three times in five minutes.

```
~/.hittable/
├── runtime/<tag>/     immutable runtime installs; runtime/current names the active one
├── models/            downloaded weights
├── run/llama.json     pid, port and model of the supervised server
├── tmp/               in-progress downloads (<name>.part)
├── logs/              process logs, ring-trimmed at 1 MB
└── config.json        the ai settings
```

## Status, off, delete

```sh
hittable model status    # runtime, model, disk, server, and the ai setting
hittable model disable   # stop the server, free ~3.2 GB of RAM, keep the files
hittable model delete    # remove the files, reclaim ~2.1 GB of disk
```

`disable` and `delete` are separate on purpose: nobody who only wants their memory back should pay for a two-gigabyte download to get it back later. After `disable`, `model enable` turns it straight back on with nothing to fetch. Both flip `ai.enabled` in `~/.hittable/config.json`.

## Using a server you already run

Set `HITTABLE_AI_ENDPOINT` (or `ai.endpoint` in the config file) to the base URL of any OpenAI-compatible server, and `HITTABLE_AI_MODEL` to the model it should request. hittable will then never spawn or download anything. `HITTABLE_AI=0` turns drafting with a model off entirely. See the [CLI reference](/docs/shellapp-cli#configuration-file) for the full config schema and precedence.
