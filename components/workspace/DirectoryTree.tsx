"use client";

import { useState, useCallback, useEffect } from "react";
import { Folder, File, Plus, RefreshCw, FolderOpen } from "lucide-react";
import { useWorkspace } from "@/context/workspaceContext";
import { TDirectoryNode } from "@/types";
import { getParentHandleForPath, findNodeByPath } from "@/utils/workspace/directoryTreeHelpers";
import DirectoryTreeNode from "./DirectoryTreeNode";

function isReservedInHittable(parentPath: string[], name: string): boolean {
  if (parentPath.length === 0) {
    return name === "env.json" || name === "env" || name === "notes";
  }
  const isHittableDir = parentPath[parentPath.length - 1] === "hittable";
  return isHittableDir && (name === "env.json" || name === "env" || name === "notes");
}

function isReservedNode(path: string[], name: string): boolean {
  if (path.length < 2) return false;
  const parentName = path[path.length - 2];
  return parentName === "hittable" && (name === "env.json" || name === "env" || name === "notes");
}

export default function DirectoryTree() {
  const { tree, activeFile, setActiveFile, refreshTree, deleteEntry, renameEntry, createEntry, directoryHandle, openFolder } = useWorkspace();
  const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set());
  const [selectedPath, setSelectedPath] = useState<string[] | null>(null);
  const [contextMenu, setContextMenu] = useState<{
    x: number;
    y: number;
    node: { name: string; handle: FileSystemFileHandle | FileSystemDirectoryHandle; kind: "file" | "directory"; path: string[] };
    parentHandle: FileSystemDirectoryHandle;
  } | null>(null);
  const [isCreating, setIsCreating] = useState<{ type: "file" | "directory"; parentPath: string[] } | null>(null);
  const [newName, setNewName] = useState("");

  const toggleDir = useCallback((path: string) => {
    setExpandedDirs((prev) => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  }, []);

  const handleSelect = useCallback((path: string[]) => {
    setSelectedPath(path);
  }, []);

  const handleOpen = useCallback((path: string[], handle: FileSystemFileHandle) => {
    setActiveFile({ path, handle });
  }, [setActiveFile]);

  const handleContextMenu = useCallback((
    e: React.MouseEvent,
    name: string,
    handle: FileSystemFileHandle | FileSystemDirectoryHandle,
    kind: "file" | "directory",
    nodePath: string[]
  ) => {
    e.preventDefault();
    e.stopPropagation();
    if (isReservedNode(nodePath, name)) return;
    let parentHandle: FileSystemDirectoryHandle | null;
    if (nodePath.length <= 1) {
      parentHandle = directoryHandle;
    } else {
      parentHandle = getParentHandleForPath(tree, nodePath);
    }
    if (!parentHandle) return;
    setContextMenu({ x: e.clientX, y: e.clientY, node: { name, handle, kind, path: nodePath }, parentHandle });
  }, [tree, directoryHandle]);

  const closeContextMenu = useCallback(() => {
    setContextMenu(null);
  }, []);

  const handleCreate = useCallback((type: "file" | "directory", parentPath: string[]) => {
    setIsCreating({ type, parentPath });
    setNewName("");
  }, []);

  const getCreateTargetHandle = useCallback((parentPath: string[]): FileSystemDirectoryHandle | null => {
    if (parentPath.length === 0) {
      return directoryHandle;
    }
    const node = findNodeByPath(tree, parentPath);
    if (!node || node.kind !== "directory") return null;
    return node.handle as FileSystemDirectoryHandle;
  }, [tree, directoryHandle]);

  const handleCreateConfirm = useCallback(async () => {
    if (!isCreating || !newName.trim()) return;
    if (isReservedInHittable(isCreating.parentPath, newName.trim())) return;
    const parentHandle = getCreateTargetHandle(isCreating.parentPath);
    if (!parentHandle) return;
    const creatingType = isCreating.type;
    const creatingParentPath = isCreating.parentPath;
    const name = newName.trim();
    setIsCreating(null);
    setNewName("");
    await createEntry(parentHandle, name, creatingType);
    if (creatingParentPath.length > 0) {
      setExpandedDirs((prev) => new Set([...prev, creatingParentPath.join("/")]));
    }
  }, [isCreating, newName, getCreateTargetHandle, createEntry]);

  const handleCreateCancel = useCallback(() => {
    setIsCreating(null);
    setNewName("");
  }, []);

  const handleDelete = useCallback(async () => {
    if (!contextMenu) return;
    await deleteEntry(contextMenu.parentHandle, contextMenu.node.name);
    if (activeFile?.path.join("/") === contextMenu.node.path.join("/")) {
      setActiveFile(null);
    }
    closeContextMenu();
  }, [contextMenu, activeFile, deleteEntry, setActiveFile, closeContextMenu]);

  const handleRename = useCallback(async (newName: string) => {
    if (!contextMenu || !newName.trim() || newName === contextMenu.node.name) {
      closeContextMenu();
      return;
    }
    await renameEntry(contextMenu.parentHandle, contextMenu.node.name, newName.trim(), contextMenu.node.kind);
    if (activeFile?.path.join("/") === contextMenu.node.path.join("/")) {
      setActiveFile(null);
    }
    closeContextMenu();
  }, [contextMenu, activeFile, renameEntry, setActiveFile, closeContextMenu]);

  useEffect(() => {
    if (tree.length > 0 && expandedDirs.size === 0) {
      setExpandedDirs(new Set([tree[0]?.name].filter(Boolean)));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only initialize on first tree load
  }, [tree]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if ((e.target as HTMLElement).closest("input, textarea")) return;
    if (!selectedPath) return;

    const flatItems: { path: string[]; kind: "file" | "directory" }[] = [];
    const flatten = (nodes: TDirectoryNode[], parentPath: string[]) => {
      for (const node of nodes) {
        const nodePath = [...parentPath, node.name];
        flatItems.push({ path: nodePath, kind: node.kind });
        if (node.kind === "directory" && expandedDirs.has(nodePath.join("/")) && node.children) {
          flatten(node.children, nodePath);
        }
      }
    };
    flatten(tree, []);

    const currentIndex = flatItems.findIndex(
      (item) => item.path.join("/") === selectedPath.join("/")
    );

    switch (e.key) {
      case "ArrowDown": {
        e.preventDefault();
        if (currentIndex < flatItems.length - 1) {
          setSelectedPath(flatItems[currentIndex + 1].path);
        }
        break;
      }
      case "ArrowUp": {
        e.preventDefault();
        if (currentIndex > 0) {
          setSelectedPath(flatItems[currentIndex - 1].path);
        }
        break;
      }
      case "ArrowRight": {
        e.preventDefault();
        const currentItem = flatItems[currentIndex];
        if (currentItem?.kind === "directory" && !expandedDirs.has(currentItem.path.join("/"))) {
          toggleDir(currentItem.path.join("/"));
        }
        break;
      }
      case "ArrowLeft": {
        e.preventDefault();
        const currentItem = flatItems[currentIndex];
        if (currentItem?.kind === "directory" && expandedDirs.has(currentItem.path.join("/"))) {
          toggleDir(currentItem.path.join("/"));
        } else if (currentItem && currentItem.path.length > 1) {
          setSelectedPath(currentItem.path.slice(0, -1));
        }
        break;
      }
      case " ":
      case "Enter": {
        e.preventDefault();
        const currentItem = flatItems[currentIndex];
        if (currentItem?.kind === "file") {
          const node = findNodeByPath(tree, currentItem.path);
          if (node) {
            handleOpen(currentItem.path, node.handle as FileSystemFileHandle);
          }
        } else if (currentItem?.kind === "directory") {
          toggleDir(currentItem.path.join("/"));
        }
        break;
      }
      case "F2": {
        e.preventDefault();
        const currentItem = flatItems[currentIndex];
        if (currentItem) {
          const node = findNodeByPath(tree, currentItem.path);
          if (node && !isReservedInHittable(currentItem.path.slice(0, -1), node.name)) {
            const parentHandle = currentItem.path.length === 1 ? directoryHandle : getParentHandleForPath(tree, currentItem.path);
            if (parentHandle) {
              setContextMenu({
                x: 0,
                y: 0,
                node: { name: node.name, handle: node.handle, kind: node.kind, path: currentItem.path },
                parentHandle,
              });
            }
          }
        }
        break;
      }
      case "Delete": {
        e.preventDefault();
        const currentItem = flatItems[currentIndex];
        if (currentItem) {
          const node = findNodeByPath(tree, currentItem.path);
          if (node && !isReservedInHittable(currentItem.path.slice(0, -1), node.name)) {
            const parentHandle = currentItem.path.length === 1 ? directoryHandle : getParentHandleForPath(tree, currentItem.path);
            if (parentHandle) {
              setContextMenu({
                x: 0,
                y: 0,
                node: { name: node.name, handle: node.handle, kind: node.kind, path: currentItem.path },
                parentHandle,
              });
            }
          }
        }
        break;
      }
    }
  }, [selectedPath, tree, expandedDirs, toggleDir, handleOpen, directoryHandle]);

  const folderName = directoryHandle?.name ?? "Explorer";

  useEffect(() => {
    if (!selectedPath) return;
    document.getElementById(`file-${encodeURIComponent(selectedPath.join("/"))}`)
      ?.scrollIntoView({ block: "nearest" });
  }, [selectedPath]);

  return (
    <div className="flex flex-col h-full bg-[#0a1628]">
      <div className="flex flex-wrap items-center gap-1 px-3 py-2 border-b border-white/10">
        <span className="text-xs font-semibold text-slate-300 truncate">
          {folderName}
        </span>
        <button
          onClick={openFolder}
          className="workspace-button"
          title="Change Folder" aria-label="Change Folder"
        >
          <FolderOpen className="w-3.5 h-3.5" />
        </button>
        <div className="ml-auto flex items-center gap-1 shrink-0">
          <button
            onClick={() => handleCreate("file", [])}
            className="workspace-button"
            title="New File" aria-label="New File"
          >
            <Plus className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={() => handleCreate("directory", [])}
            className="workspace-button"
            title="New Folder" aria-label="New Folder"
          >
            <Folder className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={() => refreshTree()}
            className="workspace-button"
            title="Refresh" aria-label="Refresh"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <div
        className="flex-1 overflow-auto py-2"
        role="tree" aria-label="Workspace files"
        aria-activedescendant={selectedPath ? `file-${encodeURIComponent(selectedPath.join("/"))}` : undefined}
        onFocus={() => { if (!selectedPath && tree[0]) setSelectedPath([tree[0].name]); }}
        tabIndex={0}
        onKeyDown={handleKeyDown}
      >
        {isCreating && isCreating.parentPath.length === 0 && (
          <div className="flex items-center gap-1 px-2 py-1" style={{ paddingLeft: "8px" }}>
            {isCreating.type === "directory" ? (
              <Folder className="w-3.5 h-3.5 text-amber-400/70" />
            ) : (
              <File className="w-3.5 h-3.5 text-white/30" />
            )}
            <input
              autoFocus
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleCreateConfirm();
                if (e.key === "Escape") handleCreateCancel();
              }}
              onBlur={handleCreateConfirm}
              className="flex-1 px-1 py-0.5 bg-[#0e1f35] border border-cyan-400/50 rounded text-xs text-white/80 focus:outline-none"
              placeholder={isCreating.type === "directory" ? "Folder name" : "File name"}
            />
          </div>
        )}

        {tree.map((node) => (
          <DirectoryTreeNode
            key={node.name}
            node={node}
            path={[node.name]}
            depth={0}
            expandedDirs={expandedDirs}
            selectedPath={selectedPath}
            activePath={activeFile?.path.join("/")}
            toggleDir={toggleDir}
            onSelect={handleSelect}
            onOpen={handleOpen}
            onContextMenu={handleContextMenu}
            isCreating={isCreating}
            creatingName={newName}
            onCreatingNameChange={setNewName}
            onCreateConfirm={handleCreateConfirm}
            onCreateCancel={handleCreateCancel}
          />
        ))}
      </div>

      {contextMenu && (
        <ContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          node={contextMenu.node}
          onClose={closeContextMenu}
          onDelete={handleDelete}
          onRename={handleRename}
          onCreate={handleCreate}
        />
      )}
    </div>
  );
}

