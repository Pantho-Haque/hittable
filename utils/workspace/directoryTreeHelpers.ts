import { TDirectoryNode } from "@/types";

export async function buildTree(
  dir: FileSystemDirectoryHandle
): Promise<TDirectoryNode[]> {
  const nodes: TDirectoryNode[] = [];

  for await (const [name, handle] of dir.entries()) {
    const node: TDirectoryNode = {
      name,
      kind: handle.kind as "file" | "directory",
      handle,
    };

    if (handle.kind === "directory") {
      node.children = await buildTree(handle as FileSystemDirectoryHandle);
    }

    nodes.push(node);
  }

  return nodes;
}

export function getParentHandleForPath(
  tree: TDirectoryNode[],
  path: string[]
): FileSystemDirectoryHandle | null {
  if (path.length <= 1) {
    return null;
  }

  const parentPath = path.slice(0, -1);
  const node = findNodeByPath(tree, parentPath);
  if (!node || node.kind !== "directory") {
    return null;
  }

  return node.handle as FileSystemDirectoryHandle;
}

export function findNodeByPath(
  tree: TDirectoryNode[],
  path: string[]
): TDirectoryNode | null {
  if (path.length === 0) return null;

  const [head, ...rest] = path;
  const node = tree.find((n) => n.name === head);
  if (!node) return null;
  if (rest.length === 0) return node;
  if (!node.children) return null;

  return findNodeByPath(node.children, rest);
}

export function getParentHandle(
  tree: TDirectoryNode[],
  path: string[]
): FileSystemDirectoryHandle | null {
  if (path.length <= 1) return null;

  const parentPath = path.slice(0, -1);
  const node = findNodeByPath(tree, parentPath);
  if (!node || node.kind !== "directory") return null;

  return node.handle as FileSystemDirectoryHandle;
}

export function isReservedRootEntry(name: string): boolean {
  return name === "env" || name === "env.json" || name === "notes";
}

export function getFileKind(name: string): "hit" | "env" | "markdown" | "text" {
  if (name.endsWith(".hit")) return "hit";
  if (name === "env" || name === "env.json") return "env";
  if (name.endsWith(".md")) return "markdown";
  return "text";
}
