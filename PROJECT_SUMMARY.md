# PROJECT_SUMMARY.md — Hittable

## 1. Project Overview

**What it does:** Hittable is a browser-based API testing client (alternative to Postman/Insomnia). It lets developers organize API endpoints into collections, send HTTP requests through a CORS-free server-side proxy, import/export curl commands, manage environment variables, and view responses with a JSON tree viewer — all driven by keyboard shortcuts.

**Target user:** Backend and full-stack developers who want a lightweight, browser-only API testing tool without installing desktop software.

**Tech stack:**
- **Framework:** Next.js 16.1.6 (App Router, Turbopack for dev)
- **Language:** TypeScript 5.x (strict mode)
- **UI:** React 19.2.0, Tailwind CSS 4.1.11, Framer Motion 12.23.0
- **State management:** React Context (no Redux/Zustand)
- **Data fetching:** TanStack React Query 5.81.5 (configured, lightly used)
- **HTTP:** Axios 1.13.3 (dep, not actively used in main flow), native `fetch`
- **UI primitives:** Radix UI Accordion, Lucide React icons
- **Utility:** clsx + tailwind-merge (cn helper), class-variance-authority, pako (gzip compression)
- **curl parsing:** `@bany/curl-to-json` library
- **Build tool:** pnpm (lockfile present), Next.js built-in
- **Commit hygiene:** Husky + commitlint (conventional commits) + lint-staged (eslint --fix)
- **Deployment:** Vercel (primary), Docker support (Dockerfile + docker-compose.yml)

**Project type:** Web application (SPA-like), with a companion Chrome extension for localhost requests.

---

## 2. Architecture

### Folder/file structure

