"use client";

import { useState } from "react";
import { FileWarning } from "lucide-react";
import { useWorkspace } from "@/context/workspaceContext";
import { humanSize } from "@/utils/workspace/fileKind";

/** Shown instead of an editor for images, binaries and files too large to edit. */
export default function FileViewer() {
  const { activeFile, fileView, fileSize, fileUrl } = useWorkspace();
  const [dims, setDims] = useState<string | null>(null);
  const name = activeFile?.path[activeFile.path.length - 1] ?? "";

  if (fileView.kind === "image" && fileUrl) {
    return (
      <div className="flex flex-1 min-h-0 flex-col items-center justify-center gap-3 p-4">
        <div className="max-h-full max-w-full overflow-auto rounded border border-white/10 bg-[repeating-conic-gradient(#1e293b_0_25%,#0f172a_0_50%)] bg-[length:16px_16px]">
          {/* eslint-disable-next-line @next/next/no-img-element -- a local object URL, not an optimisable asset */}
          <img
            src={fileUrl}
            alt={name}
            className="block max-w-full [image-rendering:pixelated]"
            onLoad={(e) => setDims(`${e.currentTarget.naturalWidth} × ${e.currentTarget.naturalHeight}`)}
          />
        </div>
        <p className="text-xs text-slate-400">
          {fileView.label}
          {dims && ` · ${dims}`} · {humanSize(fileSize)}
        </p>
      </div>
    );
  }

  const text = fileView.kind === "toolarge" ? "Too large to edit" : "Binary file";
  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <div className="flex flex-col items-center gap-2 text-center">
        <FileWarning size={28} className="text-slate-500" aria-hidden />
        <p className="text-sm text-slate-300">
          {text} · {humanSize(fileSize)}
        </p>
        <p className="text-xs text-slate-500">
          {fileView.kind === "toolarge" ? "Over the 1 MB editing limit" : "Not shown as text"}, and never changed from here.
        </p>
      </div>
    </div>
  );
}
