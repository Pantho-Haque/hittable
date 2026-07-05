"use client";

import { formatJson } from "@/utils/formatJson";
import { Dispatch, SetStateAction, useState, useCallback, useMemo } from "react";
import { modifyUrlForNewParams, getParamsfromUrl } from "@/utils/responsePanelUtils";
import { useDataContext } from "@/context/dataContext";
import { Braces, Table2, Plus, Trash2 } from "lucide-react";

type ViewMode = "json" | "table";

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
    <div className="flex flex-col gap-1.5 flex-1 overflow-y-auto p-2 md:p-3">
      {entries.length === 0 && (
        <button
          onClick={addRow}
          className="flex items-center gap-1.5 text-[10px] text-white/25 hover:text-cyan-400 transition-colors cursor-pointer py-2"
        >
          <Plus size={10} />
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
          className="flex items-center gap-1.5 text-[10px] text-white/20 hover:text-cyan-400 transition-colors cursor-pointer py-1 mt-1"
        >
          <Plus size={10} />
          Add row
        </button>
      )}
    </div>
  );
}

export default function TabEditor({
  setError,
}: {
  setError: Dispatch<SetStateAction<string | null>>;
}) {

  const { formInput, setFormInput } = useDataContext();

  const [activeTab, setActiveTab] = useState<"params" | "body" | "headers">(
    "params",
  );
  const [viewMode, setViewMode] = useState<ViewMode>("json");
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

  // Get current entries for display
  const currentEntries = useMemo(() => tableEntries[activeTab] || [["", ""]], [activeTab, tableEntries]);

  // Handle table mode changes
  const handleTableChange = useCallback((entries: [string, string][]) => {
    setTableEntries((prev) => ({ ...prev, [activeTab]: entries }));
    const json = entriesToJson(entries);
    const { output, error: jsonErr } = formatJson(json);
    setError(jsonErr);

    if (activeTab === "params") {
      const newUrl = modifyUrlForNewParams(formInput.url, output);
      setFormInput((prev) => ({ ...prev, url: newUrl, params: output }));
    } else {
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

  const isTableSupported = activeTab !== "body" || (() => {
    try {
      const obj = JSON.parse(formInput.body || "{}");
      return typeof obj === "object" && obj !== null && !Array.isArray(obj);
    } catch {
      return false;
    }
  })();

  return (
    <div
      className="flex flex-col rounded-lg border border-white/8 bg-[#0a1628]/60 overflow-hidden"
      style={{ minHeight: 240 }}
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

        {/* View mode toggle */}
        {isTableSupported && (
          <div className="ml-auto flex items-center gap-0.5 mr-1">
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
        )}
      </div>

      {/* Content */}
      {viewMode === "table" && isTableSupported ? (
        <KeyValueTable
          entries={currentEntries}
          onChange={handleTableChange}
          keyPlaceholder={activeTab === "headers" ? "Header name" : activeTab === "params" ? "Param name" : "Key"}
          valuePlaceholder={activeTab === "headers" ? "Header value" : activeTab === "params" ? "Value" : "Value"}
        />
      ) : (
        <textarea
          key={activeTab}
          className="flex-1 w-full resize-none bg-transparent p-2 md:p-4 text-[10px] md:text-[12px] text-white/70 outline-none placeholder-white/15 leading-relaxed"
          style={{ minHeight: 200 }}
          spellCheck={false}
          value={formInput[activeTab]}
          placeholder={
            activeTab === "body"
              ? '{\n  "key": "value"\n}'
              : '{\n  "Authorization": "Bearer ..."\n}'
          }
          onChange={(e) => {
            const val = e.target.value;
            const { output, error: jsonErr } = formatJson(val);
            setError(jsonErr);
            if (activeTab == "params") {
              const newUrl = modifyUrlForNewParams(formInput.url, output);
              setFormInput((prev) => ({ ...prev, url: newUrl, [activeTab]: output }));
            } else {
              setFormInput((prev) => ({ ...prev, [activeTab]: output }));
            }
          }}
        />
      )}
    </div>
  );
}