```
hittable/
├── app/                          # Next.js App Router pages and API routes
│   ├── layout.tsx                # Root layout: fonts, providers, <Topbar/>, metadata, JSON-LD
│   ├── page.tsx                  # Landing page (server component, fetches resume data)
│   ├── hittable/page.tsx         # Main app page (client component, the API client UI)
│   ├── api/proxy/route.ts        # Server-side CORS proxy endpoint (POST)
│   ├── error.tsx                 # Error boundary
│   ├── not-found.tsx             # 404 page
│   ├── robots.ts                 # SEO robots
│   └── sitemap.ts                # SEO sitemap
├── components/
│   ├── hittable/                 # Core app components
│   │   ├── Selector.tsx          # Sidebar: collections list + routes list, selection via URL params
│   │   ├── RequestForm.tsx       # Main form area: URL bar + tab editor + response panel
│   │   ├── Menu.tsx              # Context menu (rename, delete, export) for collections/routes
│   │   ├── SyntaxHighlighter.tsx # JSON syntax highlighting with URL detection
│   │   └── HistoryPanel.tsx      # Request history panel with replay functionality
│   ├── RequestForm/              # Request form sub-components
│   │   ├── UrlBar.tsx            # Method selector + URL textarea + send/save/copy buttons
│   │   ├── TabEditor.tsx         # Params/Body/Headers tabbed JSON editor
│   │   ├── ResponsePanel.tsx     # Response display with search, status badge, JSON tree
│   │   └── ResponsePanelComponents/
│   │       ├── JsonNode.tsx      # Recursive collapsible JSON tree renderer
│   │       ├── FloatingSearch.tsx # Search overlay for response JSON
│   │       ├── Highlight.tsx     # Text highlighting with match registry
│   │       ├── MatchContext.tsx   # React context for search match tracking
│   │       └── CopyButton.tsx    # Copy JSON to clipboard button
│   ├── modals/                   # All modal dialogs
│   │   ├── CreateModal.tsx       # Create collection or route
│   │   ├── RenameModal.tsx       # Rename collection or route
│   │   ├── DeleteModal.tsx       # Delete collection or route (with confirmation)
│   │   ├── EnvModal.tsx          # Environment variables editor (per collection)
│   │   ├── ImportModal.tsx       # Import collection from compressed string
│   │   ├── ExportModal.tsx       # Export collection as compressed string
│   │   ├── NoteModal.tsx         # Notebook modal with editor
│   │   ├── InfoModal.tsx         # Info accordion (keybindings + extension docs)
│   │   ├── NoExtensionModal.tsx  # Prompt to install browser extension
│   │   └── AuthModal.tsx         # Auth presets modal (Bearer, Basic, API Key)
│   ├── notes/
│   │   ├── NotePills.tsx         # Note list with search, rename, delete
│   │   └── NoteEditor.tsx        # Note content textarea
│   ├── homepage/
│   │   ├── TopBar.tsx            # Global topbar with logo, nav, CTA
│   │   ├── TerminalDemo.tsx      # Animated terminal demo for landing page
│   │   └── PortfolioSection.tsx  # Developer profile section on landing page
│   ├── ui/
│   │   ├── SharedModal.tsx       # Reusable ModalShell, ModalInput, ModalActions
│   │   └── Accordion.tsx         # Radix-based accordion component
│   └── index.ts                  # Barrel export for all components
├── context/                      # React Context providers
│   ├── providers.tsx             # Root provider wrapper (Notification → Client)
│   ├── ClientProviders.tsx       # Client-side providers: QueryClient, DataProvider, ShortcutProvider
│   ├── dataContext.tsx           # Main data context: collections, selector, form state, save logic
│   ├── notifyContext.tsx         # Toast notification system with framer-motion animations
│   └── ShortcutKeypressProvider.tsx # Global keyboard shortcut state (sidebar toggle)
├── hooks/
│   ├── useKeypress.ts            # Generic keyboard shortcut hook (meta/shift modifiers)
│   ├── useExtension.ts           # Browser extension detection via postMessage polling
│   ├── useNotify.ts              # Notification hook (info/success/error wrappers)
│   └── index.ts                  # Barrel export
├── services/
│   ├── Hittable.ts               # GetHittableCollections (localStorage), GetResume (external API)
│   └── index.ts                  # Barrel export
├── stores/
│   └── auth.ts                   # Commented-out Pinia auth store (legacy, unused)
├── utils/
│   ├── curlConverter.ts          # curl → JSON parser + JSON → curl serializer
│   ├── hittableProxy.ts          # Request routing: extension for localhost, proxy for external
│   ├── hittableCollectionModifier.ts # CRUD operations on collections (create/rename/delete/update)
│   ├── formatJson.ts             # JSON formatting/validation utility
│   ├── responsePanelUtils.ts     # JSON tree helpers: type detection, search matching, URL param utils
│   ├── noteModifier.ts           # Notes localStorage CRUD (load/save/create/delete/rename/filter)
│   ├── compressString.ts         # pako gzip compression + base64 encoding (for import/export)
│   ├── JsonStringParsing.ts      # Parse JSON string or "Key: Value" format
│   ├── historyModifier.ts        # Request history CRUD (load/save/add/clear/format)
│   ├── apiRequest.js             # Generic API request utility (legacy, not used in main flow)
│   ├── check-user-permission.js  # Commented-out permission checker (legacy, unused)
│   └── cn.ts                     # clsx + tailwind-merge utility
├── types/
│   ├── hittable.ts               # Core types: collections, curls, curlJson, responseJson, env, history
│   ├── note.ts                   # Note types: NoteId, Note, NotesStore
│   ├── notification.ts           # Notification types
│   └── index.ts                  # Barrel export + TApiResponse generic type
├── config/
│   ├── apiEndpoints.ts           # API endpoint definitions (posts CRUD, unused in main app)
│   └── index.ts                  # Config exports: EXTENSION_URL, api config
├── constants/
│   ├── hittable.ts               # HTTP methods, method colors, default collections (JSONPlaceholder)
│   ├── landing.ts                # Landing page content, feature cards, portfolio data
│   ├── misc.ts                   # Keybindings documentation array
│   └── index.ts                  # Barrel export
├── styles/
│   ├── app.css                   # Tailwind import + custom scrollbar styling
│   ├── font.css                  # Icomoon icon font definitions
│   ├── variables.css             # SCSS variable (legacy, $font-regular)
│   ├── theme/defaultTheme.js     # Legacy theme file
│   └── components/               # Legacy CSS component files (button, card, form, modal, sidebar, list)
├── public/
│   ├── assets/                   # Static assets (images, fonts)
│   ├── manifest.json             # PWA manifest
│   └── google04fc6d619347de5b.html # Google Search Console verification
├── browserExtension/             # Chrome extension source
│   ├── manifest.json             # Manifest V3: permissions for localhost, content script injection
│   ├── content.js                # Content script: bridges page ↔ extension via postMessage
│   └── background.js             # Service worker: makes fetch requests to localhost
├── package.json
├── pnpm-lock.yaml
├── tsconfig.json
├── next.config.ts
├── postcss.config.mjs
├── eslint.config.mjs
├── commitlint.config.js
├── vercel.json
├── Dockerfile
├── docker-compose.yml
├── dockerEntryPoint.sh
├── run.sh
├── .env.example                  # Empty (no env vars currently required)
├── .gitignore
├── .husky/                       # Git hooks
├── CHANGELOG.md                  # Changelog documenting all changes
├── NEW_FEATURE_IDEAS.md          # Phase 1 research findings
└── README.md
```

