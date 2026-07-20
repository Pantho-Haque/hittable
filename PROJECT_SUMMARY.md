# PROJECT_SUMMARY.md — Hittable

## 1. Project Overview

**What it does:** Hittable is a lightweight, open-source HTTP API client that runs entirely in the browser. It's designed as a developer-first alternative to Postman/Insomnia, focusing on speed, transparency, and DX (developer experience). Users can create collections of API routes (organized in nested folders), send HTTP requests via a CORS-bypassing proxy or browser extension, and view responses in real-time.

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
│   │   ├── Selector.tsx          # Left sidebar: drill-down navigation with breadcrumbs
│   │   ├── RequestForm.tsx       # Main form orchestrator (UrlBar + TabEditor + ResponsePanel)
│   │   ├── Menu.tsx              # Context menu for collection/route/folder actions
│   │   ├── HistoryPanel.tsx      # Request history modal
│   │   └── SyntaxHighlighter.tsx # Code syntax highlighting
│   ├── RequestForm/              # Request building components
│   │   ├── UrlBar.tsx            # URL input, method selector, send/save/copy buttons
│   │   ├── TabEditor.tsx         # Params/Body/Headers tabs with JSON/table modes
│   │   ├── ResponsePanel.tsx     # Response display with tabs, search, raw view toggle
│   │   └── ResponsePanelComponents/
│   │       ├── JsonNode.tsx      # Recursive JSON tree renderer
│   │       ├── FloatingSearch.tsx # Floating search bar for response
│   │       ├── Highlight.tsx     # Text match highlighting
│   │       ├── CopyButton.tsx    # Copy JSON to clipboard
│   │       ├── MatchContext.tsx  # React context for match registry
│   │       ├── HtmlPreview.tsx   # Sandboxed HTML preview with base href injection and inspect mode
│   │       ├── HtmlSourceViewer.tsx # Line-numbered, syntax-highlighted HTML source with code folding
│   │       └── ElementInspector.tsx # Element info panel for inspect mode
│   ├── modals/                   # All modal dialogs
│   │   ├── CreateModal.tsx       # Create collection/route/folder
│   │   ├── RenameModal.tsx       # Rename collection/route/folder
│   │   ├── DeleteModal.tsx       # Delete collection/route/folder
│   │   ├── ImportModal.tsx       # Import from Hittable/Postman/Insomnia formats
│   │   ├── ExportModal.tsx       # Export to Hittable/Postman/Insomnia formats
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
│   ├── Hittable.ts               # GetHittableCollections (localStorage + migration), GetResume
│   └── index.ts                  # Re-exports
├── stores/                       # Zustand stores (minimal, only auth.ts)
│   └── auth.ts                   # Auth store (minimal, not heavily used)
├── utils/                        # Pure utility functions
│   ├── curlConverter.ts          # curl ↔ JSON bidirectional conversion
│   ├── hittableProxy.ts          # Request routing logic (extension vs proxy)
│   ├── hittableCollectionModifier.ts # Collection CRUD (tree-aware)
│   ├── treeHelpers.ts            # Tree traversal, modification, migration
│   ├── responsePanelUtils.ts     # JSON search, URL param helpers
│   ├── formatJson.ts             # JSON pretty-printing
│   ├── compressString.ts         # pako compression for import/export
│   ├── historyModifier.ts        # Request history CRUD
│   ├── noteModifier.ts           # Notes CRUD
│   ├── JsonStringParsing.ts      # Parse response strings to JSON
│   ├── cn.ts                     # clsx + tailwind-merge utility
│   └── importers/                # Format-specific import/export
│       ├── postmanImporter.ts    # Postman Collection v2.1 → Hittable
│       ├── postmanExporter.ts    # Hittable → Postman Collection v2.1
│       ├── insomniaImporter.ts   # Insomnia export → Hittable
│       └── insomniaExporter.ts   # Hittable → Insomnia export
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
- `"hittable"` → `THittableCollections` (collections with nested folder tree, routes, responses, env vars)
- `"notesStore"` → `NotesStore` (markdown notes)
- `"hittable_history"` → `THistory` (request history, max 100 entries)
- `"hittable_sidebar_collapsed"` → `boolean` (sidebar UI state)

No server-side database. No user authentication for the app itself (the resume API call is external).

---

## 3. Data Model

### Nested Folder Tree (Current)

Collections contain a tree of items — routes and folders at any nesting depth:

