import type { NoteId, NotesStore } from "@/types";

const STORAGE_KEY = "notesStore";

const defaultStore: NotesStore = {
  "1": { title: "micro", content: "Lorem ipsum dolor sit amet consectetur adipisicing elit." },
  "2": { title: "nano", content: "Lorem, ipsum dolor sit amet consectetur adipisicing elit." },
};

export function loadStore(): NotesStore {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : defaultStore;
  } catch {
    return defaultStore;
  }
}

export function saveStore(store: NotesStore): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(store));
  } catch {
    // Storage quota exceeded - data will not persist across sessions
  }
}

export function createNote(store: NotesStore): { id: NoteId; updated: NotesStore } {
  const id = String(Date.now());
  const updated = { ...store, [id]: { title: "untitled", content: "" } };
  saveStore(updated);
  return { id, updated };
}

export function deleteNoteFromStore(store: NotesStore, id: NoteId): NotesStore {
  const updated = { ...store };
  delete updated[id];
  saveStore(updated);
  return updated;
}

export function renameNoteInStore(store: NotesStore, id: NoteId, title: string): NotesStore {
  const updated = { ...store, [id]: { ...store[id], title } };
  saveStore(updated);
  return updated;
}

export function updateNoteContent(store: NotesStore, id: NoteId, content: string): NotesStore {
  const updated = { ...store, [id]: { ...store[id], content } };
  saveStore(updated);
  return updated;
}

export function filterNotes(store: NotesStore, query: string): [NoteId, NotesStore[NoteId]][] {
  return Object.entries(store).filter(([, note]) =>
    note.title.toLowerCase().includes(query.toLowerCase())
  );
}