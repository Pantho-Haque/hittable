"use client";

import { X, Tag, Code2 } from "lucide-react";
import type { ElementInfo } from "./HtmlPreview";

interface ElementInspectorProps {
  info: ElementInfo;
  onClose: () => void;
}

export default function ElementInspector({ info, onClose }: ElementInspectorProps) {
  return (
    <div className="border-t border-white/5 bg-[#0a1628]/80 shrink-0">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-[#080f1a]/40 border-b border-white/5">
        <div className="flex items-center gap-2">
          <Tag className="h-3 w-3 text-cyan-400/70" />
          <span className="text-[10px] md:text-[11px] font-mono text-cyan-400 font-semibold">
            &lt;{info.tagName}&gt;
          </span>
        </div>
        <button
          onClick={onClose}
          className="p-0.5 rounded text-white/30 hover:text-white/70 hover:bg-white/5 transition-colors"
          title="Close inspector"
        >
          <X className="h-3 w-3" />
        </button>
      </div>

      {/* Attributes */}
      <div className="px-3 py-2 max-h-24 overflow-y-auto">
        {Object.keys(info.attributes).length === 0 ? (
          <span className="text-[10px] text-white/30">No attributes</span>
        ) : (
          <div className="flex flex-col gap-1">
            {Object.entries(info.attributes).map(([key, value]) => (
              <div key={key} className="flex items-start gap-2 text-[10px] font-mono">
                <span className="text-amber-300/70 shrink-0">{key}</span>
                <span className="text-white/20">=</span>
                <span className="text-emerald-300/70 break-all">{value}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Outer HTML preview */}
      <div className="px-3 py-2 border-t border-white/5">
        <div className="flex items-center gap-1.5 mb-1.5">
          <Code2 className="h-2.5 w-2.5 text-white/25" />
          <span className="text-[9px] text-white/25 tracking-wider uppercase">Preview</span>
        </div>
        <pre className="text-[9px] md:text-[10px] font-mono text-white/50 whitespace-pre-wrap break-all max-h-16 overflow-y-auto bg-[#080f1a]/30 rounded p-2 border border-white/5">
          {info.outerHTML}
        </pre>
      </div>
    </div>
  );
}