```typescript
type THittableRoute = {
  type: "route";
  name: string;
  curl: string;        // curl command string (the interchange format)
  response: string;    // last response JSON (serialized)
};

type THittableFolder = {
  type: "folder";
  name: string;
  items: THittableItem[];  // recursive: can contain routes OR sub-folders
};

type THittableItem = THittableRoute | THittableFolder;

type THittableCollection = {
  collectionName: string;
  items: THittableItem[];  // top-level items (routes and/or folders)
  env: Record<string, string>;  // environment variables for this collection
};
```

### Legacy Format (Auto-Migrated)

Old format stored routes as a flat `curls` array with "/"-joined names (e.g. "Users/Create"). On load, `migrateCollections()` detects this and converts to the nested tree format automatically, persisting the result.

### Key Internal Types

```typescript
type THittableCurlJson = {
  method: string;   // GET, POST, PUT, PATCH, DELETE, HEAD
  url: string;      // may contain <<KEY>> env var placeholders
  headers: string;  // JSON string of key-value pairs
  body: string;     // JSON string or raw text
  params: string;   // JSON string of query params
};

type THittableSelectorSelection = {
  collectionName: string;
  folderPath: string[];  // path through folder tree, e.g. ["Users", "Admin"]
  curlName: string;      // name of selected route
};

type THittableSelectorResponse = {
  collectionName: string;
  folderPath: string[];
  curlName: string;
  env?: Record<string, string>;
  curlJson: THittableCurlJson;
  responseJson?: TResponseJson;
};

type TResponseJson = {
  data?: unknown;
  status?: number;
  statusText?: string;
  ok?: boolean;
  headers?: unknown;
  error?: string;
  cookies?: unknown;
  durationMs?: number;
  sizeBytes?: number;
} | null;
```

---

## 4. Environment Variables

### Syntax
- **Hittable native:** `<<KEY>>` — used in URL, headers, body, params
- **Postman import/export:** `{{varName}}` — translated bidirectionally
- **Insomnia import/export:** `{{ _.varName }}` — translated bidirectionally

### Storage
- Env vars live in each collection's `env` object: `{ "host": "https://api.example.com", "token": "abc123" }`
- The `<<KEY>>` placeholder is stored literally in the curl string (not resolved on save)
- Resolution happens only at send time via `resolveEnv()` in `hittableCollectionModifier.ts`

### Resolution
```typescript
// In hittableCollectionModifier.ts
function resolveEnv(formInput, env) {
  // Replaces <<KEY>> with env[KEY] in url, headers, body, params
  // Unmatched placeholders remain as <<KEY>>
}
```

---

## 5. Import/Export System

### Supported Formats

| Format | Import | Export | Variable Syntax |
|--------|--------|--------|-----------------|
| Hittable native | ✅ Auto-detected | ✅ Default | `<<KEY>>` |
| Postman Collection v2.1 | ✅ Auto-detected | ✅ | `{{varName}}` |
| Insomnia Export | ✅ Auto-detected | ✅ | `{{ _.varName }}` |

### Format Detection (Import)
- **Postman:** JSON with `info.schema` containing `schema.getpostman.com`
- **Insomnia:** JSON with `_type: "export"` or `__export_format: 4`, or resources array with `_type` fields
- **Hittable:** Compressed string (fails JSON parse) or matches `{ collectionName, items/curls }` structure

### Folder Handling
- **Postman/Insomnia imports** create real nested folder trees (not flattened)
- `Item groups` (Postman) and `request_groups` (Insomnia) become `THittableFolder` items
- **Export** serializes the real nested structure back out

### What Gets Dropped
- **Postman:** Pre-request/test scripts, advanced auth (OAuth1/OAuth2/AWS/etc.), certificates, protocol behavior
- **Insomnia:** Plugins, cookie jars, client certs, WebSocket/gRPC/socket.io, mock routes, unit tests, API specs

See `IMPORT_EXPORT_FORMATS.md` for detailed schema documentation.

---

## 6. Features Implemented

### Core Features

