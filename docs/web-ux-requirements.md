# Hittable web app — UX upgrade requirements

Status: **Round 1 implemented** (tests: see each FR). Round 2 ("beyond the alternatives") is appended once
Round 1 ships and has been tested.

## How this was derived

1. **Research** into how Postman, Insomnia, Hoppscotch, Bruno, Yaak, HTTPie, Thunder
   Client, Paw/RapidAPI and Posting (TUI) handle layout, request building, response
   viewing, productivity, visual design and accessibility. That work also covered why users
   leave the incumbents: login walls, cloud lock-in, paywalls and bloat. The sources are in
   the appendix.
2. **An audit of the current app**: every route, component, shortcut and test, plus
   screenshots at 1440×900 and 390×844.

### What the audit found

| Area | Today |
| :--- | :--- |
| Readability | Labels are 7–10px, spaced-out caps at `white/15–30` contrast. axe runs with `color-contrast` **disabled**. |
| Design system | `--ink-*` tokens exist, but the workspace hard-codes about 50 hex values. Method colours are inline styles. The editor theme is a separate hand-synced hex set. |
| Discoverability | No command palette. Shortcuts are listed in three places that disagree (landing page, Info modal, code). The tool rail is unlabelled icons. |
| Request building | Bodies are JSON-or-empty only. `OPTIONS` cannot be selected. `<<KEY>>` tokens are not highlighted, so an unknown variable is invisible until the request fails. A pasted curl flashes raw for 1 s before it is parsed. |
| Sending | No cancel and no client timeout. Response size is re-serialised JSON, not the bytes received. |
| Response | Status is green / orange / red only. No 3xx distinction and no status text. |
| Extension | "No extension" modal opens on every request form mount, even for public URLs, with a red (destructive-looking) Download button. |
| Feedback | Toast class bug (`…zinc-200border-l-…` concatenated). Toasts follow the OS light theme inside a dark app. No `aria-live`. |
| Safety | Deleting in the folder explorer removes files recursively from disk with **no confirmation**. |
| Keyboard / a11y | Sidebar rows are clickable `div`s (not focusable). Request and response tabs lack tab semantics. Icon-only buttons use `title` only. `Ctrl+J` hijacks the browser's Downloads shortcut. |
| Copy / code | Copy-as-curl only. No other languages. |

## Priorities

- **R1 (must, this round):** conventions every comparable tool has, plus the defects above.
- **R2 (next round):** the differentiators. These will be re-researched after R1.

Each requirement has acceptance criteria (AC) that the tests assert.

---

## Functional requirements — Round 1

### FR-1 Command palette (`Ctrl/Cmd+K`)
- Opens from anywhere in `/hittable`, including inside inputs and editors. It also opens
  from a visible "Search / ⌘K" button in the top bar.
- Fuzzy-searches:
  - **requests** in local collections, including their folder path and method;
  - **files** when a folder is open;
  - **actions**: new request, toggle sidebar, open Environment, Import, Export, History,
    Notes, Auth, keyboard shortcuts, switch Local ↔ Folder.
- Recently used items rank first. Every action row shows its shortcut, if it has one.
- Keyboard control: `↑↓` move, `⏎` runs, `Esc` closes. Focus is trapped while open and
  returns to the trigger on close.
- **AC:** `Ctrl+K` → type part of a request name → `⏎` opens that request. Typing "env"
  → `⏎` opens the Environment dialog. Focus returns after `Esc`. axe reports no
  violations while the palette is open.

### FR-2 One shortcut registry and a cheatsheet (`?`)
- All web shortcuts come from a single list that drives the palette hints, the cheatsheet,
  the Info modal and the landing page.
- `?` (outside text inputs) and `Ctrl/Cmd+/` open a cheatsheet dialog grouped by area.
- No binding may override a core browser shortcut. **Beautify moves from `Ctrl+J`
  (browser Downloads) to `Shift+Alt+F`** (the VS Code format chord).
