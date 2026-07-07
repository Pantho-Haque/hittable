# PROJECT_SUMMARY.md — Hittable

## 1. Project Overview

**What it does:** Hittable is a lightweight, open-source HTTP API client that runs entirely in the browser. It's designed as a developer-first alternative to Postman/Insomnia, focusing on speed, transparency, and DX (developer experience). Users can create collections of API routes, send HTTP requests via a CORS-bypassing proxy or browser extension, and view responses in real-time.

**Target user:** Software developers and API developers who want a fast, keyboard-driven API testing tool without desktop app overhead.

**Tech stack:**
- **Framework:** Next.js 16 (App Router) with React 19
- **Language:** TypeScript 5 (Strict Mode)
- **Styling:** Tailwind CSS 4 + custom CSS modules
- **State Management:** React Context + Custom Hooks (no Redux/Zustand)
- **Persistence:** Browser localStorage (all data stays client-side)
- **Icons:** Lucide React
- **Animations:** Framer Motion
- **HTTP Client:** Axios + native fetch
- **Build tools:** Turbopack (dev), pnpm

**Project type:** Web application (SPA-like), deployed to Vercel at [hittable.vercel.app](https://hittable.vercel.app)

---

## 2. Architecture

### Folder Structure

```
hittable/
├── app/                          # Next.js App Router pages
│   ├── layout.tsx                # Root layout with SEO metadata, fonts, Providers
│   ├── page.tsx                  # Landing/marketing page (server component)
│   ├── hittable/page.tsx         # Main application page (client component)
│   └── api/proxy/route.ts        # Server-side proxy endpoint for CORS bypass
├── browserExtension/             # Chrome Extension for CORS bypass
│   ├── background.js             # Service worker: fetches requests via chrome API
│   ├── content.js                # Content script: bridges page ↔ extension
│   └── manifest.json             # MV3 manifest
├── components/
│   ├── hittable/                 # Core app UI components
│   │   ├── Selector.tsx          # Left sidebar: collections & routes list
│   │   ├── RequestForm.tsx       # Main form orchestrator (UrlBar + TabEditor + ResponsePanel)
│   │   ├── Menu.tsx              # Context menu for collection/route actions
│   │   ├── HistoryPanel.tsx      # Request history modal
│   │   └── SyntaxHighlighter.tsx # Code syntax highlighting
│   ├── RequestForm/              # Request building components
│   │   ├── UrlBar.tsx            # URL input, method selector, send/save/copy buttons
│   │   ├── TabEditor.tsx         # Params/Body/Headers tabs with JSON/table modes
│   │   ├── ResponsePanel.tsx     # Response display with tabs, search, raw view
│   │   └── ResponsePanelComponents/
│   │       ├── JsonNode.tsx      # Recursive JSON tree renderer
│   │       ├── FloatingSearch.tsx # Floating search bar for response
│   │       ├── Highlight.tsx     # Text match highlighting
│   │       ├── CopyButton.tsx    # Copy JSON to clipboard
│   │       └── MatchContext.tsx  # React context for match registry
│   ├── modals/                   # All modal dialogs
│   │   ├── CreateModal.tsx       # Create collection/route
│   │   ├── RenameModal.tsx       # Rename collection/route
│   │   ├── DeleteModal.tsx       # Delete collection/route
│   │   ├── ImportModal.tsx       # Import collection from compressed string
│   │   ├── ExportModal.tsx       # Export collection to compressed string
│   │   ├── EnvModal.tsx          # Environment variables editor
│   │   ├── AuthModal.tsx         # Auth presets (Bearer/Basic/API Key)
│   │   ├── NoteModal.tsx         # Notes editor modal
│   │   ├── InfoModal.tsx         # Keyboard shortcuts & extension info
│   │   └── NoExtensionModal.tsx  # Extension not found warning
│   ├── notes/                    # Notes components
│   │   ├── NoteEditor.tsx        # Markdown editor with preview
│   │   └── NotePills.tsx         # Notes list sidebar
│   ├── ui/                       # Shared UI primitives
│   │   ├── SharedModal.tsx       # ModalShell, ModalInput, ModalActions
│   │   └── Accordion.tsx         # Radix UI accordion
│   └── homepage/                 # Landing page components
│       ├── TopBar.tsx            # Navigation bar
│       ├── PortfolioSection.tsx  # Developer portfolio section
│       └── TerminalDemo.tsx      # Animated terminal demo
├── context/                      # React Context providers
│   ├── providers.tsx             # Root provider composition
│   ├── ClientProviders.tsx       # Client-side providers (React Query, Data, Shortcuts)
│   ├── dataContext.tsx           # Main app state (collections, form, response, history)
│   ├── notifyContext.tsx         # Toast notification system
│   └── ShortcutKeypressProvider.tsx # Keyboard shortcut state
├── hooks/                        # Custom React hooks
│   ├── useExtension.ts           # Detect browser extension availability
│   ├── useKeypress.ts            # Global keyboard shortcut handler
│   └── useNotify.ts              # Toast notification convenience hook
├── services/                     # API/data fetching
│   ├── Hittable.ts               # GetHittableCollections (localStorage), GetResume (external API)
│   └── index.ts                  # Re-exports
├── stores/                       # Zustand stores (minimal, only auth.ts)
│   └── auth.ts                   # Auth store (minimal, not heavily used)
├── utils/                        # Pure utility functions
│   ├── curlConverter.ts          # curl ↔ JSON bidirectional conversion
│   ├── hittableProxy.ts          # Request routing logic (extension vs proxy)
│   ├── hittableCollectionModifier.ts # Collection CRUD operations
│   ├── responsePanelUtils.ts     # JSON search, URL param helpers
│   ├── formatJson.ts             # JSON pretty-printing
│   ├── compressString.ts         # pako compression for import/export
│   ├── historyModifier.ts        # Request history CRUD
│   ├── noteModifier.ts           # Notes CRUD
│   ├── JsonStringParsing.ts      # Parse response strings to JSON
│   └── cn.ts                     # clsx + tailwind-merge utility
├── types/                        # TypeScript type definitions
│   ├── hittable.ts               # Core types (collections, requests, responses)
│   ├── note.ts                   # Note types
│   ├── notification.ts           # Notification types
│   └── index.ts                  # Re-exports
├── constants/                    # App constants
│   ├── hittable.ts               # HTTP methods, method colors, default collections
│   ├── landing.ts                # Landing page content
│   └── misc.ts                   # Miscellaneous constants
├── styles/                       # CSS files
│   ├── app.css                   # Global styles, scrollbar, note preview styles
│   ├── font.css                  # Font declarations
│   ├── variables.css             # CSS custom properties
│   └── components/               # Component-specific CSS (buttons, modals, forms, etc.)
└── public/                       # Static assets
```

### Entry Points

- **Landing page:** `app/page.tsx` — Server component, fetches resume data, renders marketing content
- **Main app:** `app/hittable/page.tsx` — Client component, orchestrates Selector + RequestForm + tool sidebar
- **API proxy:** `app/api/proxy/route.ts` — Server-side POST handler that proxies HTTP requests to bypass CORS

### Data Flow

1. **State Management:** Single `DataContext` holds all app state (collections, form inputs, response, history, extension status)
2. **Persistence:** Collections auto-save to `localStorage["hittable"]` on every change via `useEffect`. Notes persist to `localStorage["notesStore"]`. History persists to `localStorage["hittable_history"]`. Sidebar state persists to `localStorage["hittable_sidebar_collapsed"]`
3. **Request Flow:**
   - User edits form → `formInput` state updates
   - User clicks Send → `hittableProxy()` determines routing:
     - If localhost + extension available → `fetchViaExtension()` (postMessage to content script → background script)
     - If remote URL → `fetchViaProxy()` (POST to `/api/proxy` route)
     - If localhost + no extension → throws error
   - Response updates `proxyResponse` state → displayed in `ResponsePanel`
   - Request logged to `history` state → persisted to localStorage

### Persistence Layer

All data persistence is via browser `localStorage`:
- `"hittable"` → `THittableCollections` (collections with routes and responses)
- `"notesStore"` → `NotesStore` (markdown notes)
- `"hittable_history"` → `THistory` (request history, max 100 entries)
- `"hittable_sidebar_collapsed"` → `boolean` (sidebar UI state)

No server-side database. No user authentication for the app itself (the resume API call is external).

---

## 3. Features Implemented

### Core Features

1. **Smart Collections** — Create, rename, delete collections; each collection has its own environment variables and list of routes
2. **Route Management** — Create, rename, delete routes within collections; each route stores a curl command and optional response
3. **HTTP Request Builder** — URL bar with method selector (GET/POST/PUT/PATCH/DELETE/HEAD), params/headers/body editors
4. **Body Content Types** — Supports raw/JSON, x-www-form-urlencoded, raw/Text, and multipart/form-data with file upload
5. **Table Mode Editing** — Toggle between JSON and key-value table views for params, headers, and body
6. **Response Panel** — Displays response with status code, duration (ms/s), size (B/KB/MB), JSON tree view, and raw text view
7. **Response Headers Tab** — Dedicated tab showing all response headers with per-header copy
8. **Deep Search** — Floating search bar (Cmd/Ctrl+F) to find text within JSON responses and headers
9. **CORS Bypass Proxy** — Server-side proxy route (`/api/proxy`) forwards requests to bypass browser CORS restrictions
10. **Browser Extension** — Chrome extension ("Hittable Companion") for localhost CORS bypass via service worker
11. **curl Integration** — Paste curl commands into URL bar for instant parsing; copy any route as curl
12. **Environment Variables** — Per-collection `<<KEY>>` placeholder syntax; resolveEnv replaces at request time
13. **Import/Export** — Export collections as pako-compressed base64 strings; import by pasting code
14. **Request History** — Automatically logs all sent requests with timestamp, method, URL, status, duration, size; replay with one click; max 100 entries
15. **Markdown Notes** — Rich markdown editor with 3-way view (Edit/Split/Preview); per-route documentation
16. **Auth Presets** — Modal for Bearer Token, Basic Auth, and API Key; auto-populates Authorization header
17. **Keyboard Shortcuts** — Cmd/Ctrl+Enter (Send), Cmd/Ctrl+S (Save), Cmd/Ctrl+F (Search), Cmd/Ctrl+B (Toggle Sidebar), Shift+T (New Route), Esc (Close Modal)
18. **URL-based Route Selection** — Route selection encoded in URL params (`?c=CollectionName&r=RouteName`) for deep linking
19. **Breadcrumb Navigation** — Clickable collection/route dropdowns in RequestForm for quick switching without sidebar
20. **Unsaved Changes Protection** — `beforeunload` warning; visual indicator for unsaved changes
21. **Landing Page** — Marketing page with terminal demo, feature cards, keyboard shortcuts section, developer portfolio
22. **Toast Notifications** — Animated toast system (info/success/error) with position control
23. **Responsive Design** — Mobile sidebar collapse, touch-friendly targets (44px min), adaptive layouts

### Partially Implemented / In-Progress

- **SyntaxHighlighter** (`components/hittable/SyntaxHighlighter.tsx`) — exists but usage is minimal
- **Stores/auth.ts** — minimal auth store, not actively used in main flow
- **`check-user-permission.js`** — utility file, appears unused in main app
- **`apiRequest.js`** — utility file, appears unused (legacy)

### Known Issues (from NEW_FEATURE_IDEAS.md)

**Critical bugs still present:**
- `JSON.parse` in `ImportModal` and `ExportModal` can still crash on malformed data
- `set-cookie` parsing in proxy route splits on commas incorrectly (RFC 6265 issue)
- No fetch timeout on upstream requests in proxy route
- `curlConverter` produces `[object Object]` for nested header values
- `valueMatchesSearch`/`countMatches` can stack overflow on deeply nested JSON
- `decompressString` has no input validation
- Curl paste handler has 1-second setTimeout race condition

**Accessibility gaps:**
- TabEditor tabs lack ARIA tab pattern (`role="tablist"`)
- Icon-only buttons in UrlBar lack `aria-label`
- `PanelItem` in Selector uses `<div onClick>` — not keyboard-focusable

---

## 4. Core Modules/Components

### Key Files

| File | Responsibility |
|------|---------------|
| `context/dataContext.tsx` | Central state hub — collections, form, response, history, extension status, save handler |
| `utils/hittableProxy.ts` | Request routing — decides extension vs proxy, handles multipart, env resolution |
| `app/api/proxy/route.ts` | Server-side proxy — forwards HTTP requests, parses response, extracts headers/cookies |
| `utils/curlConverter.ts` | Bidirectional curl ↔ JSON conversion using `@bany/curl-to-json` |
| `utils/hittableCollectionModifier.ts` | Collection CRUD — create/rename/delete collections and routes, update env vars |
| `components/hittable/Selector.tsx` | Left sidebar — collections list, routes list, URL-based selection, keyboard shortcuts |
| `components/RequestForm/UrlBar.tsx` | URL input — method selector, send/save/copy buttons, curl paste detection |
| `components/RequestForm/TabEditor.tsx` | Params/Body/Headers editors — JSON/table modes, body type selector, multipart support |
| `components/RequestForm/ResponsePanel.tsx` | Response display — status/duration/size metrics, tabs, search, raw view toggle |
| `components/ui/SharedModal.tsx` | ModalShell — reusable modal with focus trap, ARIA attributes, portal rendering |

### Reusable Components/Hooks

| Component/Hook | Used In |
|----------------|---------|
| `ModalShell` | All modals (Import, Export, Auth, Note, Info, Create, Rename, Delete, Env, NoExtension) |
| `ModalActions` | All confirm/cancel dialogs |
| `useKeypress` | UrlBar (Send, Save), Selector (New Route), ResponsePanel (Search), EnvModal (Save) |
| `useExtension` | DataContext — detects browser extension availability |
| `useNotification` | ImportModal, NoteModal — toast feedback |
| `useDataContext` | Nearly every component — access to shared state |
| `useShortcuts` | Hittable page, Selector — sidebar toggle state |
| `JsonNode` | ResponsePanel — recursive JSON tree rendering |
| `Highlight` | JsonNode, FloatingSearch — text match highlighting |

### Custom Logic

- **`resolveEnv()`** — Replaces `<<KEY>>` placeholders in form inputs with environment variable values using regex `<<(\w+)>>`
- **`hittableProxy()`** — Routing decision tree: localhost + extension → postMessage; remote → server proxy; localhost + no extension → error
- **`compressString()`/`decompressString()`** — pako deflate/inflate with URL-safe base64 encoding for collection import/export
- **`curlConverter()`** — Parses curl strings using `@bany/curl-to-json`, shields `<<VAR>>` placeholders during parsing, extracts params from URL
- **`jsonToCurl()`** — Serializes form input back to curl command, escapes quotes in body/headers
- **`countMatches()`/`valueMatchesSearch()`** — Recursive JSON search for deep search feature

---

## 5. UI/UX Details

### Design System

- **Theme:** Dark-only (no light mode toggle). Background: `#080f1a` (deep navy), `#0a1628`, `#0c1a2e`, `#0e1f35`
- **Accent color:** Cyan (`#00e5cc`) — used for active states, borders, highlights
- **Method colors:** GET=`#00e5cc`, POST=`#4ade80`, PUT=`#fb923c`, PATCH=`#a78bfa`, DELETE=`#f87171`, HEAD=`#94a3b8`
- **Fonts:** Geist Sans (`--font-geist-sans`) for UI, Geist Mono (`--font-geist-mono`) for code/data
- **Styling approach:** Tailwind CSS 4 utility classes + custom CSS in `styles/components/` (buttons, modals, forms)
- **Decorative elements:** Corner bracket borders on modals and URL bar (cyan-500/30), ambient background glow blobs
- **Custom scrollbar:** Thin 4px scrollbar with cyan gradient thumb

### Navigation/Routing

- `/` — Landing page (server component)
- `/hittable` — Main application (client component)
- `/hittable?c=CollectionName&r=RouteName` — Deep-linked route selection

### Key User Flows

1. **First visit:** Landing page → "Launch App" → `/hittable` → Empty state with "Create collection" prompt
2. **Creating a collection:** Sidebar "+" button → CreateModal → enters name → new collection appears in sidebar
3. **Adding a route:** Click collection → Routes panel appears → "+" button → CreateModal → new route
4. **Sending a request:** Select route → edit URL/headers/body in TabEditor → Cmd/Ctrl+Enter or click Send → response appears in ResponsePanel
5. **Saving changes:** Edit form → "Unsaved" indicator appears → Cmd/Ctrl+S → changes persisted to localStorage and collection
6. **Importing a collection:** Sidebar import button → paste compressed code → collection added with unique name
7. **Using environment variables:** Click "Env Vars" → add key/value pairs → use `<<KEY>>` in URL/headers/body → resolved at send time
8. **Searching responses:** Click search icon or Cmd/Ctrl+F → type query → matches highlighted in JSON tree → navigate with arrows
9. **Viewing history:** Click history icon → modal shows recent requests → click entry to replay
10. **Adding auth:** Click shield icon → select preset (Bearer/Basic/API Key) → fill fields → Apply → Authorization header added

---

## 6. Configuration & Environment

### Build/Dev Scripts

```bash
npm run dev          # Start dev server with Turbopack (next dev --turbopack)
npm run build        # Production build (next build)
npm run start        # Start production server (next start)
npm run lint         # Run ESLint (next lint)
npm run prepare      # Set up Husky git hooks
npm run commitlint   # Validate commit messages
```

### Environment Variables

- `.env.example` — exists (template for env vars)
- No runtime environment variables needed for the app itself (all data is client-side)
- The proxy route has no env-based configuration

### Key Dependencies (from package.json)

| Package | Version | Purpose |
|---------|---------|---------|
| next | 16.1.6 | Framework |
| react / react-dom | 19.2.0 | UI library |
| typescript | ^5 | Type safety |
| tailwindcss | ^4.1.11 | Styling |
| @bany/curl-to-json | ^1.2.10 | Curl parsing |
| axios | ^1.13.3 | HTTP client |
| framer-motion | ^12.23.0 | Animations |
| lucide-react | ^0.548.0 | Icons |
| marked | ^18.0.5 | Markdown rendering |
| pako | ^2.1.0 | Compression for import/export |
| @tanstack/react-query | ^5.81.5 | Server state (used minimally) |
| @radix-ui/react-accordion | ^1.2.12 | Accordion component |
| class-variance-authority | ^0.7.1 | Component variants |
| clsx + tailwind-merge | — | Class name utilities |
| husky | ^9.1.7 | Git hooks |
| commitlint | ^19.0.0 | Commit message linting |
| lint-staged | ^15.2.0 | Pre-commit lint |

### Config Files

- `next.config.ts` — Image remote patterns, SVG allowed
- `tsconfig.json` — Strict mode, `@/*` path alias, ES2017 target
- `eslint.config.mjs` — ESLint config
- `postcss.config.mjs` — PostCSS with Tailwind plugin
- `commitlint.config.js` — Conventional commits
- `.husky/` — Git hooks directory
- `vercel.json` — Vercel deployment config
- `docker-compose.yml` / `Dockerfile` / `dockerEntryPoint.sh` — Docker support (exists but secondary to Vercel)

---

## 7. Current State

### What's Fully Working

- Collection and route CRUD (create, rename, delete)
- HTTP request sending via proxy and browser extension
- Response display with JSON tree, raw view, headers tab, search
- curl paste and copy
- Environment variables with `<<KEY>>` syntax
- Import/export collections (compressed format)
- Request history with replay
- Markdown notes with split editor
- Auth presets (Bearer, Basic, API Key)
- Body type selection (JSON, form-urlencoded, text, multipart with file upload)
- Keyboard shortcuts throughout
- Mobile responsive layout
- ARIA accessibility attributes on modals
- Unsaved changes protection
- URL-based route selection

### What's Still Being Iterated On

- The `NEW_FEATURE_IDEAS.md` contains a prioritized roadmap of 16 features
- Several bug fixes from the audit are still pending (see "Known Issues" above)
- The stores/auth.ts file exists but isn't actively used

### Recent Changes (from CHANGELOG.md)

The most recent work focused on:
- Response time/size metrics in ResponsePanel
- Response headers tab
- Raw view toggle
- Auth presets modal
- Request history system
- Markdown notes editor with split view
- Table mode for body/headers/params
- Body content type selector (JSON, form-urlencoded, text, multipart)
- Breadcrumb-based route switching
- Comprehensive bug fixes (crash prevention, encoding, accessibility)
- Mobile responsive improvements

### Next Planned Steps (from README.md roadmap)

- [ ] Response History (partially done — history exists but not full response storage)
- [ ] Auth presets (OAuth2, AWS Signature) — basic presets done, advanced ones pending
- [ ] OpenAPI/Swagger import

---

## 8. Conventions

### Coding Style

- **TypeScript strict mode** — All files use strict TypeScript
- **Client components** — Explicit `"use client"` directive at top of components using state/hooks
- **Functional components only** — No class components
- **Hooks pattern** — Custom hooks in `hooks/` directory, context hooks for shared state
- **Utility functions** — Pure functions in `utils/`, no side effects where possible
- **Type definitions** — All types in `types/` directory, prefixed with `T` (e.g., `THittableCollection`, `TResponseJson`)

### Naming Conventions

- **Files:** PascalCase for components (`Selector.tsx`, `UrlBar.tsx`), camelCase for utilities (`curlConverter.ts`)
- **Components:** PascalCase (`ResponsePanel`, `TabEditor`)
- **Types:** `T` prefix (`THittableCollections`, `TResponseJson`, `THistoryEntry`)
- **Constants:** UPPER_SNAKE_CASE (`HITTABLE_METHODS`, `METHOD_COLORS`)
- **Hooks:** `use` prefix (`useKeypress`, `useExtension`, `useDataContext`)
- **CSS classes:** Tailwind utilities + custom classes in `styles/components/` (`.modal-button-mini`, `.btn-primary`)

### Patterns Used Consistently

- **Context + useState** for state management (no external state library for core app state)
- **ModalShell** component for all modals with consistent styling
- **`useKeypress`** hook for all keyboard shortcuts
- **`useDataContext()`** for accessing shared state (destructured at component top)
- **`useCallback`/`useMemo`** for performance-critical computations
- **URL params** for route selection (deep linking support)
- **localStorage** with try/catch for all persistence operations
- **Error boundaries** at page level (`app/error.tsx`)

### Key Decisions Worth Preserving

1. **No external state library** — The app uses React Context + useState, not Redux/Zustand. This keeps bundle size small but means DataContext can cause re-renders. This was a deliberate choice for simplicity.
2. **Browser-first persistence** — All data stays in localStorage. No server-side storage. This is a core privacy/design principle.
3. **Dual proxy strategy** — Remote URLs go through server proxy; localhost goes through browser extension. This avoids needing a full proxy server while handling CORS.
4. **curl as the interchange format** — Routes are stored as curl strings internally, enabling easy import/export and curl compatibility.
5. **pako compression for import/export** — Collections are compressed to URL-safe base64 strings for easy sharing.
6. **URL-based selection** — Route selection is encoded in URL params, enabling deep linking and browser back/forward navigation.
7. **Tailwind + custom CSS hybrid** — Most styling is Tailwind utilities, but component-specific styles (buttons, modals) are in CSS files using `@apply`.
8. **Monospace font for the app** — The entire Hittable app uses Geist Mono, reinforcing the developer/terminal aesthetic.