1. **Smart Collections with Nested Folders** — Create, rename, delete collections; each collection has its own env vars and a tree of folders and routes at arbitrary depth
2. **Route Management** — Create, rename, delete routes at any nesting depth; each route stores a curl command and optional response
3. **Folder Management** — Create, rename, delete folders; folders can contain routes and sub-folders
4. **HTTP Request Builder** — URL bar with method selector (GET/POST/PUT/PATCH/DELETE/HEAD), params/headers/body editors
5. **Body Content Types** — Supports raw/JSON, x-www-form-urlencoded, raw/Text, and multipart/form-data with file upload
6. **Table Mode Editing** — Toggle between JSON and key-value table views for params, headers, and body
7. **Response Panel** — Displays response with status code, duration (ms/s), size (B/KB/MB), JSON tree view, and raw text view
8. **Response Headers Tab** — Dedicated tab showing all response headers with per-header copy and search
9. **Deep Search** — Floating search bar (Cmd/Ctrl+F) to find text within JSON responses and headers
10. **CORS Bypass Proxy** — Server-side proxy route (`/api/proxy`) forwards requests to bypass browser CORS restrictions
11. **Browser Extension** — Chrome extension ("Hittable Companion") for localhost CORS bypass via service worker
12. **curl Integration** — Paste curl commands into URL bar for instant parsing; copy any route as curl
13. **Environment Variables** — Per-collection `<<KEY>>` placeholder syntax; resolveEnv replaces at request time
14. **Import/Export** — Import/export collections in Hittable native (compressed), Postman v2.1, or Insomnia formats
15. **Request History** — Automatically logs all sent requests with timestamp, method, URL, status, duration, size; replay with one click; max 100 entries
16. **Markdown Notes** — Rich markdown editor with 3-way view (Edit/Split/Preview); per-route documentation
17. **Auth Presets** — Modal for Bearer Token, Basic Auth, and API Key; auto-populates Authorization header
18. **Keyboard Shortcuts** — Cmd/Ctrl+Enter (Send), Cmd/Ctrl+S (Save), Cmd/Ctrl+F (Search), Cmd/Ctrl+B (Toggle Sidebar), Shift+T (New Route), Esc (Close Modal)
19. **URL-based Route Selection** — Route selection encoded in URL params (`?c=Collection&r=RouteName&p=Folder1/Folder2`) for deep linking
20. **Breadcrumb Navigation** — Clickable collection/route dropdowns in RequestForm for quick switching without sidebar
21. **Drill-down Sidebar Navigation** — Single-panel sidebar with drill-down: click collection → see its contents; click folder → drill in; back button and breadcrumb trail to navigate
22. **Method Badge Indicators** — Colored badges (GET=cyan, POST=green, PUT=orange, PATCH=purple, DELETE=red, HEAD=gray) on route rows showing HTTP method at a glance
23. **Unsaved Changes Protection** — `beforeunload` warning; visual indicator for unsaved changes; revert button
24. **Landing Page** — Marketing page with terminal demo, feature cards, keyboard shortcuts section, developer portfolio
25. **Toast Notifications** — Animated toast system (info/success/error) with position control
26. **Responsive Design** — Mobile sidebar collapse, touch-friendly targets (44px min), adaptive layouts
27. **Auto-set Content-Type** — When switching body type, Content-Type header is automatically set/updated; manual overrides preserved
28. **Legacy Data Migration** — Old flat collections with "/"-joined route names are automatically migrated to nested folder tree on load

### Partially Implemented / In-Progress

- **SyntaxHighlighter** (`components/hittable/SyntaxHighlighter.tsx`) — exists but usage is minimal; HTML source highlighting uses custom implementation
- **Stores/auth.ts** — minimal auth store, not actively used in main flow
- **HTML Inspect Mode** — Basic element inspection (tag, attributes, outer HTML) works via parent-side DOM access; advanced features like computed styles, box model, and DOM tree navigation deferred to follow-up

---

## 7. UI/UX Details

### Design System

- **Theme:** Dark-only (no light mode toggle). Background: `#080f1a` (deep navy), `#0a1628`, `#0c1a2e`, `#0e1f35`
- **Accent color:** Cyan (`#00e5cc`) — used for active states, borders, highlights
- **Method colors:** GET=`#00e5cc`, POST=`#4ade80`, PUT=`#fb923c`, PATCH=`#a78bfa`, DELETE=`#f87171`, HEAD=`#94a3b8`
- **Fonts:** Geist Sans (`--font-geist-sans`) for UI, Geist Mono (`--font-geist-mono`) for code/data
- **Styling approach:** Tailwind CSS 4 utility classes + custom CSS in `styles/components/` (buttons, modals, forms)
- **Decorative elements:** Corner bracket borders on modals and URL bar (cyan-500/30), ambient background glow blobs
- **Custom scrollbar:** Thin 4px scrollbar with cyan gradient thumb

### Sidebar Navigation (Drill-down)

The sidebar is a single panel that evolves based on navigation state:

1. **Collection List** (top level): Shows all collections. Click a collection to drill in.
2. **Inside Collection**: Shows that collection's top-level items (folders and routes). Folders shown first with folder icon and item count. Routes shown with method badge and name. Back button returns to collection list.
3. **Inside Folder**: Same pattern, one level deeper. Back button returns to parent.
4. **Breadcrumb Trail**: Always visible at top when inside a collection/folder. Shows `Collection / Folder1 / Folder2`. Each segment is clickable to jump back multiple levels.

**Interaction Pattern:** Click-to-drill (not inline expand). Clicking a folder replaces the panel content with that folder's contents.

### Method Badges

Route rows display a compact bordered chip showing the HTTP method abbreviation (GET, POST, etc.) with color matching the method:
- Border: `border-{color}40`
- Background: `bg-{color}12`
- Text: `{color}`

### Toolbar Actions

Inside a collection, the toolbar shows icon-only buttons (using `modal-button-mini` CSS class) with native `title` tooltips:
- **Env Vars** → `Settings2` icon (opens EnvModal)
- **New Route** → `Plus` icon (opens CreateModal)
- **New Folder** → `Folder` icon (creates new folder inline)

### Icon-Button Pattern (`.modal-button-mini`)

```css
.modal-button-mini {
  @apply text-white/50 hover:text-cyan-300 transition-colors p-1.5 flex items-center justify-center border border-white/8 hover:border-white/15 cursor-pointer rounded-lg;
}
```

Used by: ImportModal, AuthModal, HistoryPanel, NoteModal, InfoModal, EnvModal, CreateModal.

### Navigation/Routing

- `/` — Landing page (server component)
- `/hittable` — Main application (client component)
- `/hittable?c=CollectionName&r=RouteName&p=Folder1/Folder2` — Deep-linked route selection with folder path

### Key User Flows

1. **First visit:** Landing page → "Launch App" → `/hittable` → Empty state with "Create collection" prompt
2. **Creating a collection:** Sidebar "+" button → CreateModal → enters name → new collection appears
3. **Drilling into a collection:** Click collection name → panel shows collection contents (folders + routes)
4. **Adding a route:** Click collection → "+" icon → CreateModal → new route created at current folder level
5. **Creating a folder:** Click "New Folder" icon → folder created at current level
6. **Sending a request:** Select route → edit URL/headers/body in TabEditor → Cmd/Ctrl+Enter or click Send → response appears
7. **Saving changes:** Edit form → "Unsaved" indicator appears → Cmd/Ctrl+S → changes persisted to localStorage
8. **Importing a collection:** Sidebar import button → paste Hittable/Postman/Insomnia data → auto-detected and imported
9. **Exporting a collection:** Context menu on collection → Export → choose format (Hittable/Postman/Insomnia) → copy or download
10. **Using environment variables:** Click "Env Vars" → add key/value pairs → use `<<KEY>>` in URL/headers/body → resolved at send time
11. **Searching responses:** Click search icon or Cmd/Ctrl+F → type query → matches highlighted in JSON tree
12. **Viewing history:** Click history icon → modal shows recent requests → click entry to replay
13. **Adding auth:** Click shield icon → select preset (Bearer/Basic/API Key) → fill fields → Apply → Authorization header added
14. **Navigating back:** Click back arrow or breadcrumb segment → returns to parent level

---

## 8. Core Modules/Components

### Key Files

| File | Responsibility |
|------|---------------|
| `context/dataContext.tsx` | Central state hub — collections, form, response, history, extension status, save handler |
| `utils/hittableProxy.ts` | Request routing — decides extension vs proxy, handles env resolution |
| `app/api/proxy/route.ts` | Server-side proxy — forwards HTTP requests, parses response, extracts headers/cookies |
| `utils/curlConverter.ts` | Bidirectional curl ↔ JSON conversion using `@bany/curl-to-json` |
| `utils/hittableCollectionModifier.ts` | Collection CRUD — tree-aware create/rename/delete, env var updates |
| `utils/treeHelpers.ts` | Tree traversal, modification, migration (legacy → nested format) |
| `components/hittable/Selector.tsx` | Left sidebar — drill-down navigation, breadcrumbs, method badges, search filter |
| `components/RequestForm/UrlBar.tsx` | URL input — method selector, send/save/copy buttons, curl paste detection |
| `components/RequestForm/TabEditor.tsx` | Params/Body/Headers editors — JSON/table modes, body type selector |
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
- **`migrateCollections()`** — Detects legacy flat format and converts to nested folder tree
- **`treeHelpers.ts`** — `findRoute`, `getItemsAtPath`, `insertItem`, `removeItem`, `renameItem`, `updateRoute`, `collectAllRouteNames`