### Main entry points

- **Landing page:** `app/page.tsx` — Server component, fetches resume data from external API, renders hero, features, keyboard shortcuts, portfolio, footer.
- **App page:** `app/hittable/page.tsx` — Client component (`"use client"`), the main API client UI. Composes `<Selector />` (sidebar) + `<RequestForm />` (main area) + sidebar tool buttons.
- **API proxy:** `app/api/proxy/route.ts` — Next.js Route Handler (POST), proxies HTTP requests from the browser to any target URL, bypassing CORS.

### Data flow

1. **Collections** are loaded from `localStorage` on mount via `GetHittableCollections()`. If empty, a default "JSONPlaceholder Lab" collection is seeded.
2. **`DataContext`** (React Context) holds all app state:
   - `collections` — full collection tree (persisted to localStorage on every change)
   - `selectorResponse` — currently selected collection/route/env/curlJson/responseJson
   - `formInput` — the editable form state (method, url, headers, body, params)
   - `proxyResponse` — the last API response
   - `extensionAvailable` / `extensionChecked` — browser extension status
3. **Selection** is driven by URL search params (`?c=collectionName&r=routeName`). The `Selector` component reads/writes these params via `useRouter`/`useSearchParams`.
4. **Sending a request:** `UrlBar` calls `hittableProxy()` which:
   - Resolves `<<KEY>>` environment variables in the form input
   - If the target is localhost AND the extension is available → `fetchViaExtension()` (postMessage to content script → background service worker → fetch)
   - If the target is localhost AND no extension → throws error
   - Otherwise → `fetchViaProxy()` which POSTs to `/api/proxy` (Next.js server-side fetch)
5. **Saving:** `handleSaveCollection()` updates the collection in state, which triggers the `useEffect` to persist to `localStorage`.

### Persistence layer

- **`localStorage`** with key `"hittable"` — stores the entire `THittableCollections` array as JSON.
- **`localStorage`** with key `"notesStore"` — stores all notes as a `Record<NoteId, Note>`.
- **`localStorage`** with key `"hittable_history"` — stores request history (max 100 entries).
- No server-side database. All data is client-side only.

---

## 3. Features Implemented

