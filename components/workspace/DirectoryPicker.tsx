"use client";

import { FolderOpen } from "lucide-react";
import { useWorkspace } from "@/context/workspaceContext";

export default function DirectoryPicker() {
  const { openFolder } = useWorkspace();

  return (
    <div className="flex flex-col items-center justify-center h-full gap-4 p-8 text-center">
      <div className="w-16 h-16 rounded-2xl bg-[#0e1f35] border border-white/10 flex items-center justify-center">
        <FolderOpen className="w-8 h-8 text-cyan-400/50" />
      </div>
      <div className="space-y-2">
        <h2 className="text-lg font-semibold text-white/80">
          Open a Folder
        </h2>
        <p className="text-sm text-white/40 max-w-xs">
          Select a folder on your disk to use as a workspace. A{" "}
          <code className="px-1 py-0.5 bg-white/5 rounded text-cyan-400/70">
            hittable/
          </code>{" "}
          subfolder will be created if it does not exist.
        </p>
      </div>
      <button
        onClick={openFolder}
        className="px-4 py-2 bg-cyan-400/10 text-cyan-400 border border-cyan-400/20 rounded-lg hover:bg-cyan-400/20 transition-colors text-sm font-medium"
      >
        Choose Folder
      </button>
      <p className="text-[10px] text-white/20 max-w-xs">
        Your data stays on your machine. Nothing is uploaded.
      </p>
    </div>
  );
}
