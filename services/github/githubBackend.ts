// A WorkspaceBackend over a GitHub branch: the base commit's tree (read
// through /api/github) plus your working copy (services/github/workingCopy.ts).
// Nothing reaches GitHub until commit().
//
// Deleted files stay in the tree, struck through with D, so a deletion can be
// seen and discarded like any other change; they open read-only.

import { LOCAL_ENV_PATH, type FileStatus, type WorkspaceBackend, type WsNode } from "@/services/workspace/backend";
import { api, GitHubError, isGitHubError, type CommitInput, type RepoMeta, type TreeEntry } from "@/utils/github/api";
import { bytesToBase64 } from "@/utils/uploadBody";
import { copyKey, defaultStore, type KVStore, type OverlayEntry, type WorkingCopy } from "./workingCopy";

export type Client = Pick<typeof api, "head" | "tree" | "blob" | "commit" | "compare" | "createBranch" | "pull" | "branches">;

export type Change = { path: string; status: Exclude<FileStatus, "C">; conflict: boolean; binary?: boolean };
/** Sizes for a diff that can't be shown as text; null on a side where the file doesn't exist. */
export type BinarySides = { original: number | null; modified: number | null };
export type RemoteMoved = { newHead: string; conflicts: string[] };
export type SyncState = { checking: boolean; error: GitHubError | null; remote: RemoteMoved | null };

export const LOCAL_ENV = LOCAL_ENV_PATH.join("/");
/** Larger blobs are refused by /blob (413), so they never load. */
const MAX_BLOB_BYTES = 4 * 1024 * 1024;
/** A commit request carries every file base64-encoded; the route refuses more (utils/server/github/commit.ts). */
export const MAX_COMMIT_BYTES = 4 * 1024 * 1024;
/** Base64 grows a file by a third, so this is the largest file one commit can carry. */
export const MAX_UPLOAD_BYTES = 3 * 1024 * 1024;
const POLL_MS = 60_000;

export const isLocalOnly = (path: string) => path === LOCAL_ENV;
export const isWorkflowPath = (path: string) => path === ".github/workflows" || path.startsWith(".github/workflows/");
export const WORKFLOW_MESSAGE =
  "Hittable doesn't change GitHub Actions workflows (.github/workflows): GitHub needs an extra permission for them. Edit this file on GitHub.";

const enc = new TextEncoder();
const dec = new TextDecoder();

/** The git blob SHA-1 of some text or bytes, to tell "edited back to the original" from a real change. */
export async function gitBlobSha(data: string | Uint8Array): Promise<string> {
  const body = typeof data === "string" ? enc.encode(data) : data;
  const head = enc.encode(`blob ${body.byteLength}\0`);
  const buf = new Uint8Array(head.byteLength + body.byteLength);
  buf.set(head);
  buf.set(body, head.byteLength);
  const digest = await crypto.subtle.digest("SHA-1", buf);
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

export function utf8ToBase64(text: string): string {
  return bytesToBase64(enc.encode(text));
}



/** The commit for an overlay: additions (base64 UTF-8) and deletions, never env.local.json. */
export function buildCommit(
  overlay: Record<string, OverlayEntry>,
  inBase: (path: string) => boolean,
  opts: { branch: string; expectedHeadOid: string; message: string },
): CommitInput {
  const additions: CommitInput["additions"] = [];
  const deletions: CommitInput["deletions"] = [];
  for (const [path, e] of Object.entries(overlay).sort(([a], [b]) => a.localeCompare(b))) {
    if (isLocalOnly(path)) continue;
    if ("content" in e) additions.push({ path, contents: utf8ToBase64(e.content) });
    else if ("bytes" in e) additions.push({ path, contents: bytesToBase64(e.bytes) });
    else if ("deleted" in e && inBase(path)) deletions.push({ path });
  }
  return { ...opts, additions, deletions };
}

const VERB = { M: "Update", A: "Add", D: "Delete" } as const;

const mb = (n: number) => (n >= 1024 * 1024 ? `${(n / 1024 / 1024).toFixed(1)} MB` : `${Math.ceil(n / 1024)} KB`);

function sameEntry(a: OverlayEntry | undefined, b: OverlayEntry): boolean {
  if (!a) return false;
  if ("bytes" in a || "bytes" in b) {
    if (!("bytes" in a && "bytes" in b) || a.bytes.length !== b.bytes.length) return false;
    return a.bytes.every((x, i) => x === b.bytes[i]);
  }
  return JSON.stringify(a) === JSON.stringify(b);
}

/** "Update hittable/users/get.hit" for one change; "Update 3 files" plus a list for more. */
export function prefillMessage(changes: Pick<Change, "path" | "status">[]): string {
  if (changes.length === 0) return "";
  if (changes.length === 1) return `${VERB[changes[0].status]} ${changes[0].path}`;
  return `Update ${changes.length} files\n\n${changes.map((c) => `- ${VERB[c.status]} ${c.path}`).join("\n")}`;
}

/** hittable/<slug of the subject>, for the pull-request branch. */
export function branchSlug(message: string): string {
  const slug = message
    .split("\n")[0]
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 40)
    .replace(/-+$/, "");
  return `hittable/${slug || "changes"}`;
}

