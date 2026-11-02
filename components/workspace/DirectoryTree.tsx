"use client";

import { useState, useCallback, useEffect } from "react";
import { ModalActions, ModalShell } from "@/components/ui/SharedModal";
import { Folder, File, Plus, RefreshCw } from "lucide-react";
import { useWorkspace } from "@/context/workspaceContext";
import { isReservedPath, type WsNode } from "@/services/workspace/backend";
import DirectoryTreeNode from "./DirectoryTreeNode";
import { useNotification } from "@/hooks/useNotify";

type MenuNode = { name: string; kind: "file" | "directory"; path: string[] };

export default function DirectoryTree() {
  const { tree, activeFile, setActiveFile, refreshTree, loadDirectory, deleteEntry, renameEntry, createEntry, label, readOnly, status, backendKind } = useWorkspace();
  const { error: notifyError } = useNotification();
  const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set());
  const [selectedPath, setSelectedPath] = useState<string[] | null>(null);
  const [contextMenu, setContextMenu] = useState<{
    x: number;
    y: number;
    node: MenuNode;
    renaming?: boolean;
  } | null>(null);
  const [isCreating, setIsCreating] = useState<{ type: "file" | "directory"; parentPath: string[] } | null>(null);
  const [newName, setNewName] = useState("");

  const toggleDir = useCallback((path: string) => {
    // A large GitHub tree lists a folder the first time it is opened.
    void loadDirectory(path.split("/"));
    setExpandedDirs((prev) => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  }, [loadDirectory]);

  const handleSelect = useCallback((path: string[]) => {
    setSelectedPath(path);
  }, []);

  const handleOpen = useCallback((path: string[]) => {
    setActiveFile(path);
  }, [setActiveFile]);

  // A read-only workspace and the reserved env.json/notes get no menu.
  const editable = useCallback((path: string[]) => !readOnly && !isReservedPath(path), [readOnly]);

  const handleContextMenu = useCallback((e: React.MouseEvent, node: WsNode) => {
    e.preventDefault();
    e.stopPropagation();
    if (!editable(node.path)) return;
    setContextMenu({ x: e.clientX, y: e.clientY, node: { name: node.name, kind: node.kind, path: node.path } });
  }, [editable]);

  const closeContextMenu = useCallback(() => {
    setContextMenu(null);
  }, []);

  const handleCreate = useCallback((type: "file" | "directory", parentPath: string[]) => {
    setIsCreating({ type, parentPath });
    setNewName("");
  }, []);

  const handleCreateConfirm = useCallback(async () => {
    if (!isCreating || !newName.trim()) return;
    const creatingType = isCreating.type;
    const creatingParentPath = isCreating.parentPath;
    const name = newName.trim();
    setIsCreating(null);
    setNewName("");
    try {
      await createEntry([...creatingParentPath, name], creatingType);
    } catch (err) {
      notifyError({ title: "Could not create", desc: (err as Error).message });
      return;
    }
    if (creatingParentPath.length > 0) {
      setExpandedDirs((prev) => new Set([...prev, creatingParentPath.join("/")]));
    }
  }, [isCreating, newName, createEntry, notifyError]);

  const handleCreateCancel = useCallback(() => {
    setIsCreating(null);
    setNewName("");
  }, []);

  // Deleting removes real files from disk (a folder recursively), so the menu
  // only asks; the dialog below does it.
  const [pendingDelete, setPendingDelete] = useState<NonNullable<typeof contextMenu> | null>(null);

  const handleDelete = useCallback(() => {
    if (!contextMenu) return;
    setPendingDelete(contextMenu);
    closeContextMenu();
  }, [contextMenu, closeContextMenu]);

  const confirmDelete = useCallback(async () => {
    const target = pendingDelete;
    setPendingDelete(null);
    if (!target) return;
    try {
      await deleteEntry(target.node.path);
    } catch (err) {
      notifyError({ title: "Could not delete", desc: (err as Error).message });
    }
  }, [pendingDelete, deleteEntry, notifyError]);

  const handleRename = useCallback(async (newName: string) => {
    if (!contextMenu || !newName.trim() || newName === contextMenu.node.name) {
      closeContextMenu();
      return;
    }
    try {
      await renameEntry(contextMenu.node.path, newName.trim());
    } catch (err) {
      notifyError({ title: "Could not rename", desc: (err as Error).message });
    }
    closeContextMenu();
  }, [contextMenu, renameEntry, closeContextMenu, notifyError]);

  useEffect(() => {
    if (tree.length > 0 && expandedDirs.size === 0) {
      setExpandedDirs(new Set([tree[0]?.name].filter(Boolean)));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only initialize on first tree load
  }, [tree]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if ((e.target as HTMLElement).closest("input, textarea")) return;
    if (!selectedPath) return;

    const flatItems: WsNode[] = [];
    const flatten = (nodes: WsNode[]) => {
      for (const node of nodes) {
        flatItems.push(node);
        if (node.kind === "directory" && expandedDirs.has(node.path.join("/")) && node.children) {
          flatten(node.children);
        }
      }
    };
    flatten(tree);

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
        // Ctrl/Cmd+Enter is the global "send"; reopening the file here would
        // unmount the Runner and abort the request it just started.
        if (e.ctrlKey || e.metaKey || e.altKey) break;
        e.preventDefault();
        const currentItem = flatItems[currentIndex];
        if (currentItem?.kind === "file") {
          handleOpen(currentItem.path);
        } else if (currentItem?.kind === "directory") {
          toggleDir(currentItem.path.join("/"));
        }
        break;
      }
      case "F2":
      case "Delete": {
        e.preventDefault();
        const currentItem = flatItems[currentIndex];
        if (currentItem && editable(currentItem.path)) {
          setContextMenu({
            x: 0,
            y: 0,
            node: { name: currentItem.name, kind: currentItem.kind, path: currentItem.path },
            renaming: e.key === "F2",
          });
        }
        break;
      }
    }
  }, [selectedPath, tree, expandedDirs, toggleDir, handleOpen, editable]);

  useEffect(() => {
    if (!selectedPath) return;
    document.getElementById(`file-${encodeURIComponent(selectedPath.join("/"))}`)
      ?.scrollIntoView({ block: "nearest" });
  }, [selectedPath]);

  return (
    <div className="flex flex-col h-full bg-(--surface-1)">
      <div className="flex flex-wrap items-center gap-1 px-3 py-2 border-b border-white/10">
        <span className="text-xs font-semibold text-slate-300 truncate">
          {label || "Explorer"}
        </span>
        <div className="ml-auto flex items-center gap-1 shrink-0">
          {!readOnly && (<>
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
          </>)}
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
              <File className="w-3.5 h-3.5 text-white/50" />
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
              className="flex-1 px-1 py-0.5 bg-(--surface-2) border border-cyan-400/50 rounded text-xs text-white/80 focus:outline-none"
              placeholder={isCreating.type === "directory" ? "Folder name" : "File name"}
              aria-label={isCreating.type === "directory" ? "Folder name" : "File name"}
            />
          </div>
        )}

        {tree.map((node) => (
          <DirectoryTreeNode
            key={node.name}
            node={node}
            depth={0}
            status={status}
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
          startRenaming={contextMenu.renaming}
          onClose={closeContextMenu}
          onDelete={handleDelete}
          onRename={handleRename}
          onCreate={handleCreate}
        />
      )}

      {pendingDelete && (
        <ModalShell
          title={`Delete “${pendingDelete.node.name}”?`}
          subtitle={
            backendKind === "github"
              ? "Recorded as a change (D) in Source Control: it leaves GitHub when you commit, and you can discard it until then."
              : pendingDelete.node.kind === "directory"
              ? "This deletes the folder and everything in it from your disk. It cannot be undone here."
              : "This deletes the file from your disk. It cannot be undone here."
          }
          onClose={() => setPendingDelete(null)}
        >
          <ModalActions
            onCancel={() => setPendingDelete(null)}
            onConfirm={confirmDelete}
            confirmLabel="Delete"
            confirmDanger
          />
        </ModalShell>
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
  startRenaming = false,
}: {
  x: number;
  y: number;
  node: MenuNode;
  onClose: () => void;
  onDelete: () => void;
  onRename: (newName: string) => void;
  onCreate: (type: "file" | "directory", parentPath: string[]) => void;
  startRenaming?: boolean;
}) {
  const [isRenaming, setIsRenaming] = useState(startRenaming);
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
          className="fixed z-50 bg-(--surface-2) border border-white/10 rounded-lg shadow-xl p-2"
          style={{ left: x, top: y }}
        >
          <input
            autoFocus
            aria-label={`Rename ${node.name}`}
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleRenameConfirm();
              if (e.key === "Escape") onClose();
            }}
            className="w-48 px-2 py-1 bg-(--surface-1) border border-white/10 rounded text-xs text-white/80 focus:outline-none focus:border-cyan-400/50"
          />
          <div className="flex gap-1 mt-1">
            <button
              onClick={handleRenameConfirm}
              className="px-2 py-0.5 text-[11px] bg-cyan-400/10 text-cyan-400 rounded hover:bg-cyan-400/20"
            >
              Rename
            </button>
            <button
              onClick={onClose}
              className="px-2 py-0.5 text-[11px] text-white/60 rounded hover:bg-white/5"
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
        className="fixed z-50 bg-(--surface-2) border border-white/10 rounded-lg shadow-xl py-1 min-w-[140px]"
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
