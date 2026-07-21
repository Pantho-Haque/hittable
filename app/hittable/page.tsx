"use client";
import { Suspense } from "react";
import { RequestForm, Selector, ImportModal, InfoModal, NoteModal, AuthModal, HistoryPanel } from "@/components";
import { useShortcuts } from "@/context/ShortcutKeypressProvider";
import { useWorkspace } from "@/context/workspaceContext";
import DirectoryTree from "@/components/workspace/DirectoryTree";
import DirectoryPicker from "@/components/workspace/DirectoryPicker";
import HitFileEditor from "@/components/workspace/HitFileEditor";
import PlainTextEditor from "@/components/workspace/PlainTextEditor";
import MarkdownEditor from "@/components/workspace/MarkdownEditor";

export default function Hittable() {
  const {
    shortcuts: { toggleSidebar },
    toggle,
  } = useShortcuts();
  const { mode, activeFile } = useWorkspace();

  if (mode === "directory") {
    return (
      <div className="h-[calc(100vh-44px)] w-full flex bg-[#080f1a] overflow-hidden font-mono">
        <div className="pointer-events-none fixed inset-0 z-0">
          <div className="absolute top-[-20%] left-[-10%] w-[500px] h-[500px] rounded-full bg-cyan-500/5 blur-[120px]" />
          <div className="absolute bottom-[-20%] right-[-10%] w-[400px] h-[400px] rounded-full bg-cyan-400/4 blur-[100px]" />
        </div>

        <div className="flex h-full w-full relative z-10">
          <div className="w-64 shrink-0 h-full border-r border-white/5 overflow-hidden">
            <DirectoryTree />
          </div>
          <div className="flex-1 h-full overflow-hidden">
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
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="h-[calc(100vh-44px)] w-full flex bg-[#080f1a] overflow-hidden font-mono">
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
        <div className="flex-1 h-full overflow-auto">
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