/** name → name-2 → name-3, for a branch name that is taken. */
export function nextBranchName(name: string): string {
  const m = name.match(/^(.*)-(\d+)$/);
  return m ? `${m[1]}-${Number(m[2]) + 1}` : `${name}-2`;
}

type Base = {
  commit: string;
  files: Map<string, { sha: string; size: number }>;
  dirs: Set<string>;
  /** Directories not listed yet (GitHub truncated the recursive tree): path → tree SHA. */
  unloaded: Map<string, string>;
};

const under = (path: string, dir: string) => dir === "" || path === dir || path.startsWith(dir + "/");

function buildTree(files: Iterable<string>, dirs: Iterable<string>, unloaded: Map<string, string>): WsNode[] {
  type N = { node: WsNode; kids: Map<string, N> };
  const root = new Map<string, N>();
  const level = (parts: string[]) => {
    let kids = root;
    parts.forEach((name, i) => {
      let n = kids.get(name);
      if (!n) kids.set(name, (n = { node: { name, path: parts.slice(0, i + 1), kind: "directory" }, kids: new Map() }));
      kids = n.kids;
    });
    return kids;
  };
  for (const d of dirs) level(d.split("/"));
  for (const f of files) {
    const parts = f.split("/");
    level(parts.slice(0, -1)).set(parts.at(-1)!, { node: { name: parts.at(-1)!, path: parts, kind: "file" }, kids: new Map() });
  }
  const finish = (kids: Map<string, N>): WsNode[] =>
    [...kids.values()]
      .sort((a, b) => (a.node.kind === b.node.kind ? a.node.name.localeCompare(b.node.name) : a.node.kind === "directory" ? -1 : 1))
      .map(({ node, kids }) =>
        node.kind === "file" || unloaded.has(node.path.join("/")) ? node : { ...node, children: finish(kids) },
      );
  return finish(root);
}

export type GitHubBackend = WorkspaceBackend & {
  kind: "github";
  owner: string;
  repo: string;
  branch: string;
  meta: RepoMeta;
  status(): Record<string, FileStatus>;
  subscribe(onChange: () => void): () => void;
  baseSha(): string;
  /** Bumped on every change, for useSyncExternalStore. */
  version(): number;
  changes(): Change[];
  hasLocalEnv(): boolean;
  sync(): SyncState;
  readOnlyReason(path: string[]): string | null;
  loadDirectory(path: string[]): Promise<void>;
  /** The text at the base commit, null when the file is new (or not text). */
  readBase(path: string): Promise<string | null>;
  /** Your text, null when deleted. */
  readWorking(path: string): Promise<string | null>;
  /** Their text at the moved head, null when they deleted it. */
  readTheirs(path: string): Promise<string | null>;
  /** Byte sizes when either side is binary (no text diff), else null. */
  binarySides(path: string, theirs?: boolean): Promise<BinarySides | null>;
  discard(paths: string[]): Promise<void>;
  discardAll(): Promise<void>;
  /** Poll the branch head (throttled to once a minute unless forced). */
  checkRemote(opts?: { force?: boolean }): Promise<void>;
  keepMine(): Promise<void>;
  takeTheirs(): Promise<void>;
  commit(message: string): Promise<{ sha: string; url: string }>;
  commitToNewBranch(o: { branch: string; message: string; title: string; body: string }): Promise<{
    commit: { sha: string; url: string };
    pr: { number: number; url: string };
  }>;
  /** Creates a branch at the base commit and moves the working copy onto it. */
  createBranchFromCurrent(name: string): Promise<void>;
  initialize(): Promise<void>;
};

