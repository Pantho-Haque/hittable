"use client";
import { Suspense } from "react";
import { PanelLeft } from "lucide-react";
import { RequestForm, Selector, ImportModal, InfoModal, NoteModal, AuthModal, HistoryPanel } from "@/components";
import { useShortcuts } from "@/context/ShortcutKeypressProvider";
import { useWorkspace } from "@/context/workspaceContext";
import DirectoryTree from "@/components/workspace/DirectoryTree";
import DirectoryPicker from "@/components/workspace/DirectoryPicker";
import HitFileEditor from "@/components/workspace/HitFileEditor";
import PlainTextEditor from "@/components/workspace/PlainTextEditor";
import MarkdownEditor from "@/components/workspace/MarkdownEditor";
import WorkspaceConnecting from "@/components/workspace/WorkspaceConnecting";

export default function Hittable() {
  const {
    shortcuts: { toggleSidebar },
    toggle,
  } = useShortcuts();
  const { mode, activeFile, connectingStage } = useWorkspace();

  if (mode === "directory") {
    return (
      <div className="workspace relative h-[calc(100dvh-44px)] w-full flex flex-col bg-[#0c1422] overflow-hidden">
        {connectingStage && <WorkspaceConnecting stage={connectingStage} />}
        <div className="flex min-h-11 shrink-0 items-center gap-3 border-b border-white/10 px-3">
          <button className="workspace-button" onClick={() => toggle("toggleSidebar")}
            aria-expanded={!toggleSidebar} aria-controls="directory-explorer" title="Toggle explorer (Ctrl/Cmd+B)">
            <PanelLeft size={16} aria-hidden="true" /> Explorer
          </button>
          <span className="text-xs text-slate-400">Directory workspace</span>
        </div>
        <div className="flex min-h-0 flex-1 w-full relative">
          <aside id="directory-explorer" aria-label="File explorer"
            className={`${toggleSidebar ? "hidden" : "block"} w-48 md:w-64 max-w-[45vw] shrink-0 h-full border-r border-white/10 overflow-hidden`}>
            <DirectoryTree />
          </aside>
          <main className="flex-1 min-w-0 h-full overflow-hidden" aria-label="File editor">
            <div className="h-full w-full flex flex-col">
              {activeFile ? (
                activeFile.kind === "hit" ? (
                  <HitFileEditor />
                ) : activeFile.kind === "markdown" ? (
                  <MarkdownEditor />
                ) : (
                  <PlainTextEditor />
                )
              ) : (
                <DirectoryPicker />
              )}
            </div>
          </main>
        </div>
      </div>
    );
  }

  return (
    <div className="workspace relative h-[calc(100dvh-44px)] w-full flex bg-[#080f1a] overflow-hidden font-sans">
      {connectingStage && <WorkspaceConnecting stage={connectingStage} />}
      {/* Ambient background glow */}
      <div className="pointer-events-none fixed inset-0 z-0">
        <div className="absolute top-[-20%] left-[-10%] w-[500px] h-[500px] rounded-full bg-cyan-500/5 blur-[120px]" />
        <div className="absolute bottom-[-20%] right-[-10%] w-[400px] h-[400px] rounded-full bg-cyan-400/4 blur-[100px]" />
      </div>

      {/* Mobile backdrop when sidebar is expanded */}
      {!toggleSidebar && (
        <div
          className="md:hidden fixed inset-0 z-30 bg-black/40"
          onClick={() => toggle("toggleSidebar")}
        />
      )}

      <div className="flex h-full w-full relative z-10">
        <Suspense fallback={null}>
          <Selector />
        </Suspense>
        <div className="flex-1 min-w-0 h-full overflow-auto">
          <div className="h-full w-full flex flex-col p-2 md:p-6">
            <RequestForm/>
          </div>
        </div>

        {!toggleSidebar && (<div className="hidden md:flex w-12 h-full flex-col border-l border-white/5 bg-[#0a1628]/80 items-center justify-start">
          <ImportModal />
          <AuthModal />
          <HistoryPanel />
          <NoteModal />
          <InfoModal />
        </div>)}
      </div>
    </div>
  );
}
