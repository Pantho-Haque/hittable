---
title: Development
description: Repository layout, running both apps locally, tests, and the conventions pull requests are expected to follow.
---

## Repository layout

One repository, three deliverables:

```
hittable/
├── app/  components/  context/  hooks/  services/  utils/  types/  styles/
│                      the Next.js web app (pnpm)
├── browserExtension/  the Chrome extension: manifest.json, background.js, content.js
├── shellapp/          the Go terminal app (its own go.mod)
│   ├── cmd/hittable/  entry point and CLI commands
│   ├── internal/      .hit schema, env interpolation, HTTP, document store, git wrapper,
│   │                  collection import/export, commit-message drafting, model host
│   └── ui/            the Bubble Tea UI: screens, components, theme
├── content/docs/      these pages, one markdown file each
├── hittable/          a live example project folder
├── video/             Remotion project for the walkthrough video (separate workspace)
└── install.sh         the terminal app installer
```

`shellapp/CLAUDE.md` is the terminal app's full specification and changelog; `shellapp/README.md` the user-facing version. `IMPORT_EXPORT_FORMATS.md` is the format reference the importers follow.

## Web app

```sh
pnpm install
pnpm dev            # http://localhost:3000, Turbopack
pnpm lint
pnpm build && pnpm start
```

Conventions: TypeScript strict, the `@/` alias points at the repository root, components are re-exported from `components/index.ts`, interactive components carry `"use client"`, icons come from `lucide-react`, styling is Tailwind utility classes with a few `@apply` helpers in `styles/`. There is no light theme.

To add a docs page, drop `content/docs/<slug>.md` with a `title:` and `description:` frontmatter and add the slug to a group in `constants/docs.ts`. Pages left out of the manifest still render, under a trailing "More" group. Headings become the sidebar search index and the on-page table of contents; fenced code gets a copy button; tables scroll horizontally.

## Terminal app

Go 1.25 or newer.

```sh
cd shellapp
make build      # ./hittable
make install    # build, install to ~/.local/bin, overwrite every hittable on PATH
make test       # go vet ./... && go test ./... -timeout 300s
```

`make test` passes on a machine with nothing downloaded, no `llama-server` and no network. Tests point `HITTABLE_HOME` at a temp directory and set `HITTABLE_AI=0`. Among them, a frame-overflow test renders every view at seven terminal sizes down to 50×16 and asserts the frame is exactly the terminal's size, because one row too many scrolls the terminal and misaligns every mouse coordinate.

Useful while debugging the UI:

```sh
HITTABLE_KEYLOG=/tmp/keys.log hittable .     # what your terminal sends for each key
HITTABLE_PTYLOG=/tmp/pty.log hittable .      # what a program in the integrated terminal writes
```

Direct dependencies: Bubble Tea and Bubbles, Lip Gloss, Glamour, Chroma, `creack/pty`, `vt10x`, `bubblezone`, Cobra, `mermaid-ascii`. Storage is `encoding/json` only.

## Browser extension

Load `browserExtension/` unpacked from `chrome://extensions` with Developer mode on. There is no build step. Add your dev origin to `content_scripts.matches` if it is not `http://localhost:3000`.

## Releases

| What | Trigger | Output |
| :--- | :--- | :--- |
| Browser extension | tag `v*` | `hittable-extension.zip` on a GitHub release |
| Terminal app | tag `shellapp-v*` | six static binaries (darwin, linux, windows × amd64, arm64), `checksums.txt`, release notes with the install one-liner |
| Web app | push to `main` | Vercel |

The terminal release runs `go vet` and `go test` first. A release that does not pass its own tests is not a release.

## Commits

Conventional commits are enforced by Husky and commitlint: `type(scope): subject`, lowercase type from `feat fix docs style refactor perf test build ci chore revert`, no trailing period, a blank line before the body. The terminal app's commit drafter follows the same rules, so a message it writes passes the hook. `lint-staged` runs `eslint --fix` on staged TypeScript.

```sh
git commit -m "feat(shellapp): fold files in the commit view"
```

## Reporting a bug

Say which app, and for the terminal app which terminal emulator and OS. For a key that does nothing or a program that misrenders in the integrated terminal, attach the `HITTABLE_KEYLOG` or `HITTABLE_PTYLOG` output. For the web app, the browser and whether the extension was installed.
