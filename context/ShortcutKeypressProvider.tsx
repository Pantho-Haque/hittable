"use client";

import { createContext, useContext, useState, ReactNode, useEffect } from "react";
import useKeypress from "@/hooks/useKeypress";

type Shortcuts = {
  toggleSidebar: boolean;
};

const STORAGE_KEY = "hittable_sidebar_collapsed";

function loadSidebarState(): boolean {
  if (typeof window === "undefined") return false;
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored !== null) return stored === "true";
  } catch {
    // localStorage unavailable
  }
  return false;
}

function saveSidebarState(collapsed: boolean) {
  try {
    localStorage.setItem(STORAGE_KEY, String(collapsed));
  } catch {
    // Storage full
  }
}

const defaultShortcuts: Shortcuts = {
  toggleSidebar: false,
};

const ShortcutContext = createContext<{
  shortcuts: Shortcuts,
  toggle: (key: keyof Shortcuts) => void
}>({
  shortcuts: defaultShortcuts,
  toggle: () => { },
});

export function ShortcutProvider({ children }: { children: ReactNode }) {
  const [shortcuts, setShortcuts] = useState<Shortcuts>(() => ({
    toggleSidebar: loadSidebarState(),
  }));

  // Persist sidebar state on change
  useEffect(() => {
    saveSidebarState(shortcuts.toggleSidebar);
  }, [shortcuts.toggleSidebar]);

  function toggle(key: keyof Shortcuts) {
    setShortcuts((prev) => ({ ...prev, [key]: !prev[key] }));
  }

  useKeypress({
    key: "b",
    isMeta: true,
    func: () => toggle("toggleSidebar"),
  });

  return (
    <ShortcutContext.Provider value={{ shortcuts, toggle }}>
      {children}
    </ShortcutContext.Provider>
  );
}

export function useShortcuts() {
  const ctx = useContext(ShortcutContext);
  if (!ctx) throw new Error("useShortcuts must be used inside <ShortcutProvider>");
  return ctx;
}