export const EXAMPLE_HIT =
  JSON.stringify(
    { method: "GET", url: "https://jsonplaceholder.typicode.com/todos/1", headers: {}, params: {}, body: "", response: null },
    null,
    2,
  ) + "\n";

type Options = { owner: string; repo: string; branch: string; meta: RepoMeta; client?: Client; kv?: KVStore };

export async function createGitHubBackend({ owner, repo, branch, meta, client = api, kv = defaultStore() }: Options): Promise<GitHubBackend> {
  const key = copyKey(owner, repo, branch);
  const readOnly = !meta.canPush || meta.archived;

  async function loadBase(commit: string): Promise<Base> {
    const t = await client.tree(owner, repo, commit);
    const base: Base = { commit, files: new Map(), dirs: new Set(), unloaded: new Map() };
    // Truncated: keep the top level and list each directory when it is opened.
    addEntries(base, t.truncated ? t.entries.filter((e) => !e.path.includes("/")) : t.entries, t.truncated);
    if (base.unloaded.has("hittable")) await loadDir(base, "hittable");
    return base;
  }
  function addEntries(base: Base, entries: TreeEntry[], lazy: boolean) {
    for (const e of entries) {
      if (e.type === "blob") base.files.set(e.path, { sha: e.sha, size: e.size });
      else {
        base.dirs.add(e.path);
        if (lazy) base.unloaded.set(e.path, e.sha);
      }
    }
  }
  async function loadDir(b: Base, dir: string) {
    const sha = b.unloaded.get(dir);
    if (!sha) return;
    const t = await client.tree(owner, repo, sha, dir);
    b.unloaded.delete(dir);
    addEntries(b, t.entries, true);
  }
  async function loadAllUnder(dir: string) {
    for (let d; (d = [...base.unloaded.keys()].find((p) => under(p, dir))); ) await loadDir(base, d);
  }

  // A missing branch: an empty repository has none at all.
  async function head() {
    try {
      return (await client.head(owner, repo, branch))!;
    } catch (err) {
      if (!isGitHubError(err, "not_found")) throw err;
      const branches = await client.branches(owner, repo).catch(() => null);
      if (branches && branches.length === 0) throw new GitHubError("empty_repo", 409);
      throw new GitHubError("branch_not_found", 404, `There is no branch "${branch}" in ${owner}/${repo}.`);
    }
  }

  const saved = await kv.get<WorkingCopy>("copies", key).catch(() => undefined);
  const first = await head();
  let etag = first.etag ?? undefined;
  let base = await loadBase(saved?.baseCommitSha ?? first.sha).catch(async (err) => {
    // The saved base is gone (force-pushed away): start from the head and keep the overlay.
    if (!saved || !isGitHubError(err, "not_found")) throw err;
    return loadBase(first.sha);
  });
  const overlay: Record<string, OverlayEntry> = saved?.overlay ?? {};
  let remote: (RemoteMoved & { base: Base }) | null = null;
  let checking = false;
  let syncError: GitHubError | null = null;
  let nextCheckAt = 0;
  let disposed = false;
  const lfs = new Set<string>();
  const listeners = new Set<() => void>();
  let version = 0;
  let treeCache: WsNode[] | null = null;
  let statusCache: { json: string; value: Record<string, FileStatus> } | null = null;

  // Network work that changes the base runs one at a time.
  let queue: Promise<unknown> = Promise.resolve();
  const serial = <T>(fn: () => Promise<T>): Promise<T> => {
    const run = queue.then(fn, fn);
    queue = run.catch(() => {});
    return run;
  };

  function emit(structural = true) {
    if (structural) treeCache = null;
    version++;
    for (const l of [...listeners]) l();
  }
  async function save() {
    if (disposed) return;
    try {
      if (Object.keys(overlay).length === 0) await kv.del("copies", key);
      else await kv.set("copies", key, { baseCommitSha: base.commit, overlay, updatedAt: Date.now() } satisfies WorkingCopy);
    } catch (err) {
      console.error("Could not save the working copy:", err);
    }
  }

  const k = (path: string[]) => path.join("/");
  const content = (p: string) => {
    const e = overlay[p];
    return e && "content" in e ? e.content : undefined;
  };
  /** Your text or bytes for a path, undefined when unchanged or deleted. */
  const working = (p: string): string | Uint8Array | undefined => {
    const e = overlay[p];
    return !e ? undefined : "content" in e ? e.content : "bytes" in e ? e.bytes : undefined;
  };
  const hasData = (e: OverlayEntry) => "content" in e || "bytes" in e;
  const isDeleted = (p: string) => !!overlay[p] && "deleted" in overlay[p];
  const visibleFiles = () => {
    const files = new Set(base.files.keys());
    for (const [p, e] of Object.entries(overlay)) if (hasData(e)) files.add(p);
    return files;
  };
  const isDir = (p: string) =>
    base.dirs.has(p) || (overlay[p] && "dir" in overlay[p]) || [...visibleFiles()].some((f) => f.startsWith(p + "/"));
  const exists = (p: string) => working(p) !== undefined || (base.files.has(p) && !isDeleted(p)) || isDir(p);
  const filesUnder = (p: string) => [...visibleFiles()].filter((f) => under(f, p));

  function guard(paths: string[]) {
    if (readOnly) throw new Error("This repository is read-only");
    if (paths.some(isWorkflowPath)) throw new Error(WORKFLOW_MESSAGE);
  }

  async function blob(sha: string, size: number): Promise<Uint8Array> {
    if (size > MAX_BLOB_BYTES) throw new GitHubError("too_large", 413);
    const cached = await kv.get<Uint8Array>("blobs", sha).catch(() => undefined);
    if (cached) return cached;
    const bytes = await client.blob(owner, repo, sha);
    void kv.set("blobs", sha, bytes).catch(() => {});
    return bytes;
  }
  /** Your bytes for a path, or with `from` that commit's bytes. */
  async function readBytes(p: string, from?: Base): Promise<Uint8Array> {
    const mine = from ? undefined : working(p);
    if (mine !== undefined) return typeof mine === "string" ? enc.encode(mine) : mine;
    const entry = (from ?? base).files.get(p);
    if (!entry) throw new GitHubError("not_found", 404, `${p} does not exist`);
    const bytes = await blob(entry.sha, entry.size);
    if (bytes.length < 512 && dec.decode(bytes.subarray(0, 40)).startsWith("version https://git-lfs.github.com/spec/")) lfs.add(p);
    return bytes;
  }
  const isText = (bytes: Uint8Array) => bytes.length <= 1024 * 1024 && !bytes.subarray(0, 8192).includes(0);

  /** Record your text or bytes for a path; equal to the base drops the change. */
  async function setContent(p: string, data: string | Uint8Array) {
    const b = base.files.get(p);
    if (b && (await gitBlobSha(data)) === b.sha) delete overlay[p];
    else overlay[p] = typeof data === "string" ? { content: data } : { bytes: data };
  }
  function markRemoved(p: string) {
    if (base.files.has(p)) overlay[p] = { deleted: true };
    else delete overlay[p];
  }
  /** Drop entries the base now matches (after the base moved). */
  async function normalize() {
    for (const [p, e] of Object.entries(overlay)) {
      if (hasData(e)) await setContent(p, working(p)!);
      else if ("deleted" in e && !base.files.has(p)) delete overlay[p];
      else if ("dir" in e && (base.dirs.has(p) || filesUnder(p).length > 0)) delete overlay[p];
    }
  }
  async function setBase(next: Base) {
    base = next;
    remote = null;
    await normalize();
    await save();
    emit();
  }

  const committable = () => Object.keys(overlay).filter((p) => !isLocalOnly(p) && !("dir" in overlay[p]));

  async function handleMove(newHead: string) {
    const [cmp, next] = await Promise.all([client.compare(owner, repo, base.commit, newHead), loadBase(newHead)]);
    const changed = new Set(cmp.files);
    const conflicts: string[] = [];
    for (const p of committable()) {
      if (!cmp.truncated && !changed.has(p)) continue;
      // Both sides made the same change: not a conflict.
      const theirs = next.files.get(p);
      const mine = working(p);
      const same = mine === undefined ? !theirs : !!theirs && (await gitBlobSha(mine)) === theirs.sha;
      if (!same) conflicts.push(p);
    }
    if (conflicts.length === 0) await setBase(next);
    else {
      remote = { newHead, conflicts, base: next };
      emit();
    }
  }

  async function checkRemote({ force = false } = {}) {
    if (disposed || (!force && Date.now() < nextCheckAt)) return;
    nextCheckAt = Date.now() + POLL_MS;
    checking = true;
    emit(false);
    try {
      await serial(async () => {
        // A forced check (after head_moved) skips the ETag, which may predate the move.
        const h = await client.head(owner, repo, branch, force ? undefined : etag);
        if (!h) return;
        etag = h.etag ?? undefined;
        if (h.sha === base.commit || h.sha === remote?.newHead) return;
        await handleMove(h.sha);
      });
      syncError = null;
    } catch (err) {
      syncError = err instanceof GitHubError ? err : new GitHubError("upstream", 0, (err as Error).message);
      if (isGitHubError(err, "rate_limited")) nextCheckAt = Date.now() + (err.retryAfter ?? 60) * 1000;
    } finally {
      checking = false;
      emit(false);
    }
  }

  function clearCommitted(snapshot: Record<string, OverlayEntry>) {
    // Text typed while the commit was in flight stays a change.
    for (const [p, e] of Object.entries(snapshot)) if (sameEntry(overlay[p], e)) delete overlay[p];
  }
  function commitInput(targetBranch: string, message: string) {
    if (readOnly) throw new Error("This repository is read-only");
    if (remote) throw new Error("Resolve the conflicts with the remote changes first");
    const input = buildCommit(overlay, (p) => base.files.has(p), { branch: targetBranch, expectedHeadOid: base.commit, message });
    if (input.additions.length + input.deletions.length === 0) throw new Error("There is nothing to commit");
    const size = input.additions.reduce((n, a) => n + a.contents.length + a.path.length, 0);
    if (size > MAX_COMMIT_BYTES) {
      throw new Error(
        `These changes are ${mb(size)} encoded; one commit from the web can carry ${mb(MAX_COMMIT_BYTES)}. Commit fewer files at a time (discard some, commit, then add them back), or commit large files with git.`,
      );
    }
    const snapshot = Object.fromEntries(committable().map((p) => [p, structuredClone(overlay[p])]));
    return { input, snapshot };
  }

  const backend: GitHubBackend = {
    kind: "github",
    label: `${owner}/${repo}`,
    readOnly,
    owner,
    repo,
    branch,
    meta,
    baseSha: () => base.commit,
    version: () => version,

    async listTree() {
      if (!treeCache) {
        const dirs = new Set(base.dirs);
        for (const [p, e] of Object.entries(overlay)) if ("dir" in e) dirs.add(p);
        treeCache = buildTree(visibleFiles(), dirs, base.unloaded);
      }
      return treeCache;
    },
    async loadDirectory(path) {
      if (!base.unloaded.has(k(path))) return;
      await loadDir(base, k(path));
      emit();
    },
    readFile: (path) => readBytes(k(path)),

    async writeFile(path, text) {
      const p = k(path);
      guard([p]);
      const known = exists(p);
      await setContent(p, text);
      await save();
      emit(!known);
    },
    async writeBytes(path, bytes) {
      const p = k(path);
      guard([p]);
      if (bytes.length > MAX_UPLOAD_BYTES) {
        throw new Error(`${path.at(-1)} is ${mb(bytes.length)}. Files added from the web can be up to ${mb(MAX_UPLOAD_BYTES)}, so they fit in a commit. Add larger files with git.`);
      }
      const known = exists(p);
      await setContent(p, new Uint8Array(bytes));
      await save();
      emit(!known);
    },
    async createFile(path, text = "") {
      const p = k(path);
      guard([p]);
      if (exists(p)) throw new Error(`"${path.at(-1)}" already exists`);
      await setContent(p, text);
      await save();
      emit();
    },
    async createDirectory(path) {
      const p = k(path);
      guard([p]);
      if (exists(p)) throw new Error(`"${path.at(-1)}" already exists`);
      overlay[p] = { dir: true };
      await save();
      emit();
    },
    async rename(from, to) {
      const [f, t] = [k(from), k(to)];
      if (exists(t)) throw new Error(`"${to.at(-1)}" already exists`);
      await loadAllUnder(f);
      const moves = filesUnder(f).filter((p) => !isDeleted(p));
      guard([f, t, ...moves, ...moves.map((p) => t + p.slice(f.length))]);
      const datas = await Promise.all(moves.map((p) => readBytes(p)));
      if (moves.some((p) => lfs.has(p))) throw new Error("Files stored with Git LFS can't be moved here. Move them with git.");
      for (const [i, p] of moves.entries()) {
        markRemoved(p);
        await setContent(t + p.slice(f.length), isText(datas[i]) ? dec.decode(datas[i]) : datas[i]);
      }
      for (const p of Object.keys(overlay)) if ("dir" in overlay[p] && under(p, f)) {
        delete overlay[p];
        overlay[t + p.slice(f.length)] = { dir: true };
      }
      await save();
      emit();
    },
    async remove(path) {
      const p = k(path);
      await loadAllUnder(p);
      const files = filesUnder(p);
      guard([p, ...files]);
      for (const f of files) markRemoved(f);
      for (const d of Object.keys(overlay)) if ("dir" in overlay[d] && under(d, p)) delete overlay[d];
      await save();
      emit();
    },

    status() {
      const out: Record<string, FileStatus> = {};
      for (const p of committable()) out[p] = hasData(overlay[p]) ? (base.files.has(p) ? "M" : "A") : "D";
      for (const p of remote?.conflicts ?? []) out[p] = "C";
      const json = JSON.stringify(out);
      if (statusCache?.json !== json) statusCache = { json, value: out };
      return statusCache.value;
    },
    changes() {
      const conflicts = new Set(remote?.conflicts);
      return committable()
        .sort()
        .map((path) => ({
          path,
          status: hasData(overlay[path]) ? (base.files.has(path) ? "M" : "A") : "D",
          conflict: conflicts.has(path),
          ...("bytes" in overlay[path] && { binary: true }),
        }));
    },
    hasLocalEnv: () => content(LOCAL_ENV) !== undefined,
    sync: () => ({ checking, error: syncError, remote: remote && { newHead: remote.newHead, conflicts: remote.conflicts } }),
    subscribe(fn) {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
    readOnlyReason(path) {
      const p = k(path);
      if (isWorkflowPath(p)) return WORKFLOW_MESSAGE;
      if (isDeleted(p)) return "This file is deleted in your changes. Discard the deletion in Source Control to restore it.";
      if (lfs.has(p)) return "This file is stored with Git LFS. Only its pointer is in the repository, so it is read-only here.";
      if ((base.files.get(p)?.size ?? 0) > MAX_BLOB_BYTES && content(p) === undefined) return "This file is over 4 MB, too large to load here.";
      return null;
    },

    async readBase(p) {
      if (!base.files.has(p)) return null;
      const bytes = await readBytes(p, base);
      return isText(bytes) ? dec.decode(bytes) : null;
    },
    async readWorking(p) {
      if (isDeleted(p)) return null;
      const bytes = await readBytes(p);
      return isText(bytes) ? dec.decode(bytes) : null;
    },
    async readTheirs(p) {
      const from = remote?.base ?? base;
      if (!from.files.has(p)) return null;
      const bytes = await readBytes(p, from);
      return isText(bytes) ? dec.decode(bytes) : null;
    },

    async binarySides(p, theirs = false) {
      const from = theirs ? (remote?.base ?? base) : base;
      const original = from.files.has(p) ? await readBytes(p, from) : null;
      const modified = isDeleted(p) ? null : working(p) !== undefined || base.files.has(p) ? await readBytes(p) : null;
      if ((!original || isText(original)) && (!modified || isText(modified))) return null;
      return { original: original?.length ?? null, modified: modified?.length ?? null };
    },

    async discard(paths) {
      for (const p of paths) delete overlay[p];
      if (remote) remote = { ...remote, conflicts: remote.conflicts.filter((p) => !paths.includes(p)) };
      if (remote && remote.conflicts.length === 0) await setBase(remote.base);
      else {
        await save();
        emit();
      }
    },
    async discardAll() {
      const local = overlay[LOCAL_ENV];
      for (const p of Object.keys(overlay)) delete overlay[p];
      if (local) overlay[LOCAL_ENV] = local; // local-only, never a "change"
      if (remote) await setBase(remote.base);
      else {
        await save();
        emit();
      }
    },

    checkRemote,
    keepMine: () => serial(async () => void (remote && (await setBase(remote.base)))),
    takeTheirs: () =>
      serial(async () => {
        if (!remote) return;
        for (const p of remote.conflicts) delete overlay[p];
        await setBase(remote.base);
      }),

    commit: (message) =>
      serial(async () => {
        const { input, snapshot } = commitInput(branch, message);
        let res;
        try {
          res = await client.commit(owner, repo, input);
        } catch (err) {
          if (isGitHubError(err, "head_moved")) {
            // Someone pushed: find the overlapping files; nothing is lost.
            const h = await client.head(owner, repo, branch);
            if (h) {
              etag = h.etag ?? undefined;
              if (h.sha !== base.commit) await handleMove(h.sha);
            }
          }
          throw err;
        }
        clearCommitted(snapshot);
        await setBase(await loadBase(res.sha));
        nextCheckAt = 0;
        return res;
      }),

    commitToNewBranch: ({ branch: target, message, title, body }) =>
      serial(async () => {
        const { input, snapshot } = commitInput(target, message);
        await client.createBranch(owner, repo, target, base.commit);
        const commit = await client.commit(owner, repo, input);
        const pr = await client.pull(owner, repo, { head: target, base: branch, title, body });
        clearCommitted(snapshot);
        await save();
        emit();
        return { commit, pr };
      }),

    createBranchFromCurrent: (name) =>
      serial(async () => {
        await client.createBranch(owner, repo, name, base.commit);
        if (Object.keys(overlay).length) {
          await kv.set("copies", copyKey(owner, repo, name), { baseCommitSha: base.commit, overlay, updatedAt: Date.now() });
          await kv.del("copies", key);
        }
        disposed = true; // the working copy lives on the new branch now
      }),

    async initialize() {
      await backend.createFile(["hittable", "env.json"], "{}\n");
      await backend.createFile(["hittable", "requests", "example.hit"], EXAMPLE_HIT);
      await backend.createFile(["hittable", ".gitignore"], "env.local.json\n");
    },
  };

  // Opened on a base older than the head (changes from an earlier visit): reconcile now.
  if (base.commit !== first.sha) {
    try {
      await handleMove(first.sha);
    } catch (err) {
      syncError = err instanceof GitHubError ? err : null;
    }
  }
  nextCheckAt = Date.now() + POLL_MS;
  return backend;
}
