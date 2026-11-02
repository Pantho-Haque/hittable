# Hittable web v3: GitHub workspaces

The web app drops its browser-storage "Local" mode. It becomes a workspace over **your
GitHub repositories** (sign in, pick a repo, edit, review changes, commit to GitHub), with
**Open a local folder** kept as the alternative. The terminal app and the browser
extension are out of scope and are not changed.

Status: in progress. Supersedes the Local-mode parts of `docs/web-ux-requirements.md`.

## Decisions

| Topic | Decision | Why |
| :--- | :--- | :--- |
| Auth model | **GitHub OAuth App**, scope `repo read:user` | The owner's choice: a user sees all their repos straight after sign-in. |
| Auth flow | Hand-rolled OAuth web flow with `state` + PKCE S256; no Auth.js | One provider and no database. About 150 lines, and we control the refresh. |
| Token storage | **Server only.** An AES-256-GCM encrypted, `HttpOnly; Secure; SameSite=Lax` session cookie. Every GitHub call goes through `/api/github/*`. | The app renders untrusted HTML previews and sends arbitrary requests, so a write-capable token must never be readable by page JavaScript. |
| Git model | **GitHub API only**, with a working-copy overlay in IndexedDB. No isomorphic-git. | An in-browser clone needs a CORS proxy and the whole history. The overlay is what github.dev does. |
| Commit | GraphQL `createCommitOnBranch` with `expectedHeadOid` | One atomic multi-file commit, optimistic locking, and verified commits. |
| Diff view | `@codemirror/merge` | CodeMirror is already the editor. |
| Session crypto | `node:crypto` (`aes-256-gcm`) | No new dependency. |

Sources are in the appendix.

## Functional requirements

### FR-G1 Sign in / out
- **Sign in with GitHub**, from the landing page and from `/hittable`.
- Sign-in goes through `/api/github/login`, then GitHub authorize, then
  `/api/github/callback`. The callback verifies `state` and the PKCE verifier, exchanges
  the code server-side, reads `GET /user` and sets the session cookie.
- The user returns to the page they started from.
- The avatar menu shows the login, Switch repository, Manage access on GitHub, and
  **Sign out**.
- Sign out revokes the grant on GitHub (`DELETE /applications/{client_id}/grant`), then
  clears the cookie.
- If there are uncommitted changes, sign out first asks: "Keep them on this device /
  Discard".
- **AC:**
  - A forged `state` is rejected.
  - A callback without the PKCE cookie is rejected.
  - The cookie is httpOnly and its value is not readable as plaintext.
  - Sign-out calls the revoke endpoint and clears the cookie.

### FR-G2 Repository picker
- Shows every repository the token can see (`GET /user/repos`, all pages, sorted by last
  push).
- Fuzzy search, plus owner filter chips (you and your orgs).
- A **Recent** section kept on this device.
- Badges: Private, Fork, Archived (read-only), and Read-only when there is no push
  permission. Each repo shows "pushed 3 days ago".
- Footer: "Missing an organisation's repos? Grant access on GitHub →", linking to the app's
  connection settings page, since org OAuth restrictions are on by default.
- Choosing a repo opens `/hittable/gh/{owner}/{repo}` on its default branch, or
  `?ref={branch}` when one is given.
- **AC:**
  - Search narrows the list.
  - An archived repo opens read-only.
  - The last repo opened is listed under Recent.

### FR-G3 Repository workspace
- The same explorer, Runner, text and markdown editors and command palette as folder
  mode, now over the repo tree.
