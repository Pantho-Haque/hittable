// What the workspace UI works on: a local folder or a GitHub repository.
// Paths are segments relative to the workspace root, e.g. ["hittable", "env.json"].

export type FileStatus = "M" | "A" | "D" | "C"; // modified, added, deleted, conflict

export type WsNode = { name: string; path: string[]; kind: "file" | "directory"; children?: WsNode[] };

export interface WorkspaceBackend {
  kind: "folder" | "github";
  /** Folder name, or owner/repo. */
  label: string;
  /** The whole workspace is read-only (GitHub: no push permission, or archived). */
  readOnly: boolean;
  listTree(): Promise<WsNode[]>;
  /** Raw bytes; callers decide text or binary with utils/workspace/fileKind.ts. */
  readFile(path: string[]): Promise<Uint8Array>;
  writeFile(path: string[], text: string): Promise<void>;
  /** Raw bytes, e.g. a file picked for an upload. Creates missing folders; replaces a file that is there. */
  writeBytes(path: string[], bytes: Uint8Array): Promise<void>;
  /** Fails when the path is taken. */
  createFile(path: string[], text?: string): Promise<void>;
  createDirectory(path: string[]): Promise<void>;
  /** Fails when the target is taken. */
  rename(from: string[], to: string[]): Promise<void>;
  /** Recursive for directories. */
  remove(path: string[]): Promise<void>;
  /** path.join("/") → status. GitHub only. */
  status?(): Record<string, FileStatus>;
  /** Backend-initiated changes (status, remote). Returns an unsubscribe. */
  subscribe?(onChange: () => void): () => void;
  /** Why this one file can't be edited (e.g. a GitHub workflow), or null. Asked after readFile. */
  readOnlyReason?(path: string[]): string | null;
  /** List a directory the tree left unloaded (a directory node without `children`). */
  loadDirectory?(path: string[]): Promise<void>;
}

export const HITTABLE_DIR = "hittable";
export const ENV_PATH = [HITTABLE_DIR, "env.json"];
// FR-17: values that must not reach git, layered over env.json.
export const LOCAL_ENV_PATH = [HITTABLE_DIR, "env.local.json"];

async function readFlatMap(backend: WorkspaceBackend, path: string[]): Promise<Record<string, string>> {
  try {
    const parsed = JSON.parse(new TextDecoder().decode(await backend.readFile(path)));
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return {};
    return Object.fromEntries(Object.entries(parsed).map(([k, v]) => [k, String(v)]));
  } catch {
    return {};
  }
}

/** hittable/env.json with hittable/env.local.json layered on top (the local value wins). */
export async function readEnv(backend: WorkspaceBackend): Promise<Record<string, string>> {
  return { ...(await readFlatMap(backend, ENV_PATH)), ...(await readFlatMap(backend, LOCAL_ENV_PATH)) };
}

/** env.json, env and notes directly inside hittable/ (or at the root) can't be created, renamed or deleted from the explorer. */
export function isReservedPath(path: string[]): boolean {
  const name = path[path.length - 1];
  const parent = path[path.length - 2];
  return (path.length === 1 || parent === HITTABLE_DIR) && (name === "env.json" || name === "env" || name === "notes");
}

export function findNode(tree: WsNode[], path: string[]): WsNode | null {
  let nodes: WsNode[] | undefined = tree;
  let node: WsNode | undefined;
  for (const part of path) {
    node = nodes?.find((n) => n.name === part);
    if (!node) return null;
    nodes = node.children;
  }
  return node ?? null;
}
