"use client";

import { Dispatch, SetStateAction, useState, useEffect, useRef } from "react";
import { EllipsisVertical } from "lucide-react";
import { THittableCollections, THittableSelectorSelection } from "@/types";
import { DeleteModal, ExportModal, RenameModal } from "@/components";

export default function Menu({
  type,
  collectionCurlList,
  currentName,
  collectionName,
  folderPath,
  exportString,
  setCollections,
  setSelection,
}: {
  type: "collection" | "route" | "folder";
  collectionCurlList: { [key: string]: string[] };
  currentName: string;
  collectionName?: string;
  folderPath?: string[];
  exportString?: string;
  setCollections: Dispatch<SetStateAction<THittableCollections>>;
  setSelection: Dispatch<SetStateAction<THittableSelectorSelection>>;
}) {
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState({ top: 0, left: 0 });
  const triggerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handleClickOutside = (e: MouseEvent) => {
      if ((e.target as HTMLElement).closest("[data-modal]")) return;

      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("click", handleClickOutside);
    return () => document.removeEventListener("click", handleClickOutside);
  }, [open]);

  return (
    <div
      ref={menuRef}
      className="hidden group-hover/menu:flex items-center justify-center shrink-0 pr-1"
    >
      <div
        onClick={() => {
          if (!open && triggerRef.current) {
            const rect = triggerRef.current.getBoundingClientRect();
            setPos({ top: rect.top, left: rect.right + 4 });
          }
          setOpen((prev) => !prev);
        }}
        ref={triggerRef}
        className="p-1 rounded text-white/20 hover:text-cyan-400 hover:bg-cyan-500/10 transition-colors cursor-pointer"
      >
        <EllipsisVertical size={14} />
      </div>

      {open && (
        <div className="absolute left-full top-0 z-999">
          <div
            style={{
              position: "fixed",
              top: pos.top,
              left: pos.left,
              zIndex: 9999,
            }}
            className="flex flex-col min-w-[130px] bg-[#0e1f35] border border-white/10 shadow-2xl shadow-black/60 rounded-lg backdrop-blur-md"
          >
            <RenameModal
              currentName={currentName}
              type={type}
              collectionCurlList={collectionCurlList}
              collectionName={collectionName}
              folderPath={folderPath}
              setCollections={setCollections}
              setSelection={setSelection}
            />
            <div className="h-px bg-white/5 mx-2" />
            <DeleteModal
              currentName={currentName}
              type={type}
              collectionName={collectionName}
              folderPath={folderPath}
              setCollections={setCollections}
              setSelection={setSelection}
            />
            <div className="h-px bg-white/5 mx-2" />
            {exportString && (
              <ExportModal
                exportString={exportString}
                collectionName={currentName}
              />
            )}
          </div>
        </div>
      )}
    </div>
  );
}
