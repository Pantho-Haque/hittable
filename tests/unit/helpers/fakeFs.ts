// In-memory stand-in for the File System Access API (directory + file
// handles), enough for the workspace code paths. Build one from a plain
// object: strings are files, objects are directories.
export type FakeTree = { [name: string]: string | FakeTree };

const notFound = (name: string) => new DOMException(`${name} not found`, "NotFoundError");
const mismatch = (name: string) => new DOMException(`${name} is the wrong kind`, "TypeMismatchError");

export class FakeFile {
  readonly kind = "file" as const;
  constructor(public name: string, public content = "", public writes: string[] = []) {}
  /** Raw bytes, set when a Blob is written (binary-safe copies). */
  bytes?: Uint8Array;
  async getFile() {
    return new File([(this.bytes as Uint8Array<ArrayBuffer> | undefined) ?? this.content], this.name);
  }
  async createWritable() {
    let buffer = "";
    let bytes: Uint8Array | undefined;
    return {
      write: async (chunk: string | Blob) => {
        if (typeof chunk === "string") buffer += chunk;
        else {
          bytes = new Uint8Array(await chunk.arrayBuffer());
          buffer += new TextDecoder().decode(bytes);
        }
      },
      close: async () => { this.content = buffer; this.bytes = bytes; this.writes.push(buffer); },
    } as unknown as FileSystemWritableFileStream;
  }
  isSameEntry(other: unknown) { return other === this; }
}

export class FakeDir {
  readonly kind = "directory" as const;
  children = new Map<string, FakeFile | FakeDir>();
  permission: PermissionState = "granted";
  constructor(public name: string, tree: FakeTree = {}) {
    for (const [k, v] of Object.entries(tree)) {
      this.children.set(k, typeof v === "string" ? new FakeFile(k, v) : new FakeDir(k, v));
    }
  }
  async getFileHandle(name: string, opts?: { create?: boolean }) {
    const existing = this.children.get(name);
    if (existing && existing.kind !== "file") throw mismatch(name);
    if (existing) return existing;
    if (!opts?.create) throw notFound(name);
    const f = new FakeFile(name);
    this.children.set(name, f);
    return f;
  }
  async getDirectoryHandle(name: string, opts?: { create?: boolean }) {
    const existing = this.children.get(name);
    if (existing && existing.kind !== "directory") throw mismatch(name);
    if (existing) return existing;
    if (!opts?.create) throw notFound(name);
    const d = new FakeDir(name);
    this.children.set(name, d);
    return d;
  }
  async removeEntry(name: string, opts?: { recursive?: boolean }) {
    const entry = this.children.get(name);
    if (!entry) throw notFound(name);
    if (entry.kind === "directory" && entry.children.size > 0 && !opts?.recursive) {
      throw new DOMException(`${name} is not empty`, "InvalidModificationError");
    }
    this.children.delete(name);
  }
  async *entries(): AsyncGenerator<[string, FakeFile | FakeDir]> {
    for (const e of [...this.children.entries()]) yield e;
  }
  async queryPermission() { return this.permission; }
  async requestPermission() { return this.permission; }
  isSameEntry(other: unknown) { return other === this; }

  /** Snapshot back to a plain object, for assertions. */
  toTree(): FakeTree {
    const out: FakeTree = {};
    for (const [k, v] of this.children) out[k] = v.kind === "file" ? v.content : v.toTree();
    return out;
  }
  /** Walk a slash path to an entry. */
  at(path: string): FakeFile | FakeDir | undefined {
    const [head, ...rest] = path.split("/");
    const child = this.children.get(head);
    if (!child || rest.length === 0) return child;
    return child.kind === "directory" ? child.at(rest.join("/")) : undefined;
  }
}

export const asDir = (d: FakeDir) => d as unknown as FileSystemDirectoryHandle;
