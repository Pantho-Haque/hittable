# Changelog

All notable changes to Hittable will be documented in this file.

## [Unreleased]

### Added
- **Response time & size metrics** — ResponsePanel now displays request duration (ms/s) and payload size (B/KB/MB) in the header bar
- **Response headers tab** — ResponsePanel now has Body/Headers tabs; headers table shows all response headers with per-header copy button
- **Raw response view toggle** — ResponsePanel now has a toggle button to switch between JSON tree view and raw text view (useful for HTML/XML/non-JSON responses)
- **Auth presets** — New Auth modal with Bearer Token, Basic Auth, and API Key presets; auto-populates Authorization header based on selection; available in the right sidebar
- **Request history** — Automatically logs all sent requests with timestamp, method, URL, status, and duration; stored in localStorage (max 100 entries); replay past requests with one click; displayed in a ModalShell modal matching the app's modal visual language; realtime updates — history modal reflects new entries immediately
- **Icon-only Auth and History buttons** — Auth and History buttons now use icon-only format with tooltips, matching the existing `modal-button-mini` convention used by Import, Notes, and Info buttons
- **Markdown notes editor** — Notes now support Markdown with 3-way view toggle (Edit/Split/Preview); Split mode shows side-by-side raw editor + live preview (stacked on mobile); preview renders headings, lists, code blocks, blockquotes, tables, links; uses `marked` library
- **Notes list UX upgrade** — NotePills now show content preview snippet, FileText icon per note, cyan accent on selected note, improved hover/active states, search icon in search field, empty state with icon
- **Table mode for Body/Headers/Params** — TabEditor now has JSON/Table mode toggle (Braces/Table2 icons); Table mode provides key-value row-based input with add/remove rows; both modes stay in sync with the underlying JSON state
- **Notes modal redesign** — Notebook modal now uses 95vw/85vh sizing (max 1100px), custom modal markup with corner brackets, dark theme, spacious layout with word count footer; replaces generic ModalShell for this flagship feature

### Fixed
- **Left sidebar collapse missing tools** — Collapsed left sidebar now shows Auth and History buttons (matching the right sidebar's tool set); previously these were missing when the sidebar was collapsed via Ctrl/Cmd+B
- **Mobile responsive layout** — Sidebar auto-collapses on screens < 768px; expanded sidebar shows as overlay with backdrop on mobile; right sidebar hidden on mobile (tools available in collapsed left sidebar); modal sizes use `max-width` to fit small viewports; touch targets increased to 44px minimum on mobile for buttons, tabs, and method selector
- **History not realtime** — History modal now updates live when requests are sent; history state lifted to DataContext so UrlBar writes propagate to HistoryPanel immediately
- **Table mode add row broken** — New empty rows (["", ""]) no longer get silently dropped during JSON serialization; table entries are now maintained as independent state per tab, synced from JSON on mode/tab switch
- **Removed debug `console.log` statements** from `app/page.tsx`, `ResponsePanel.tsx`, and `NoteModal.tsx`
- **Crash on corrupted localStorage** — `GetHittableCollections` now catches `JSON.parse` errors and falls back to defaults
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
