"use client";

import { Save, FolderOpen, ChevronRight, ChevronLeft } from "lucide-react";
import { useState, useMemo } from "react";
import { useRouter } from "next/navigation";
import {
  THittableItem,
} from "@/types";
import {
  createCollectionName,
  createCurlName,
  createFolderName,
  isAlreadyExistsInPath,
} from "@/utils/hittableCollectionModifier";
import { jsonToCurl } from "@/utils/curlConverter";
import { getItemsAtPath } from "@/utils/treeHelpers";
import { ModalInput, ModalShell, ModalActions } from "@/components";
import { useDataContext } from "@/context/dataContext";

function FolderPicker({
  items,
  folderPath,
  onSelect,
  onCreateFolder,
}: {
  items: THittableItem[];
  folderPath: string[];
  onSelect: (folderPath: string[]) => void;
  onCreateFolder: (name: string) => void;
}) {
  const [newFolderName, setNewFolderName] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const currentItems = useMemo(
    () => getItemsAtPath(items, folderPath),
    [items, folderPath],
  );

  const folders = useMemo(
    () => currentItems.filter((i): i is THittableItem & { type: "folder" } => i.type === "folder"),
    [currentItems],
  );

  const handleCreate = () => {
    const trimmed = newFolderName.trim();
    if (!trimmed) return;
    onCreateFolder(trimmed);
    setNewFolderName("");
    setIsCreating(false);
  };

  return (
    <div className="flex flex-col gap-1 mt-1">
      <div className="flex items-center gap-1 text-[10px] text-white/30 uppercase tracking-wider">
        {folderPath.length > 0 && (
          <button
            onClick={() => onSelect(folderPath.slice(0, -1))}
            className="text-white/30 hover:text-white/50 transition-colors cursor-pointer"
            title="Go up one level"
          >
            <ChevronLeft size={10} />
          </button>
        )}
        <span>
          {folderPath.length > 0 ? folderPath[folderPath.length - 1] : "Folders"}
        </span>
      </div>

      {folderPath.length > 0 && (
        <button
          onClick={() => onSelect(folderPath)}
          className="flex items-center gap-2 px-2 py-1.5 rounded text-[11px] text-cyan-400 bg-cyan-500/10 transition-colors cursor-pointer text-left"
        >
          <FolderOpen size={10} className="text-cyan-500/50 shrink-0" />
          Save here
        </button>
      )}

      {folders.map((folder) => (
        <button
          key={folder.name}
          onClick={() => onSelect([...folderPath, folder.name])}
          className="flex items-center gap-2 px-2 py-1.5 rounded text-[11px] text-white/40 hover:text-white/60 hover:bg-white/5 transition-colors cursor-pointer text-left"
        >
          <FolderOpen size={10} className="text-cyan-500/40 shrink-0" />
          {folder.name}
          <ChevronRight size={8} className="ml-auto text-white/20" />
        </button>
      ))}

      {folderPath.length === 0 && (
        <button
          onClick={() => onSelect([])}
          className="flex items-center gap-2 px-2 py-1.5 rounded text-[11px] text-white/40 hover:text-white/60 hover:bg-white/5 transition-colors cursor-pointer text-left"
        >
          <span className="text-white/20">/</span>
          Top level
        </button>
      )}

      {isCreating ? (
        <div className="flex items-center gap-1 px-2">
          <input
            autoFocus
            value={newFolderName}
            onChange={(e) => setNewFolderName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleCreate();
              if (e.key === "Escape") {
                setIsCreating(false);
                setNewFolderName("");
              }
            }}
            placeholder="Folder name"
            className="flex-1 min-w-0 bg-white/5 border border-white/10 rounded px-2 py-1 text-[11px] text-white/70 placeholder-white/20 outline-none focus:border-cyan-500/30"
          />
          <button
            onClick={handleCreate}
            className="text-[10px] text-cyan-400 hover:text-cyan-300 transition-colors cursor-pointer px-1"
          >
            ✓
          </button>
          <button
            onClick={() => { setIsCreating(false); setNewFolderName(""); }}
            className="text-[10px] text-white/30 hover:text-white/50 transition-colors cursor-pointer px-1"
          >
            ✕
          </button>
        </div>
      ) : (
        <button
          onClick={() => setIsCreating(true)}
          className="flex items-center gap-2 px-2 py-1.5 rounded text-[11px] text-cyan-400/60 hover:text-cyan-400 transition-colors cursor-pointer text-left"
        >
          + New folder
        </button>
      )}
    </div>
  );
}

