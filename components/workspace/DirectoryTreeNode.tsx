"use client";

import { useCallback, memo } from "react";
import { ChevronRight, ChevronDown, Folder, File, Lock } from "lucide-react";
import { LOCAL_ENV_PATH, type FileStatus, type WsNode } from "@/services/workspace/backend";
import { getFileIcon, getFolderIcon } from "@/utils/workspace/fileIcons";

// Git status (GitHub workspaces). The letter is always shown, so colour is
// never the only signal.
const STATUS: Record<FileStatus, { label: string; className: string }> = {
  M: { label: "Modified", className: "text-(--warning)" },
  A: { label: "Added", className: "text-(--success)" },
  D: { label: "Deleted", className: "text-(--danger)" },
  C: { label: "Conflict", className: "text-(--danger)" },
};

type DirectoryTreeNodeProps = {
  node: WsNode;
  depth: number;
  status: Record<string, FileStatus>;
  expandedDirs: Set<string>;
  selectedPath: string[] | null;
  activePath?: string;
  toggleDir: (path: string) => void;
  onSelect: (path: string[]) => void;
  onOpen: (path: string[]) => void;
  onContextMenu: (e: React.MouseEvent, node: WsNode) => void;
  isCreating: { type: "file" | "directory"; parentPath: string[] } | null;
  creatingName: string;
  onCreatingNameChange: (name: string) => void;
  onCreateConfirm: () => void;
  onCreateCancel: () => void;
};

const DirectoryTreeNode = memo(function DirectoryTreeNode({
  node,
  depth,
  status,
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
  const { path } = node;
  const pathStr = path.join("/");
  const isExpanded = expandedDirs.has(pathStr);
  const isSelected = selectedPath?.join("/") === pathStr;
  const isActive = activePath === pathStr;
  const isDirectory = node.kind === "directory";
  const fileStatus = isDirectory ? undefined : status[pathStr];
  const hasChanges = isDirectory && Object.keys(status).some((k) => k.startsWith(pathStr + "/"));

  const handleClick = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    onSelect(path);
    if (isDirectory) {
      toggleDir(pathStr);
    } else {
      onOpen(path);
    }
  }, [path, pathStr, isDirectory, toggleDir, onSelect, onOpen]);

  const handleContextMenu = useCallback((e: React.MouseEvent) => {
    onContextMenu(e, node);
  }, [node, onContextMenu]);

  const { Icon, className: iconClassName } = isDirectory
    ? getFolderIcon(node.name, isExpanded)
    : getFileIcon(node.name);

  const shouldShowCreateInput = isCreating &&
    isDirectory &&
    isCreating.parentPath.join("/") === pathStr &&
    isExpanded;

  // env.local.json holds secrets: kept on this device, never committed.
  const localOnly = pathStr === LOCAL_ENV_PATH.join("/");
  const statusText = fileStatus ? STATUS[fileStatus].label : hasChanges ? "Contains changes" : localOnly ? "Local only, never committed" : "";
  const id = `file-${encodeURIComponent(pathStr)}`;

  return (
    <div role="treeitem" id={id}
      aria-label={node.name} aria-describedby={statusText ? `${id}-status` : undefined}
      aria-selected={isSelected}
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
        title={statusText ? `${pathStr} (${statusText})` : pathStr}
      >
        {isDirectory &&
          (isExpanded ? (
            <ChevronDown className="w-4 h-4 text-white/60 shrink-0" />
          ) : (
            <ChevronRight className="w-4 h-4 text-white/60 shrink-0" />
          ))}

        <Icon className={`w-4 h-4 shrink-0 ${iconClassName}`} />

        <span
          className={`text-[13px] truncate flex-1 ${isDirectory ? "text-slate-200" : "text-slate-300"} ${fileStatus === "D" ? "line-through" : ""}`}
        >
          {node.name}
        </span>

        {fileStatus && (
          <span className={`shrink-0 font-mono text-[11px] font-bold ${STATUS[fileStatus].className}`} title={statusText} aria-hidden="true">
            {fileStatus}
          </span>
        )}
        {localOnly && <Lock className="w-3.5 h-3.5 shrink-0 text-(--muted)" aria-hidden="true" />}
        {hasChanges && (
          <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-(--warning)" title={statusText} aria-hidden="true" />
        )}
        {statusText && <span id={`${id}-status`} className="sr-only">{statusText}</span>}
      </div>

      {isDirectory && isExpanded && node.children && (
        <div role="group">
          {shouldShowCreateInput && (
            <div className="flex items-center gap-1 h-6" style={{ paddingLeft: `${(depth + 1) * 16 + 8}px`, paddingRight: "8px" }}>
              {isCreating.type === "directory" ? (
                <Folder className="w-4 h-4 text-amber-400/60 shrink-0" />
              ) : (
                <File className="w-4 h-4 text-white/50 shrink-0" />
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
                className="flex-1 px-1 py-0.5 bg-(--surface-2) border border-cyan-400/50 rounded text-[13px] text-white/80 focus:outline-none min-w-0"
                placeholder={isCreating.type === "directory" ? "Folder name" : "File name"}
                aria-label={isCreating.type === "directory" ? "Folder name" : "File name"}
              />
            </div>
          )}

          {node.children.map((child) => (
            <DirectoryTreeNode
              key={child.name}
              node={child}
              depth={depth + 1}
              status={status}
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
