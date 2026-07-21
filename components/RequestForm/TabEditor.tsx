"use client";

import { formatJson } from "@/utils/formatJson";
import { Dispatch, SetStateAction, useState, useCallback, useEffect, useMemo, useRef } from "react";
import { modifyUrlForNewParams } from "@/utils/responsePanelUtils";
import { useDataContext } from "@/context/dataContext";
import useKeypress from "@/hooks/useKeypress";
import { Braces, Table2, Plus, Trash2, Sparkles } from "lucide-react";
import type { DataSource } from "@/types/workspace";

type ViewMode = "json" | "table";

function JsonEditor({
  value,
  onChange,
  placeholder,
  tab,
  error,
}: {
  value: string;
  onChange: (val: string) => void;
  placeholder: string;
  tab: string;
  error: string | null;
}) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const lineNumbersRef = useRef<HTMLDivElement>(null);

  const lineCount = useMemo(() => {
    const lines = (value || "").split("\n").length;
    return Math.max(lines, 1);
  }, [value]);

  const syncScroll = useCallback(() => {
    if (textareaRef.current && lineNumbersRef.current) {
      lineNumbersRef.current.scrollTop = textareaRef.current.scrollTop;
    }
  }, []);

  useEffect(() => {
    syncScroll();
  }, [value, syncScroll]);

  return (
    <div className="flex-1 flex flex-col relative min-h-0">
      {/* Error indicator */}
      {error && (
        <div className="px-3 py-1.5 text-[10px] text-red-400 bg-red-500/10 border-b border-red-500/20">
          <span className="truncate">{error}</span>
        </div>
      )}

      <div className="flex-1 flex min-h-0">
        {/* Line numbers gutter */}
        <div
          ref={lineNumbersRef}
          className="shrink-0 overflow-hidden select-none border-r border-white/5 bg-[#080f1a]/30"
          style={{ width: 40 }}
          aria-hidden="true"
        >
          <div className="p-2 md:p-4 text-right">
            {Array.from({ length: lineCount }, (_, i) => (
              <div
                key={i}
                className="text-[10px] md:text-[12px] leading-relaxed text-white/20 font-mono"
              >
                {i + 1}
              </div>
            ))}
          </div>
        </div>

        {/* Textarea */}
        <textarea
          ref={textareaRef}
          key={tab}
          className="flex-1 w-full resize-none bg-transparent p-2 md:p-4 text-[10px] md:text-[12px] text-white/70 outline-none placeholder-white/15 leading-relaxed font-mono overflow-y-auto"
          style={{ tabSize: 2 }}
          spellCheck={false}
          value={value}
          placeholder={placeholder}
          onChange={(e) => onChange(e.target.value)}
          onScroll={syncScroll}
        />
      </div>
    </div>
  );
}

function KeyValueTable({
  entries,
  onChange,
  keyPlaceholder,
  valuePlaceholder,
}: {
  entries: [string, string][];
  onChange: (entries: [string, string][]) => void;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
}) {
  const addRow = () => onChange([...entries, ["", ""]]);
  const removeRow = (idx: number) => onChange(entries.filter((_, i) => i !== idx));
  const updateKey = (idx: number, key: string) => {
    const next = [...entries];
    next[idx] = [key, next[idx][1]];
    onChange(next);
  };
  const updateValue = (idx: number, value: string) => {
    const next = [...entries];
    next[idx] = [next[idx][0], value];
    onChange(next);
  };

  return (
    <div className="flex flex-col gap-1.5 flex-1 min-h-0 overflow-y-auto p-2 md:p-3">
      {entries.length === 0 && (
        <button
          onClick={addRow}
          className="flex items-center justify-center gap-1.5 text-[11px] text-white/40 hover:text-cyan-400 transition-colors cursor-pointer py-3 border border-dashed border-white/10 hover:border-cyan-500/30 rounded-md bg-white/2 hover:bg-cyan-500/5"
        >
          <Plus size={12} />
          Add first row
        </button>
      )}

      {entries.map(([key, value], idx) => (
        <div key={idx} className="flex items-center gap-1.5 group">
          <input
            type="text"
            value={key}
            onChange={(e) => updateKey(idx, e.target.value)}
            placeholder={keyPlaceholder || "Key"}
            className="flex-1 min-w-0 bg-white/3 border border-white/5 rounded px-2 py-1.5 text-[10px] md:text-[11px] text-white/70 placeholder-white/15 outline-none focus:border-cyan-500/30 transition-colors font-mono"
          />
          <input
            type="text"
            value={value}
            onChange={(e) => updateValue(idx, e.target.value)}
            placeholder={valuePlaceholder || "Value"}
            className="flex-1 min-w-0 bg-white/3 border border-white/5 rounded px-2 py-1.5 text-[10px] md:text-[11px] text-white/70 placeholder-white/15 outline-none focus:border-cyan-500/30 transition-colors font-mono"
          />
          <button
            onClick={() => removeRow(idx)}
            className="p-1 text-white/15 hover:text-red-400 transition-colors cursor-pointer opacity-0 group-hover:opacity-100 shrink-0"
            title="Remove row"
          >
            <Trash2 size={10} />
          </button>
        </div>
      ))}

      {entries.length > 0 && (
        <button
          onClick={addRow}
          className="flex items-center justify-center gap-1.5 text-[11px] text-white/30 hover:text-cyan-400 transition-colors cursor-pointer py-2 mt-1 border border-dashed border-white/8 hover:border-cyan-500/25 rounded-md bg-white/2 hover:bg-cyan-500/5"
        >
          <Plus size={12} />
          Add row
        </button>
      )}
    </div>
  );
}

