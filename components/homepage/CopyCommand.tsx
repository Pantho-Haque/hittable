"use client";

import { useState } from "react";

// A shell one-liner with a copy button. The command stays selectable text, so
// it still works without JavaScript or clipboard permission.
export function CopyCommand({ command }: { command: string }) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      setTimeout(() => setCopied(false), 1600);
    } catch {
      // No clipboard permission: the text can still be selected by hand.
    }
  };

  return (
    <div className="flex items-stretch overflow-hidden rounded-md border border-(--border-strong) bg-(--ink-700)">
      <pre tabIndex={0} className="min-w-0 flex-1 overflow-x-auto px-4 py-3 font-mono text-[13px] leading-6 text-(--text)">
        <span className="select-none text-(--muted)">$ </span>
        {command}
      </pre>
      <button
        type="button"
        onClick={copy}
        className="shrink-0 border-l border-(--border-strong) px-4 text-[13px] font-medium text-(--text) transition-colors hover:bg-(--ink-600)"
      >
        <span aria-live="polite">{copied ? "Copied" : "Copy"}</span>
      </button>
    </div>
  );
}
