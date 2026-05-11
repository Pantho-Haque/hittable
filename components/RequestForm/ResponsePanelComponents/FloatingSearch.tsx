
'use client';
import { useRef, useEffect } from "react";
import { Search, ArrowUp, ArrowDown, X } from "lucide-react";

export default function FloatingSearch({
  value,
  onChange,
  total,
  activeIndex,
  onNext,
  onPrev,
  onClose,
}: {
  value: string;
  onChange: (v: string) => void;
  total: number;
  activeIndex: number;
  onNext: () => void;
  onPrev: () => void;
  onClose: () => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      if (e.shiftKey) {
        onPrev();
      } else {
        onNext();
      }
    }
    if (e.key === "Escape") onClose();
  };

  const current = total === 0 ? 0 : activeIndex + 1;

  return (
    <div className="absolute top-[38px] right-3 z-30 flex items-center gap-1.5 rounded-lg border border-white/10 bg-[#0b1a2e]/95 shadow-2xl shadow-black/50 backdrop-blur-sm px-2.5 py-1.5">
      <Search className="h-3 w-3 text-white/30 shrink-0" />
      <input
        ref={inputRef}
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="Search keys & values…"
        className="w-40 bg-transparent text-[9px] md:text-[11px] text-white/75 placeholder:text-white/20 outline-none"
      />
      {value && (
        <>
          <span
            className="text-[8px] md:text-[10px] tabular-nums shrink-0 px-1"
            style={{
              color: total === 0 ? "rgba(248,113,113,0.7)" : "rgba(255,255,255,0.3)",
            }}
          >
            {current}/{total}
          </span>
          <div className="flex items-center gap-0.5 border-l border-white/8 pl-1">
            <button
              onClick={onPrev}
              title="Previous (Shift+Enter)"
              className="p-0.5 rounded text-white/30 hover:text-white/70 hover:bg-white/8 transition-colors disabled:opacity-30"
              disabled={total === 0}
            >
              <ArrowUp className="h-2.5 w-2.5 md:h-3 md:w-3" />
            </button>
            <button
              onClick={onNext}
              title="Next (Enter)"
              className="p-0.5 rounded text-white/30 hover:text-white/70 hover:bg-white/8 transition-colors disabled:opacity-30"
              disabled={total === 0}
            >
              <ArrowDown className="h-2.5 w-2.5 md:h-3 md:w-3" />
            </button>
          </div>
        </>
      )}
      <button
        onClick={onClose}
        className="p-0.5 rounded text-white/20 hover:text-white/60 hover:bg-white/8 transition-colors ml-0.5"
      >
        <X className="h-2.5 w-2.5 md:h-3 md:w-3" />
      </button>
    </div>
  );
}