- Top bar: `owner/repo`, a **branch switcher** (search, switch, "Create branch from
  current") and a sync pill.
- Large trees load lazily per directory when GitHub truncates the recursive tree.
- Binary, LFS-pointer and very large files (over 1 MB) open read-only with a notice.
- **AC:**
  - Opening a `.hit` file shows the Runner.
  - Switching branch reloads the tree.
  - A 2 MB file is read-only.

### FR-G4 Working copy and git status
- Every edit autosaves to the **working copy** in IndexedDB, keyed by
  owner/repo/branch.
- Edits are never written to GitHub until a commit.
- Each file has a status:
  - **M** modified (amber);
  - **A** added (green);
  - **D** deleted (red, struck through).
- Folders with changes show a dot. Colour is never the only signal: the letter is always
  shown.
- New file, new folder, rename (recorded as D + A) and delete all work.
- A file edited back to its original content stops counting as changed.
- `hittable/env.local.json` is **never committed**: it is excluded from the changes list
  and shown with a lock badge.
- Reloading the page keeps the working copy.
- **AC:**
  - Edit, then reload: the change is still there and marked M.
  - Reverting the text clears the M.
  - Deleting a file shows D.

### FR-G5 Source Control panel
- A **Changes** list with a count badge.
- Clicking a file opens its **diff**: inline or side-by-side against the base.
- Per-file **Discard**, with confirmation. Discard all.
- Commit box:
  - the message is prefilled from the changes ("Update hittable/users/get.hit",
    "Update 3 files"), with subject and body;
  - primary button **Commit to `{branch}`**;
  - secondary **Commit to new branch & open pull request** (branch name prefilled
    `hittable/{slug}`, then PR title and body).
- Commit uses `createCommitOnBranch` with `expectedHeadOid`. On success the working copy
  clears and a toast shows the short SHA with "View on GitHub".
- **AC:**
  - Two edited files plus one deletion produce one commit containing exactly those
    three changes.
  - The PR path creates the branch, commits and opens a PR, and links to it.

### FR-G6 Remote changes and conflicts
- On focus, and at most every 60 s, the branch head is checked (ETag, which is free when
  the answer is 304).
- If the head moved:
  - with no overlapping files, the base advances silently;
  - with overlapping files, those files are marked **conflict** and the pill offers
    "Keep mine", "Take theirs" or "Compare".
- If a commit is rejected because the head moved, the same flow runs, and nothing is
  lost.
- A protected-branch rejection offers "Commit to new branch & open pull request" with
  the same changes.
- **AC:** with the head moved and an overlapping file, Commit does not overwrite; the
  file is marked conflict.

### FR-G7 Initialize Hittable in a repo
- A repo without `hittable/` shows an empty state, **Initialize Hittable here**.
- It stages `hittable/env.json` (`{}`), a sample request `hittable/requests/example.hit`
  and `hittable/.gitignore` (`env.local.json`) as changes, which the user reviews and
  commits like any other.
- **AC:** after Initialize, the three files are listed as A.

### FR-G8 Read-only and permission states
- The repo is read-only when there is no push permission, when it is archived, or when it
  is an empty repository (a message explains how to add a first commit on GitHub).
- Read-only repos still run requests and can download files, but cannot edit.
- Editing `.github/workflows/*` is blocked with an explanation.
- **AC:** with no push permission the editors are read-only and Commit is absent.

### FR-G9 Local folder
- **Open a local folder** keeps today's behaviour: File System Access, Chrome/Edge only,
  autosave to disk.
- It is offered next to the GitHub sign-in, and works without an account.
- Other browsers get an explanation.

### FR-G10 Migration off Local mode
- If the old Local-mode data (`localStorage.hittable`) is found, a one-time banner offers
  **Download your old collections**: a Postman v2.1 collection, plus `.hit` files in a
  `.zip` if that is cheap.
- **Dismiss** forgets it. Nothing else reads that storage.
- **AC:**
  - With old data present the banner shows and the download works.
  - Without old data there is no banner.

### FR-G11 Theme and landing
- The home page and the app page share one design language that ties **HTTP requests and
  version control** together.
- The landing page explains, in this order:
  1. what it is;
  2. **Sign in with GitHub** and **Open a local folder**;
  3. what the GitHub permission is used for, what is not requested, and where the token
     lives;
  4. open source, plus the terminal app.
- The personal portfolio section moves off the landing page and becomes a small "Made by"
  footer link.
- Dark theme first. Every colour is a token, and contrast meets WCAG AA.

## Non-functional requirements

- **NFR-G1 Security**
  - The token and client secret never reach the browser.
  - `state` and PKCE are checked on every sign-in.
  - Mutating `/api/github/*` calls require `Origin` = host and the header
    `X-Hittable: 1`.
  - The GitHub API proxy only allows the endpoints the app uses: no arbitrary paths.
  - `SESSION_SECRET` is at least 32 bytes, and the app refuses to start sign-in without it.
  - No secret is ever logged.
  - The `HtmlPreview` sandbox stays strict: never `allow-same-origin`, so page scripts (on by default since v3.1) run in an opaque origin. See `docs/web-ux-requirements.md`, "v3.1".
- **NFR-G2 Rate limits**
  - Blobs are fetched by SHA and cached in IndexedDB.
  - Branch-head polling uses ETags.
  - Writes are serialised.
  - A 403/429 secondary-limit response is backed off according to `retry-after`.
- **NFR-G3 Testability**
  - The GitHub base URLs come from env (`GITHUB_API_URL`, `GITHUB_OAUTH_URL`), so the e2e
    suite runs against a **fake GitHub** fixture server.
  - No test calls real GitHub.
- **NFR-G4 Accessibility and responsiveness**
  - The Round 1 rules still hold: axe with contrast on, no text under 11px, keyboard access
    everywhere, and layouts from 320px wide.
  - The Source Control panel and the repo picker are fully keyboard operable.
- **NFR-G5 Clean repo**
  - The Local-mode code, tests and docs are deleted, not hidden.
  - Unused dependencies and files are removed.
  - `shellapp/`, `browserExtension/`, `install.sh` and `Makefile` are untouched.
- **NFR-G6 Quality gates**
  - lint, typecheck, unit tests, build and the full e2e suite (desktop, WebKit, mobile)
    are green.
  - Every FR has tests.

## Server contract (`/api/github/*`)

Every handler reads the session cookie and returns `401 {error:"signed_out"}` without one.
Responses are JSON.

| Route | Method | Purpose |
| :--- | :--- | :--- |
| `/api/github/login?next=/path` | GET | Start OAuth: sets the state and PKCE cookie, then redirects to GitHub. |
| `/api/github/callback` | GET | Finish OAuth and set the session, then redirect to `next`. |
| `/api/github/logout` | POST | Revoke the grant and clear the session. |
| `/api/github/session` | GET | `{ user: {login, name, avatarUrl} } \| { user: null }` |
| `/api/github/repos` | GET | `[{fullName, owner, name, private, fork, archived, canPush, defaultBranch, pushedAt, description}]` (all pages) |
| `/api/github/repos/{o}/{r}/branches` | GET | `[{name, sha, protected}]` |
| `/api/github/repos/{o}/{r}/head?branch=b` | GET | `{sha}`. Uses a conditional request, so it is cheap. |
| `/api/github/repos/{o}/{r}/tree?sha=…` | GET | `{sha, truncated, entries:[{path, type:"blob"\|"tree", sha, size}]}` |
| `/api/github/repos/{o}/{r}/blob/{sha}` | GET | The raw bytes (`application/octet-stream`), cacheable forever. |
| `/api/github/repos/{o}/{r}/commit` | POST | `{branch, expectedHeadOid, message, additions:[{path, contents(base64)}], deletions:[{path}]}` returns `{sha, url}`. `409 {error:"head_moved"}` or `403 {error:"protected"}` on failure. |
| `/api/github/repos/{o}/{r}/branches` | POST | `{name, fromSha}` returns `{name, sha}` |
| `/api/github/repos/{o}/{r}/pulls` | POST | `{head, base, title, body}` returns `{number, url}` |
| `/api/github/repos/{o}/{r}/compare?base=&head=` | GET | `{files:[path]}`, the paths changed between two commits. |

## Appendix: sources
- OAuth web flow, PKCE and scopes: https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps ,
  https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps
- Org OAuth restrictions: https://docs.github.com/en/organizations/managing-oauth-access-to-your-organizations-data/about-oauth-app-access-restrictions
- Revoke a grant: https://docs.github.com/en/rest/apps/oauth-applications
- Repos, trees, blobs and contents: https://docs.github.com/en/rest/repos/repos ,
  https://docs.github.com/en/rest/git/trees , https://docs.github.com/en/rest/git/blobs
- `createCommitOnBranch`: https://docs.github.com/en/graphql/reference/commits
- Rate limits and conditional requests: https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
- github.dev working copy: https://docs.github.com/en/codespaces/the-githubdev-web-based-editor
- `@codemirror/merge`: https://github.com/codemirror/merge
- Why not isomorphic-git in a browser app: https://github.com/isomorphic-git/isomorphic-git
- Comparable products: Keystatic https://keystatic.com/docs/github-mode , Decap
  https://decapcms.org/docs/github-backend/ , Vercel
  https://vercel.com/kb/guide/unable-to-find-github-repository
