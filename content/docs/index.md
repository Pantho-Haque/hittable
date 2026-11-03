---
title: Hittable documentation
description: An open-source, local-first API client. Requests are .hit files in your repository. Work on them in the browser, from a GitHub repository or a local folder, or in the terminal. These pages describe the current main branch.
---

![Hittable: a terminal API client, .hit files shared with the web app](/docs/intro.png)

## What Hittable is

Hittable is an open-source, local-first API client. Your requests are `.hit` JSON files, your variables are `hittable/env.json`, your notes are markdown, all inside a `hittable/` folder in your own repository. There is no Hittable account and no database. Two tools read and write that folder:

| | Web app | Terminal app |
| :--- | :--- | :--- |
| Runs | in the browser at [hittable.vercel.app](https://hittable.vercel.app/hittable), or self-hosted | in your terminal, one static Go binary |
| Workspace | a GitHub repository you sign in to, or a local folder (Chrome and Edge) | the directory you launch it in |
| Saving | a working copy on this device, committed back to GitHub; or autosave to the local folder | autosave to disk |
| Request runner | method, URL, params / headers / body, JSON tree, raw and HTML preview, JSONPath filter, response compare | same request format, response written back into the file |
| Beyond requests | CodeMirror editor, copy as cURL / fetch / Python / HTTPie, share links | editor, markdown preview, fuzzy find and grep, Git panel, integrated shell, Postman and Insomnia import and export |
| Localhost APIs | through a small browser extension | directly |

Edit a request in the browser, run it from the shell, commit the file, `git pull` on another machine and open it in either. There is no sync step and nothing to migrate.

## Where to start

- **Never used it?** [Getting started](/docs/getting-started) goes from a repository to your first response in both apps.
- **Want the file format?** [Project format](/docs/project-format) is the contract both apps follow: the `.hit` schema, `env.json`, the `<<KEY>>` templates.
- **Living in the browser?** [Web app](/docs/web-app), [Browser extension](/docs/browser-extension), [Self-hosting](/docs/self-hosting).
- **Moving from Postman or Insomnia?** [Import and export](/docs/import-export).
- **Living in the terminal?** [Terminal app](/docs/shellapp), then the [CLI reference](/docs/shellapp-cli), the [keyboard reference](/docs/shellapp-keyboard), the [Git panel](/docs/shellapp-git), [commit messages with a local model](/docs/shellapp-ai), the [editor, terminal and find](/docs/shellapp-editor-terminal), and [VS Code extensions](/docs/shellapp-extensions).
- **Hacking on it?** [Development](/docs/development) covers the repository layout, running both apps, tests and commit conventions. The [FAQ](/docs/faq) collects the questions that come up.

## About these docs

There is one version of this documentation and it tracks the `main` branch. Release notes on GitHub record what changed when. Every page has an **Edit this page** link to its markdown source in `content/docs/`. Code blocks have a copy button, headings are deep-linkable, and the sidebar search matches page titles and section headings.
