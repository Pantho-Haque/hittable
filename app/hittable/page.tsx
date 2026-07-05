"use client";
import { Suspense } from "react";
import { RequestForm, Selector, ImportModal, InfoModal, NoteModal, AuthModal, HistoryPanel } from "@/components";
import { useShortcuts } from "@/context/ShortcutKeypressProvider";

export default function Hittable() {

  const {
    shortcuts: { toggleSidebar },
    toggle,
  } = useShortcuts();
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

      <div className="flex h-full w-full">
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

