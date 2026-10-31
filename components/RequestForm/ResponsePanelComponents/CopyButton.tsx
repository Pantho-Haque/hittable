'use client';
import { useState, useCallback } from "react";
import { Check, Copy } from "lucide-react";

export default function CopyButton({ data }: { data: unknown }) {
  const [copied, setCopied] = useState(false);
  const copy = useCallback(async () => {
    await navigator.clipboard.writeText(JSON.stringify(data, null, 2));
    setCopied(true);
    setTimeout(() => setCopied(false), 1800);
  }, [data]);

  return (
    <button
      onClick={copy}
      className="flex items-center gap-1 rounded px-2 py-0.5 text-[11px] text-white/50 hover:text-white/70 hover:bg-white/5 transition-all"
      title="Copy JSON"
    >
      {copied ? (
        <Check className="h-3 w-3 text-emerald-400" />
      ) : (
        <Copy className="h-3 w-3" />
      )}
      {copied ? "Copied" : "Copy"}
    </button>
  );
}
