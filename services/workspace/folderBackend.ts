import type { WorkspaceBackend, WsNode } from "./backend";
import {
  createDirectory,
  createFile,
  directoryExists,
  fileExists,
  moveEntry,
  removeEntry,
  writeFileText,
} from "@/utils/workspace/fsHelpers";

async function dirAt(root: FileSystemDirectoryHandle, path: string[], create = false) {
  let dir = root;
  for (const part of path) dir = await dir.getDirectoryHandle(part, { create });
  return dir;
}

const parentOf = (path: string[]) => path.slice(0, -1);
const nameOf = (path: string[]) => {
  const name = path[path.length - 1];
  if (!name) throw new Error("Empty path");
  return name;
};

async function walk(dir: FileSystemDirectoryHandle, path: string[]): Promise<WsNode[]> {
  const nodes: WsNode[] = [];
  for await (const [name, handle] of dir.entries()) {
    const p = [...path, name];
    nodes.push(
      handle.kind === "directory"
        ? { name, path: p, kind: "directory", children: await walk(handle as FileSystemDirectoryHandle, p) }
        : { name, path: p, kind: "file" },
    );
  }
  return nodes;
}

async function refuseTaken(dir: FileSystemDirectoryHandle, name: string) {
  // createFile truncates an existing file, so never create over a name in use.
  if ((await fileExists(dir, name)) || (await directoryExists(dir, name))) {
    throw new Error(`"${name}" already exists`);
  }
}

/** A workspace over a folder on disk (File System Access API). Scaffolding is done by the caller. */
export function createFolderBackend(root: FileSystemDirectoryHandle): WorkspaceBackend {
  return {
    kind: "folder",
    label: root.name,
    readOnly: false,
    listTree: () => walk(root, []),
    async readFile(path) {
      const file = await (await (await dirAt(root, parentOf(path))).getFileHandle(nameOf(path))).getFile();
      return new Uint8Array(await file.arrayBuffer());
    },
    async writeFile(path, text) {
      const dir = await dirAt(root, parentOf(path), true);
      await writeFileText(await dir.getFileHandle(nameOf(path), { create: true }), text);
    },
    async writeBytes(path, bytes) {
      const dir = await dirAt(root, parentOf(path), true);
      await writeFileText(await dir.getFileHandle(nameOf(path), { create: true }), bytes as Uint8Array<ArrayBuffer>);
    },
    async createFile(path, text = "") {
      const dir = await dirAt(root, parentOf(path), true);
      await refuseTaken(dir, nameOf(path));
      await createFile(dir, nameOf(path), text);
    },
    async createDirectory(path) {
      const dir = await dirAt(root, parentOf(path), true);
      await refuseTaken(dir, nameOf(path));
      await createDirectory(dir, nameOf(path));
    },
    async rename(from, to) {
      await moveEntry(await dirAt(root, parentOf(from)), nameOf(from), await dirAt(root, parentOf(to), true), nameOf(to));
    },
    async remove(path) {
      await removeEntry(await dirAt(root, parentOf(path)), nameOf(path));
    },
  };
}