---

## 9. Configuration & Environment

### Build/Dev Scripts

```bash
npm run dev          # Start dev server with Turbopack (next dev --turbopack)
npm run build        # Production build (next build)
npm run start        # Production server (next start)
npm run lint         # ESLint (next lint)
npm run prepare      # Husky git hooks
npm run commitlint   # Commit message validation
```

### Key Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| next | 16.1.6 | Framework |
| react / react-dom | 19.2.0 | UI library |
| typescript | ^5 | Type safety |
| tailwindcss | ^4.1.11 | Styling |
| @bany/curl-to-json | ^1.2.10 | Curl parsing |
| framer-motion | ^12.23.0 | Animations |
| lucide-react | ^0.548.0 | Icons |
| marked | ^18.0.5 | Markdown rendering |
| pako | ^2.1.0 | Compression for import/export |
| @radix-ui/react-accordion | ^1.2.12 | Accordion component |

### Config Files

- `next.config.ts` — Image remote patterns, SVG allowed
- `tsconfig.json` — Strict mode, `@/*` path alias, ES2017 target
- `eslint.config.mjs` — ESLint config
- `commitlint.config.js` — Conventional commits
- `.husky/` — Git hooks directory

---

## 10. Conventions

### Coding Style

- **TypeScript strict mode** — All files use strict TypeScript
- **Client components** — Explicit `"use client"` directive at top of components using state/hooks
- **Functional components only** — No class components
- **Hooks pattern** — Custom hooks in `hooks/`, context hooks for shared state
- **Utility functions** — Pure functions in `utils/`, no side effects where possible
- **Type definitions** — All types in `types/` directory, prefixed with `T` (e.g., `THittableCollection`, `TResponseJson`)

### Naming Conventions

- **Files:** PascalCase for components (`Selector.tsx`, `UrlBar.tsx`), camelCase for utilities (`curlConverter.ts`)
- **Components:** PascalCase (`ResponsePanel`, `TabEditor`)
- **Types:** `T` prefix (`THittableCollections`, `TResponseJson`, `THistoryEntry`)
- **Constants:** UPPER_SNAKE_CASE (`HITTABLE_METHODS`, `METHOD_COLORS`)
- **Hooks:** `use` prefix (`useKeypress`, `useExtension`, `useDataContext`)
- **CSS classes:** Tailwind utilities + custom classes in `styles/components/` (`.modal-button-mini`)

### Patterns Used Consistently

- **Context + useState** for state management (no external state library for core app state)
- **ModalShell** component for all modals with consistent styling
- **`useKeypress`** hook for all keyboard shortcuts
- **`useDataContext()`** for accessing shared state (destructured at component top)
- **`useCallback`/`useMemo`** for performance-critical computations
- **URL params** for route selection (deep linking support)
- **localStorage** with try/catch for all persistence operations
- **Tree helpers** for all collection structure operations (never mutate directly)

### Key Decisions Worth Preserving

1. **No external state library** — React Context + useState, not Redux/Zustand. Keeps bundle size small.
2. **Browser-first persistence** — All data stays in localStorage. Core privacy/design principle.
3. **Dual proxy strategy** — Remote URLs go through server proxy; localhost goes through browser extension.
4. **curl as the interchange format** — Routes stored as curl strings internally. Enables easy import/export and curl compatibility.
5. **Nested folder tree** — Collections use `items: (THittableRoute | THittableFolder)[]` for arbitrary depth nesting.
6. **URL-based selection with folder path** — `?c=Collection&r=Route&p=Folder1/Folder2` enables deep linking.
7. **Tree helpers for all CRUD** — `treeHelpers.ts` handles all traversal/modification. Never mutate tree directly.
8. **Legacy auto-migration** — Old flat format auto-detected and converted on load.
9. **Env vars stored separately from curl** — `<<KEY>>` stored literally in curl string; values in collection `env` object; resolved only at send time.
10. **Icon-only buttons with tooltips** — `.modal-button-mini` pattern for toolbar actions, consistent across all modals.
11. **Method badges** — Colored bordered chips showing HTTP method on route rows, using `METHOD_COLORS` constants.
12. **Drill-down sidebar** — Single panel that evolves with navigation state, not multi-panel layout.
13. **Monospace font** — Entire app uses Geist Mono, reinforcing developer/terminal aesthetic.