export default function SaveFromHistoryModal() {
  const { collections, setCollections, setSelectorResponse, selectorResponse } = useDataContext();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [routeName, setRouteName] = useState("");
  const [selectedCollection, setSelectedCollection] = useState<string>("");
  const [folderPath, setFolderPath] = useState<string[]>([]);
  const [newCollectionName, setNewCollectionName] = useState("");
  const [isCreatingCollection, setIsCreatingCollection] = useState(false);
  const [error, setError] = useState("");

  const isOrphaned =
    selectorResponse?.collectionName === "" ||
    selectorResponse?.curlName.startsWith("History:");

  const selectedCol = useMemo(
    () => collections.find((c) => c.collectionName === selectedCollection),
    [collections, selectedCollection],
  );

  const handleCreateFolder = (name: string) => {
    if (!selectedCollection) return;
    let folderName = name;
    if (selectedCol && isAlreadyExistsInPath(selectedCol, folderPath, folderName)) {
      folderName += " - New";
    }
    setCollections((prev) =>
      createFolderName(prev, selectedCollection, folderName, folderPath),
    );
    setFolderPath([...folderPath, folderName]);
  };

  const handleOpen = () => {
    setRouteName("");
    setSelectedCollection(collections[0]?.collectionName ?? "");
    setFolderPath([]);
    setNewCollectionName("");
    setIsCreatingCollection(false);
    setError("");
    setOpen(true);
  };

  const handleSave = () => {
    if (!selectorResponse) return;

    let targetCollection = selectedCollection;
    if (isCreatingCollection) {
      const trimmed = newCollectionName.trim();
      if (!trimmed) {
        setError("Collection name is required");
        return;
      }
      let name = trimmed;
      if (collections.some((c) => c.collectionName === name)) {
        name += " - New";
      }
      setCollections((prev) => createCollectionName(prev, name));
      targetCollection = name;
    }

    const trimmedRoute = routeName.trim();
    if (!trimmedRoute) {
      setError("Route name is required");
      return;
    }

    let finalName = trimmedRoute;
    const col = isCreatingCollection
      ? collections.find((c) => c.collectionName === targetCollection)
      : selectedCol;
    if (col && isAlreadyExistsInPath(col, folderPath, finalName)) {
      finalName += " - New";
    }

    const curlString = jsonToCurl(selectorResponse.curlJson);
    setCollections((prev) =>
      createCurlName(prev, targetCollection, finalName, curlString, folderPath),
    );

    const pathParam =
      folderPath.length > 0
        ? `&p=${encodeURIComponent(folderPath.join("/"))}`
        : "";
    router.push(
      `/hittable?c=${encodeURIComponent(targetCollection)}&r=${encodeURIComponent(finalName)}${pathParam}`,
    );

    setSelectorResponse({
      collectionName: targetCollection,
      folderPath,
      curlName: finalName,
      curlJson: selectorResponse.curlJson,
      responseJson: selectorResponse.responseJson,
    });

    setOpen(false);
  };

  if (!isOrphaned) return null;

  return (
    <>
      <button
        onClick={(e) => {
          e.stopPropagation();
          handleOpen();
        }}
        title="Save to collection"
        className="flex items-center gap-1.5 rounded-md border border-white/10 bg-white/5 px-3 py-2 md:py-1.5 text-[8px] md:text-xs font-semibold text-white/40 transition-all cursor-pointer hover:border-cyan-500/30 hover:text-cyan-400 min-h-[44px] md:min-h-0"
      >
        <Save className="h-3 w-3 md:h-3 md:w-3" />
        <span className="hidden sm:inline">Save to…</span>
      </button>

      {open && (
        <ModalShell
          title="Save from History"
          subtitle="Add this request to a collection"
          onClose={() => setOpen(false)}
        >
          {error && <p className="text-[10px] text-red-400 -mt-2">{error}</p>}

          {/* Collection selection */}
          <div className="flex flex-col gap-1.5">
            <span className="text-[10px] text-white/30 uppercase tracking-wider">Collection</span>
            {isCreatingCollection ? (
              <div className="flex flex-col gap-1.5">
                <ModalInput
                  autoFocus
                  value={newCollectionName}
                  onChange={(v) => {
                    setNewCollectionName(v);
                    setError("");
                  }}
                  placeholder="New collection name"
                  onKeyDown={(e) => {
                    if (e.key === "Enter") handleSave();
                    if (e.key === "Escape") setOpen(false);
                  }}
                />
                <button
                  onClick={() => setIsCreatingCollection(false)}
                  className="text-[10px] text-white/30 hover:text-white/50 transition-colors cursor-pointer text-left"
                >
                  ← Choose existing collection
                </button>
              </div>
            ) : (
              <div className="flex flex-col gap-1">
                <div className="flex flex-col gap-0.5 max-h-[120px] overflow-y-auto rounded-md border border-white/5 bg-white/3">
                  {collections.map((col) => (
                    <button
                      key={col.collectionName}
                      onClick={() => {
                        setSelectedCollection(col.collectionName);
                        setFolderPath([]);
                        setError("");
                      }}
                      className={`w-full text-left px-2.5 py-1.5 text-[11px] transition-colors cursor-pointer ${
                        selectedCollection === col.collectionName
                          ? "text-cyan-400 bg-cyan-500/10"
                          : "text-white/50 hover:text-white/80 hover:bg-white/5"
                      }`}
                    >
                      {col.collectionName}
                    </button>
                  ))}
                </div>
                <button
                  onClick={() => setIsCreatingCollection(true)}
                  className="text-[10px] text-cyan-400/60 hover:text-cyan-400 transition-colors cursor-pointer text-left"
                >
                  + Create new collection
                </button>
              </div>
            )}
          </div>

          {/* Folder selection */}
          {!isCreatingCollection && selectedCol && (
            <div className="flex flex-col gap-1.5">
              <span className="text-[10px] text-white/30 uppercase tracking-wider">
                Folder <span className="text-white/15">(optional)</span>
              </span>
              <div className="rounded-md border border-white/5 bg-white/3 p-1.5 max-h-[100px] overflow-y-auto">
                <FolderPicker
                  items={selectedCol.items}
                  folderPath={folderPath}
                  onSelect={setFolderPath}
                  onCreateFolder={handleCreateFolder}
                />
              </div>
            </div>
          )}

          {/* Route name */}
          <div className="flex flex-col gap-1.5">
            <span className="text-[10px] text-white/30 uppercase tracking-wider">Route name</span>
            <ModalInput
              value={routeName}
              onChange={(v) => {
                setRouteName(v);
                setError("");
              }}
              placeholder="e.g. get-users"
              onKeyDown={(e) => {
                if (e.key === "Enter") handleSave();
                if (e.key === "Escape") setOpen(false);
              }}
            />
          </div>

          <ModalActions
            onCancel={() => setOpen(false)}
            onConfirm={handleSave}
            confirmLabel="Save"
          />
        </ModalShell>
      )}
    </>
  );
}
