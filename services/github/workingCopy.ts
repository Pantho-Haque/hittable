// The working copy of a GitHub repository: what you changed on top of the
// branch's base commit, kept in IndexedDB until you commit it. Blobs are
// cached by SHA (immutable), so a file is downloaded once.

/** Your version of one path. `dir` keeps an empty new folder visible (git has no empty folders). */
export type OverlayEntry = { content: string } | { deleted: true } | { dir: true };

export type WorkingCopy = {
  baseCommitSha: string;
  overlay: Record<string, OverlayEntry>;
  updatedAt: number;
};

/** The storage boundary: IndexedDB in the browser, a Map in tests. */
export interface KVStore {
  get<T>(store: "copies" | "blobs", key: string): Promise<T | undefined>;
  set(store: "copies" | "blobs", key: string, value: unknown): Promise<void>;
  del(store: "copies" | "blobs", key: string): Promise<void>;
  entries<T>(store: "copies" | "blobs"): Promise<[string, T][]>;
  clear(store: "copies" | "blobs"): Promise<void>;
}

export const copyKey = (owner: string, repo: string, branch: string) => `${owner}/${repo}@${branch}`;

export function memoryStore(): KVStore {
  const maps = { copies: new Map<string, unknown>(), blobs: new Map<string, unknown>() };
  return {
    get: async (s, k) => structuredClone(maps[s].get(k)) as never,
    set: async (s, k, v) => void maps[s].set(k, structuredClone(v)),
    del: async (s, k) => void maps[s].delete(k),
    entries: async (s) => [...maps[s].entries()].map(([k, v]) => [k, structuredClone(v)]) as never,
    clear: async (s) => maps[s].clear(),
  };
}

const DB_NAME = "hittable-github";

function idbStore(): KVStore {
  let db: Promise<IDBDatabase> | null = null;
  const open = () =>
    (db ??= new Promise((resolve, reject) => {
      const req = indexedDB.open(DB_NAME, 1);
      req.onupgradeneeded = () => {
        req.result.createObjectStore("copies");
        req.result.createObjectStore("blobs");
      };
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    }));
  const run = async <T>(store: "copies" | "blobs", mode: IDBTransactionMode, fn: (s: IDBObjectStore) => IDBRequest<T>) => {
    const tx = (await open()).transaction(store, mode);
    const req = fn(tx.objectStore(store));
    return new Promise<T>((resolve, reject) => {
      tx.oncomplete = () => resolve(req.result);
      tx.onerror = tx.onabort = () => reject(tx.error ?? req.error);
    });
  };
  return {
    get: (s, k) => run(s, "readonly", (o) => o.get(k)),
    set: async (s, k, v) => void (await run(s, "readwrite", (o) => o.put(v, k))),
    del: async (s, k) => void (await run(s, "readwrite", (o) => o.delete(k))),
    async entries<T>(s: "copies" | "blobs") {
      const [keys, values] = await Promise.all([run(s, "readonly", (o) => o.getAllKeys()), run(s, "readonly", (o) => o.getAll())]);
      return keys.map((k, i) => [String(k), values[i] as T] as [string, T]);
    },
    clear: async (s) => void (await run(s, "readwrite", (o) => o.clear())),
  };
}

let shared: KVStore | null = null;

/** The browser's store (IndexedDB, or memory where it is missing). Asks once for persistent storage. */
export function defaultStore(): KVStore {
  if (shared) return shared;
  if (typeof indexedDB === "undefined") return (shared = memoryStore());
  // Best effort: without it the browser may evict uncommitted work under storage pressure.
  void navigator.storage?.persist?.().catch(() => {});
  return (shared = idbStore());
}

/** True when any repository has a working copy with something in it (sign-out asks first). */
export async function hasWorkingCopies(kv: KVStore = defaultStore()): Promise<boolean> {
  try {
    const copies = await kv.entries<WorkingCopy>("copies");
    return copies.some(([, c]) => Object.keys(c.overlay ?? {}).length > 0);
  } catch {
    return false;
  }
}

/** Forget every working copy and cached blob on this device. */
export async function clearWorkingCopies(kv: KVStore = defaultStore()): Promise<void> {
  await Promise.all([kv.clear("copies"), kv.clear("blobs")]);
}
