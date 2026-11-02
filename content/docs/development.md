---
title: Development
description: Repository layout, running both apps locally, tests, and the conventions pull requests are expected to follow.
---

## Repository layout

One repository, three deliverables:

```
hittable/
├── app/                   Next.js App Router
│   ├── page.tsx           landing page
│   ├── hittable/          the web app at /hittable
│   ├── docs/              these docs, rendered from content/docs
│   └── api/
│       ├── proxy/         the request proxy
│       └── github/        OAuth (login, callback, logout, session) and the repo routes:
│                          repos, branches, head, tree, blob, commit, compare, pulls
├── components/
│   ├── workspace/         explorer, Runner host, text, markdown and code editors
│   ├── RequestForm/       URL bar, tab editor, Copy as menu, response panel and its tools
│   ├── palette/           command palette
│   ├── modals/            shortcuts cheatsheet, extension status
│   ├── hittable/          share-link preview
│   ├── notes/  ui/  docs/  homepage/
├── context/               workspace, shortcuts and notification providers
├── hooks/                 useExtension, useKeypress, useNotify
├── services/              the local-folder (File System Access) service, handle store, landing-page data
├── utils/                 send pipeline, curl converter, codegen, JSONPath, JSON diff, share links,
│   │                      env resolution, production guard, docs pipeline
│   ├── server/github/     OAuth, encrypted session cookie, GitHub client, commit, guards
│   ├── server/proxy.ts    the proxy's guards and limits
│   └── workspace/         file kinds, editor languages, tree helpers
├── constants/  types/  styles/  config/
├── tests/unit/            Vitest
├── tests/e2e/             Playwright, with a fake upstream API and a fake GitHub in fixtures/
├── browserExtension/      the Chrome extension: manifest.json, background.js, content.js
├── shellapp/              the Go terminal app (its own go.mod)
│   ├── cmd/hittable/      entry point and CLI commands
│   ├── internal/          .hit schema, env interpolation, HTTP, document store, git wrapper,
│   │                      collection import/export, commit-message drafting, model host
│   └── ui/                the Bubble Tea UI: screens, components, theme
├── content/docs/          these pages, one markdown file each
├── docs/                  requirement specs, such as github-workspace-requirements.md
├── hittable/              a live example project folder
└── install.sh             the terminal app installer
```

`shellapp/CLAUDE.md` is the terminal app's full specification and changelog; `shellapp/README.md` the user-facing version. `IMPORT_EXPORT_FORMATS.md` is the format reference the importers follow. `docs/github-workspace-requirements.md` is the v3 spec for the GitHub workspace.

## Web app

```sh
pnpm install
pnpm dev            # http://localhost:3000, Turbopack
pnpm lint
pnpm typecheck
pnpm test           # Vitest: unit and component tests in tests/unit
pnpm build && pnpm test:e2e   # Playwright against the production build, tests/e2e
pnpm build && pnpm start
```

Node 22.19 or newer. To try GitHub sign-in locally, create a development OAuth app with the callback `http://localhost:3000/api/github/callback` and set `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` and `SESSION_SECRET` in `.env.local` (see [Self-hosting](/docs/self-hosting#github-sign-in)). Without them, everything except sign-in works.

The E2E suite starts its own fake upstream API, a fake GitHub (OAuth, REST and GraphQL) and a production server on port 3100, so no test calls real GitHub. Run `pnpm exec playwright install chromium webkit` once. It covers Chrome, Safari (desktop and iPhone) and phone layouts locally; CI adds Firefox. CI (`.github/workflows/ci.yml`) runs all of the above for every push and pull request.

Conventions: TypeScript strict, the `@/` alias points at the repository root, interactive components carry `"use client"`, icons come from `lucide-react`, styling is Tailwind utility classes with design tokens and a few `@apply` helpers in `styles/`. Code under `utils/server/` runs only on the server; the GitHub token and client secret must never reach a client component.

To add a docs page, drop `content/docs/<slug>.md` with a `title:` and `description:` frontmatter and add the slug to a group in `constants/docs.ts`. Pages left out of the manifest still render, under a trailing "More" group. Headings become the sidebar search index and the on-page table of contents; fenced code gets a copy button; tables scroll horizontally.

## Terminal app

Go 1.25 or newer.

```sh
cd shellapp
make build      # ./hittable
make install    # build, install to ~/.local/bin, overwrite every hittable on PATH
make test       # go vet ./... && go test ./... -timeout 300s  (CI also runs -race)
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
| Browser extension | tag `ext-v*` | `hittable-extension.zip` on a GitHub release, marked latest (the site's download link points at `releases/latest`) |
| Terminal app | tag `sh-v*` | six static binaries (darwin, linux, windows × amd64, arm64), `checksums.txt`, release notes with the install one-liner; never marked latest; `install.sh` and `hittable update` look up the newest `sh-v*` tag |
| Web app | push to `main` | Vercel |

The terminal release runs `go vet` and `go test` first. A release that does not pass its own tests is not a release.

## Commits

Conventional commits are enforced by Husky and commitlint: `type(scope): subject`, lowercase type from `feat fix docs style refactor perf test build ci chore revert`, no trailing period, a blank line before the body. The terminal app's commit drafter follows the same rules, so a message it writes passes the hook. `lint-staged` runs `eslint --fix` on staged TypeScript.

```sh
git commit -m "feat(shellapp): fold files in the commit view"
```

## Reporting a bug

Say which app, and for the terminal app which terminal emulator and OS. For a key that does nothing or a program that misrenders in the integrated terminal, attach the `HITTABLE_KEYLOG` or `HITTABLE_PTYLOG` output. For the web app, the browser and whether the extension was installed.
