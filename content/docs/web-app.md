---
title: Web app
description: The browser client. Two workspace modes, the request runner, the response viewer, history, notes, the CodeMirror editor, and the keyboard.
---

The web app lives at `/hittable`. It is a client-side application: there is no account and no server-side state. Where your data lives depends on the workspace mode in the top bar.

## Two workspace modes

| | Local mode (default) | Directory mode |
| :--- | :--- | :--- |
| Storage | the browser's `localStorage` | real files on disk, through the File System Access API |
| Unit of work | collection → folders → routes; each route is stored as a curl command | a folder tree of `.hit` JSON files, plus `hittable/env.json` and markdown notes |
| Sidebar | a drill-down collection selector | a file explorer over the folder you picked |
| Variables | per-collection env and secrets | one `hittable/env.json` for the workspace |
| Shared with the terminal app | no | yes, byte for byte |
| Browsers | any modern browser | Chromium-based (Chrome, Edge, Opera); the toggle tells you when the API is missing |

Switch with the **Local** / **Directory** button in the top bar. In Directory mode the button disconnects; in Local mode it opens the folder picker. The choice is remembered.

### Directory mode

Pick any folder, including a repository root. The app creates `hittable/`, `hittable/env.json`, `hittable/notes/sample.md` and `hittable/testcollection/test.hit` if they are missing, stores the directory handle in IndexedDB so the next visit reconnects with one permission click, and reads the folder recursively into the explorer. Large folders take a moment to scan, so prefer a project folder over your home directory.

Files are routed by name: `.hit` opens the request editor, `.md` the markdown editor, `env.json` and everything else the code editor. Saves are automatic and serialised, so fast typing never drops a write. Saving `env.json` re-reads it immediately, so `<<KEY>>` resolution picks up new variables at once. `env.json`, `env` and `notes` are reserved names inside `hittable/` and cannot be created, renamed or deleted from the explorer.

The explorer is a proper tree: <kbd>↑</kbd> <kbd>↓</kbd> move, <kbd>→</kbd> <kbd>←</kbd> expand and collapse (or jump to the parent), <kbd>Enter</kbd> opens, <kbd>F2</kbd> renames, <kbd>Delete</kbd> deletes, right-click for the menu. Rename is implemented as copy-then-delete because the browser API has no rename, so renaming a big folder rewrites its files. <kbd>Ctrl/Cmd+B</kbd> hides and shows the explorer.

### `.hit` files: Text and Runner

Every `.hit` file has two views. **Text** is the CodeMirror editor on the raw JSON, with a schema-aware completer: top-level keys, the HTTP verbs after `"method"`, common header names inside `"headers"`, content types after `Content-Type`. **Runner** is the same URL bar, tabs and response panel you get in Local mode, reading and writing the file directly. Invalid JSON blocks the Runner until you fix it in Text mode. Sending a request in the Runner writes the response into the file's `response` field, so sending mutates the file on purpose: the last result travels with the request.

## The request runner

**URL bar.** The method dropdown offers `GET POST PUT PATCH DELETE HEAD OPTIONS`, and the bar's border takes the method's colour. `:param` segments in the path render amber; `<<KEY>>` variables render teal when the environment defines them and red with a wavy underline when it does not, and a note under the bar names unknown variables with an **Open Environment** link (values are never shown in the bar). Editing the URL's query string updates the Params tab, and editing Params rewrites the query string; the two are one value. Buttons: **Save** (<kbd>Ctrl/Cmd+S</kbd>), **Send** (<kbd>Ctrl/Cmd+Enter</kbd>), **Copy as**, and **Save to…** when the open request came from history and belongs to no collection yet.

**Send and Cancel.** While a request is in flight, Send turns into **Cancel**; <kbd>Esc</kbd> in the URL bar cancels too. A cancelled request says so and is not logged in history.

**Copy as** copies the request as **cURL**, **JavaScript fetch**, **Python requests** or **HTTPie**, with variables filled in, or as a **Share link** (below). The command palette has **Copy request as cURL** too.

**Paste a curl command** into the URL field and it is parsed at once into method, URL, headers, params and body; a toast offers **Undo**. `{{var}}` (Postman) and `{{ _.var }}` (Insomnia) references found in it are created as empty variables on the collection.

