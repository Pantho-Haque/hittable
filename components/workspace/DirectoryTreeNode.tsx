"use client";

import { useCallback, memo } from "react";
import { ChevronRight, ChevronDown, Folder, File } from "lucide-react";
import { TDirectoryNode } from "@/types";
import { isReservedRootEntry } from "@/utils/workspace/directoryTreeHelpers";
import { getFileIcon, getFolderIcon } from "@/utils/workspace/fileIcons";

function isReservedNode(path: string[], name: string): boolean {
  if (path.length < 2) return false;
  const parentName = path[path.length - 2];
  return parentName === "hittable" && (name === "env.json" || name === "env" || name === "notes");
}

type DirectoryTreeNodeProps = {
  node: TDirectoryNode;
  path: string[];
  depth: number;
  expandedDirs: Set<string>;
  selectedPath: string[] | null;
  activePath?: string;
  toggleDir: (path: string) => void;
  onSelect: (path: string[]) => void;
  onOpen: (path: string[], handle: FileSystemFileHandle) => void;
  onContextMenu: (
    e: React.MouseEvent,
    name: string,
    handle: FileSystemFileHandle | FileSystemDirectoryHandle,
    kind: "file" | "directory",
    nodePath: string[]
  ) => void;
  isCreating: { type: "file" | "directory"; parentPath: string[] } | null;
  creatingName: string;
  onCreatingNameChange: (name: string) => void;
  onCreateConfirm: () => void;
  onCreateCancel: () => void;
};

const DirectoryTreeNode = memo(function DirectoryTreeNode({
  node,
  path,
  depth,
  expandedDirs,
  selectedPath,
  activePath,
  toggleDir,
  onSelect,
  onOpen,
  onContextMenu,
  isCreating,
  creatingName,
  onCreatingNameChange,
  onCreateConfirm,
  onCreateCancel,
}: DirectoryTreeNodeProps) {
  const pathStr = path.join("/");
  const isExpanded = expandedDirs.has(pathStr);
  const isSelected = selectedPath?.join("/") === pathStr;
  const isActive = activePath === pathStr;
  const isReserved = depth === 0 && isReservedRootEntry(node.name) || isReservedNode(path, node.name);
  const isDirectory = node.kind === "directory";

  const handleClick = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    onSelect(path);
    if (isDirectory) {
      toggleDir(pathStr);
    } else {
      onOpen(path, node.handle as FileSystemFileHandle);
    }
  }, [path, pathStr, isDirectory, toggleDir, onSelect, onOpen, node]);

  const handleContextMenu = useCallback((e: React.MouseEvent) => {
    if (isReserved) return;
    onContextMenu(e, node.name, node.handle, node.kind, path);
  }, [node, path, isReserved, onContextMenu]);

  const { Icon, className: iconClassName } = isDirectory
    ? getFolderIcon(node.name, isExpanded)
    : getFileIcon(node.name);

  const shouldShowCreateInput = isCreating &&
    isDirectory &&
    isCreating.parentPath.join("/") === path.join("/") &&
    isExpanded;

  return (
    <div role="treeitem" id={`file-${encodeURIComponent(pathStr)}`}
      aria-label={node.name} aria-selected={isSelected}
      aria-expanded={isDirectory ? isExpanded : undefined}>
      <div
        onClick={handleClick}
        onContextMenu={handleContextMenu}
        className={`
          flex items-center gap-2 h-8 cursor-pointer select-none group border-l-2
          transition-colors duration-75
          ${isSelected ? "bg-cyan-400/10 border-cyan-300" : "border-transparent hover:bg-white/5"}
          ${isActive ? "font-medium" : ""}
        `}
        style={{ paddingLeft: `${depth * 16 + 8}px`, paddingRight: "8px" }}
        title={pathStr}
      >
        {isDirectory &&
          (isExpanded ? (
            <ChevronDown className="w-4 h-4 text-white/40 shrink-0" />
          ) : (
            <ChevronRight className="w-4 h-4 text-white/40 shrink-0" />
          ))}

        <Icon className={`w-4 h-4 shrink-0 ${iconClassName}`} />

        <span className="text-[13px] truncate flex-1" style={{ color: isDirectory ? "#dce6f3" : "#b8c5d6" }}>
          {node.name}
        </span>
      </div>

      {isDirectory && isExpanded && node.children && (
        <div role="group">
          {shouldShowCreateInput && (
            <div className="flex items-center gap-1 h-6" style={{ paddingLeft: `${(depth + 1) * 16 + 8}px`, paddingRight: "8px" }}>
              {isCreating.type === "directory" ? (
                <Folder className="w-4 h-4 text-amber-400/60 shrink-0" />
              ) : (
                <File className="w-4 h-4 text-white/30 shrink-0" />
              )}
              <input
                autoFocus
                value={creatingName}
                onChange={(e) => onCreatingNameChange(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") onCreateConfirm();
                  if (e.key === "Escape") onCreateCancel();
                }}
                onBlur={onCreateConfirm}
                className="flex-1 px-1 py-0.5 bg-[#0e1f35] border border-cyan-400/50 rounded text-[13px] text-white/80 focus:outline-none min-w-0"
                placeholder={isCreating.type === "directory" ? "Folder name" : "File name"}
              />
            </div>
          )}

          {node.children.map((child) => (
            <DirectoryTreeNode
              key={child.name}
              node={child}
              path={[...path, child.name]}
              depth={depth + 1}
              expandedDirs={expandedDirs}
              selectedPath={selectedPath}
              activePath={activePath}
              toggleDir={toggleDir}
              onSelect={onSelect}
              onOpen={onOpen}
              onContextMenu={onContextMenu}
              isCreating={isCreating}
              creatingName={creatingName}
              onCreatingNameChange={onCreatingNameChange}
              onCreateConfirm={onCreateConfirm}
              onCreateCancel={onCreateCancel}
            />
          ))}
        </div>
      )}
    </div>
  );
});

export default DirectoryTreeNode;
