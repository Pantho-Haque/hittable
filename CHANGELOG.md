# Changelog

All notable changes to Hittable will be documented in this file.

## [Unreleased]

### Security
- **Unused JSON highlighter removed** — `SyntaxHighlighter` wrote response bodies into the page unescaped; it was never rendered anywhere, so it was deleted rather than kept around as a trap.
- **Notes XSS** — markdown notes, which can arrive from a shared folder or an import, are sanitized with DOMPurify before rendering.
- **Proxy hardening** — `/api/proxy` refuses loopback, private, link-local (cloud metadata) and other reserved addresses, checked at connect time so DNS rebinding and redirects are covered; adds a per-client rate limit, a 30 s upstream timeout, request and response size caps, and rejects `CONNECT`/`TRACE` and malformed headers. Errors carry real HTTP statuses (400/403/413/429/502/504). Tunable via `PROXY_*` env vars (see `.env.example`).
- **Asset proxy removed** — `GET /api/proxy/asset` was unused and served any URL's content, HTML included, from the app's origin with `Access-Control-Allow-Origin: *`.
- **Extension messaging** — requests carry an id so concurrent replies cannot swap; messages are posted to the page's origin instead of `*`; the service worker refuses non-localhost URLs. Extension 1.0.1.
- **Security headers** — `X-Content-Type-Options`, `X-Frame-Options`, CSP `frame-ancestors`, `Referrer-Policy`, `Permissions-Policy`, HSTS; `X-Powered-By` dropped.