export default function TabEditor({
  setError,
  dataSource: propDataSource,
}: {
  setError: Dispatch<SetStateAction<string | null>>;
  dataSource?: DataSource;
}) {
  const contextData = useDataContext();
  const { formInput, setFormInput } = propDataSource ?? contextData;

  const [activeTab, setActiveTab] = useState<"params" | "body" | "headers">(
    "params",
  );
  const [viewMode, setViewMode] = useState<ViewMode>("json");

  const currentContent = formInput[activeTab];
  const jsonError = useMemo(() => {
    if (viewMode !== "json") return null;
    const { error } = formatJson(currentContent);
    return error;
  }, [currentContent, viewMode]);

  const [tableEntries, setTableEntries] = useState<Record<string, [string, string][]>>({
    params: [],
    headers: [],
    body: [],
  });

  // Parse JSON to key-value pairs
  const parseJsonToEntries = useCallback((json: string): [string, string][] => {
    try {
      const obj = JSON.parse(json || "{}");
      if (typeof obj !== "object" || obj === null || Array.isArray(obj)) return [];
      const entries = Object.entries(obj).map(([k, v]) => [k, typeof v === "string" ? v : JSON.stringify(v)] as [string, string]);
      return entries.length > 0 ? entries : [["", ""]];
    } catch {
      return [["", ""]];
    }
  }, []);

  // Parse key-value pairs to JSON string (only include non-empty keys)
  const entriesToJson = useCallback((entries: [string, string][]): string => {
    const obj: Record<string, string> = {};
    entries.forEach(([k, v]) => {
      if (k.trim()) obj[k] = v;
    });
    return JSON.stringify(obj, null, "\t");
  }, []);

  // Sync table entries from formInput when switching to table mode or changing tabs
  const syncTableFromJson = useCallback((tab: string, json: string) => {
    const parsed = parseJsonToEntries(json);
    setTableEntries((prev) => ({ ...prev, [tab]: parsed }));
  }, [parseJsonToEntries]);

  // Sync headers table entries when formInput.headers changes
  useEffect(() => {
    if (activeTab !== "headers") return;
    try {
      const parsed = parseJsonToEntries(formInput.headers);
      const current = tableEntries["headers"];
      if (JSON.stringify(parsed) !== JSON.stringify(current)) {
        // eslint-disable-next-line react-hooks/set-state-in-effect -- guard check prevents cascading renders
        setTableEntries((prev) => ({ ...prev, headers: parsed }));
      }
    } catch {
      // Invalid JSON — don't sync
    }
  }, [formInput.headers, activeTab, parseJsonToEntries]); // eslint-disable-line react-hooks/exhaustive-deps -- tableEntries read via guard check, not needed as dependency

  // Get current entries for display
  const currentEntries = useMemo(() => tableEntries[activeTab] || [["", ""]], [activeTab, tableEntries]);

  // Handle table mode changes
  const handleTableChange = useCallback((entries: [string, string][]) => {
    setTableEntries((prev) => ({ ...prev, [activeTab]: entries }));

    if (activeTab === "params") {
      const json = entriesToJson(entries);
      const { output, error: jsonErr } = formatJson(json);
      setError(jsonErr);
      const newUrl = modifyUrlForNewParams(formInput.url, output);
      setFormInput((prev) => ({ ...prev, url: newUrl, params: output }));
    } else {
      const json = entriesToJson(entries);
      const { output, error: jsonErr } = formatJson(json);
      setError(jsonErr);
      setFormInput((prev) => ({ ...prev, [activeTab]: output }));
    }
  }, [activeTab, formInput.url, entriesToJson, setFormInput, setError]);

  // Sync table state when switching to table mode
  const handleViewModeChange = useCallback((mode: ViewMode) => {
    setViewMode(mode);
    if (mode === "table") {
      syncTableFromJson(activeTab, formInput[activeTab]);
    }
  }, [activeTab, formInput, syncTableFromJson]);

  // Sync table state when switching tabs while in table mode
  const handleTabChange = useCallback((tab: "params" | "body" | "headers") => {
    setActiveTab(tab);
    if (viewMode === "table") {
      syncTableFromJson(tab, formInput[tab]);
    }
  }, [viewMode, formInput, syncTableFromJson]);

  const getPlaceholder = () => {
    if (activeTab === "body") {
      return '{\n  "key": "value"\n}';
    }
    return '{\n  "Authorization": "Bearer ..."\n}';
  };

  const handleBeautify = useCallback(() => {
    const { output, error: jsonErr } = formatJson(formInput[activeTab]);
    if (jsonErr) {
      setError(jsonErr);
    } else {
      setError(null);
      if (activeTab === "params") {
        const newUrl = modifyUrlForNewParams(formInput.url, output);
        setFormInput((prev) => ({ ...prev, url: newUrl, params: output }));
      } else {
        setFormInput((prev) => ({ ...prev, [activeTab]: output }));
      }
    }
  }, [activeTab, formInput, setFormInput, setError]);

  useKeypress({
    key: "j",
    isMeta: true,
    func: handleBeautify,
  });

  return (
    <div
      className="flex flex-col rounded-lg border border-white/8 bg-[#0a1628]/60 overflow-hidden"
      style={{ minHeight: 240, maxHeight: "60vh" }}
    >
      {/* Tab bar */}
      <div className="flex items-center border-b border-white/5 bg-[#0e1f35]/50 px-1 pt-1 shrink-0">
        {(["params", "body", "headers"] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => handleTabChange(tab)}
            className="relative px-3 py-2 md:px-4 md:py-2 text-[9px] md:text-[10px] font-semibold tracking-[0.2em] uppercase transition-colors cursor-pointer min-h-[44px] md:min-h-0"
            style={{
              color: activeTab === tab ? "#00e5cc" : "rgba(255,255,255,0.25)",
            }}
          >
            {tab.charAt(0).toUpperCase() + tab.slice(1)}
            {activeTab === tab && (
              <span className="absolute bottom-0 left-3 right-3 h-px bg-cyan-400" />
            )}
          </button>
        ))}

        <div className="ml-auto flex items-center gap-0.5 mr-1">
            {viewMode === "json" && (
              <button
                onClick={handleBeautify}
                title="Beautify JSON (Ctrl/Cmd+J)"
                className="p-1.5 rounded transition-colors cursor-pointer text-white/30 hover:text-cyan-400 hover:bg-cyan-400/10"
              >
                <Sparkles size={12} />
              </button>
            )}
            <button
              onClick={() => handleViewModeChange("json")}
              title="JSON mode"
              className={`p-1.5 rounded transition-colors cursor-pointer ${
                viewMode === "json"
                  ? "text-cyan-400 bg-cyan-400/10"
                  : "text-white/20 hover:text-white/40"
              }`}
            >
              <Braces size={12} />
            </button>
            <button
              onClick={() => handleViewModeChange("table")}
              title="Table mode"
              className={`p-1.5 rounded transition-colors cursor-pointer ${
                viewMode === "table"
                  ? "text-cyan-400 bg-cyan-400/10"
                  : "text-white/20 hover:text-white/40"
              }`}
            >
              <Table2 size={12} />
            </button>
          </div>
      </div>

      {/* Content */}
      {viewMode === "table" ? (
        <KeyValueTable
          entries={currentEntries}
          onChange={handleTableChange}
          keyPlaceholder={activeTab === "headers" ? "Header name" : activeTab === "params" ? "Param name" : "Key"}
          valuePlaceholder={activeTab === "headers" ? "Header value" : activeTab === "params" ? "Value" : "Value"}
        />
      ) : (
        <JsonEditor
          value={formInput[activeTab]}
          onChange={(val) => {
            if (activeTab === "params") {
              const newUrl = modifyUrlForNewParams(formInput.url, val);
              setFormInput((prev) => ({ ...prev, url: newUrl, params: val }));
            } else {
              setFormInput((prev) => ({ ...prev, [activeTab]: val }));
            }
            // Sync error to parent for Send button blocking
            const { error } = formatJson(val);
            setError(error);
          }}
          placeholder={getPlaceholder()}
          tab={activeTab}
          error={jsonError}
        />
      )}
    </div>
  );
}