function ContextMenu({
  x,
  y,
  node,
  onClose,
  onDelete,
  onRename,
  onCreate,
}: {
  x: number;
  y: number;
  node: { name: string; handle: FileSystemFileHandle | FileSystemDirectoryHandle; kind: "file" | "directory"; path: string[] };
  onClose: () => void;
  onDelete: () => void;
  onRename: (newName: string) => void;
  onCreate: (type: "file" | "directory", parentPath: string[]) => void;
}) {
  const [isRenaming, setIsRenaming] = useState(false);
  const [newName, setNewName] = useState(node.name);

  const handleRenameConfirm = () => {
    onRename(newName);
    setIsRenaming(false);
  };

  if (isRenaming) {
    return (
      <>
        <div className="fixed inset-0 z-40" onClick={onClose} />
        <div
          className="fixed z-50 bg-[#0e1f35] border border-white/10 rounded-lg shadow-xl p-2"
          style={{ left: x, top: y }}
        >
          <input
            autoFocus
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleRenameConfirm();
              if (e.key === "Escape") onClose();
            }}
            className="w-48 px-2 py-1 bg-[#0a1628] border border-white/10 rounded text-xs text-white/80 focus:outline-none focus:border-cyan-400/50"
          />
          <div className="flex gap-1 mt-1">
            <button
              onClick={handleRenameConfirm}
              className="px-2 py-0.5 text-[10px] bg-cyan-400/10 text-cyan-400 rounded hover:bg-cyan-400/20"
            >
              Rename
            </button>
            <button
              onClick={onClose}
              className="px-2 py-0.5 text-[10px] text-white/40 rounded hover:bg-white/5"
            >
              Cancel
            </button>
          </div>
        </div>
      </>
    );
  }

  return (
    <>
      <div className="fixed inset-0 z-40" onClick={onClose} />
      <div
        className="fixed z-50 bg-[#0e1f35] border border-white/10 rounded-lg shadow-xl py-1 min-w-[140px]"
        style={{ left: x, top: y }}
      >
        {node.kind === "directory" && (
          <>
            <button
              onClick={() => { onCreate("file", node.path); onClose(); }}
              className="w-full px-3 py-1.5 text-left text-xs text-white/70 hover:bg-white/5 hover:text-white/90 transition-colors"
            >
              New File
            </button>
            <button
              onClick={() => { onCreate("directory", node.path); onClose(); }}
              className="w-full px-3 py-1.5 text-left text-xs text-white/70 hover:bg-white/5 hover:text-white/90 transition-colors"
            >
              New Folder
            </button>
            <div className="h-px bg-white/5 my-1" />
          </>
        )}
        <button
          onClick={() => setIsRenaming(true)}
          className="w-full px-3 py-1.5 text-left text-xs text-white/70 hover:bg-white/5 hover:text-white/90 transition-colors"
        >
          Rename
        </button>
        <button
          onClick={onDelete}
          className="w-full px-3 py-1.5 text-left text-xs text-red-400/70 hover:bg-red-400/10 hover:text-red-400 transition-colors"
        >
          Delete
        </button>
      </div>
    </>
  );
}
