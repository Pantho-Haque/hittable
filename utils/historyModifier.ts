import { THistory, THistoryEntry } from "@/types";

const STORAGE_KEY = "hittable_history";
const MAX_ENTRIES = 100;

export function loadHistory(): THistory {
  if (typeof window === "undefined") return [];
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) return [];
    return JSON.parse(stored) as THistory;
  } catch {
    return [];
  }
}

export function saveHistory(history: THistory): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(history));
  } catch {
    // Storage quota exceeded
  }
}

export function addHistoryEntry(
  history: THistory,
  entry: Omit<THistoryEntry, "id" | "timestamp">
): THistory {
  const newEntry: THistoryEntry = {
    ...entry,
    id: `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
    timestamp: Date.now(),
  };
  const updated = [newEntry, ...history].slice(0, MAX_ENTRIES);
  setTimeout(() => saveHistory(updated), 0);
  return updated;
}

export function removeHistoryEntry(history: THistory, id: string): THistory {
  const updated = history.filter((e) => e.id !== id);
  setTimeout(() => saveHistory(updated), 0);
  return updated;
}

export function clearHistory(): THistory {
  setTimeout(() => saveHistory([]), 0);
  return [];
}

export function formatTimestamp(ts: number): string {
  const date = new Date(ts);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return "just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;

  return date.toLocaleDateString();
}
