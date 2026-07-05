"use client";

import { useState } from "react";
import { Eye, Pencil, Columns2 } from "lucide-react";
import { marked } from "marked";
import type { NoteId } from "@/types";

marked.setOptions({
  gfm: true,
  breaks: true,
});

type ViewMode = "edit" | "split" | "preview";

interface Props {
  selectedId: NoteId | null;
  selectedTitle: string | null;
  content: string;
  isUnsaved: boolean;
  onChange: (v: string) => void;
}

export default function NoteEditor({
  selectedId,
  selectedTitle,
  content,
  isUnsaved,
  onChange,
}: Props) {
  const [viewMode, setViewMode] = useState<ViewMode>("preview");

  const renderedHtml = content
    ? marked.parse(content, { async: false }) as string
    : '<span style="opacity:0.3">Nothing to preview</span>';

  const cycleViewMode = () => {
    setViewMode((prev) => {
      if (prev === "preview") return "edit";
      if (prev === "edit") return "split";
      return "preview";
    });
  };

  const modeIcon = viewMode === "preview" ? <Eye size={14} /> : viewMode === "edit" ? <Pencil size={14} /> : <Columns2 size={14} />;
  const modeLabel = viewMode === "preview" ? "Preview" : viewMode === "edit" ? "Edit" : "Split";

  return (
    <div className="flex flex-col flex-1 min-h-0">
      {/* header bar */}
      <div className="flex items-center justify-between shrink-0 px-1 py-1">
        <span className="text-xs font-medium text-white/40 truncate">
          {selectedTitle ?? "Select a note"}
        </span>
        <div className="flex items-center gap-2">
          {isUnsaved && (
            <span className="shrink-0 text-[10px] px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-400 font-medium">
              unsaved
            </span>
          )}
          {selectedId && (
            <button
              onClick={cycleViewMode}
              title={`${modeLabel} mode`}
              className={`flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] transition-colors cursor-pointer ${
                viewMode !== "edit"
                  ? "text-cyan-400 bg-cyan-400/10"
                  : "text-white/30 hover:text-white/50"
              }`}
            >
              {modeIcon}
            </button>
          )}
        </div>
      </div>

      {/* content area */}
      <div className="flex-1 min-h-0 flex">
        {viewMode === "split" ? (
          <>
            {/* edit pane */}
            <div className="flex-1 min-w-0 flex flex-col border border-white/5 rounded-l-lg overflow-hidden max-md:hidden">
              <div className="px-2 py-1 text-[9px] text-white/20 uppercase tracking-wider border-b border-white/5 bg-white/2 shrink-0">
                Edit
              </div>
              <textarea
                disabled={!selectedId}
                value={content}
                onChange={(e) => onChange(e.target.value)}
                placeholder="Write markdown..."
                className="flex-1 w-full resize-none bg-transparent p-3 text-[13px] text-white/80 outline-none placeholder-white/15 leading-relaxed font-mono"
              />
            </div>
            {/* divider */}
            <div className="w-px bg-white/5 shrink-0 max-md:hidden" />
            {/* preview pane */}
            <div className="flex-1 min-w-0 flex flex-col border border-white/5 border-l-0 rounded-r-lg overflow-hidden max-md:hidden">
              <div className="px-2 py-1 text-[9px] text-white/20 uppercase tracking-wider border-b border-white/5 bg-white/2 shrink-0">
                Preview
              </div>
              <div
                className="flex-1 overflow-y-auto p-3 text-[13px] text-white/70 leading-relaxed note-preview"
                dangerouslySetInnerHTML={{ __html: renderedHtml }}
              />
            </div>
            {/* mobile fallback: stacked */}
            <div className="flex flex-col flex-1 min-h-0 md:hidden">
              <textarea
                disabled={!selectedId}
                value={content}
                onChange={(e) => onChange(e.target.value)}
                placeholder="Write markdown..."
                className="flex-1 w-full resize-none bg-transparent p-3 text-[13px] text-white/80 outline-none placeholder-white/15 leading-relaxed font-mono"
              />
              <div className="h-px bg-white/5 shrink-0" />
              <div
                className="flex-1 overflow-y-auto p-3 text-[13px] text-white/70 leading-relaxed note-preview"
                dangerouslySetInnerHTML={{ __html: renderedHtml }}
              />
            </div>
          </>
        ) : viewMode === "preview" ? (
          <div
            className="flex-1 overflow-y-auto p-3 text-[13px] text-white/70 leading-relaxed note-preview"
            dangerouslySetInnerHTML={{ __html: renderedHtml }}
          />
        ) : (
          <textarea
            disabled={!selectedId}
            value={content}
            onChange={(e) => onChange(e.target.value)}
            placeholder="Write markdown..."
            className="flex-1 w-full resize-none bg-transparent p-3 text-[13px] text-white/80 outline-none placeholder-white/15 leading-relaxed font-mono"
          />
        )}
      </div>

      {/* footer */}
      <div className="flex items-center justify-between shrink-0 px-1 py-1 border-t border-white/5">
        <span className="text-[10px] text-white/20">
          {content.length} chars{content.trim() ? ` · ${content.trim().split(/\s+/).length} words` : ""}
        </span>
        {selectedId && (
          <span className="text-[10px] text-white/15">
            {modeLabel} mode
          </span>
        )}
      </div>
    </div>
  );
}
