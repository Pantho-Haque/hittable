"use client";

import { useState } from "react";
import { Folder, HardDrive, AlertCircle, Loader2 } from "lucide-react";
import { useWorkspace } from "@/context/workspaceContext";

const isSupported = typeof window !== "undefined" && "showDirectoryPicker" in window;

export default function WorkspaceSwitcher() {
  const { mode, openFolder, disconnectFolder, connectingStage } = useWorkspace();
  const isConnecting = connectingStage !== null;
  const [showTooltip, setShowTooltip] = useState(false);

  const handleToggle = async () => {
    if (mode === "directory") {
      disconnectFolder();
    } else {
      if (!isSupported) {
        setShowTooltip(true);
        setTimeout(() => setShowTooltip(false), 3000);
        return;
      }
      await openFolder();
    }
  };

  return (
    <div className="relative">
      <button
        type="button"
        aria-label={mode === "directory" ? "Switch to local workspace" : "Open a directory workspace"}
        aria-describedby={showTooltip && !isSupported ? "directory-support-hint" : undefined}
        onClick={handleToggle}
        disabled={isConnecting}
        aria-busy={isConnecting}
        onFocus={() => !isSupported && setShowTooltip(true)}
        onBlur={() => setShowTooltip(false)}
        onKeyDown={(event) => { if (event.key === "Escape") setShowTooltip(false); }}
        onMouseEnter={() => !isSupported && setShowTooltip(true)}
        onMouseLeave={() => setShowTooltip(false)}
        className={`
          flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium transition-all
          ${mode === "directory"
            ? "bg-cyan-400/10 text-cyan-400 border border-cyan-400/20"
            : "bg-white/5 text-slate-300 border border-white/15 hover:bg-white/10 hover:text-white"
          }
          ${isConnecting ? "cursor-progress opacity-60" : ""}
        `}
      >
        {isConnecting ? (
          <>
            <Loader2 className="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
            <span>Opening…</span>
          </>
        ) : mode === "directory" ? (
          <>
            <Folder className="w-3.5 h-3.5" />
            <span>Directory</span>
          </>
        ) : (
          <>
            <HardDrive className="w-3.5 h-3.5" />
            <span>Local</span>
          </>
        )}
      </button>

      {showTooltip && !isSupported && (
        <div id="directory-support-hint" role="status" className="absolute top-full mt-2 right-0 z-50 bg-[#0e1f35] border border-white/10 rounded-lg shadow-xl p-3 w-64 max-w-[90vw]">
          <div className="flex items-start gap-2">
            <AlertCircle className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
            <div>
              <p className="text-xs text-white/80 font-medium">
                Browser Not Supported
              </p>
              <p className="text-xs leading-5 text-slate-300 mt-1">
                Directory Mode requires the File System Access API, which is only
                available in Chromium-based browsers (Chrome, Edge, Opera).
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
