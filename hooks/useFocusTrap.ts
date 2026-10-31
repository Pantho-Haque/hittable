import { RefObject, useEffect } from "react";

const FOCUSABLE = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

/**
 * While `active`, Tab and Shift+Tab cycle inside `ref`, focus starts on the
 * first field, and on close it goes back to whatever had it before — what
 * ModalShell does, for dialogs that draw their own shell.
 */
export function useFocusTrap(ref: RefObject<HTMLElement | null>, active: boolean) {
  useEffect(() => {
    if (!active || !ref.current) return;
    const root = ref.current;
    const previous = document.activeElement as HTMLElement | null;
    const items = () => Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).filter((el) => !el.hasAttribute("disabled"));
    (root.querySelector<HTMLElement>("input, textarea") ?? items()[0])?.focus();

    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Tab") return;
      const list = items();
      if (list.length === 0) return;
      e.preventDefault();
      const i = list.indexOf(document.activeElement as HTMLElement);
      const next = e.shiftKey ? (i <= 0 ? list.length : i) - 1 : (i + 1) % list.length;
      list[next].focus();
    };
    root.addEventListener("keydown", onKey);
    return () => {
      root.removeEventListener("keydown", onKey);
      if (previous?.isConnected) previous.focus();
    };
  }, [ref, active]);
}
