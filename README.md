<p align="center">
  <img src="docs/media/intro.png" alt="Hittable — a terminal API client · .hit files, shared with the web app" width="820">
</p>

# <p align="center">📦 Hittable</p>

<p align="center">
  <strong>A developer-first HTTP client built for speed, transparency, and DX.</strong><br>
  No accounts, no bloat. Just fire requests.
</p>

<p align="center">
  <a href="https://nextjs.org"><img src="https://img.shields.io/badge/Next.js-16-black?style=for-the-badge&logo=next.js" alt="Next.js"></a>
  <a href="https://www.typescriptlang.org"><img src="https://img.shields.io/badge/TypeScript-5-3178c6?style=for-the-badge&logo=typescript" alt="TypeScript"></a>
  <a href="https://tailwindcss.com"><img src="https://img.shields.io/badge/Tailwind-4-38bdf8?style=for-the-badge&logo=tailwindcss" alt="Tailwind"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-00e5cc?style=for-the-badge" alt="License"></a>
</p>

---

### 🚀 Live at [hittable.vercel.app](https://hittable.vercel.app)

## What is Hittable?

Hittable is a lightweight, open-source HTTP API client that runs entirely in your browser. It hits the "sweet spot" between simplified online testers and heavy desktop apps like Postman.

### 🆕 Latest Upgrades (May 2026)
- **Searchable Response Panel**: Find text within large JSON responses instantly using the new floating search bar.
- **Route Documentation**: Add context to your requests with a built-in markdown Notes Editor.
- **Intelligent UI Indicators**: Real-time visual feedback for unsaved changes (`Ctrl/Cmd+S`) and request errors.
- **Hittable Companion Extension**: A rebuilt browser extension proxy that seamlessly bypasses CORS for all remote APIs.
- **Redesigned Landing Experience**: Added a live terminal demo and professional portfolio integration.
- **Codebase Refactor**: Highly modularized component architecture for better performance and maintainability.

### Why Hittable?
- **Zero Install**: Lives in your browser, fully persistent to `localStorage`.
- **CORS Bypass**: Leverage the Hittable Companion extension to bypass CORS restrictions without a proxy server.
- **Keyboard First**: Every major action has a shortcut. Stop reaching for the mouse.
- **Privacy Centric**: Your data never leaves your browser. No cloud syncing of sensitive API keys.

---

## ✨ Key Features

- **📂 Smart Collections**: Group routes into logical folders. Rename, duplicate, or delete them with an intuitive context menu.
- **🔄 Import/Export**: Export your collections and share configurations with your team seamlessly.
- **🌐 Environment Variables**: Define `<<KEY>>` placeholders to easily switch between environments like `localhost` and `production`.
- **⚡ curl Integration**: 
  - Paste a raw `curl` command into the URL bar for instant parsing
  - Copy any route as a `curl` command with one click
- **🔗 Browser Extension**: The Hittable Companion extension automatically detects your app and proxies requests to avoid CORS.
- **🖱️ Clickable Links**: URLs inside JSON responses are fully clickable.
- **📝 Route Notes**: Write rich markdown notes to document API parameters and expected behaviors.
- **🔍 Deep Search**: Find exactly what you need in massive API payloads with floating text match highlights.
- **⌨️ Pro Shortcuts**: `Cmd/Ctrl + Enter` (Send), `Cmd/Ctrl + S` (Save), `Shift + T` (New Route), `Cmd/Ctrl + F` (Search).
- **🛡️ Type Safe**: Built with strict TypeScript for maximum reliability and a bulletproof developer experience.

---

## hittable.sh — the same thing in your terminal

`hittable.sh` is a terminal (TUI) API client written in Go. It opens a project directory,
shows the whole tree, and runs the requests inside it. It reads and writes the same plain
`.hit` JSON files and the same `hittable/env.json` as the web app, so one folder opens in
either — edit a request in the browser, send it from the shell, commit the file.

```sh
curl -fsSL https://raw.githubusercontent.com/Pantho-Haque/hittable/main/install.sh | sh
```

![The explorer: the whole project as one tree, single click opens, git status colours every row](docs/media/s1.png)

Beyond requests it is a small editor: syntax highlighting, folding, local autocompletion,
markdown preview, fuzzy file find and live grep, and an integrated shell on a real PTY.

