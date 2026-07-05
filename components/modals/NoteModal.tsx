"use client";

import { NotebookIcon } from "lucide-react";
import { useState } from "react";
import { NotePills, NoteEditor } from "@/components";
import { loadStore, updateNoteContent } from "@/utils/noteModifier";
import type { NoteId, NotesStore } from "@/types";
import { ModalActions } from "@/components";

export default function NoteModal() {
  const [noteStore, setNoteStore] = useState<NotesStore>(() => loadStore());
  const [open, setOpen] = useState(false);
  const [editContent, setEditContent] = useState("");
  const [originalContent, setOriginalContent] = useState("");
  const [selectedId, setSelectedId] = useState<NoteId | null>(null);

  const isUnsaved = selectedId !== null && editContent !== originalContent;

  function saveNote() {
    if (!selectedId) return;
    const updated = updateNoteContent(noteStore, selectedId, editContent);
    setNoteStore(updated);
    setOriginalContent(editContent);
  }

  return (
    <>
      <button
        onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        className="modal-button-mini mt-2"
      >
        <NotebookIcon size={12} />
      </button>

      {open && (
        <div
          data-modal
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          onMouseDown={(e) => {
            if (e.target === e.currentTarget) setOpen(false);
          }}
        >
          <div
            className="relative bg-[#0a1628] border border-white/10 rounded-xl shadow-2xl shadow-black/80 w-[95vw] max-w-[1100px] h-[85vh] flex flex-col font-mono outline-none"
            style={{ boxShadow: "0 0 0 1px rgba(0,229,204,0.08), 0 24px 80px rgba(0,0,0,0.8)" }}
            onMouseDown={(e) => e.stopPropagation()}
          >
            {/* Corner brackets */}
            <span className="absolute top-0 left-0 w-4 h-4 border-t border-l border-cyan-500/30 rounded-tl-xl" aria-hidden="true" />
            <span className="absolute top-0 right-0 w-4 h-4 border-t border-r border-cyan-500/30 rounded-tr-xl" aria-hidden="true" />
            <span className="absolute bottom-0 left-0 w-4 h-4 border-b border-l border-cyan-500/30 rounded-bl-xl" aria-hidden="true" />
            <span className="absolute bottom-0 right-0 w-4 h-4 border-b border-r border-cyan-500/30 rounded-br-xl" aria-hidden="true" />

            {/* Header */}
            <div className="flex items-center justify-between px-5 py-3 border-b border-white/5 shrink-0">
              <div>
                <p className="text-[9px] tracking-[0.3em] uppercase text-cyan-500/60 mb-0.5">
                  Hittable
                </p>
                <h2 className="text-sm font-bold text-white/90">Notebook</h2>
              </div>
              <button
                onClick={() => setOpen(false)}
                className="text-white/20 hover:text-white/50 transition-colors cursor-pointer p-1"
                title="Close"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M18 6 6 18" /><path d="m6 6 12 12" />
                </svg>
              </button>
            </div>

            {/* Content */}
            <div className="flex flex-1 min-h-0 gap-0">
              {/* left — pills */}
              <div className="w-52 shrink-0 flex flex-col gap-2 border-r border-white/5 p-3 min-h-0">
                <NotePills
                  noteStore={noteStore}
                  setNoteStore={setNoteStore}
                  selectedId={selectedId}
                  setSelectedId={setSelectedId}
                  isUnsaved={isUnsaved}
                  setEditContent={setEditContent}
                  setOriginalContent={setOriginalContent}
                />
              </div>

              {/* right — editor */}
              <div className="flex-1 min-w-0 flex flex-col min-h-0 p-2">
                <NoteEditor
                  selectedId={selectedId}
                  selectedTitle={selectedId ? noteStore[selectedId]?.title : null}
                  content={editContent}
                  isUnsaved={isUnsaved}
                  onChange={setEditContent}
                />
              </div>
            </div>

            {/* Footer */}
            <div className="px-5 py-2.5 border-t border-white/5 shrink-0">
              <ModalActions
                onCancel={() => {
                  setSelectedId(null);
                  setEditContent("");
                  setOriginalContent("");
                  setOpen(false);
                }}
                onConfirm={isUnsaved ? saveNote : () => setOpen(false)}
                confirmLabel={isUnsaved ? "Save" : "Close"}
              />
            </div>
          </div>
        </div>
      )}
    </>
  );
}
