"use client";

import { lazy, Suspense, useCallback, useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import EnvModal from "@/components/modals/EnvModal";
import ExportModal from "@/components/modals/ExportModal";
import { useDataContext } from "@/context/dataContext";
import { useWorkspace } from "@/context/workspaceContext";
import { useToolListener } from "@/utils/uiEvents";

// The dialog and its search code stay out of the first load (NFR-5); they
// are fetched once the page is idle so the first Ctrl+K is instant.
const loadDialog = () => import("./PaletteDialog");
const PaletteDialog = lazy(loadDialog);

/**
 * Ctrl/Cmd+K anywhere in /hittable, including inside inputs and editors, or
 * openTool("palette") from the top bar button.
 */
function Palette() {
  const [open, setOpen] = useState(false);
  const openRef = useRef(open);
  useEffect(() => {
    openRef.current = open;
  }, [open]);
  const { mode } = useWorkspace();
  const { collections } = useDataContext();
  const c = useSearchParams().get("c") ?? "";
  const collection = mode === "local" ? collections.find((x) => x.collectionName === c) : undefined;


  // From Ctrl+K until the dialog's input is ready (it is loaded lazily), typed
  // characters are held here and handed over as the starting query; otherwise
  // a quick "Ctrl+K env Enter" loses "env" to whatever had focus.
  const typed = useRef<string | null>(null);
  const typedSoFar = useCallback(() => typed.current ?? "", []);
  const takeTyped = useCallback(() => {
    const t = typed.current ?? "";
    typed.current = null;
    return t;
  }, []);
  // Armed synchronously on open: the dialog's effect (a child's) runs before
  // this component's, so arming in an effect would re-arm after the handover.
  const openPalette = useCallback(() => {
    typed.current = "";
    setOpen(true);
  }, []);
  const closePalette = useCallback(() => {
    typed.current = null;
    setOpen(false);
  }, []);
  useToolListener("palette", openPalette);

  // Always listening, but only holds keys while armed (typed.current is a
  // string), so nothing typed right after Ctrl+K slips past before React has
  // rendered anything.
  useEffect(() => {
    const hold = (e: KeyboardEvent) => {
      if (typed.current === null || e.ctrlKey || e.metaKey || e.altKey) return;
      if (e.key.length === 1) typed.current += e.key;
      else if (e.key === "Backspace") typed.current = typed.current.slice(0, -1);
      else if (e.key === "Enter") typed.current += "\n"; // run the top result once loaded
      else return;
      e.preventDefault();
      e.stopPropagation();
    };
    window.addEventListener("keydown", hold, true);
    return () => window.removeEventListener("keydown", hold, true);
  }, []);

  useEffect(() => {
    const id = setTimeout(() => void loadDialog().catch(() => {}), 1500);
    return () => clearTimeout(id);
  }, []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey) || e.altKey || e.shiftKey || e.key.toLowerCase() !== "k") return;
      // Capture phase, before an editor or input handles the key.
      e.preventDefault();
      e.stopPropagation();
      if (openRef.current) closePalette();
      else openPalette();
    };
    window.addEventListener("keydown", handler, true);
    return () => window.removeEventListener("keydown", handler, true);
  }, [openPalette, closePalette]);

  return (
    <>
      {/* Environment and Export for the open collection, which the sidebar
          may not have mounted (collapsed rail, or drilled into the collection).
          Their dialogs portal to <body>, so only the triggers stay hidden. */}
      {collection && (
        <div hidden>
          <EnvModal collectionName={collection.collectionName} />
          <ExportModal exportString={JSON.stringify(collection)} collectionName={collection.collectionName} />
        </div>
      )}
      {open && (
        <Suspense fallback={null}>
          <PaletteDialog onClose={closePalette} collectionName={collection?.collectionName ?? ""} typedSoFar={typedSoFar} takeTyped={takeTyped} />
        </Suspense>
      )}
    </>
  );
}

export default function CommandPalette() {
  // useSearchParams needs a Suspense boundary to prerender.
  return (
    <Suspense fallback={null}>
      <Palette />
    </Suspense>
  );
}