### Fixed (opening files — web and terminal now behave the same)
- **Images and binaries were opened as text in the web app** — a PNG showed as garbage in the code editor, and the next keystroke autosaved the garbage over it. Images now show as a picture with type, size and dimensions; other binaries and text over 1 MiB show a notice; none of them is ever written back. The terminal app shows the same labels ("PNG image · 3 × 2 · 73 B").
- **SVG** opens as code in both apps; the web app adds a picture Preview (drawn through `<img>`, so an SVG's scripts never run).
- **.hit files open in the Runner** in the web app too, as in the terminal; invalid files still open in Text.
- **Half-typed Params/Headers JSON in the web Runner was saved as `{}`**, wiping them on disk and in Text view. The last valid value is kept, and the tab is no longer reformatted under the cursor after each save.
- **Opening a .hit rewrote it** (reformatted JSON, noise in git) in both apps; a file is now only written when the request changes.
- **Web saves coalesce** — a burst of typing writes the latest text once instead of the whole file per keystroke (a 5,000-line file was rewritten ~200 KB per key).

### Fixed (edge-case users)
- **Blocked storage** — with cookies/storage disabled, reading localStorage threw on startup and the whole app failed to render. It now runs without persistence.
- **Safari: HTML preview Inspect** never worked — WebKit runs no event listeners inside a script-less sandboxed iframe. Inspect now works from a layer over the preview using `elementFromPoint`; the highlight also tracks the preview's own scroll and survives a new response.
- **Safari: dialogs leaked keyboard focus** — Safari's default Tab skips buttons and jumped out of the dialog. Dialogs now move focus themselves (editors that indent with Tab are left alone).
- **Reduced motion** — the OS "reduce motion" setting now turns off transitions and toast animations.
- **Docs for keyboard users** — wide code blocks and tables are focusable so they can be scrolled without a mouse.

### Removed
- Dead code and files nothing loads: `AppHeader`, `EnvFileEditor`, unused `DirectoryService` helpers and types, an icon font and Roboto/Titillium fonts from an old template, an unused Bootstrap-style CSS kit, unused images, a stale `browserExtension.crx`, a 21.8 MB macOS binary committed in `hittable/`, and the dependencies `axios`, `class-variance-authority`, `js-beautify`, `tw-animate-css` and `@bany/curl-to-json`.
- Shell app: the `internal/envfile`, `internal/explorer` and `internal/httpclient` packages, which nothing imported.
- Out-of-date planning notes: `PROJECT_SUMMARY.md`, `NEW_FEATURE_IDEAS.md`, `UX_PLAN.md`, `shellapp/docs/`.
- The home page photo is now served from the app (`public/assets/images/avatar.png`, resized to 162 KB) instead of hotlinked; the résumé fetch times out after 3s and is cached, so a slow portfolio API no longer hangs the home page.

### Added
- Vitest unit/component suite and Playwright end-to-end suite (`pnpm test`, `pnpm test:e2e`); CI workflow running lint, typecheck, tests, build and E2E for the web app and vet/race tests for the shell app on every push and PR.
- Production `Dockerfile` (standalone server, non-root, health check).

### Fixed
- **HTML preview asset loading** — Assets (images, CSS) now load correctly in HTML preview. Added `<base href>` injection using the original request URL's origin and directory path, so relative URLs resolve against the source server. Same-origin assets are rewritten to route through `/api/proxy/asset` endpoint to bypass CORS restrictions. Cross-origin assets still subject to browser CORS rules.
- **HTML preview inspect mode** — Inspect Element mode now works correctly. Removed the injected-script approach (which couldn't run without `allow-scripts`) and replaced with direct DOM access via `iframe.contentDocument`. Hover highlights elements with a cyan overlay; click captures tag name, attributes, and outer HTML.
- **HTML source view readability** — Minified HTML source now displays properly formatted with line numbers. Added `js-beautify` for HTML/CSS/JS reformatting, ensuring minified inline `<script>` and `<style>` blocks are expanded into readable multi-line format. Line numbers and fold indicators now correspond to meaningful, readable lines.
- **Asset proxy endpoint** — New GET `/api/proxy/asset` route for fetching HTML sub-resources (images, CSS) through the server to bypass CORS. Returns assets with proper Content-Type headers and cache headers.
- **Auto-set Content-Type header** — When switching body type (JSON, form-urlencoded, text, multipart), the corresponding Content-Type header is automatically set/updated. Manual overrides are preserved — only auto-set values are replaced. Multipart Content-Type is left to fetch/proxy to set with proper boundary.
- **File row compact chip UI** — File rows now display selected files as compact chips (paperclip icon + filename, truncated at 160px max) with an × button to remove individual files. An "+ Add file" button appears below chips for adding more files. Cleaner visual hierarchy than the previous full-width button.

### Fixed
- **Multipart body not persisted correctly after save** — Root cause: `bodyType` and `multipartFileKeys` were stored in `formInput` but NOT persisted in the collection's stored data (only the curl string was saved, which doesn't encode these fields). After reload, the route was loaded without `bodyType`/`multipartFileKeys`, so the reconstruction logic fell through and didn't rebuild multipart rows. Fix: added `bodyType` and `multipartFileKeys` fields to `THittableCurl` type, persisted them via `updateCurl`, and merged them back when loading routes in Selector.
- **Table/JSON view inconsistency after save** — Same root cause as above: Table view derived from `multipartRows` (empty after reload due to missing persistence), while JSON view showed `formInput.body` (which was correct). Now both views show consistent data because the multipart rows are correctly reconstructed from persisted data.
- **Text field values lost in multipart after save/reload** — Text fields in multipart bodies were serialized to `formInput.body` correctly but the reconstruction logic couldn't identify them without `multipartFileKeys` being persisted. Now fixed with the same persistence fix.

### Changed
- **Multi-file per field** — File rows now support multiple files per field (via `files: File[]` instead of `file?: File`). Each file gets its own chip with individual remove. Standard multipart behavior: multiple files under the same field name are sent as separate parts.
- **File input re-pick after remove** — Fixed classic browser bug where clearing file state didn't reset the `<input type="file">` DOM value. Now resets input value on every removal, so re-picking always works.
- **Icon-only Auth and History buttons** — Auth and History buttons now use icon-only format with tooltips, matching the existing `modal-button-mini` convention used by Import, Notes, and Info buttons
- **Markdown notes editor** — Notes now support Markdown with 3-way view toggle (Edit/Split/Preview); Split mode shows side-by-side raw editor + live preview (stacked on mobile); preview renders headings, lists, code blocks, blockquotes, tables, links; uses `marked` library
- **Notes list UX upgrade** — NotePills now show content preview snippet, FileText icon per note, cyan accent on selected note, improved hover/active states, search icon in search field, empty state with icon
- **Table mode for Body/Headers/Params** — TabEditor now has JSON/Table mode toggle (Braces/Table2 icons); Table mode provides key-value row-based input with add/remove rows; both modes stay in sync with the underlying JSON state
- **Notes modal redesign** — Notebook modal now uses 95vw/85vh sizing (max 1100px), custom modal markup with corner brackets, dark theme, spacious layout with word count footer; replaces generic ModalShell for this flagship feature
- **Info modal expanded** — Added sections for How the Proxy Works, Environment Variables syntax (`<<KEY>>`), and Data & Privacy (everything stays local); Escape keybinding documented
- **Notes unsaved-changes protection** — Closing the Notes modal with unsaved edits now shows a confirmation dialog with Save & Close / Discard / Cancel options; Discard reverts to last-saved state
- **Body content type selector** — TabEditor Body tab now has a content-type dropdown supporting raw/JSON, x-www-form-urlencoded, raw/Text, and multipart/form-data; each type sets the correct Content-Type header when sending; form types use table mode with key-value pairs; text type uses raw textarea without JSON validation
- **Breadcrumb-based route switching** — Clicking collection or route names in the RequestForm breadcrumb now opens a dropdown menu listing all collections/routes; selecting one navigates to it via URL params; works without opening the sidebar

### Fixed
- **Left sidebar collapse missing tools** — Collapsed left sidebar now shows Auth and History buttons (matching the right sidebar's tool set); previously these were missing when the sidebar was collapsed via Ctrl/Cmd+B
- **Mobile responsive layout** — Sidebar auto-collapses on screens < 768px; expanded sidebar shows as overlay with backdrop on mobile; right sidebar hidden on mobile (tools available in collapsed left sidebar); modal sizes use `max-width` to fit small viewports; touch targets increased to 44px minimum on mobile for buttons, tabs, and method selector
- **History not realtime** — History modal now updates live when requests are sent; history state lifted to DataContext so UrlBar writes propagate to HistoryPanel immediately
- **Table mode add row broken** — New empty rows (["", ""]) no longer get silently dropped during JSON serialization; table entries are now maintained as independent state per tab, synced from JSON on mode/tab switch
- **Right sidebar icon sizing** — Sidebar tool icons rebalanced (consistent 14px icons, tighter padding, rounded-lg containers); History count badge now visible with cyan background and proper top-right positioning
- **Cmd/Ctrl+S saves notes** — Note modal now intercepts Cmd/Cmd+S to save the current note, preventing the UrlBar save-collection handler from firing while the notebook is open
- **Add row button visibility** — Table mode add-row buttons now have visible border, background, and proper sizing so they read clearly as actionable buttons
- **Response panel buttons on Headers tab** — Search now filters response headers; Copy button copies all headers as JSON; Raw view toggle hidden on Headers tab (not applicable)
- **Body content type header sync** — Content-Type header is now automatically set based on the selected body type (JSON/form-urlencoded/text/multipart) when sending requests; user-set Content-Type in Headers tab takes precedence
- **Breadcrumb navigation full page reload** — Collection/route dropdown switching in RequestForm breadcrumb now uses `router.push()` for client-side navigation, matching the Selector sidebar's behavior exactly (zero page reload)
- **Sidebar state not persisting** — Sidebar collapsed/expanded state now persists to localStorage and restores on page load; toggling via Ctrl/Cmd+B saves the state immediately
- **Removed debug `console.log` statements** from `app/page.tsx`, `ResponsePanel.tsx`, and `NoteModal.tsx`
- **Crash on corrupted localStorage** — `GetHittableCollections` now catches `JSON.parse` errors and falls back to defaults
- **Set-Cookie parsing** — cookies whose `Expires` date contains a comma were split apart; every `Set-Cookie` header is now read separately.
- **Crash on network errors in proxy route** — `app/api/proxy/route.ts` now wraps upstream `fetch` in try/catch and returns structured error responses
- **Crash on malformed request body** — `req.json()` in proxy route now has try/catch
- **Crash on non-JSON proxy errors** — `fetchViaProxy` in `hittableProxy.ts` now handles non-JSON responses gracefully
- **Crash on malformed headers** — `JSON.parse(headers)` in `hittableProxy.ts` now has try/catch
- **Crash on malformed import data** — `ImportModal` now validates decompressed data and shows error messages
- **Crash on malformed export data** — `ExportModal` now handles JSON.parse failures gracefully
- **Stack overflow on large exports** — `compressString` now uses loop instead of spread syntax
- **Silent data loss on storage full** — `localStorage.setItem` calls now catch `QuotaExceededError`
- **Unsaved changes lost on close** — Added `beforeunload` warning when there are unsaved changes
- **Windows line endings break curl** — `curlConverter` now normalizes `\r\n` to `\n` before parsing
- **URL params not encoded** — `modifyUrlForNewParams` now uses `encodeURIComponent` for proper encoding
- **Fragment identifiers in URL params** — `getParamsfromUrl` now strips `#fragment` before parsing
- **Duplicate import names** — `ImportModal` now loops to find truly unique names (e.g., "Collection - 1", "Collection - 2")
- **Curl body with single quotes** — `jsonToCurl` now escapes single quotes in body
- **Curl headers with double quotes** — `jsonToCurl` now escapes double quotes in header values
- **Nested objects in curl headers** — `cleanObj` in `curlConverter` now handles non-string values
- **Loose equality** — Changed `==` to `===` in `renameCurlName` and `deleteCurlName`
- **`isAlreadyExists` undefined return** — Now always returns `boolean` instead of `undefined`

### Added
- **ARIA attributes on modals** — `ModalShell` now has `role="dialog"`, `aria-modal`, `aria-labelledby`, focus trap, and focus restoration
- **`aria-label` on modal inputs** — `ModalInput` now accepts and applies `ariaLabel` prop
- **`aria-label` on modal buttons** — Cancel and confirm buttons now have accessible labels
- **`aria-live` on response status** — Response panel status badge now announces changes to screen readers
- **`aria-hidden` on decorative elements** — Corner bracket decorations in modals are now hidden from assistive tech
- **Focus management** — Modals now trap focus and restore previous focus on close
- **Error feedback in modals** — Import and Export modals now show error messages to users
- **`GetResume` error handling** — Landing page gracefully handles resume API failures