### Core features
1. **Collection management** — Create, rename, delete collections. Collections are named groups of API routes.
2. **Route management** — Create, rename, delete routes within a collection. Each route stores a curl string and its last response.
3. **curl import** — Paste a curl command into the URL bar or route creation modal; it auto-parses method, URL, headers, body, and query params using `@bany/curl-to-json`.
4. **curl export** — Copy the current request as a curl command via the "Copy as CURL" button.
5. **CORS-free proxy** — Server-side proxy at `/api/proxy` forwards requests from the browser to any URL, avoiding CORS restrictions.
6. **Browser extension for localhost** — Chrome extension (Manifest V3) that bridges requests to `localhost`/`127.0.0.1` via `postMessage` → content script → background service worker → `fetch()`.
7. **Environment variables** — Per-collection key-value pairs referenced as `<<KEY>>` in URLs, headers, and body. Resolved at request time via `resolveEnv()`.
8. **Persistent storage** — Collections auto-save to localStorage. Close the tab and return — everything persists.
9. **JSON response viewer** — Collapsible recursive `JsonNode` tree with type-colored values (strings=green, numbers=blue, booleans=purple, null=red).
10. **Response search** — `Ctrl/Cmd+F` opens a floating search bar. Matches are highlighted in yellow with active match glow. Enter/Shift+Enter navigates between matches.
11. **Copy response** — One-click copy of formatted JSON response to clipboard.
12. **Tab editor** — Params/Body/Headers tabs with JSON-formatted textarea. Editing params auto-updates the URL query string.
13. **Keyboard shortcuts:**
    - `Ctrl/Cmd+Enter` — Send request
    - `Ctrl/Cmd+S` — Save changes to collection
    - `Ctrl/Cmd+F` — Search in response
    - `Ctrl/Cmd+B` — Toggle sidebar
    - `Shift+T` — Create new route in current collection
14. **Import/Export collections** — Collections can be exported as compressed (pako + base64) strings and imported on another instance.
15. **Notes notebook** — Create, rename, delete, search, and edit notes. Persisted to localStorage. Available via the notebook modal.
16. **Notification system** — Toast notifications (info/success/error) with configurable position and timeout, animated with Framer Motion.
17. **Unsaved changes indicator** — Amber badge shows when the form differs from the saved collection state.
18. **Method color coding** — Each HTTP method has a distinct color (GET=cyan, POST=green, PUT=orange, PATCH=purple, DELETE=red, HEAD=gray) applied to the URL bar border, glow, and send button.
19. **URL detection in responses** — URLs in JSON responses are highlighted and clickable. Click copies; Cmd+click opens in new tab.
20. **Response time & size metrics** — ResponsePanel header displays request duration (ms/s) and payload size (B/KB/MB) after each request.
21. **Response headers tab** — ResponsePanel has Body/Headers tabs; headers table shows all response headers with per-header copy button and count badge.
22. **Raw response view toggle** — Toggle between JSON tree view and raw text view in ResponsePanel (useful for HTML/XML/non-JSON responses).
23. **Auth presets** — Auth modal with Bearer Token, Basic Auth, and API Key presets; auto-populates Authorization header based on selection; available in the right sidebar.
24. **Request history** — Automatically logs all sent requests with timestamp, method, URL, status, and duration; stored in localStorage (max 100 entries); replay past requests with one click; available in the right sidebar.
20. **Landing page** — Marketing page with hero section, animated terminal demo, feature cards, keyboard shortcuts section, developer portfolio section, and footer. Server-rendered with SEO metadata and JSON-LD.

### Partially implemented / legacy
- **`stores/auth.ts`** — Entirely commented out. Was a Pinia-based auth store from a previous project. Not used.
- **`utils/apiRequest.js`** — Generic API request utility. Not used by the main app flow (the proxy approach replaced it).
- **`utils/check-user-permission.js`** — Commented out. Legacy permission checker.
- **`config/apiEndpoints.ts`** — Defines posts CRUD endpoints. Not used by the main app.
- **`styles/components/*.css`** — Legacy CSS files (button, card, form, modal, sidebar, list). Not imported by the main app (Tailwind handles styling).
- **`styles/variables.css`** — SCSS variable `$font-regular: 'Roboto'`. Not used (app uses Geist fonts).
- **`styles/theme/defaultTheme.js`** — Legacy theme file. Not imported.

### Known issues / observations
- `console.log(res)` in `app/page.tsx:59` — debug log left in production code.
- `console.log(matchEls.current.length)` in `ResponsePanel.tsx:45` — debug log left in production code.
- `console.log(selectedId, editContent)` in `NoteModal.tsx:22` — debug log left in production code.
- The `stores/auth.ts` and some utility files are vestiges of a previous project (possibly "Sancus") and should be cleaned up.
- No test files exist anywhere in the project.

