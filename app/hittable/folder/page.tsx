"use client";
import { useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { FolderOpen, X } from "lucide-react";
import { useWorkspace } from "@/context/workspaceContext";
import WorkspaceShell from "@/components/workspace/WorkspaceShell";

export default function FolderWorkspace() {
  const { backendKind, label, connectingStage, reconnectFolder, closeWorkspace } = useWorkspace();
  const router = useRouter();
  const tried = useRef(false);

  // Opened directly (a reload or a bookmark): reopen the last folder if the
  // browser kept its permission; otherwise the launcher offers Reconnect.
  useEffect(() => {
    if (backendKind === "folder" || connectingStage) return;
    if (tried.current) {
      router.replace("/hittable");
      return;
    }
    tried.current = true;
    void reconnectFolder().then((ok) => {
      if (!ok) router.replace("/hittable");
    });
  }, [backendKind, connectingStage, reconnectFolder, router]);

  return (
    <WorkspaceShell
      header={
        <>
          <span className="flex min-w-0 items-center gap-2 text-xs text-white/70">
            <FolderOpen size={14} className="shrink-0 text-(--brand)" aria-hidden="true" />
            <span className="truncate">{label || "Folder"}</span>
          </span>
          <button
            type="button"
            className="workspace-button ml-auto"
            onClick={() => {
              closeWorkspace();
              router.push("/hittable");
            }}
          >
            <X size={14} aria-hidden="true" /> Close
          </button>
        </>
      }
    />
  );
}