![The welcome screen beside the file tree, listing the shell commands: init, import, export, uninstall](docs/media/still-term.png)

`ctrl+r` sends. The response panel shows status, timing and size, with the body
highlighted and searchable.

![Response panel: 200 OK, 199ms, 421B, with the JSON body highlighted](docs/media/s3.png)

There is a full Git panel — status, commits, branches, stashes, blame — with click-to-stage
rows and a diff pane that does inline or side-by-side.

![The Git panel showing four changed files and the diff of a new .hit file](docs/media/s2.png)

Press `c` in the Git panel and it drafts the commit message. That part is opt-in and
entirely local: nothing is downloaded until you run `hittable model enable`, and nothing
leaves the machine after that.

**[Full documentation → `shellapp/README.md`](shellapp/README.md)**

---

## 🛠 Tech Stack

| Component | Choice |
| :--- | :--- |
| **Framework** | Next.js 15 (App Router) |
| **Language** | TypeScript 5 (Strict Mode) |
| **Styling** | Vanilla CSS + Tailwind CSS 4 |
| **Icons** | Lucide React |
| **State Management** | React Context + Custom Hooks |
| **Persistence** | Browser `localStorage` |

---

## 🏗 Project Structure

```bash
hittable/
├── app/
│   ├── layout.tsx         # Global layout with SEO & Topbar
│   ├── page.tsx           # High-conversion landing page
│   └── hittable/          # Main application engine
│       └── page.tsx       # State orchestrator
├── browserExtension/      # Hittable Companion Chrome Extension
│   ├── background.js      # CORS bypass proxy
│   └── content.js         # DOM injection & detection
├── components/
│   ├── homepage/          # Landing page (PortfolioSection, TerminalDemo)
│   ├── hittable/          # Core App UI (RequestForm, Selector, Menu)
│   ├── RequestForm/       # Request internals (UrlBar, TabEditor, ResponsePanel)
│   ├── notes/             # Markdown notes integration
│   └── ui/                # Shared modals and micro-components
├── utils/                 # Parsers (curl, JSON) and state modifiers
└── hooks/                 # Custom React hooks (useExtension, etc.)
```

---

## 🏁 Getting Started

### 1. Clone & Install
```bash
git clone https://github.com/Pantho-Haque/hittable.git
cd hittable
npm install
```

### 2. Run Development
```bash
npm run dev
```

### 3. Deploy
One-click deployment to Vercel:

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https://github.com/Pantho-Haque/hittable)

---

## ⌨️ Keyboard Shortcuts

| Shortcut | Action |
| :--- | :--- |
| `Ctrl/Cmd + Enter` | **Send Request** |
| `Ctrl/Cmd + S` | **Save to Collection** |
| `Ctrl/Cmd + J` | **Beautify JSON** |
| `Ctrl/Cmd + F` | **Search Response Payload** |
| `Ctrl/Cmd + B` | **Toggle Sidebar** |
| `Shift + T` | **Create New Untitled Route** |
| `Esc` | **Close Modal** |
| `Enter` (in modal) | **Confirm Action** |

> 💡 Press the **Info button** in the app to view the complete list of keybindings and extension instructions.

---

## 🗺 Roadmap

- [x] Collections & Route management
- [x] Environment Variables (`<<VAR>>` syntax)
- [x] curl command parsing & generation
- [x] Import/Export collections
- [x] Browser extension for CORS bypass
- [x] Payload Search & Highlighting
- [x] Route Notes & Documentation
- [x] Clickable response links
- [x] URL-based route selection
- [ ] Response History
- [ ] Auth presets (OAuth2, AWS Signature)
- [ ] OpenAPI/Swagger import

---

## 🤝 Contributing

We love contributions! Whether it's adding a feature, fixing a bug, or improving the documentation:

1. **Fork** the repo.
2. **Branch**: `git checkout -b feat/your-feature`.
3. **Commit**: Keep messages descriptive.
4. **Push**: Open a Pull Request.

---

## 📄 License

MIT © [Pantho Haque](https://github.com/Pantho-Haque)

---

<p align="center">
  Built with ❤️ for the developer community.<br>
  <strong>Don't forget to star ⭐ if you use it!</strong>
</p>