- **AC:** every binding the app handles is listed. Pressing `Ctrl+J` no longer beautifies.
  `?` opens the cheatsheet and `Esc` closes it.

### FR-3 Variable awareness in the URL bar
- `<<KEY>>` tokens are highlighted in the URL bar:
  - **teal** when the key exists in env or secrets;
  - **red with a wavy underline** when it does not.
- A secret value is never shown in a tooltip.
- An unresolved variable shows an inline note under the URL bar: "Unknown variable
  `<<KEY>>` — add it in Environment", with a button that opens the Environment dialog.
- **AC:** with `BASE_URL` defined and `MISSING` not, the URL `<<BASE_URL>>/x/<<MISSING>>`
  renders one resolved and one unresolved token, and the note names `MISSING`.

### FR-4 Request bodies beyond JSON
- A body-type selector offers **None, JSON, Text, Form (x-www-form-urlencoded)**.
  - JSON keeps today's editor, table mode and beautify.
  - Text sends the string as-is.
  - Form edits key/value rows and sends `a=1&b=2` (URL-encoded).
- Choosing a type sets a `Content-Type` header only when none is present. The user's
  header always wins.
- On load, the type is inferred from the stored body and `Content-Type`, so existing
  requests open unchanged and JSON stays the default.
- Invalid JSON blocks sending only when the type is JSON.
- **AC:** a Form body with `a=1`, `b=x y` reaches the test API as `a=1&b=x+y` with
  `application/x-www-form-urlencoded`. A Text body arrives byte-for-byte. Round-trip
  through save, reload and curl export keeps the type.

### FR-5 Complete method list
- `OPTIONS` can be selected. Every method has a colour that comes from one token set.
- **AC:** an OPTIONS request reaches the server with method `OPTIONS`.

### FR-6 Cancel and timeout
- While a request is in flight, **Send becomes Cancel** (`Esc` also cancels when focus is
  in the request area).
- Cancelling aborts the browser fetch and shows "Request cancelled", with no error toast.
- A client-side timeout is in place as well, as a backstop to the server's.
- **AC:** against a slow endpoint, Cancel returns the UI to idle within 200 ms, and the
  next Send works normally.

### FR-7 Honest, semantic response metadata
- The status badge is coloured by class (2xx success, 3xx info, 4xx warning, 5xx and
  network errors danger). It shows the reason phrase as well as the code, so colour is
  never the only signal.
- Size is the byte length of the body as received. Time stays labelled as round-trip.
- **AC:** 201 → "201 Created" in the success colour. 302 → "302 Found" in the info colour.
  404 → "404 Not Found" in the warning colour. 503 → danger colour. A 1,000-byte body
  reports "1.0 KB" (or "1000 B"), not the re-serialised length.

### FR-8 Copy as code
- A "Copy as" menu next to Send generates **cURL, JavaScript fetch, Python requests and
  HTTPie**. Variables are resolved, and secrets are included only in the copied text.
  `Ctrl/Cmd+Shift+C` still copies cURL.
- **AC:** each generator round-trips the method, URL with query, headers and each body
  type in unit tests. The generated cURL pastes back into the URL bar as the same
  request.

### FR-9 Instant, undoable curl paste
- Pasting a curl command into the URL bar parses it immediately, with no 1 s raw flash.
- A toast "Imported from curl — Undo" restores the previous request.
- **AC:** after paste, the form shows parsed values at once. Undo restores the prior
  method, URL, headers and body.

### FR-10 Extension guidance that does not nag
- Remove the auto-opening "no extension" modal.
- Show the extension status as a small indicator in the workspace (connected or not
  installed). Clicking it explains the extension and offers the download.
- When a localhost request needs the extension and it is missing, show an **inline,
  actionable error in the response area** with an Install button. The Install button
  uses the primary style, not a danger style.
- **AC:** opening the app and sending to a public URL shows no dialog. Sending to
  `localhost` without the extension shows the inline error with an Install button.

### FR-11 Safe destructive actions in folder mode
- Deleting a file or folder from disk asks for confirmation, naming the item and saying
  that the delete is recursive for folders.
