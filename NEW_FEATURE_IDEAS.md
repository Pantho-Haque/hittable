# NEW_FEATURE_IDEAS.md — Phase 1 Research Findings

> Generated from competitive analysis of Postman, Hoppscotch, Bruno, Insomnia,
> React 19 / Next.js 16 production pattern audit, and curl/JSON/URL edge case audit.
> No code changes — research only.

---

## Competitive Analysis Summary

Hittable's lightweight, zero-install browser approach is a strong differentiator. The biggest gaps versus competitors:

| Capability | Hittable | Postman | Hoppscotch | Bruno | Insomnia |
|------------|----------|---------|------------|-------|----------|
| Scripting (pre/post) | ❌ | ✅ | ✅ | ✅ | ✅ |
| Auth helpers | ❌ | ✅ | ✅ | ✅ | ✅ |
| Request history | ❌ | ✅ | ✅ | ✅ | ✅ |
| Response headers tab | ❌ | ✅ | ✅ | ✅ | ✅ |
| GraphQL | ❌ | ✅ | ✅ | ✅ | ✅ |
| WebSocket/SSE | ❌ | ❌ | ✅ | ✅ | ✅ |
| OpenAPI import | ❌ | ✅ | ✅ | ✅ | ✅ |
| Folders in collections | ❌ | ✅ | ❌ | ✅ | ✅ |
| Environment switcher | per-collection | ✅ | ✅ | ✅ | ✅ |

---

## Prioritized Feature Ideas

### TIER 1 — High Impact, Low-Medium Effort (implement first)

#### 1. Response Time & Size Metrics
**Effort:** Low | **Impact:** High
- Display request duration (ms) and response payload size (bytes/KB) in ResponsePanel header
- Trivial to add: capture `Date.now()` before/after proxy call, `Content-Length` or `JSON.stringify(data).length`
- Every competitor has this; developers rely on it for performance debugging
- Touches: `UrlBar.tsx` (capture timing), `ResponsePanel.tsx` (display metrics)

#### 2. Response Headers Tab
**Effort:** Low | **Impact:** High
- Add a "Headers" tab next to the response body in ResponsePanel
- Display response headers as a key-value table with copy-per-header
- Essential for debugging auth (WWW-Authenticate), caching (Cache-Control), CORS (Access-Control-*), cookies
- Touches: `ResponsePanel.tsx` (new tab), no new components needed

#### 3. Raw Response View Toggle
**Effort:** Low | **Impact:** Medium
- Toggle between JSON tree view and raw text view in ResponsePanel
- Useful for HTML/XML/non-JSON responses that currently render as raw strings
- Touches: `ResponsePanel.tsx` (add toggle button)

#### 4. Auth Presets
**Effort:** Low-Medium | **Impact:** High
- Built-in helpers for Bearer Token, Basic Auth, and API Key auth
- Modal/dropdown in UrlBar area that auto-populates the Headers tab
- "Bearer Token" → adds `Authorization: Bearer <<token>>` header
- "Basic Auth" → base64-encodes `user:password` into `Authorization: Basic ...`
- "API Key" → adds a customizable header or query param
- Already on the Hittable roadmap
- Touches: new `AuthModal.tsx` or dropdown in UrlBar, `hittableCollectionModifier.ts`

#### 5. Request History
**Effort:** Medium | **Impact:** High
- Automatically log every sent request with timestamp, method, URL, status, duration
- Display in a collapsible panel or separate view
- Click to reload a previous request
- Persist to localStorage (separate key, e.g. `"hittable_history"`)
- Cap at N entries (e.g. 100) with FIFO eviction
- Universal feature across all competitors
- Touches: new `useHistory` hook, new `HistoryPanel.tsx`, localStorage integration