**Params, Body, Headers** each have a **JSON mode** (a textarea with line numbers and an inline parse-error banner) and a **Table mode** (key / value rows). <kbd>Shift+Alt+F</kbd> beautifies JSON. A JSON error in any JSON tab disables Send and explains why when you try anyway.

**Body types.** The Body tab picks **None**, **JSON**, **Text** or **Form** (URL-encoded key / value rows). Picking one sets a matching `Content-Type` unless you set your own. The type is read from the body and its `Content-Type`, never stored separately, so files and collections stay exactly as they were; only a JSON body has to be valid JSON.

**Auth** (shield icon) writes a header for you: **Bearer Token**, **Basic Auth** (base64 of `user:pass`) or **API Key** with a custom header name, default `X-API-Key`. Only the resulting header is stored with the request, so exports carry the header, not the preset.

**Variables** use `<<KEY>>` in any string field and resolve only when you send; the template stays in storage. Unknown keys are left as-is, and the URL bar names them. In Local mode each collection has **Env Vars** and **Secrets** tabs (variable icon); secrets win on a name clash and are excluded from exports, but they are stored in plain `localStorage` like everything else. In Directory mode `hittable/env.json` is the source, with `hittable/env.local.json` layered on top for values that must not be committed (see [Project format](/docs/project-format)).

**Production guard.** Tick **Production environment** in the Environment dialog (or set `HITTABLE_PRODUCTION` to `true` in `env.json`) and requests show a red **PRODUCTION** badge, and anything but `GET`, `HEAD` or `OPTIONS` asks "Send POST to production?" with the host before it goes. It is an ordinary variable, so it travels with exports and the terminal app honours it too.

**Sending.** Remote URLs go through the app's `/api/proxy` so browser CORS does not get in the way. URLs on `localhost` or `127.0.0.1` cannot be reached by a server, so they are handed to the [browser extension](/docs/browser-extension), which makes the request from the user's own machine and also returns the cookies for that origin. Duration is the round trip measured in the browser, proxy hop included; size is the body's bytes as received.

## Response viewer

The header shows the status with its reason phrase (filled in when an HTTP/2 server sends none) coloured by class — green 2xx, blue 1xx and 3xx, amber 4xx, red 5xx and network errors — plus duration and size. A localhost request without the extension shows what to do and an **Install the extension** button; a cancelled one just says so. Two tabs: **body** and **headers** (with a count).

The body picks a view from the content type and lets you override it until the next response arrives:

- **JSON tree**: collapsible nodes with type colours, `N items` / `N keys` on collapsed nodes, open two levels deep, and nodes expand on their own to reveal search matches.
- **Raw text**: whitespace preserved, matches highlighted.
- **HTML preview**: a sandboxed iframe that never runs the page's scripts, with a `<base href>` injected so relative assets resolve, beside a formatted, line-numbered source pane. Turn on **Inspect**, hover to outline elements, click one to see its tag and attributes and jump to its line in the source.

**Search** (<kbd>Ctrl/Cmd+F</kbd>) counts matches across keys and values, <kbd>Enter</kbd> and <kbd>Shift+Enter</kbd> step through them, <kbd>Esc</kbd> closes. On the headers tab it filters rows. **Copy** copies the body; each header row has its own copy button and there is **Copy all headers**.

**Filter** (funnel icon, JSON bodies) narrows the body with a JSONPath: `$.items[*].id`, `$..id`, `$['odd key']`, `$[0]`, `$[-1]`, `$[1:3]`. An invalid path is marked as such and the full body stays; a valid one that matches nothing says so. A new response clears it. Search and Copy work on the filtered result.

**Compare** diffs the response against the previous one for the same request — the response saved with it, or the one before this send (in Local mode it falls back to history). It lists every added, removed and changed path with before → after values, key order does not count as a change, and changed headers are listed separately. The **Ignore paths** box takes body paths (`$.updatedAt`, `$.items[*].id`) and header names (`date`), so timestamps don't drown the real changes.

## Share links

**Copy as → Share link** copies a link that carries the request itself, compressed in the part after `#`, which browsers never send to a server — nothing is uploaded and there is no account. Variables stay `<<KEY>>`; secrets are never included; credentials in `Authorization`, `Cookie`, API-key headers, the URL's `user:password@` and query parameters such as `api_key` or `access_token` are replaced with variables (the toast lists what was replaced). Opening a link shows a preview — nothing is sent — with **Import** (into a **Shared** collection, or `hittable/shared/` in Directory mode) and **Dismiss**. Links are capped at 32 KB.

