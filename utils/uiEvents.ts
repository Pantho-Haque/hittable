import { useEffect, useRef } from "react";

// A tiny window-event bus so the palette (and others) can open tools whose
// open state lives inside their own component.
export const OPEN_TOOL_EVENT = "hittable:open-tool";
export const COPY_CURL_EVENT = "hittable:copy-curl";

// "env" opens hittable/env.json in the workspace (the unknown-variable note).
export type ToolName = "env" | "shortcuts" | "palette";

type OpenToolDetail = { name: ToolName; target?: string };

/**
 * Ask the tool to open. Several copies of a tool can be mounted (rail and
 * sidebar); the first one listening claims the event so only one opens.
 * `target` narrows it to the copy for that collection. Returns whether one opened.
 */
export function openTool(name: ToolName, target?: string): boolean {
  if (typeof window === "undefined") return false;
  const event = new CustomEvent<OpenToolDetail>(OPEN_TOOL_EVENT, { detail: { name, target }, cancelable: true });
  return !window.dispatchEvent(event);
}

export function requestCopyCurl() {
  window.dispatchEvent(new CustomEvent(COPY_CURL_EVENT));
}

/**
 * Open this component when `openTool(name)` is called. `key` identifies this
 * copy (e.g. its collection); an empty-string key means "not available here".
 */
export function useToolListener(name: ToolName, open: () => void, key?: string) {
  const openRef = useRef(open);
  useEffect(() => {
    openRef.current = open;
  });

  useEffect(() => {
    if (key === "") return;
    const handler = (e: Event) => {
      const { detail } = e as CustomEvent<OpenToolDetail>;
      if (detail?.name !== name || e.defaultPrevented) return;
      if (detail.target !== undefined && key !== undefined && detail.target !== key) return;
      e.preventDefault();
      openRef.current();
    };
    window.addEventListener(OPEN_TOOL_EVENT, handler);
    return () => window.removeEventListener(OPEN_TOOL_EVENT, handler);
  }, [name, key]);
}
