'use client';
import { useState, useMemo, useEffect } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { JsonValue } from "@/types";
import { getType, valueMatchesSearch } from "@/utils/responsePanelUtils";
import { Highlight } from "./Highlight";

export default function JsonNode({
  keyName,
  value,
  depth,
  searchQuery,
  defaultOpen,
  isLast,
}: {
  keyName?: string;
  value: JsonValue;
  depth: number;
  searchQuery: string;
  defaultOpen?: boolean;
  isLast?: boolean;
}) {
  const type = getType(value);
  const isExpandable = type === "object" || type === "array";
  const childCount = isExpandable
    ? Array.isArray(value)
      ? value.length
      : Object.keys(value as object).length
    : 0;

  const shouldBeOpen = useMemo(() => {
    if (!searchQuery) return defaultOpen ?? depth < 2;
    return valueMatchesSearch(value, searchQuery);
  }, [searchQuery, value, defaultOpen, depth]);

  const [open, setOpen] = useState(shouldBeOpen);
  useEffect(() => {
    setOpen(shouldBeOpen);
  }, [shouldBeOpen]);

  const typeColors: Record<string, string> = {
    string: "text-emerald-300",
    number: "text-sky-300",
    boolean: "text-violet-300",
    null: "text-rose-300/70",
  };

  const renderPrimitive = () => {
    const raw = value === null ? "null" : String(value);
    const displayVal = type === "string" ? `"${raw}"` : raw;
    return (
      <span className={typeColors[type] ?? "text-white/60"}>
        <Highlight text={displayVal} query={searchQuery} />
      </span>
    );
  };

  const indent = depth * 16;

  return (
    <div className="font-mono text-[11px] md:text-[11px] leading-[1.7]">
      <div
        className="flex items-start group hover:bg-white/3 rounded transition-colors"
        style={{ paddingLeft: indent }}
      >
        {/* Only containers get a toggle; a leaf keeps the indent with a plain
            spacer (an empty button there was an unnamed tab stop per value). */}
        {isExpandable ? (
          <button
            onClick={() => setOpen((o) => !o)}
            aria-label={open ? "Collapse" : "Expand"}
            aria-expanded={open}
            className="w-3.5 h-4 md:w-4 md:h-5 flex items-center justify-center shrink-0 transition-colors cursor-pointer text-white/60 hover:text-white/80"
          >
            {open ? (
              <ChevronDown className="w-2.5 h-2.5 md:w-3 md:h-3" />
            ) : (
              <ChevronRight className="w-2.5 h-2.5 md:w-3 md:h-3" />
            )}
          </button>
        ) : (
          <span aria-hidden="true" className="w-3.5 h-4 md:w-4 md:h-5 shrink-0" />
        )}

        {keyName !== undefined && (
          <>
            <span className="text-sky-300/80 shrink-0">
              <Highlight text={`"${keyName}"`} query={searchQuery} />
            </span>
            <span className="text-white/50 mx-1 shrink-0">:</span>
          </>
        )}

        {isExpandable ? (
          <span className="text-white/60">
            {type === "array" ? "[" : "{"}
            {!open && (
              <>
                <span className="text-white/50 mx-1 text-[11px] md:text-[11px] italic">
                  {childCount} {type === "array" ? "items" : "keys"}
                </span>
                {type === "array" ? "]" : "}"}
                {!isLast && <span className="text-white/50">,</span>}
              </>
            )}
          </span>
        ) : (
          <span>
            {renderPrimitive()}
            {!isLast && <span className="text-white/50">,</span>}
          </span>
        )}
      </div>

      {isExpandable && open && (
        <>
          {Array.isArray(value)
            ? value.map((item, i) => (
                <JsonNode
                  key={i}
                  value={item}
                  depth={depth + 1}
                  searchQuery={searchQuery}
                  isLast={i === value.length - 1}
                />
              ))
            : Object.entries(value as Record<string, JsonValue>).map(
                ([k, v], i, arr) => (
                  <JsonNode
                    key={k}
                    keyName={k}
                    value={v as JsonValue}
                    depth={depth + 1}
                    searchQuery={searchQuery}
                    isLast={i === arr.length - 1}
                  />
                ),
              )}
          <div
            className="font-mono text-[11px] md:text-[11px] text-white/60 leading-[1.7]"
            style={{ paddingLeft: indent + 4 }}
          >
            {type === "array" ? "]" : "}"}
            {!isLast && <span className="text-white/50">,</span>}
          </div>
        </>
      )}
    </div>
  );
}
