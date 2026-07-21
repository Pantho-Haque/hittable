"use client";

import { useState } from "react";
import { Folder, HardDrive, AlertCircle } from "lucide-react";
import { useWorkspace } from "@/context/workspaceContext";

const isSupported = typeof window !== "undefined" && "showDirectoryPicker" in window;

export default function WorkspaceSwitcher() {
  const { mode, openFolder, disconnectFolder } = useWorkspace();
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
        onClick={handleToggle}
        onMouseEnter={() => !isSupported && setShowTooltip(true)}
        onMouseLeave={() => setShowTooltip(false)}
        className={`
          flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium transition-all
          ${mode === "directory"
            ? "bg-cyan-400/10 text-cyan-400 border border-cyan-400/20"
            : "bg-white/5 text-white/50 border border-white/10 hover:bg-white/10 hover:text-white/70"
          }
          ${!isSupported ? "opacity-50 cursor-not-allowed" : "cursor-pointer"}
        `}
        disabled={!isSupported}
      >
        {mode === "directory" ? (
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
        <div className="absolute top-full mt-2 left-0 z-50 bg-[#0e1f35] border border-white/10 rounded-lg shadow-xl p-3 w-64">
          <div className="flex items-start gap-2">
            <AlertCircle className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
            <div>
              <p className="text-xs text-white/80 font-medium">
                Browser Not Supported
              </p>
              <p className="text-[10px] text-white/40 mt-1">
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
