"use client";

import { useEffect, useRef } from "react";

// Adds a copy button to every code block in the rendered markdown. Done
// after mount so the article itself stays a plain server-rendered string.
export default function CopyCodeButtons({ children }: { children: React.ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const root = ref.current;
    if (!root) return;
    const blocks = root.querySelectorAll<HTMLElement>("figure.docs-code");
    blocks.forEach((fig) => {
      if (fig.querySelector("button")) return;
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "docs-copy";
      btn.textContent = "Copy";
      btn.addEventListener("click", async () => {
        const code = fig.querySelector("code")?.textContent ?? "";
        try {
          await navigator.clipboard.writeText(code);
          btn.textContent = "Copied";
        } catch {
          btn.textContent = "Press ⌘C";
        }
        setTimeout(() => (btn.textContent = "Copy"), 1500);
      });
      fig.appendChild(btn);
    });
  }, []);

  return <div ref={ref}>{children}</div>;
}