- **AC:** Delete → Cancel leaves the file. Delete → Confirm removes it.

### FR-12 Reliable feedback
- Fix the toast class concatenation bug.
- Toasts use the app's dark tokens regardless of OS theme. They are announced through
  `role="status"` / `aria-live="polite"`, or `role="alert"` for errors. Each toast has an
  optional action (used by FR-9's Undo).
- **AC:** an error toast has the danger border and `role="alert"`. A success toast has
  `role="status"`.

---

## Non-functional requirements — Round 1

### NFR-1 Readability
- No visible UI text below **11px**. Body and control text is 12–13px, and long
  all-caps tracking is used only on small section headers.
- Text contrast meets **WCAG 2.2 AA**: 4.5:1 for normal text and 3:1 for large text and UI
  boundaries.
- **AC:** the axe suite runs **with `color-contrast` enabled** on `/`, `/docs`,
  `/docs/getting-started`, `/hittable` with a request open, and `/hittable` with the
  palette open, and reports zero serious or critical violations.

### NFR-2 Design tokens
- The workspace uses CSS custom properties (surface, raised, border, text, muted,
  accent, success, info, warning, danger, method colours), not hard-coded hex values.
- The CodeMirror theme reads the same values.
- **AC:** a lint-style unit test fails if a hex colour literal appears in
  `components/{hittable,RequestForm,workspace,modals}` outside the token file.

### NFR-3 Keyboard and screen-reader access
- Every interactive element is reachable and operable by keyboard, with a visible
  focus ring.
- Sidebar rows are buttons or links. Request and response tabs use the ARIA tab
  pattern (`tablist`, `tab`, `aria-selected`, arrow keys).
- Icon-only buttons have an `aria-label` and a visible tooltip on hover or focus.
- Dialogs trap focus and return it on close, including the Environment dialog.
- **AC:** a Playwright test completes "open request → edit header → send → read status"
  using only the keyboard. Tab elements expose `role="tab"` and `aria-selected`.

### NFR-4 Responsiveness
- 320px to 2560px wide with no horizontal page scroll.
- At 44px+ touch targets on mobile, the palette and dialogs fit the viewport.
- **AC:** the existing 320px and mobile suites pass, plus the palette at 390×844.

### NFR-5 Performance and weight
- Remove dependencies nothing uses (`@tanstack/react-query`).
- The palette and code generators are loaded on demand.
- Typing in the URL bar stays under one frame of work.
- **AC:** `pnpm build` succeeds, and the `/hittable` first-load JS does not grow by more
  than 15 KB gzip versus the baseline.

### NFR-6 Quality gates
- `pnpm lint`, `pnpm typecheck`, `pnpm test` and `pnpm build` are clean.
- The full Playwright suite passes on desktop Chrome, WebKit, mobile Chrome and mobile
  Safari locally, and on Firefox in CI.
- Every FR has unit and/or e2e coverage.
- No existing test is deleted to make a change pass. A test is changed only where the
  requirement deliberately changes the behaviour, and the change says why.

### NFR-7 Compatibility
- `.hit`, `env.json`, local collections and import/export formats stay readable by the
  previous release and by the terminal app. Body type is inferred, not stored in a new
  field.

---

## Appendix — research sources (abridged)

- Bruno keybindings: https://docs.usebruno.com/get-started/configure/keybindings
- Bruno layout toggle: https://github.com/usebruno/bruno/pull/3860
- Postman variables (highlight, tooltip, autocomplete):
  https://learning.postman.com/docs/use/send-requests/variables/view-variables
- Postman responses (status / time / size hovers, filter, preview, large responses):
  https://learning.postman.com/docs/use/send-requests/response-data/responses
- Postman curl paste: https://learning.postman.com/docs/getting-started/importing-and-exporting/importing-curl-commands
- Postman code generation: https://learning.postman.com/docs/use/send-requests/create-requests/generate-code-snippets
- Hoppscotch shortcuts and Spotlight: https://docs.hoppscotch.io/documentation/features/shortcuts ,
  https://docs.hoppscotch.io/documentation/features/spotlight
- Hoppscotch bulk edit, themes and PWA: https://github.com/hoppscotch/hoppscotch
- Hoppscotch interceptor (browser extension for CORS): https://docs.hoppscotch.io/documentation/features/interceptor
- Insomnia responses and filter: https://docs.insomnia.rest/insomnia/responses
- Yaak (local-first, no account, method colours, environment colours):
  https://yaak.app/ , https://yaak.app/changelog/2025.4.0
- Posting (TUI: jump mode, palette, curl paste): https://github.com/darrenburns/posting
- Linear density and keyboard-first UI: https://linear.app/changelog/2024-03-20-new-linear-ui
- VS Code accessibility guidelines: https://github.com/microsoft/vscode/wiki/Accessibility-Guidelines
- Why users leave (login walls, paywalls):
  https://learning.postman.com/docs/getting-started/basics/using-scratch-pad ,
  https://github.com/Kong/insomnia/discussions/6626 ,
  https://www.usebruno.com/compare/bruno-vs-thunderclient

---

# Round 2 — beyond the alternatives

## What the second research pass found

Round 2 asked what would make Hittable feel professional, and what it could offer that
users can't get elsewhere, or only with an account or a paywall. Sources are in the
Round 2 appendix.

**Facts that changed the plan**
- Postman's 2026 plans make manual collection runs unlimited, so a free runner no
  longer stands out against Postman.
- Postman's Free plan is now **single-user**, so sharing a request with a teammate needs
  a paid seat or an account. Hoppscotch's shared requests are also stored on its
  servers.

**Rare or paywalled elsewhere**
- **Response diff** is paid-only in Kreya, requested in Insomnia (#8115), and absent in
  Bruno and Yaak.
- **A production guard** (confirm before changing production) exists nowhere. Postman
  and Yaak only colour the environment.
- **Git-safe secrets** sit behind Insomnia's Team plan; Bruno makes you set up a `.env`
  file by hand.

## Scope

| Picked | Why |
| :--- | :--- |
| JSONPath filter | Every comparable tool has one, and the same engine serves diffing |
| Response compare | Rare everywhere, paywalled where it exists |
| Share links with no server | Fits "no accounts"; unique |
| Production guard | Small to build, and no competitor has it |
| Git-safe secrets | Small, and it works in both the web and terminal apps |

**Deferred, with reasons**
- **Request tabs (L):** they need the single-request state in `dataContext` rewritten.
- **Assertions and a runner (M+M):** they add a field to `.hit`. Both apps must learn it
  in the same release, because the terminal app drops unknown fields when it saves.
- **Light theme:** it touches every `text-white/*` class; it needs a token pass of its
  own first.
- **OpenAPI import (M).**
- **GraphQL and WebSocket/SSE:** blocked by browser header limits.
- **In-app AI:** low value and a trust risk for this audience.

## Functional requirements — Round 2

### FR-13 JSONPath filter
- A **Filter** button in the response panel (JSON bodies only) opens an input that
  takes RFC 9535-style paths: `$`, `.key`, `['key']`, `[n]`, `[*]`, `..key`, `[start:end]`.
- The tree then shows the matches.
- Feedback:
  - an invalid path is marked **invalid**;
  - a valid path that matches nothing says **no results**;
  - while a filter is active, the button shows a dot.
- The filter clears when a new response arrives.
- No `eval`.
- **AC:**
  - `$.users[*].name` on `{users:[{name:"a"},{name:"b"}]}` shows `["a","b"]`.
  - `$..id` finds nested ids.
  - `$[` is invalid.
  - `$.nope` gives no results.

### FR-14 Compare with the previous response
- **Compare** diffs the current response against the previous response for the same
  request:
  - **local mode:** the latest earlier history entry with the same method and URL
    template;
  - **folder mode:** the response saved in the `.hit` before this send.
- The diff covers status, the JSON body path by path (added, removed or changed, with
  keys sorted, so key order is not a change) and headers.
- An "ignore" box takes comma-separated paths (e.g. `$.updatedAt`).
- When there is no earlier response, Compare is disabled and says why.
- **AC:** changing one field, adding one and removing one between two runs lists
  exactly those three paths. A reordered object diffs empty.

### FR-15 Share link with no server
- **Share link** in the Copy-as menu copies
  `…/hittable#share=<deflate+base64url JSON>`.
- The link carries the method, the URL, headers, params and body **exactly as written**.
- `<<KEY>>` stays a variable, and secrets are never included. Headers that commonly
  carry credentials (`Authorization`, `Cookie`, `X-Api-Key`, …) are replaced by
  `<<KEY>>`-style placeholders.
- Opening the link shows a **preview** dialog: method, URL, headers and body, plus a
  note that nothing has been sent. **Import** adds it as a new request in a "Shared"
  collection, and **Dismiss** removes the fragment.
- A shared request is **never sent automatically**.
- Links that would exceed 32 KB are refused, with a message.
- **AC:**
  - A round-trip through a link preserves the method, URL, headers and body.
  - An `Authorization: Bearer abc` header arrives as `Bearer <<TOKEN>>`.
  - No network request is made on open.
  - A malformed fragment shows an error, not a crash.

### FR-16 Production guard
- An environment is marked production by the variable **`HITTABLE_PRODUCTION=true`**.
  It is a normal variable, so env.json, the local collections, import/export and the
  terminal app all keep working.
- The Environment dialog has a toggle for it.
- While it is set:
  - a red **PRODUCTION** badge sits next to the request;
  - any method other than GET, HEAD or OPTIONS asks for confirmation before sending
    ("Send POST to production?"), naming the host.
- **AC:**
  - A POST waits for confirmation; Cancel sends nothing; Confirm sends once.
  - A GET is not asked about.
  - The toggle writes the variable.

### FR-17 Git-safe secrets (folder mode, both apps)
- `hittable/env.local.json` (a flat string map) overrides `env.json` values when
  resolving.
- Creating or opening a folder makes sure `hittable/.gitignore` lists
  `env.local.json`. It appends the line, never overwrites the file.
- The terminal app resolves `<<KEY>>` against the same merge.
- **AC:**
  - With `TOKEN` in both files, the local value is sent.
  - `.gitignore` gains the line once, even across reopens.
  - The terminal app sends the local value.

## Non-functional — Round 2
- Every Round 1 NFR still holds, including axe with contrast and the token test.
- The JSONPath engine, the diff engine and share encoding are pure functions with unit
  tests.
- Share decoding treats the fragment as untrusted input: size cap, schema check, no
  HTML rendering of its values.

## Appendix — Round 2 sources (abridged)
- Postman pricing and usage: https://www.postman.com/pricing/ ,
  https://learning.postman.com/docs/billing/resource-usage
- Postman response filter: https://learning.postman.com/docs/use/send-requests/response-data/responses
- Postman environment colours: https://learning.postman.com/docs/use/send-requests/variables/environment-colors
- Insomnia response diff request: https://github.com/Kong/insomnia/discussions/8115
- Insomnia Git Sync and secrets: https://docs.insomnia.rest/insomnia/git-sync
- Kreya snapshot testing (Pro): https://kreya.app/docs/scripting-and-tests/tests/snapshot-testing/
- Bruno assertions: https://docs.usebruno.com/testing/tests/assertions.md
- Bruno dotenv: https://docs.usebruno.com/v3/secrets-management/dotenv-file
- Yaak chaining: https://yaak.app/docs/using-yaak/request-chaining-scripting
- Yaak changelog: https://yaak.app/changelog
- Hoppscotch shared requests: https://docs.hoppscotch.io/documentation/features/widgets
- URL length limits: https://simonwillison.net/2025/Dec/23/url-limits-investigation/
- Chrome Local Network Access: https://developer.chrome.com/blog/local-network-access
