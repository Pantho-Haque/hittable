import { useEffect, useRef } from "react";

export default function useKeypress({ 
  key, 
  isMeta = false, 
  isShift = false, 
  ignoreInInputs = undefined, 
  func 
}: { 
  key: string; 
  isMeta?: boolean; 
  isShift?: boolean; 
  ignoreInInputs?: boolean;
  func: (e: KeyboardEvent) => void 
}) {
  const funcRef = useRef(func);

  useEffect(() => {
    funcRef.current = func;
  });

  useEffect(() => {
    const listener = (e: KeyboardEvent) => {
      // By default, ignore if typing inside inputs unless it's a meta shortcut.
      const shouldIgnore = ignoreInInputs !== undefined ? ignoreInInputs : (!isMeta);
      
      if (shouldIgnore) {
        const tag = (e.target as HTMLElement).tagName;
        const isTyping = tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || (e.target as HTMLElement).isContentEditable;
        if (isTyping) return; 
      }

      const isTargetKey  = e.key.toLowerCase() === key.toLowerCase();
      // If we require meta, it must be present. If we don't, it must NOT be present!
      const isMetaOk     = isMeta ? (e.metaKey || e.ctrlKey) : !(e.metaKey || e.ctrlKey);
      const isShiftOk    = isShift ? e.shiftKey : !e.shiftKey;

      if (isTargetKey && isMetaOk && isShiftOk) {
        e.preventDefault();
        funcRef.current(e);
      }
    };
    window.addEventListener("keydown", listener);
    return () => window.removeEventListener("keydown", listener);
  }, [key, isMeta, isShift, ignoreInInputs]);
}