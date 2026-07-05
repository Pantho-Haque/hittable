"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Pencil, Trash2, Check, X, Plus, FileText, Search } from "lucide-react";
import { createNote, deleteNoteFromStore, filterNotes, renameNoteInStore } from "@/utils/noteModifier";
import type { NoteId, NotesStore } from "@/types";

export default function NotePills({
  noteStore,
  setNoteStore,
  selectedId,
  setSelectedId,
  isUnsaved,
  setEditContent,
  setOriginalContent,
}: {
  noteStore: NotesStore;
  setNoteStore: (store: NotesStore) => void;
  selectedId: NoteId | null;
  setSelectedId: (id: NoteId | null) => void;
  isUnsaved: boolean;
  setEditContent: (v: string) => void;
  setOriginalContent: (v: string) => void;
}) {

  const [renamingId, setRenamingId] = useState<NoteId | null>(null);
  const renameRef = useRef<HTMLInputElement>(null);
  const [renameValue, setRenameValue] = useState("");
  useEffect(() => {
    if (renamingId && renameRef.current) renameRef.current.focus();
  }, [renamingId]);

  const [search, setSearch] = useState("");
  const filtered = useCallback(
    () => filterNotes(noteStore, search),
    [noteStore, search],
  );

  function selectNote(id: NoteId) {
    if (isUnsaved && selectedId !== id) {
      if (!confirm("You have unsaved changes. Discard?")) return;
    }
    setSelectedId(id);
    setEditContent(noteStore[id].content);
    setOriginalContent(noteStore[id].content);
    setRenamingId(null);
  }

  function addNote() {
    const { id, updated } = createNote(noteStore);
    setNoteStore(updated);
    selectNote(id);
    setTimeout(() => {
      setRenamingId(id);
      setRenameValue("untitled");
    }, 50);
  }

  function handleDelete(id: NoteId) {
    if (!confirm("Delete this note?")) return;
    const updated = deleteNoteFromStore(noteStore, id);
    setNoteStore(updated);
    if (selectedId === id) {
      setSelectedId(null);
      setEditContent("");
      setOriginalContent("");
    }
  }

  function startRename(id: NoteId) {
    setRenamingId(id);
    setRenameValue(noteStore[id].title);
  }

  function confirmRename() {
    if (!renamingId || !renameValue.trim()) return;
    const update = renameNoteInStore(noteStore, renamingId, renameValue.trim());
    setNoteStore(update);
    setRenamingId(null);
  }

  function getPreview(content: string): string {
    const plain = content.replace(/[#*`>\-\[\]()!]/g, "").trim();
    return plain.length > 60 ? plain.slice(0, 60) + "..." : plain;
  }

  return (
    <>
      {/* Search */}
      <div className="relative">
        <Search size={12} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-neutral-400 pointer-events-none" />
        <input
          type="text"
          placeholder="Search notes..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-full pl-7 pr-2.5 py-1.5 text-xs rounded-lg border border-neutral-200 dark:border-neutral-700 bg-neutral-50 dark:bg-neutral-800 text-neutral-900 dark:text-neutral-100 outline-none focus:border-cyan-500/50 transition-colors"
        />
      </div>

      {/* Note list */}
      <div className="flex flex-col gap-1 flex-1 overflow-y-auto">
        {filtered().length === 0 && (
          <div className="flex flex-col items-center justify-center py-6 text-neutral-400">
            <FileText size={20} className="mb-2 opacity-40" />
            <p className="text-xs">{search ? "No matching notes" : "No notes yet"}</p>
          </div>
        )}

        {filtered().map(([id, note]) => {
          const isSelected = selectedId === id;
          const isRenaming = renamingId === id;
          const preview = getPreview(note.content);

          return (
            <div
              key={id}
              onClick={() => !isRenaming && selectNote(id)}
              className={`
                group relative flex flex-col gap-1 px-2.5 py-2 rounded-lg border cursor-pointer transition-all
                ${
                  isSelected
                    ? "border-cyan-500/40 bg-cyan-500/8"
                    : "border-transparent hover:border-white/10 hover:bg-white/3"
                }
              `}
            >
              {isRenaming ? (
                <input
                  ref={renameRef}
                  value={renameValue}
                  onChange={(e) => setRenameValue(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") confirmRename();
                    if (e.key === "Escape") setRenamingId(null);
                  }}
                  onClick={(e) => e.stopPropagation()}
                  className="w-full text-xs font-medium bg-transparent border-b border-cyan-500/40 outline-none text-white/90"
                />
              ) : (
                <>
                  <div className="flex items-center gap-2">
                    <FileText size={11} className={`shrink-0 ${isSelected ? "text-cyan-400" : "text-white/20"}`} />
                    <span
                      className={`text-xs font-medium truncate ${isSelected ? "text-cyan-300" : "text-white/60"}`}
                    >
                      {note.title}
                    </span>
                  </div>
                  {preview && (
                    <p className="text-[10px] text-white/20 truncate pl-[19px] leading-relaxed">
                      {preview}
                    </p>
                  )}
                </>
              )}

              {/* action row — only when selected */}
              {isSelected && !isRenaming && (
                <div
                  className="flex justify-end gap-1 mt-0.5"
                  onClick={(e) => e.stopPropagation()}
                >
                  <button
                    onClick={() => startRename(id)}
                    title="Rename"
                    className="text-white/20 hover:text-cyan-400 transition-colors cursor-pointer p-1 rounded hover:bg-white/5"
                  >
                    <Pencil size={11} />
                  </button>
                  <button
                    onClick={() => handleDelete(id)}
                    title="Delete"
                    className="text-white/20 hover:text-red-400 transition-colors cursor-pointer p-1 rounded hover:bg-white/5"
                  >
                    <Trash2 size={11} />
                  </button>
                </div>
              )}

              {/* rename confirm/cancel */}
              {isRenaming && (
                <div
                  className="flex justify-end gap-1 mt-0.5"
                  onClick={(e) => e.stopPropagation()}
                >
                  <button
                    onClick={confirmRename}
                    title="Confirm rename"
                    className="text-white/30 hover:text-green-400 transition-colors cursor-pointer p-1 rounded hover:bg-white/5"
                  >
                    <Check size={11} />
                  </button>
                  <button
                    onClick={() => setRenamingId(null)}
                    title="Cancel"
                    className="text-white/30 hover:text-white/60 transition-colors cursor-pointer p-1 rounded hover:bg-white/5"
                  >
                    <X size={11} />
                  </button>
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* add new — pinned to bottom */}
      <button
        onClick={addNote}
        className="flex items-center gap-1.5 px-2.5 py-2 rounded-lg border border-dashed border-white/10 text-white/30 hover:border-cyan-500/30 hover:text-cyan-400 transition-all text-xs w-full cursor-pointer"
      >
        <Plus size={11} />
        New note
      </button>
    </>
  );
}