---

## 4. Core Modules/Components

### Key components

| Component | File | Responsibility |
|-----------|------|----------------|
| `Selector` | `components/hittable/Selector.tsx` | Two-panel sidebar: collections list + routes list. Drives selection via URL params. Handles Shift+T for new route creation. |
| `RequestForm` | `components/hittable/RequestForm.tsx` | Main content area. Shows empty state or composes UrlBar + TabEditor + ResponsePanel. |
| `UrlBar` | `components/RequestForm/UrlBar.tsx` | Method dropdown + URL textarea + Save/Send/Copy buttons. Handles curl paste detection, keyboard shortcuts, auto-resize. |
| `TabEditor` | `components/RequestForm/TabEditor.tsx` | Params/Body/Headers tabbed editor. Params tab syncs with URL query string. |
| `ResponsePanel` | `components/RequestForm/ResponsePanel.tsx` | Response display with status badge, search, copy, and JSON tree. Manages match registry for search navigation. |
| `JsonNode` | `components/RequestForm/ResponsePanelComponents/JsonNode.tsx` | Recursive JSON tree renderer with collapsible objects/arrays, type coloring, and search highlighting. |
| `ModalShell` | `components/ui/SharedModal.tsx` | Reusable modal wrapper with portal, backdrop blur, corner bracket decorations, and size variants. |
| `Menu` | `components/hittable/Menu.tsx` | Context menu for collections/routes with rename, delete, export options. |

### Reusable hooks

| Hook | File | Usage |
|------|------|-------|
| `useKeypress` | `hooks/useKeypress.ts` | Generic keyboard shortcut hook. Used in UrlBar (Ctrl+Enter, Ctrl+S), Selector (Shift+T), ResponsePanel (Ctrl+F), ShortcutProvider (Ctrl+B). |
| `useExtension` | `hooks/useExtension.ts` | Detects browser extension availability via postMessage polling (10 attempts, 500ms interval). |
| `useNotification` | `hooks/useNotify.ts` | Convenience wrapper around NotificationContext with `info()`, `success()`, `error()` methods. |
| `useShortcuts` | `context/ShortcutKeypressProvider.tsx` | Accesses global shortcut state (currently just `toggleSidebar`). |

### Key utilities

| Utility | File | Purpose |
|---------|------|---------|
| `curlConverter` | `utils/curlConverter.ts` | Parses curl strings to `THittableCurlJson` using `@bany/curl-to-json`. Handles `<<ENV>>` variable shielding. |
| `jsonToCurl` | `utils/curlConverter.ts` | Serializes `THittableCurlJson` back to curl command string. |
| `hittableProxy` | `utils/hittableProxy.ts` | Routes requests: extension for localhost, proxy endpoint for everything else. |
| `resolveEnv` | `utils/hittableCollectionModifier.ts` | Replaces `<<KEY>>` placeholders with environment variable values in all form fields. |
| `updateCurl` / `createCurlName` / `deleteCurlName` / etc. | `utils/hittableCollectionModifier.ts` | Immutable collection CRUD operations returning new state. |
| `formatJson` | `utils/formatJson.ts` | Parses and pretty-prints JSON with tab indentation. Returns `{ output, error }`. |
| `compressString` / `decompressString` | `utils/compressString.ts` | pako deflate/inflate + URL-safe base64 encoding for collection import/export. |
| `countMatches` / `valueMatchesSearch` | `utils/responsePanelUtils.ts` | Recursive JSON search utilities for the response panel search feature. |
| `getParamsfromUrl` / `modifyUrlForNewParams` | `utils/responsePanelUtils.ts` | Bidirectional URL ↔ JSON params conversion. |
| `cn` | `utils/cn.ts` | clsx + tailwind-merge class name utility. |

---

## 5. UI/UX Details

### Design system & theming

