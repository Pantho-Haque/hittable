"use client";

import { Import } from "lucide-react";
import { useState } from "react";
import { ModalShell, ModalActions } from "@/components";
import { decompressString } from "@/utils/compressString";
import { THittableCollection } from "@/types";
import { useDataContext } from "@/context/dataContext";
import { useNotification } from "@/hooks";

export default function ImportModal() {

  const { collections, setCollections } = useDataContext();
  const toast = useNotification();

  const [open, setOpen] = useState(false);
  const [compressedString, setCompressedString] = useState("");
  const [error, setError] = useState("");

  const getUniqueName = (baseName: string) => {
    const existingNames = collections.map((c) => c.collectionName);
    if (!existingNames.includes(baseName)) return baseName;
    let counter = 1;
    while (existingNames.includes(`${baseName} - ${counter}`)) {
      counter++;
    }
    return `${baseName} - ${counter}`;
  };

  const importCollection = () => {
    setError("");
    try {
      const decompressed = decompressString(compressedString);
      let parsed: THittableCollection;
      try {
        parsed = JSON.parse(decompressed) as THittableCollection;
      } catch {
        setError("Invalid data format");
        return;
      }

      if (!parsed || typeof parsed.collectionName !== "string" || !Array.isArray(parsed.curls)) {
        setError("Invalid collection structure");
        return;
      }

      const uniqueName = getUniqueName(parsed.collectionName);
      setCollections((prev) => [...prev, {...parsed, collectionName: uniqueName}]);
      setOpen(false);
      setCompressedString("");
      toast.success({ title: "Imported", desc: `Collection "${uniqueName}" imported successfully` });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to import");
    }
  };

  return (
    <>
      <button
        onClick={(e) => {
          e.stopPropagation();
          setError("");
          setOpen(true);
        }}
        className="modal-button-mini mt-2"
      >
        <Import size={14} />
      </button>

      {open && (
        <ModalShell
          title={`Import`}
          subtitle={`Paste the code here to add to collections`}
          onClose={() => setOpen(false)}
        >
          <div className="w-full h-full flex flex-col gap-2">
            {error && <p className="text-[10px] text-red-400">{error}</p>}
            <textarea
              value={compressedString}
              onChange={(e) => { setCompressedString(e.target.value); setError(""); }}
              className="w-full h-full bg-transparent border border-white/10 rounded-lg p-2 text-xs text-white/50 focus:outline-none focus:border-cyan-500/50 resize-none"
              placeholder="Paste collection code here..."
            />
          </div>
          <ModalActions
            onCancel={() => setOpen(false)}
            onConfirm={importCollection}
            confirmLabel="Import"
          />
        </ModalShell>
      )}
    </>
  );
}
