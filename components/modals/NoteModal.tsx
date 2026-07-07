"use client";

import { NotebookIcon } from "lucide-react";
import { useState, useEffect, useCallback } from "react";
import { NotePills, NoteEditor, ModalShell } from "@/components";
import { loadStore, updateNoteContent } from "@/utils/noteModifier";
import type { NoteId, NotesStore } from "@/types";
import { ModalActions } from "@/components";

export default function NoteModal() {
  const [noteStore, setNoteStore] = useState<NotesStore>(() => loadStore());
  const [open, setOpen] = useState(false);
  const [editContent, setEditContent] = useState("");
  const [originalContent, setOriginalContent] = useState("");
  const [selectedId, setSelectedId] = useState<NoteId | null>(null);
  const [showUnsavedDialog, setShowUnsavedDialog] = useState(false);

  const isUnsaved = selectedId !== null && editContent !== originalContent;

  const saveNote = useCallback(() => {
    if (!selectedId) return;
    const updated = updateNoteContent(noteStore, selectedId, editContent);
    setNoteStore(updated);
    setOriginalContent(editContent);
  }, [selectedId, noteStore, editContent]);

  const discardChanges = useCallback(() => {
    if (selectedId) {
      setEditContent(originalContent);
    }
    setSelectedId(null);
    setEditContent("");
    setOriginalContent("");
    setOpen(false);
    setShowUnsavedDialog(false);
  }, [selectedId, originalContent]);

  const saveAndClose = useCallback(() => {
    saveNote();
    setSelectedId(null);
    setEditContent("");
    setOriginalContent("");
    setOpen(false);
    setShowUnsavedDialog(false);
  }, [saveNote]);

  const cancelClose = useCallback(() => {
    setShowUnsavedDialog(false);
  }, []);

  const handleCloseAttempt = useCallback(() => {
    if (isUnsaved) {
      setShowUnsavedDialog(true);
    } else {
      setSelectedId(null);
      setEditContent("");
      setOriginalContent("");
      setOpen(false);
    }
  }, [isUnsaved]);

  const handleBackdropClick = useCallback(() => {
    if (isUnsaved) {
      setShowUnsavedDialog(true);
    } else {
      setOpen(false);
    }
  }, [isUnsaved]);

  // Cmd/Ctrl+S saves note when modal is open, prevents UrlBar handler from firing
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "s") {
        e.preventDefault();
        e.stopPropagation();
        saveNote();
      }
      if (e.key === "Escape") {
        if (isUnsaved) {
          e.preventDefault();
          e.stopPropagation();
          setShowUnsavedDialog(true);
        }
      }
    };
    window.addEventListener("keydown", handler, true);
    return () => window.removeEventListener("keydown", handler, true);
  }, [open, saveNote, isUnsaved]);

  return (
    <>
      <button
        onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        className="modal-button-mini mt-2"
      >
        <NotebookIcon size={14} />
      </button>

      {open && (
        <div
          data-modal
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          onMouseDown={(e) => {
            if (e.target === e.currentTarget) handleBackdropClick();
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
                onClick={handleCloseAttempt}
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
                onCancel={handleCloseAttempt}
                onConfirm={isUnsaved ? saveNote : () => setOpen(false)}
                confirmLabel={isUnsaved ? "Save" : "Close"}
              />
            </div>
          </div>
        </div>
      )}

      {/* Unsaved changes confirmation dialog */}
      {showUnsavedDialog && (
        <ModalShell
          title="Unsaved Changes"
          subtitle="You have unsaved changes in this note"
          onClose={cancelClose}
          size="sm"
        >
          <p className="text-xs text-white/40 leading-relaxed">
            What would you like to do with your unsaved changes?
          </p>
          <div className="flex justify-end gap-2 pt-2">
            <button
              onClick={cancelClose}
              className="px-4 py-1.5 text-xs rounded-md border border-white/10 text-white/40 hover:bg-white/5 hover:text-white/70 transition-colors cursor-pointer"
            >
              Cancel
            </button>
            <button
              onClick={discardChanges}
              className="px-4 py-1.5 text-xs rounded-md border border-red-500/20 text-red-400/80 hover:bg-red-500/10 hover:text-red-400 transition-colors cursor-pointer"
            >
              Discard
            </button>
            <button
              onClick={saveAndClose}
              className="px-4 py-1.5 text-xs rounded-md font-bold cursor-pointer active:scale-95 transition-all"
              style={{
                background: "rgba(0,229,204,0.15)",
                border: "1px solid rgba(0,229,204,0.3)",
                color: "#00e5cc",
              }}
            >
              Save & Close
            </button>
          </div>
        </ModalShell>
      )}
    </>
  );
}
