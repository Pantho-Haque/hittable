"use client";

import { useEffect, useState } from "react";

declare global {
  interface Window { __hittableExtension?: boolean; }
}

export function useExtension() {
  const [available, setAvailable] = useState(false);
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    const handler = (e: MessageEvent) => {
      if (e.source === window && e.data?.type === "HITTABLE_EXTENSION_READY") {
        clearInterval(poll);
        window.__hittableExtension = true;
        setAvailable(true);
        setChecked(true);
      }
    };

    window.addEventListener("message", handler);

    let attempts = 0;
    const poll = setInterval(() => {
      window.postMessage({ type: "HITTABLE_PING" }, window.location.origin);
      attempts++;
      if (attempts >= 10) {
        clearInterval(poll);
        setChecked(true); // ← gave up, mark as checked
      }
    }, 500);

    return () => {
      window.removeEventListener("message", handler);
      clearInterval(poll);
    };
  }, []);

  return { available, checked };
}