- **Dark theme only** — No light mode toggle. The app uses a deep navy/dark blue color scheme.
- **Primary accent:** Cyan (`#00e5cc` / `cyan-500`) — used for active states, borders, glows, buttons.
- **Background:** `#080f1a` (app), `#0a1628` (panels), `#0e1f35` (modals/dropdowns), `#060d18` (topbar).
- **Text:** White with various opacity levels (`text-white/80`, `text-white/40`, `text-white/15`).
- **Fonts:** Geist Sans (body) + Geist Mono (code/UI elements), loaded via `next/font/google`.
- **Method colors:** GET=`#00e5cc`, POST=`#4ade80`, PUT=`#fb923c`, PATCH=`#a78bfa`, DELETE=`#f87171`, HEAD=`#94a3b8`.
- **Styling method:** Tailwind CSS 4.1.11 with `@tailwindcss/postcss`. Legacy CSS files exist but are not used by the main app.
- **Custom scrollbar:** Thin 4px width with cyan gradient thumb (`styles/app.css`).
- **Modal decorations:** Corner bracket borders on modals and URL bar (design signature).
- **Ambient effects:** Subtle blur glow orbs in the app background (`bg-cyan-500/5 blur-[120px]`).

### Navigation/routing

- `/` — Landing page (server-rendered)
- `/hittable` — Main app (client-rendered)
- `/hittable?c=CollectionName&route=RouteName` — App with pre-selected route (URL-param driven selection)
- No authentication, no login flow.

### Key user flows

1. **First visit:** User sees landing page → clicks "Launch App" → arrives at `/hittable` → sees default "JSONPlaceholder Lab" collection with 6 pre-configured routes.
2. **Sending a request:** Select collection → select route → URL bar populates with curl data → edit if needed → press `Ctrl+Enter` or click "Send" → response appears in ResponsePanel.
3. **Creating a new route:** Press `Shift+T` or click "New Route" → enter name (optionally paste curl) → route created and selected.
4. **Saving changes:** Edit URL/headers/body → "Unsaved" badge appears → press `Ctrl+S` or click "Save" → changes persisted to localStorage.
5. **Importing a collection:** Click Import icon in sidebar → paste compressed string → collection added (auto-renamed if name conflict).
6. **Using environment variables:** Click "Env Vars" → add key-value pairs → reference as `<<KEY>>` in requests → resolved at send time.
7. **Searching responses:** After receiving response → press `Ctrl+F` → type search query → matches highlighted in yellow → Enter/Shift+Enter to navigate.

---

## 6. Configuration & Environment

### Build/dev scripts

```bash
pnpm dev          # Next.js dev server with Turbopack
pnpm build        # Production build
pnpm start        # Start production server
pnpm lint         # ESLint
pnpm prepare      # Install Husky hooks
pnpm commitlint   # Validate commit message
```

### Environment variables

The `.env.example` file is empty. No environment variables are currently required for the app to function. The config file references `NEXT_API_BASE_URL` and `NEXT_API_CLIENT_SECRET` but these are not used by the active codebase.

### Key config files

| File | Purpose |
|------|---------|
| `next.config.ts` | Image remote patterns (localhost:3000, domiknows.vercel.app), SVG allowed |
| `tsconfig.json` | Strict mode, `@/*` path alias to project root, ES2017 target |
| `postcss.config.mjs` | Tailwind CSS PostCSS plugin |
| `eslint.config.mjs` | ESLint flat config: Next.js core-web-vitals + TypeScript + TanStack Query |
| `commitlint.config.js` | Conventional commits enforcement (feat/fix/docs/style/refactor/perf/test/build/ci/chore/revert) |
| `vercel.json` | Sitemap cache headers |
| `Dockerfile` | Node.js bookworm + pnpm 10.12.4 |
| `docker-compose.yml` | Docker Compose config |

### Dependencies (from package.json)

**Runtime:**
- next 16.1.6, react 19.2.0, react-dom 19.2.0
- @tanstack/react-query 5.81.5
- @radix-ui/react-accordion 1.2.12
- framer-motion 12.23.0
- lucide-react 0.548.0
- axios 1.13.3
- @bany/curl-to-json 1.2.10
- pako 2.1.0
- clsx 2.1.1, tailwind-merge 3.3.1, class-variance-authority 0.7.1
- postcss 8.5.6