#### 6. Unsaved Changes Protection
**Effort:** Low | **Impact:** High (bug fix category, but has feature-like scope)
- Add `beforeunload` event handler when there are unsaved changes
- Guard route switching in Selector — confirm before navigating away from dirty form
- Prevents silent data loss (currently the #1 user-damaging issue)
- Touches: `dataContext.tsx`, `Selector.tsx`

### TIER 2 — High Impact, Medium-High Effort

#### 7. OpenAPI/Swagger Import
**Effort:** Medium | **Impact:** High
- Import `.yaml` / `.json` OpenAPI 3.x specs
- Parse endpoints, methods, parameters, headers, request bodies
- Auto-generate collections with routes pre-filled
- Already on the Hittable roadmap
- Would need a YAML parser (consider `js-yaml` or `yaml` package — ~30KB)

#### 8. Folders Within Collections
**Effort:** Medium | **Impact:** Medium
- Nested grouping of routes within a collection
- Collapsible folder nodes in the Routes panel
- Data shape change: add optional `folders` array to `THittableCollection`
- Touches: `Selector.tsx`, `types/hittable.ts`, `hittableCollectionModifier.ts`

#### 9. Multiple Named Environments Per Collection
**Effort:** Medium | **Impact:** Medium
- Instead of one `env` per collection, support named environments (dev/staging/prod)
- Dropdown to switch between environments
- "Base" environment that others inherit from
- Touches: `EnvModal.tsx`, `types/hittable.ts`, `resolveEnv()`

#### 10. Secret Variable Masking
**Effort:** Low | **Impact:** Medium
- Mark environment variables as "secret" in the EnvModal
- Display masked (••••••) in the UI, reveal on hover/click
- Never export secrets in collection export
- Touches: `EnvModal.tsx`, `ExportModal.tsx`, `types/hittable.ts`

### TIER 3 — Medium Impact, Variable Effort

#### 11. Export as Postman-Compatible JSON
**Effort:** Medium | **Impact:** Medium
- Export collections in Postman Collection v2.1 format
- Enables cross-tool migration
- Touches: new `exportPostman.ts` utility

#### 12. Code Snippet Generation
**Effort:** Medium | **Impact:** Low-Medium
- Generate code snippets in Python (requests), JavaScript (fetch), cURL from current request
- Postman-style "Code" button
- Touches: new `codeGenerator.ts` utility

#### 13. Pre-request Scripts (Sandboxed)
**Effort:** High | **Impact:** High
- Sandboxed JS execution before requests (e.g. generate timestamps, sign requests)
- Would need `iframe` sandbox or `vm` polyfill for browser safety
- Complex to implement securely; defer to later

#### 14. Post-request Test Scripts
**Effort:** High | **Impact:** High
- Assert on status code, response body fields, headers
- Would need a mini test runner and assertion library
- Complex; defer to later

#### 15. GraphQL Support
**Effort:** Medium-High | **Impact:** Medium
- Dedicated GraphQL tab with query editor, variables, schema introspection
- Touches: new `GraphQLEditor.tsx`, response display changes

#### 16. WebSocket Testing
**Effort:** Medium | **Impact:** Medium
- Connect, send/receive messages, view message log
- Would be a new protocol alongside HTTP
- Touches: new `WebSocketPanel.tsx`, new route type

---

## React/Next.js Production Pattern Gaps

### Critical (affects reliability)

| # | Issue | Files | Effort |
|---|-------|-------|--------|
| P1 | `JSON.parse` on localStorage without try/catch — corrupted data crashes entire app | `services/Hittable.ts:9` | Low |
| P2 | `localStorage.setItem` without QuotaExceededError handling — silent data loss | `context/dataContext.tsx:95`, `utils/noteModifier.ts:20` | Low |
| P3 | Upstream `fetch` in proxy route has no try/catch — opaque 500 on network errors | `app/api/proxy/route.ts:29` | Low |
| P4 | `req.json()` in proxy route has no try/catch — malformed request body crashes handler | `app/api/proxy/route.ts:4` | Low |
| P5 | `compressString` spread syntax on large arrays causes stack overflow | `utils/compressString.ts:6` | Low |
| P6 | `JSON.parse(headers)` in hittableProxy not protected | `utils/hittableProxy.ts:70` | Low |
| P7 | `JSON.parse` in ImportModal and ExportModal not protected | `components/modals/ImportModal.tsx:19`, `ExportModal.tsx:19` | Low |

### High (affects user experience)

| # | Issue | Files | Effort |
|---|-------|-------|--------|
| P8 | No focus trap in modals — Tab escapes behind overlay | `components/ui/SharedModal.tsx` | Medium |
| P9 | No `role="dialog"`, `aria-modal`, `aria-labelledby` on modals | `components/ui/SharedModal.tsx` | Low |
| P10 | No `aria-live` region for response updates | `components/RequestForm/ResponsePanel.tsx` | Low |
| P11 | No `beforeunload` warning for unsaved changes | `context/dataContext.tsx` | Low |
| P12 | No guard against navigating away from dirty form | `components/hittable/Selector.tsx` | Medium |
| P13 | Monolithic DataContext causes excessive re-renders | `context/dataContext.tsx` | High |
| P14 | `JsonNode` recursive component not memoized | `components/RequestForm/ResponsePanelComponents/JsonNode.tsx` | Low |
| P15 | No loading skeleton for proxy requests | `components/RequestForm/ResponsePanel.tsx` | Low-Medium |
| P16 | `Suspense fallback={null}` provides zero visual feedback | `app/hittable/page.tsx:20` | Low |

### Medium (code quality)

| # | Issue | Files | Effort |
|---|-------|-------|--------|
| P17 | `aria-expanded` missing on JSON tree toggle buttons | `JsonNode.tsx` | Low |
| P18 | TabEditor tabs lack ARIA tab pattern (`role="tablist"`) | `TabEditor.tsx` | Low |
| P19 | Icon-only buttons lack `aria-label` | `UrlBar.tsx`, modals | Low |
| P20 | `PanelItem` in Selector is `<div onClick>` — not keyboard-focusable | `Selector.tsx` | Low |
| P21 | `ShortcutContext` value not memoized | `ShortcutKeypressProvider.tsx` | Low |
| P22 | `isUnsaved` is a function, not a derived boolean | `dataContext.tsx:99` | Low |

---

## Curl/JSON/URL Edge Case Bugs

### Critical (crashes)

| # | Issue | File:Line |
|---|-------|-----------|
| B1 | `JSON.parse` on localStorage without try/catch | `services/Hittable.ts:9` |
| B2 | `localStorage.setItem` with no QuotaExceededError handling | `context/dataContext.tsx:95` |
| B3 | Upstream fetch with no try/catch in proxy route | `app/api/proxy/route.ts:29` |
| B4 | `req.json()` with no try/catch in proxy route | `app/api/proxy/route.ts:4` |
| B5 | `compressString` spread on large array → stack overflow | `utils/compressString.ts:6` |
| B6 | `JSON.parse(headers)` unprotected in proxy path | `utils/hittableProxy.ts:70` |
| B7 | `JSON.parse` in ImportModal/ExportModal unprotected | `modals/ImportModal.tsx:19`, `ExportModal.tsx:19` |

### High (incorrect behavior)

| # | Issue | File:Line |
|---|-------|-----------|
| B8 | No `\r\n` handling for Windows line endings in curl | `utils/curlConverter.ts` |
| B9 | Fragment identifiers not stripped from URL params | `utils/responsePanelUtils.ts:51` |
| B10 | `modifyUrlForNewParams` double-decodes already-encoded URLs | `utils/responsePanelUtils.ts:70` |
| B11 | `modifyUrlForNewParams` doesn't re-encode params with `encodeURIComponent` | `utils/responsePanelUtils.ts:66-68` |
| B12 | `postMessage` with `"*"` origin leaks request data | `utils/hittableProxy.ts:23` |
| B13 | Duplicate name on import only gets one `- New` suffix | `modals/ImportModal.tsx:21-23` |
| B14 | `jsonToCurl` breaks on single quotes in body | `utils/curlConverter.ts:77` |
| B15 | `jsonToCurl` breaks on double quotes in header values | `utils/curlConverter.ts:67` |
| B16 | Proxy route `response.json()` unguarded for non-JSON errors | `utils/hittableProxy.ts:57` |
| B17 | `GetResume` fetch has no error handling | `services/Hittable.ts:13-20` |

### Medium (incorrect behavior / robustness)

| # | Issue | File:Line |
|---|-------|-----------|
| B18 | `set-cookie` parsing splits on commas incorrectly | `app/api/proxy/route.ts:71-82` |
| B19 | Only one `set-cookie` header retrieved | `app/api/proxy/route.ts:71` |
| B20 | No fetch timeout on upstream requests | `app/api/proxy/route.ts:29` |
| B21 | `curlConverter` `cleanObj` produces `[object Object]` for nested values | `utils/curlConverter.ts:37` |
| B22 | `isAlreadyExists` returns `undefined` instead of `false` | `utils/hittableCollectionModifier.ts:73` |
| B23 | `valueMatchesSearch`/`countMatches` stack overflow on deep JSON | `utils/responsePanelUtils.ts:16,38` |
| B24 | `decompressString` has no input validation | `utils/compressString.ts:12-16` |
| B25 | Curl paste handler has 1-second setTimeout race condition | `components/RequestForm/UrlBar.tsx:62-68` |
| B26 | Debug `console.log` statements left in production code | `ResponsePanel.tsx:45`, `page.tsx:59`, `NoteModal.tsx:22` |

### Low (minor)

| # | Issue | File:Line |
|---|-------|-----------|
| B27 | `==` instead of `===` in `renameCurlName` and `deleteCurlName` | `hittableCollectionModifier.ts:51,109` |
| B28 | `createNote` uses `Date.now()` ID — collision risk | `utils/noteModifier.ts:24` |
| B29 | `modifyUrlForNewParams` returns trailing `?` on empty params | `utils/responsePanelUtils.ts:65-69` |
| B30 | `selectorResponse!` non-null assertion can crash | `UrlBar.tsx:23` |
| B31 | `handleSaveCollection` non-null assertions | `dataContext.tsx:79,85-86` |

---

## Recommended Implementation Order

### Phase 2 (Bug Hunt) — Fix these first
1. Remove debug `console.log` statements (B26) — trivial
2. Wrap all `JSON.parse` calls in try/catch (B1-B7) — critical crash prevention
3. Add try/catch to proxy route (B3-B4) — critical
4. Fix `localStorage.setItem` quota handling (B2) — critical
5. Fix `compressString` stack overflow (B5) — critical
6. Add `beforeunload` for unsaved changes (P11) — high UX impact
7. Add basic ARIA attributes to modals (P9) — high accessibility impact
8. Add `aria-live` to response panel (P10) — high accessibility impact
9. Fix duplicate name import loop (B13) — high correctness
10. Fix Windows line endings in curl (B8) — high correctness
11. Fix URL param encoding (B10-B11) — high correctness

### Phase 3 (New Features) — Implement in order
1. **Response Time & Size Metrics** — lowest effort, highest immediate value
2. **Response Headers Tab** — low effort, high value
3. **Raw Response View Toggle** — low effort, medium value
4. **Auth Presets** — medium effort, high value, on roadmap
5. **Request History** — medium effort, high value, universal feature