## History

Every send is logged, failures included, up to 100 entries, newest first. The clock icon opens **Request History** with method, URL, status, duration, size and relative time per row; click one to load it back into the runner. A request loaded from history belongs to no collection, so the URL bar offers **Save to…**, which picks or creates a collection and folder and saves it there. **Clear all** empties the log. History is a Local-mode feature; the Directory-mode Runner does not record it.

## Notes

In Local mode the notebook icon opens a notes modal: a searchable list of markdown notes with **Edit**, **Preview** and **Split** views, word and character counts, <kbd>Ctrl/Cmd+S</kbd> to save and a save-or-discard prompt on <kbd>Esc</kbd> with unsaved edits. In Directory mode any `.md` file in the workspace opens in the same editor and autosaves to disk. Markdown is GitHub-flavoured with line breaks preserved.

## Code editor

Every non-`.hit` file in Directory mode, and the Text view of `.hit` files, is a CodeMirror 6 editor: line numbers, active line, bracket matching and auto-close, fold gutter (`▾` / `▸`) with **Collapse all** and **Expand all**, search panel, selection-match highlighting, 2-space indentation, <kbd>Tab</kbd> indents. <kbd>Mod+S</kbd> saves, <kbd>Alt+Z</kbd> toggles word wrap, <kbd>Ctrl+Shift+[</kbd> and <kbd>]</kbd> fold and unfold. The status bar shows line and column, the detected language, line count and encoding.

Grammars load on demand, only for the file type you open: TypeScript, TSX, JavaScript, JSX, JSON (including `.hit`), Markdown, HTML, CSS and Sass, XML and SVG, YAML, Python, Go, Rust, SQL, Shell (plus `Makefile` and rc files), TOML, INI, Dockerfile, Ruby, Lua, C and C++, Java, C#, Swift and diff. Languages with their own completion (TypeScript, JavaScript, HTML, CSS, Python, SQL) use it; everything else gets completion from words already in the buffer.

## Import and export

The import icon accepts pasted Postman v2.1 JSON, Insomnia export JSON, or a Hittable native string, and detects which. A collection's `⋮` menu exports it as Postman, Insomnia or the compressed Hittable string. Saved responses are stripped from exports. The full mapping is on [Import and export](/docs/import-export).

## Keyboard shortcuts

<kbd>Ctrl</kbd> and <kbd>Cmd</kbd> are interchangeable. Shortcuts with a modifier work even while typing in a field; <kbd>Shift+T</kbd> does not, so it never fires mid-sentence.

| Keys | Action |
| :--- | :--- |
| `Ctrl/Cmd + K` | command palette: requests, files and actions |
| `?` or `Ctrl/Cmd + /` | keyboard shortcuts |
| `Ctrl/Cmd + Enter` | send the request |
| `Esc` (URL bar) | cancel the request in flight |
| `Ctrl/Cmd + S` | save the route or file |
| `Shift + Alt + F` | beautify JSON in the active tab |
| `Ctrl/Cmd + F` | search the response |
| `Ctrl/Cmd + B` | toggle the sidebar or explorer |
| `Shift + T` | new route in the current folder |
| `Esc` | close the open modal |
| `Enter` | confirm the input in create, rename and save dialogs |
| `Tab` / `Shift+Tab` | move inside a modal; focus returns to where you were on close |

All of these come from one list, which the `?` cheatsheet, the command palette's hints and the info dialog all show.

**Command palette** (<kbd>Ctrl/Cmd+K</kbd>, or **Search** in the top bar) fuzzy-searches every request in every collection, the files of an open folder, and actions — new request, environment, import, export, history, notes, auth, copy as cURL, switch Local / Directory. Recently used items come first.

## Unsaved changes

The request form compares normalised JSON, so reformatting alone is not a change. When something differs you get an amber **Unsaved · Ctrl/Cmd+S** pill, a **Discard changes** button that restores the last saved version, and a browser warning if you try to close the tab.

## Privacy

Collections, variables, notes and history stay in your browser, or in the files you chose. There is no account, no telemetry and no analytics. The one thing to know: a request to a remote URL passes through the `/api/proxy` of whatever instance you are using, with its headers and body, because that is how CORS is avoided. Localhost traffic through the extension never leaves your machine. If that proxy should be yours, [self-host](/docs/self-hosting).
