"use client";

import { Folder, Home } from "lucide-react";
import Link from "next/link";
import WorkspaceSwitcher from "@/components/workspace/WorkspaceSwitcher";
import { useWorkspace } from "@/context/workspaceContext";

export default function AppHeader() {
  const { mode, directoryHandle, disconnectFolder } = useWorkspace();

  return (
    <header className="h-11 flex items-center justify-between px-4 bg-[#0a1628] border-b border-white/5 shrink-0">
      <div className="flex items-center gap-4">
        <Link
          href="/"
          className="flex items-center gap-2 text-white/40 hover:text-white/70 transition-colors"
        >
          <Home className="w-4 h-4" />
          <span className="text-xs font-semibold tracking-wider uppercase">
            Hittable
          </span>
        </Link>

        <div className="h-4 w-px bg-white/10" />

        <WorkspaceSwitcher />
      </div>

      <div className="flex items-center gap-3">
        {mode === "directory" && directoryHandle && (
          <div className="flex items-center gap-2 text-[10px] text-white/30">
            <Folder className="w-3 h-3" />
            <span className="max-w-[200px] truncate">
              {directoryHandle.name}
            </span>
          </div>
        )}
      </div>
    </header>
  );
}
