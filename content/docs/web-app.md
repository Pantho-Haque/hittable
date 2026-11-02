---
title: Web app
description: The browser client. Sign in with GitHub and work on a repository's .hit files, or open a local folder. The request runner, the response viewer, share links, the editor and the keyboard.
---

The web app lives at `/hittable`. It works on `.hit` request files that live in a `hittable/` folder, either in one of your GitHub repositories or in a folder on your disk. There is no database: requests live in files you own.

## Two workspace modes

| | GitHub repository | Local folder |
| :--- | :--- | :--- |
| How you open it | **Sign in with GitHub**, then pick a repository | **Open a local folder** and pick a directory |
| Account | your GitHub account | none |
| Where edits go | a working copy saved on this device (IndexedDB) until you commit | straight to the files on disk (autosave) |
| Getting changes out | review the diff, then commit to the branch, or to a new branch with a pull request | `git commit` yourself, or use the terminal app's Git panel |
| Browsers | any modern browser | Chrome and Edge (needs the File System Access API) |
| Shared with the terminal app | yes, through the repository | yes, the same files byte for byte |

The GitHub workflow is the v3 direction and is being rolled out; parts of it described below may not be in the hosted app yet. Open a local folder works today.

### GitHub repository

1. **Sign in with GitHub**. The app asks for the `repo` and `read:user` scopes (see [Privacy and security](#privacy-and-security)).
2. Pick a repository. The picker lists every repository your account can see, most recently pushed first, with search, owner filters (you and your organisations) and a **Recent** list kept on this device. Badges mark repositories that are **Private**, a **Fork**, **Archived**, or **Read-only** (you have no push permission).
3. The repository's tree opens in the explorer. `.hit` files under `hittable/` open in the Runner, markdown in the markdown editor, everything else in the code editor.
4. Edit and send. Every edit is kept in a working copy on this device, keyed by repository and branch, so a reload does not lose it. Nothing is written to GitHub until you commit.
5. Review your changes. Each changed file is marked **M** (modified), **A** (added) or **D** (deleted), and opens as a diff against the branch.
6. Commit to the current branch, or commit to a new branch and open a pull request.

`hittable/env.local.json` is never committed. Archived repositories, repositories you cannot push to and empty repositories open read-only: you can still send requests, but not edit. If an organisation's repositories are missing, see [the FAQ](/docs/faq#organisation-repositories-are-missing).

### Local folder

**Open a local folder** uses the browser's File System Access API, which Chrome and Edge have and Safari and Firefox do not. Pick any folder, including a repository root. The app creates `hittable/`, `hittable/env.json`, `hittable/notes/sample.md` and `hittable/testcollection/test.hit` if they are missing, adds `env.local.json` to `hittable/.gitignore`, and reads the folder into the explorer. The folder handle is stored in IndexedDB, so the next visit reconnects with one permission click. Large folders take a moment to scan, so pick a project folder rather than your home directory.

Saves are automatic and serialised, so fast typing never drops a write. Saving `env.json` or `env.local.json` re-reads both, so new variables resolve at once. `env.json`, `env` and `notes` are reserved names inside `hittable/` and cannot be created, renamed or deleted from the explorer.

The explorer is a tree: <kbd>↑</kbd> <kbd>↓</kbd> move, <kbd>→</kbd> <kbd>←</kbd> expand and collapse, <kbd>Enter</kbd> opens, <kbd>F2</kbd> renames, <kbd>Delete</kbd> deletes, right-click for the menu. Rename is copy-then-delete because the browser API has no rename, so renaming a big folder rewrites its files. <kbd>Ctrl/Cmd+B</kbd> hides and shows the explorer. **Close folder** in the command palette disconnects.

### `.hit` files: Text and Runner

Every `.hit` file has two views. **Text** is the CodeMirror editor on the raw JSON, with a schema-aware completer: top-level keys, HTTP verbs after `"method"`, common header names inside `"headers"`, content types after `Content-Type`. **Runner** is the URL bar, tabs and response panel. A file that parses opens in the Runner; invalid JSON opens in Text and blocks the Runner until it is fixed. Sending writes the response into the file's `response` field on purpose, so the last result travels with the request. The format is on [Project format](/docs/project-format).

## The request runner

**URL bar.** The method and the URL are one field; the method is coloured by verb and the whole field lights up when focused. The method dropdown offers `GET POST PUT PATCH DELETE HEAD OPTIONS`. `:param` segments render amber; `<<KEY>>` variables render teal when the environment defines them and red with a wavy underline when it does not, and a note under the bar names the unknown ones. Values are never shown in the bar. The query string and the Params tab are one value: editing either updates the other. **Send** is <kbd>Enter</kbd> in the URL or <kbd>Ctrl/Cmd+Enter</kbd> anywhere, and **Save** is <kbd>Ctrl/Cmd+S</kbd>; a dot on Save marks unsaved changes.

**Send and Cancel.** While a request is in flight, Send turns into **Cancel**; <kbd>Esc</kbd> in the URL bar cancels too.

**Paste a curl command** into the URL field and it is parsed into method, URL, headers, params and body.

**Params, Body, Headers** each have a **JSON mode** (line numbers and an inline parse-error banner) and a **Table mode** (key / value rows). The tabs show how many params and headers are set, and a dot when there is a body. <kbd>Shift+Alt+F</kbd> beautifies JSON. A JSON error in any tab disables Send and says why.

**Body types.** The Body tab picks **None**, **JSON**, **Text** or **Form** (URL-encoded key / value rows). Picking one sets a matching `Content-Type` unless you set your own. The type is read from the body and its `Content-Type`, never stored separately, so files stay as they were. Only a JSON body has to be valid JSON.

**Variables** use `<<KEY>>` in any string field and resolve only when you send; the template stays in the file. They come from `hittable/env.json`, with the git-ignored `hittable/env.local.json` layered on top for values that must not be committed. Unknown keys are left as-is. See [Project format](/docs/project-format#envjson-and-templates).

**Production guard.** Set `HITTABLE_PRODUCTION` to `true` in `env.json` or `env.local.json` and requests show a red **PRODUCTION** badge, and anything but `GET`, `HEAD` or `OPTIONS` asks for confirmation, naming the host, before it goes. The terminal app honours the same variable.

**Copy as** copies the request as **cURL**, **JavaScript fetch**, **Python requests** or **HTTPie**, with variables filled in, or as a **Share link** (below). The command palette has **Copy request as cURL** too.

**Sending.** Remote URLs go through the app's `/api/proxy` so browser CORS does not get in the way. A server cannot reach `localhost` or `127.0.0.1` on your machine, so those requests go to the [browser extension](/docs/browser-extension). Duration is the round trip measured in the browser, proxy hop included; size is the body's bytes as received.

## Response viewer

The header shows the status with its reason phrase, coloured by class (green 2xx, blue 1xx and 3xx, amber 4xx, red 5xx and network errors), plus duration and size. A localhost request without the extension explains why and links to the download; a cancelled one just says so. Two tabs: **body** and **headers** (with a count).

The body picks a view from the content type, and you can override it until the next response:

- **JSON tree**: collapsible nodes with type colours, `N items` / `N keys` on collapsed nodes, open two levels deep. Nodes expand on their own to reveal search matches.
- **Raw text**: whitespace preserved, matches highlighted.
- **HTML preview**: the page as a browser would draw it. Its CSS, images and scripts load from the URL it came from (after redirects), and its links open in a new tab.
  - **Scripts on/off** (remembered): page scripts run in an isolated frame with no access to Hittable, your session, its storage or its API. Turn them off to see the server-rendered HTML only.
  - **Open in browser** is there for pages that refuse to load inside a frame.
  - **Inspect**: hover to outline an element, click it to see its tag and attributes and mark its line in the source.
- **Source** (beside the preview): the HTML in a read-only editor that scrolls as one block.
  - Fold tags, or collapse and expand all.
  - Wrap long lines (remembered), and switch between pretty-printed and as-received.
  - Search with <kbd>Ctrl/Cmd+F</kbd> once the source has focus; go to a line with <kbd>Ctrl/Cmd+Alt+G</kbd>.

**Search** (<kbd>Ctrl/Cmd+F</kbd>) counts matches across keys and values; <kbd>Enter</kbd> and <kbd>Shift+Enter</kbd> step through them, <kbd>Esc</kbd> closes. On the headers tab it filters rows. **Copy** copies the body, and each header row has its own copy button.

**Filter** (funnel icon, JSON bodies) narrows the body with a JSONPath: `$.items[*].id`, `$..id`, `$['odd key']`, `$[0]`, `$[-1]`, `$[1:3]`. An invalid path is marked and the full body stays; a valid one that matches nothing says so. A new response clears it. Search and Copy work on the filtered result.

**Compare** diffs the response against the previous one for the same request: the response saved in the file, or the one before this send. It lists every added, removed and changed path with before and after values. Key order does not count as a change, and changed headers are listed separately. **Ignore paths** takes body paths (`$.updatedAt`, `$.items[*].id`) and header names (`date`), so timestamps do not drown the real changes.

## Layout

Every divider moves: the explorer, Source control (GitHub), request and response, preview and source.

- **Drag** a divider to resize. Drag a pane well past its minimum and it collapses; a **Show …** bar brings it back.
- **Double-click** a divider to reset it.
- **Keyboard**: <kbd>Tab</kbd> to a divider, then use the arrows (<kbd>Shift</kbd> for bigger steps), <kbd>Home</kbd> / <kbd>End</kbd> for the extremes, and <kbd>Enter</kbd> to collapse or restore.
- **Response beside or below the request**: the button left of the extension status. The **source beside or below the preview** button is in the preview's header.
- Sizes, collapsed panes and both choices are remembered in this browser. On a phone, panes always stack.

## Share links

**Copy as → Share link** copies a link that carries the request itself, compressed into the part after `#`. Browsers never send that part to a server, so nothing is uploaded. Variables stay `<<KEY>>`. Credentials in `Authorization`, `Cookie` and API-key headers, in the URL's `user:password@` and in query parameters such as `api_key` or `access_token` are replaced with variables, and the toast lists what was replaced. Opening a link shows a preview and sends nothing, with **Import** (into `hittable/shared/`) and **Dismiss**. Links are capped at 32 KB.

## Code editor

Markdown files, every other non-`.hit` file, and the Text view of `.hit` files use CodeMirror 6: line numbers, bracket matching and auto-close, a fold gutter, a search panel, selection-match highlighting and 2-space indentation. <kbd>Ctrl/Cmd+S</kbd> saves, <kbd>Alt+Z</kbd> toggles line wrap.

Grammars load on demand for the file type you open: TypeScript, JavaScript, JSON (including `.hit`), Markdown, HTML, CSS, XML, YAML, Python, Go, Rust, SQL, Shell and more. Languages with their own completion use it; everything else gets completion from words already in the buffer.

## Keyboard shortcuts

<kbd>Ctrl</kbd> and <kbd>Cmd</kbd> are interchangeable. Shortcuts with a modifier work even while typing in a field. Press <kbd>?</kbd> in the app for the same list.

| Keys | Action |
| :--- | :--- |
| `Ctrl/Cmd + K` | open the command palette |
| `?` | show keyboard shortcuts |
| `Ctrl/Cmd + /` | show keyboard shortcuts (anywhere) |
| `Esc` | close modals |
| `Enter` (URL bar), `Ctrl/Cmd + Enter` | send the request |
| `Ctrl/Cmd + S` | save the request |
| `Esc` (URL bar) | cancel the running request |
| `Ctrl/Cmd + F` | search in the response |
| `Ctrl/Cmd + B` | toggle the explorer |
| `Shift + Alt + F` | beautify JSON |
| `Alt + Z` | toggle line wrap |

The cheatsheet, the command palette's hints and the landing page all render from one list in `constants/shortcuts.ts`.

**Command palette** (<kbd>Ctrl/Cmd+K</kbd>) fuzzy-searches the files and requests of the open workspace, plus actions such as toggle explorer, copy request as cURL, and open or close a local folder.

## Privacy and security

There is no database, no telemetry and no analytics.

- **GitHub sign-in.** The access token lives only on the server, in an AES-256-GCM encrypted, `HttpOnly` session cookie. Page JavaScript never sees it. Every GitHub call goes through the app's own `/api/github/*` routes.
- **Scopes.** `repo` is used to list your repositories, read their `.hit` files and commit your changes back. `read:user` is used for your name and avatar. Nothing admin-, organisation- or workflow-specific is requested.
- **Revoking.** Signing out revokes the grant on GitHub and clears the cookie. You can also revoke it any time in GitHub under [Settings → Applications](https://github.com/settings/applications).
- **Organisations** may need to approve the OAuth app before their repositories show up.
- **The proxy.** A request to a remote URL passes through the `/api/proxy` of whatever instance you are using, with its headers and body, because that is how CORS is avoided. Nothing is stored there. Localhost traffic through the extension never leaves your machine. If that proxy should be yours, [self-host](/docs/self-hosting).
