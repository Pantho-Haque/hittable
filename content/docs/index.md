---
title: Hittable documentation
description: One API client in two shapes. A browser app that keeps your collections in a folder you own, and a terminal app that opens the same folder with an editor, a Git panel and a shell. These pages describe the current main branch.
---

## What Hittable is

Hittable is an API client without an account, a cloud or a database. Your requests are `.hit` files, your variables are a `hittable/env.json`, your notes are markdown, all inside your own repository. Two tools read and write that folder:

| | Web app | Terminal app |
| :--- | :--- | :--- |
| Runs | in a Chromium browser at [hittable.vercel.app](https://hittable.vercel.app/hittable) or self-hosted | in your terminal, one static Go binary |
| Workspace | a folder you pick (Directory mode), or the browser's local storage (Local mode) | the directory you launch it in |
| Request runner | method, URL, params / headers / body, JSON tree, raw and HTML preview, history | same request format, response written back into the file |
| Beyond requests | CodeMirror editor for every file type, markdown notes, Postman and Insomnia import and export | editor with folding and completion, markdown preview, fuzzy find and grep, Git panel, integrated shell, commit messages from a local model |
| Localhost APIs | through a small browser extension | directly |

Edit a request in the browser, run it from the shell, commit the file, `git pull` on another machine and open it in either. There is no sync step and nothing to migrate.

## Where to start

- **Never used it?** [Getting started](/docs/getting-started) walks you from an empty folder to your first response in both apps in about five minutes.
- **Want the file format?** [Project format](/docs/project-format) is the contract both apps follow: the `.hit` schema, `env.json`, the `<<KEY>>` templates.
- **Living in the browser?** [Web app](/docs/web-app), [Browser extension](/docs/browser-extension), [Import and export](/docs/import-export), [Self-hosting](/docs/self-hosting).
- **Living in the terminal?** [Terminal app](/docs/shellapp), then the [CLI reference](/docs/shellapp-cli), the [keyboard reference](/docs/shellapp-keyboard), the [Git panel](/docs/shellapp-git), [commit messages with a local model](/docs/shellapp-ai), and the [editor, terminal and find](/docs/shellapp-editor-terminal).
- **Hacking on it?** [Development](/docs/development) covers the repository layout, how to run both apps, tests and the commit conventions. [FAQ](/docs/faq) collects the questions that come up.

## About these docs

There is one version of this documentation and it tracks the `main` branch. Neither the web app nor the terminal app keeps versioned documentation sets; a release note on GitHub is the record of what changed when. Every page has an **Edit this page** link to its markdown source in `content/docs/`. Code fences have a copy button, headings are deep-linkable, and the sidebar search matches page titles and section headings.
