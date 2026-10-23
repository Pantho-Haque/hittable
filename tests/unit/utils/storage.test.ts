// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { loadHistory, saveHistory, addHistoryEntry, removeHistoryEntry, clearHistory, formatTimestamp } from "@/utils/historyModifier";
import { loadStore, saveStore, createNote, deleteNoteFromStore, renameNoteInStore, updateNoteContent, filterNotes } from "@/utils/noteModifier";
import type { THistory, THistoryEntry } from "@/types";

const entry = (url: string): Omit<THistoryEntry, "id" | "timestamp"> => ({
  method: "GET", url, status: 200, curlJson: { method: "GET", url, headers: "{}", body: "{}", params: "{}" },
});

beforeEach(() => {
  localStorage.clear();
  vi.useFakeTimers();
});
afterEach(() => vi.useRealTimers());

describe("request history", () => {
  it("adds the newest entry first, with an id and timestamp, and persists it", () => {
    vi.setSystemTime(new Date("2026-01-01T00:00:00Z"));
    let h: THistory = [];
    h = addHistoryEntry(h, entry("https://a.dev"));
    h = addHistoryEntry(h, entry("https://b.dev"));
    expect(h.map((e) => e.url)).toEqual(["https://b.dev", "https://a.dev"]);
    expect(h[0].timestamp).toBe(Date.parse("2026-01-01T00:00:00Z"));
    expect(h[0].id).not.toBe(h[1].id);
    expect(localStorage.getItem("hittable_history")).toBeNull(); // written after the current task
    vi.runAllTimers();
    expect(loadHistory().map((e) => e.url)).toEqual(["https://b.dev", "https://a.dev"]);
  });

  it("keeps only the 100 most recent entries", () => {
    let h: THistory = [];
    for (let i = 0; i < 105; i++) h = addHistoryEntry(h, entry(`https://x.dev/${i}`));
    expect(h).toHaveLength(100);
    expect(h[0].url).toBe("https://x.dev/104");
    expect(h[99].url).toBe("https://x.dev/5");
  });

  it("logs failed requests too (an entry without status)", () => {
    const h = addHistoryEntry([], { ...entry("https://down.dev"), status: undefined, responseJson: { error: "ECONNREFUSED" } });
    expect(h[0]).toMatchObject({ url: "https://down.dev", responseJson: { error: "ECONNREFUSED" } });
  });

  it("removes one entry by id and clears all", () => {
    let h = addHistoryEntry(addHistoryEntry([], entry("a")), entry("b"));
    h = removeHistoryEntry(h, h[0].id);
    expect(h.map((e) => e.url)).toEqual(["a"]);
    vi.runAllTimers();
    expect(loadHistory()).toHaveLength(1);
    expect(clearHistory()).toEqual([]);
    vi.runAllTimers();
    expect(loadHistory()).toEqual([]);
  });

  it("loads [] from empty or corrupt storage", () => {
    expect(loadHistory()).toEqual([]);
    localStorage.setItem("hittable_history", "{not json");
    expect(loadHistory()).toEqual([]);
  });

  it("a full storage quota does not throw", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("full", "QuotaExceededError");
    });
    expect(() => saveHistory([])).not.toThrow();
  });

  it("formats relative times", () => {
    vi.setSystemTime(new Date("2026-03-10T12:00:00Z"));
    const now = Date.now();
    expect(formatTimestamp(now - 10_000)).toBe("just now");
    expect(formatTimestamp(now - 5 * 60_000)).toBe("5m ago");
    expect(formatTimestamp(now - 3 * 3_600_000)).toBe("3h ago");
    expect(formatTimestamp(now - 2 * 86_400_000)).toBe("2d ago");
    expect(formatTimestamp(now - 30 * 86_400_000)).toBe(new Date(now - 30 * 86_400_000).toLocaleDateString());
  });
});

describe("notes store", () => {
  it("starts with the sample notes when nothing is stored, or storage is corrupt", () => {
    expect(Object.values(loadStore()).map((n) => n.title)).toEqual(["micro", "nano"]);
    localStorage.setItem("notesStore", "{bad");
    expect(Object.values(loadStore()).map((n) => n.title)).toEqual(["micro", "nano"]);
  });

  it("creates, renames, edits and deletes notes, persisting every change", () => {
    vi.setSystemTime(1000);
    const { id, updated } = createNote({});
    expect(id).toBe("1000");
    expect(updated[id]).toEqual({ title: "untitled", content: "" });
    let store = renameNoteInStore(updated, id, "API notes");
    store = updateNoteContent(store, id, "# Hello");
    expect(loadStore()).toEqual({ "1000": { title: "API notes", content: "# Hello" } });
    store = deleteNoteFromStore(store, id);
    expect(store).toEqual({});
    expect(loadStore()).toEqual({});
  });

  it("does not mutate the store it was given", () => {
    const store = { a: { title: "t", content: "c" } };
    renameNoteInStore(store, "a", "x");
    deleteNoteFromStore(store, "a");
    expect(store).toEqual({ a: { title: "t", content: "c" } });
  });

  it("filters by title, case-insensitively", () => {
    const store = { a: { title: "Auth Flow", content: "" }, b: { title: "billing", content: "auth" } };
    expect(filterNotes(store, "AUTH").map(([id]) => id)).toEqual(["a"]);
    expect(filterNotes(store, "").map(([id]) => id)).toEqual(["a", "b"]);
  });

  it("a full storage quota does not throw", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("full", "QuotaExceededError");
    });
    expect(() => saveStore({})).not.toThrow();
  });
});
