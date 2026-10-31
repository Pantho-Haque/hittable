'use client';
import { useEffect, useRef } from "react";
import { ListFilter, X } from "lucide-react";
import type { QueryResult } from "@/utils/jsonPath";

/** One-line JSONPath input under the response tabs (FR-13). */
export default function JsonPathFilter({
  value,
  onChange,
  onClear,
  result,
}: {
  value: string;
  onChange: (v: string) => void;
  onClear: () => void;
  /** null while the input is empty. */
  result: QueryResult | null;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  useEffect(() => inputRef.current?.focus(), []);

  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 border-b border-white/5 bg-(--surface-2)/30 px-2 md:px-3 py-1 shrink-0">
      <ListFilter className="h-3 w-3 text-white/60 shrink-0" aria-hidden="true" />
      <input
        ref={inputRef}
        type="text"
        aria-label="JSONPath filter"
        aria-invalid={result ? !result.ok : undefined}
        spellCheck={false}
        autoComplete="off"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => e.key === "Escape" && onClear()}
        placeholder="JSONPath, e.g. $.items[*].id"
        className="min-w-0 flex-1 rounded bg-transparent px-1 py-1 font-mono text-xs text-white/85 placeholder:text-white/60"
      />
      <span aria-live="polite" className="text-xs shrink-0">
        {result && !result.ok && <span className="text-(--danger)">Invalid JSONPath: {result.error}</span>}
        {result?.ok && result.matches.length === 0 && <span className="text-white/70">No results for {value.trim()}</span>}
        {result?.ok && result.matches.length > 0 && (
          <span className="text-white/60">
            {result.matches.length} {result.matches.length === 1 ? "match" : "matches"}
          </span>
        )}
      </span>
      <button
        type="button"
        onClick={onClear}
        aria-label="Clear filter"
        title="Clear filter (Esc)"
        className="p-1 rounded text-white/60 hover:text-white/80 hover:bg-white/5 shrink-0"
      >
        <X className="h-3 w-3" aria-hidden="true" />
      </button>
    </div>
  );
}
