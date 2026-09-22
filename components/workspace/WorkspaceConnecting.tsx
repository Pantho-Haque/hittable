"use client";

import { FolderOpen, Loader2 } from "lucide-react";
import { connectingLabel, type TConnectingStage } from "@/context/workspaceContext";

const STAGE_ORDER: TConnectingStage[] = ["preparing", "scanning"];

/**
 * Covers the gap between the folder picker closing and directory mode being
 * ready. Both steps are unbounded — scaffolding writes files, and the tree walk
 * recurses through the whole folder — so the stage is named rather than shown
 * as an anonymous spinner.
 */
export default function WorkspaceConnecting({ stage }: { stage: TConnectingStage }) {
  const isReconnecting = stage === "reconnecting";
  const currentIndex = STAGE_ORDER.indexOf(stage);

  return (
    <div
      role="status"
      aria-live="polite"
      className="absolute inset-0 z-50 flex flex-col items-center justify-center gap-5 bg-[#080f1a]/90 p-8 text-center backdrop-blur-sm"
    >
      <div className="relative flex h-16 w-16 items-center justify-center rounded-2xl border border-white/10 bg-[#0e1f35]">
        <FolderOpen className="h-7 w-7 text-cyan-300" aria-hidden="true" />
        <Loader2
          className="absolute -bottom-1 -right-1 h-6 w-6 animate-spin rounded-full bg-[#0e1f35] p-1 text-cyan-400"
          aria-hidden="true"
        />
      </div>

      <div className="space-y-1.5">
        <p className="text-base font-medium text-slate-100">{connectingLabel(stage)}</p>
        <p className="max-w-xs text-sm leading-6 text-slate-400">
          {isReconnecting
            ? "Restoring the workspace you had open."
            : "Large folders take a moment to scan."}
        </p>
      </div>

      {!isReconnecting && (
        <ol className="flex items-center gap-2" aria-hidden="true">
          {STAGE_ORDER.map((entry, index) => (
            <li
              key={entry}
              className={`h-1 w-10 rounded-full transition-colors duration-300 ${
                index <= currentIndex ? "bg-cyan-400/70" : "bg-white/10"
              }`}
            />
          ))}
        </ol>
      )}
    </div>
  );
}