**Dev:**
- typescript 5.x, @types/node, @types/react, @types/react-dom, @types/pako
- tailwindcss 4.1.11, @tailwindcss/postcss 4.1.11, tw-animate-css 1.4.0
- eslint 9.x, eslint-config-next 16.0.0, @tanstack/eslint-plugin-query 5.81.2
- husky 9.1.7, lint-staged 15.2.0
- @commitlint/cli 19.x, @commitlint/config-conventional 19.x
- baseline-browser-mapping 2.9.19

---

## 7. Current State

### Fully working
- Complete collection/route CRUD with localStorage persistence
- curl import (paste in URL bar or creation modal)
- curl export (copy as curl)
- CORS-free server-side proxy (`/api/proxy`)
- Browser extension for localhost requests (Manifest V3 Chrome extension)
- Environment variable system with `<<KEY>>` resolution
- JSON response tree viewer with collapsible nodes
- Response search with match highlighting and navigation
- Keyboard shortcuts (Ctrl+Enter, Ctrl+S, Ctrl+F, Ctrl+B, Shift+T)
- Collection import/export via compressed strings
- Notes notebook with CRUD and search
- Toast notification system
- Landing page with SEO metadata, JSON-LD, and animated demo
- Unsaved changes detection

### Still being iterated on / cleanup needed
- Debug `console.log` statements left in production code (3 instances)
- Legacy files from a previous project (`stores/auth.ts`, `utils/apiRequest.js`, `utils/check-user-permission.js`, `config/apiEndpoints.ts`, `styles/components/*.css`, `styles/variables.css`, `styles/theme/defaultTheme.js`)
- No test suite exists
- `.env.example` is empty despite config referencing env vars

### Recent / last major work
- Based on the codebase state, recent work appears to have focused on:
  - Browser extension integration (localhost request routing)
  - Import/Export collection feature
  - Notes notebook feature
  - Landing page with developer portfolio section
  - Response panel search functionality

---

## 8. Conventions

### Coding patterns
- **Client components:** All interactive components use `"use client"` directive
- **Server components:** Landing page (`app/page.tsx`) is a server component that fetches data
- **State management:** React Context only (no external state library)
- **Styling:** Tailwind CSS utility classes exclusively. No CSS modules or styled-components for new code.
- **Portals:** Modals use `createPortal(...)` to render into `document.body`
- **Immutability:** All collection mutations return new arrays/objects (no mutation)
- **URL-driven selection:** Route selection is encoded in URL search params, making it shareable/bookmarkable

### Naming conventions
- **Files:** PascalCase for components (`Selector.tsx`), camelCase for utilities (`curlConverter.ts`)
- **Types:** `T` prefix for types (`THittableCollections`, `TResponseJson`), no prefix for interfaces
- **Exports:** Named exports for components and utilities, barrel `index.ts` files for re-exports
- **Components:** Default exports for page/layout components, named exports for shared UI
- **Hooks:** `use` prefix (`useKeypress`, `useExtension`, `useNotification`)
- **Path alias:** `@/*` maps to project root (e.g., `@/components`, `@/utils`, `@/types`)

### Decisions worth preserving
- **Chose Next.js App Router** over Pages Router for server components and modern routing
- **Chose React Context** over Redux/Zustand because the state tree is simple (collections + selection)
- **Chose server-side proxy** over client-side CORS bypass for reliability and security
- **Chose browser extension** for localhost requests because service workers can bypass CORS natively
- **Chose pako compression** for import/export to keep shared strings manageable in length
- **Chose URL params** for selection state to enable bookmarking and browser history navigation
- **Chose `<<KEY>>` syntax** for environment variables (double angle brackets) to avoid conflicts with template literals
- **No dark/light toggle** — the app is intentionally dark-only to match the developer tool aesthetic
- **Chose Geist fonts** (sans + mono) for a modern, technical feel consistent with Vercel's ecosystem
