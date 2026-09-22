"use client";

import { FolderOpen, Loader2 } from "lucide-react";
import { useWorkspace } from "@/context/workspaceContext";

export default function DirectoryPicker() {
  const { openFolder, directoryHandle, connectingStage } = useWorkspace();
  const isConnecting = connectingStage !== null;

  return (
    <div className="flex flex-col items-center justify-center h-full gap-4 p-8 text-center">
      <div className="w-16 h-16 rounded-2xl bg-[#0e1f35] border border-white/10 flex items-center justify-center">
        <FolderOpen className="w-8 h-8 text-cyan-300" />
      </div>
      <div className="space-y-2">
        <h2 className="text-2xl font-semibold tracking-tight text-slate-100">
          {directoryHandle ? "Your workspace is ready" : "Open a workspace"}
        </h2>
        <p className="text-sm leading-6 text-slate-400 max-w-sm">
          {directoryHandle ? "Select a file in the explorer to edit a request, environment, or note." : <>Select a folder on your disk to use as a workspace. A{" "}
          <code className="px-1 py-0.5 bg-white/5 rounded text-cyan-400/70">
            hittable/
          </code>{" "}
          subfolder will be created if it does not exist.</>}
        </p>
      </div>
      <button
        onClick={openFolder}
        disabled={isConnecting}
        aria-busy={isConnecting}
        className="inline-flex items-center gap-2 px-4 py-2 bg-cyan-400/10 text-cyan-400 border border-cyan-400/20 rounded-lg hover:bg-cyan-400/20 transition-colors text-sm font-medium disabled:cursor-progress disabled:opacity-60"
      >
        {isConnecting && <Loader2 className="w-4 h-4 animate-spin" aria-hidden="true" />}
        {isConnecting ? "Opening…" : directoryHandle ? "Change folder" : "Choose folder"}
      </button>
      <p className="text-xs leading-5 text-slate-400 max-w-xs">
        File edits are saved to your selected folder.
      </p>
    </div>
  );
}
