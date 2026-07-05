"use client";

import { Upload } from "lucide-react";
import { useCallback, useState } from "react";
import { ModalShell, ModalActions } from "@/components";
import { compressString } from "@/utils/compressString";
import { THittableCurl } from "@/types";

export default function ExportModal({
  exportString,
  collectionName,
}: {
  exportString: string;
  collectionName: string;
}) {
  const [open, setOpen] = useState(false);
  const [error, setError] = useState("");

  const compressed = useCallback(() => {
    try {
      const parsed = JSON.parse(exportString);
      const stripped = {
        ...parsed,
        curls: parsed.curls?.map((c: THittableCurl) => ({ ...c, response: "" })),
      };
      return compressString(JSON.stringify(stripped));
    } catch {
      return null;
    }
  }, [exportString]);

  const copyToClipboard = () => {
    const code = compressed();
    if (!code) {
      setError("Failed to generate export code");
      return;
    }
    navigator.clipboard.writeText(code);
    setOpen(false);
  };

  return (
    <>
      <button
        onClick={(e) => {
          e.stopPropagation();
          setError("");
          setOpen(true);
        }}
        className="flex items-center gap-2 px-3 py-2 text-xs text-white/50 hover:bg-white/5 hover:text-cyan-400 transition-colors w-full text-left cursor-pointer"
      >
        <Upload size={12} />
        Export
      </button>

      {open && (
        <ModalShell
          title={`Export`}
          subtitle={`Copy this code to Import ${collectionName} anytime`}
          onClose={() => setOpen(false)}
        >
          <div className="w-full h-full flex flex-col gap-2">
            {error && <p className="text-[10px] text-red-400">{error}</p>}
            <div className="w-full h-full overflow-scroll">
              <p className="text-xs text-white/50">{compressed() ?? "Error generating export"}</p>
            </div>
          </div>
          <ModalActions
            onCancel={() => setOpen(false)}
            onConfirm={copyToClipboard}
            confirmLabel="Copy"
          />
        </ModalShell>
      )}
    </>
  );
}
