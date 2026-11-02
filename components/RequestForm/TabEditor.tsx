"use client";

import { formatJson } from "@/utils/formatJson";
import { Dispatch, SetStateAction, useState, useCallback, useEffect, useMemo, useRef } from "react";
import { modifyUrlForNewParams } from "@/utils/responsePanelUtils";
import { Braces, Table2, Plus, Trash2, Sparkles } from "lucide-react";
import type { DataSource } from "@/types/workspace";
import { FileBodyEditor, MultipartEditor, rowsFromBody, rowsToBody } from "./UploadFields";
import {
  BODY_TYPES,
  BodyType,
  entriesToForm,
  formToEntries,
  headersForBodyType,
  inferBodyType,
} from "@/utils/bodyType";

const TABS = ["params", "body", "headers"] as const;
type Tab = (typeof TABS)[number];

type ViewMode = "json" | "table";

function JsonEditor({
  value,
  onChange,
  placeholder,
  tab,
  error,
  label,
}: {
  value: string;
  onChange: (val: string) => void;
  placeholder: string;
  tab: string;
  error: string | null;
  label?: string;
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
        <div role="status" className="flex shrink-0 items-center gap-1.5 border-b border-(--danger)/25 bg-(--danger)/10 px-3 py-1.5 font-mono text-[11px] text-(--danger)">
          <span className="truncate">{error}</span>
        </div>
      )}

      <div className="flex-1 flex min-h-0">
        {/* Line numbers gutter */}
        <div
          ref={lineNumbersRef}
          className="shrink-0 overflow-hidden select-none border-r border-(--border) bg-(--ink-700)"
          style={{ width: 44 }}
          aria-hidden="true"
        >
          <div className="py-3 pr-2.5 text-right">
            {Array.from({ length: lineCount }, (_, i) => (
              <div
                key={i}
                className="font-mono text-[13px] leading-5 text-(--muted)/60"
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
          aria-label={label ?? `${tab} JSON`}
          wrap="off"
          className="flex-1 min-w-0 w-full resize-none overflow-auto bg-(--ink-700) px-3 py-3 font-mono text-[13px] leading-5 text-(--text) placeholder:text-(--muted)/50"
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

  const cell = "h-8 w-full min-w-0 bg-transparent px-2.5 font-mono text-xs text-(--text) placeholder:text-(--muted)/50 outline-none focus:bg-(--surface-2)";
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto p-2 md:p-3">
      <div className="overflow-hidden rounded-md border border-(--border)">
        <div className="grid grid-cols-[1fr_1fr_2rem] border-b border-(--border) bg-(--surface-3) text-[11px] font-medium text-(--muted)" aria-hidden="true">
          <span className="px-2.5 py-1.5">{keyPlaceholder || "Key"}</span>
          <span className="border-l border-(--border) px-2.5 py-1.5">{valuePlaceholder || "Value"}</span>
          <span />
        </div>
        {entries.map(([key, value], idx) => (
          <div key={idx} className="group grid grid-cols-[1fr_1fr_2rem] border-b border-(--border) last:border-b-0 hover:bg-(--surface-2)/40">
            <input
              type="text"
              aria-label={`${keyPlaceholder || "Key"} ${idx + 1}`}
              value={key}
              onChange={(e) => updateKey(idx, e.target.value)}
              placeholder={keyPlaceholder || "Key"}
              className={cell}
            />
            <input
              type="text"
              aria-label={`${valuePlaceholder || "Value"} ${idx + 1}`}
              value={value}
              onChange={(e) => updateValue(idx, e.target.value)}
              placeholder={valuePlaceholder || "Value"}
              className={`${cell} border-l border-(--border)`}
            />
            <button
              type="button"
              onClick={() => removeRow(idx)}
              className="flex items-center justify-center text-(--muted) opacity-0 transition-opacity hover:text-(--danger) focus-visible:opacity-100 group-hover:opacity-100 group-focus-within:opacity-100"
              title="Remove row"
              aria-label="Remove row"
            >
              <Trash2 size={12} aria-hidden="true" />
            </button>
          </div>
        ))}
        <button
          type="button"
          onClick={addRow}
          className={`flex w-full items-center gap-1.5 px-2.5 py-2 text-xs text-(--muted) transition-colors hover:bg-(--surface-2)/60 hover:text-(--brand) ${entries.length ? "border-t border-(--border)" : ""}`}
        >
          <Plus size={12} aria-hidden="true" />
          {entries.length === 0 ? "Add first row" : "Add row"}
        </button>
      </div>
    </div>
  );
}

/**
 * The body after switching type: fields carry over between Form and Multipart;
 * a file reference doesn't survive leaving File (it would read as a file again).
 */
function bodyForType(body: string, from: BodyType, to: BodyType): string {
  if (to === "none" || from === "file") return "";
  if (to === "file") return "";
  if (to === "multipart" && from === "form") {
    return rowsToBody(formToEntries(body).filter(([k]) => k).map(([name, value]) => ({ name, file: false, value })));
  }
  if (to === "multipart") return rowsFromBody(body) ? body : "";
  if (to === "form" && from === "multipart") {
    return entriesToForm((rowsFromBody(body) ?? []).filter((r) => !r.file).map((r) => [r.name, r.value]));
  }
  return body;
}

/** Entries in a JSON object, for the tab badge; null when it is not one. */
function countKeys(json: string | undefined): number | null {
  try {
    const v = JSON.parse(json || "{}");
    return v && typeof v === "object" && !Array.isArray(v) ? Object.keys(v).length : null;
  } catch {
    return null;
  }
}

export default function TabEditor({
  setError,
  dataSource,
}: {
  setError: Dispatch<SetStateAction<string | null>>;
  dataSource: DataSource;
}) {
  const { formInput, setFormInput } = dataSource;

  const [activeTab, setActiveTab] = useState<Tab>("params");
  const [viewMode, setViewMode] = useState<ViewMode>("json");

  // The body type is read from the body and its Content-Type, never stored.
  // "None" and an empty "File" cannot be inferred (an empty body opens as
  // JSON), so picking one is remembered here until the body says otherwise.
  const [chosen, setChosen] = useState<"none" | "file" | null>(null);
  const emptyBody = !formInput.body?.trim();
  const bodyType: BodyType =
    chosen === "none" && emptyBody ? "none"
      : chosen === "file" && emptyBody ? "file"
        : inferBodyType(formInput.body ?? "", formInput.headers ?? "");
  // Only a JSON body has to be valid JSON; a text or form body is sent as is.
  const isJsonTab = (t: Tab) => t !== "body" || bodyType === "json";

  const currentContent = formInput[activeTab];
  const jsonError = useMemo(() => {
    if (viewMode !== "json" || !isJsonTab(activeTab)) return null;
    const { error } = formatJson(currentContent);
    return error;
  }, [currentContent, viewMode, activeTab, bodyType]); // eslint-disable-line react-hooks/exhaustive-deps -- isJsonTab derives from bodyType

  // Send stays blocked while any tab holds invalid JSON, not just the one
  // being edited.
  useEffect(() => {
    setError(TABS.filter(isJsonTab).map((t) => formatJson(formInput[t]).error).find(Boolean) ?? null);
  }, [formInput.params, formInput.body, formInput.headers, bodyType, setError]); // eslint-disable-line react-hooks/exhaustive-deps -- formInput read field by field

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
  const handleTabChange = useCallback((tab: Tab) => {
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

  // Shift+Alt+F, the VS Code format chord. (Ctrl+J used to be bound here, but
  // that is the browser's Downloads shortcut.) e.code, because on a Mac
  // Option+Shift+F types a symbol rather than "F".
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.altKey && e.shiftKey && !e.metaKey && !e.ctrlKey && e.code === "KeyF") {
        if (activeTab === "body" && bodyType !== "json") return;
        e.preventDefault();
        handleBeautify();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [handleBeautify, activeTab, bodyType]);

  // Form rows live here, not only in the body: a row being typed (or a fresh
  // empty one from "Add row") has no key yet, so it is not in the encoded body
  // and would vanish if the rows were re-read from it on every render.
  // `rowsFor` is the body text the rows were read from or wrote, so a body
  // changed anywhere else (load, paste, Text view) is noticed without
  // re-encoding, which would not round-trip for arbitrary text.
  const [form, setForm] = useState(() => ({ rows: formToEntries(formInput.body ?? ""), rowsFor: formInput.body ?? "" }));
  if ((formInput.body ?? "") !== form.rowsFor) {
    setForm({ rows: formToEntries(formInput.body ?? ""), rowsFor: formInput.body ?? "" });
  }
  const changeFormRows = (rows: [string, string][]) => {
    const body = entriesToForm(rows);
    setForm({ rows, rowsFor: body });
    setFormInput((prev) => ({ ...prev, body }));
  };

  const changeBodyType = useCallback((type: BodyType) => {
    setChosen(type === "none" || type === "file" ? type : null);
    setFormInput((prev) => ({
      ...prev,
      body: bodyForType(prev.body ?? "", bodyType, type),
      headers: headersForBodyType(prev.headers, type),
    }));
    if (type !== "json") setViewMode("json"); // the table toggle is JSON-only
  }, [setFormInput, bodyType]);

  // Arrow keys move between tabs (WAI-ARIA tabs pattern).
  const onTabKeyDown = (e: React.KeyboardEvent, tab: Tab) => {
    const i = TABS.indexOf(tab);
    const next = e.key === "ArrowRight" ? TABS[(i + 1) % TABS.length]
      : e.key === "ArrowLeft" ? TABS[(i + TABS.length - 1) % TABS.length]
      : e.key === "Home" ? TABS[0] : e.key === "End" ? TABS[TABS.length - 1] : null;
    if (!next) return;
    e.preventDefault();
    handleTabChange(next);
    document.getElementById(`req-tab-${next}`)?.focus();
  };

  const showJsonTools = isJsonTab(activeTab);

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-lg border border-(--border) bg-(--surface-1)">
      {/* Tab bar */}
      <div className="flex h-10 shrink-0 items-stretch border-b border-(--border) bg-(--surface-3) pl-1 pr-2">
        <div role="tablist" aria-label="Request" className="flex items-stretch">
          {TABS.map((tab) => {
            const n = tab === "body" ? null : countKeys(formInput[tab]);
            const hasBody = tab === "body" && !!formInput.body?.trim();
            return (
              <button
                key={tab}
                id={`req-tab-${tab}`}
                role="tab"
                type="button"
                aria-selected={activeTab === tab}
                aria-controls="req-tabpanel"
                tabIndex={activeTab === tab ? 0 : -1}
                onClick={() => handleTabChange(tab)}
                onKeyDown={(e) => onTabKeyDown(e, tab)}
                className={`relative flex items-center gap-1.5 px-3 text-xs font-medium transition-colors min-h-[44px] md:min-h-0 ${
                  activeTab === tab ? "text-(--text)" : "text-(--muted) hover:text-(--text)"
                }`}
              >
                {tab.charAt(0).toUpperCase() + tab.slice(1)}
                {!!n && (
                  <span aria-hidden="true" className="rounded-full bg-(--surface-2) px-1.5 font-mono text-[11px] leading-4 text-(--muted)">
                    {n}
                  </span>
                )}
                {hasBody && <span aria-hidden="true" className="h-1.5 w-1.5 rounded-full bg-(--brand)" />}
                {activeTab === tab && (
                  <span className="absolute inset-x-2 bottom-0 h-0.5 rounded-full bg-(--brand)" aria-hidden="true" />
                )}
              </button>
            );
          })}
        </div>

        <div className="ml-auto flex items-center gap-2">
          {viewMode === "json" && showJsonTools && (
            <button
              type="button"
              onClick={handleBeautify}
              title="Beautify JSON (Shift+Alt+F)"
              aria-label="Beautify JSON (Shift+Alt+F)"
              className="flex h-7 items-center gap-1 rounded px-2 text-xs text-(--muted) transition-colors hover:bg-(--surface-2) hover:text-(--text)"
            >
              <Sparkles size={12} aria-hidden="true" />
              <span className="max-sm:hidden">Format</span>
            </button>
          )}
          {showJsonTools && (
            <div className="flex h-7 items-center rounded-md border border-(--border) p-0.5">
              {(["json", "table"] as const).map((m) => (
                <button
                  key={m}
                  type="button"
                  onClick={() => handleViewModeChange(m)}
                  title={m === "json" ? "JSON mode" : "Table mode"}
                  aria-label={m === "json" ? "JSON mode" : "Table mode"}
                  aria-pressed={viewMode === m}
                  className={`flex h-full items-center gap-1 rounded px-2 text-[11px] font-medium transition-colors ${
                    viewMode === m ? "bg-(--surface-2) text-(--text)" : "text-(--muted) hover:text-(--text)"
                  }`}
                >
                  {m === "json" ? <Braces size={11} aria-hidden="true" /> : <Table2 size={11} aria-hidden="true" />}
                  <span className="max-sm:hidden">{m === "json" ? "JSON" : "Table"}</span>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Content */}
      <div
        id="req-tabpanel"
        role="tabpanel"
        aria-labelledby={`req-tab-${activeTab}`}
        className="flex-1 flex flex-col min-h-0"
      >
      {activeTab === "body" && (
        <div className="flex shrink-0 items-center gap-2 border-b border-(--border) px-3 py-2">
        <span className="text-[11px] text-(--muted)" aria-hidden="true">Type</span>
        <div role="radiogroup" aria-label="Body type" className="flex items-center rounded-md border border-(--border) p-0.5">
          {BODY_TYPES.map((t) => (
            <button
              key={t.id}
              type="button"
              role="radio"
              aria-checked={bodyType === t.id}
              onClick={() => changeBodyType(t.id)}
              className={`rounded px-2.5 py-1 text-[11px] font-medium transition-colors ${
                bodyType === t.id ? "bg-(--accent-soft) text-(--brand)" : "text-(--muted) hover:text-(--text)"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
        </div>
      )}
      {activeTab === "body" && bodyType === "none" ? (
        <p className="flex-1 flex items-center justify-center p-6 text-xs text-(--muted)">
          This request has no body. Pick JSON, Text or Form to add one.
        </p>
      ) : activeTab === "body" && bodyType === "multipart" ? (
        <MultipartEditor ds={dataSource} />
      ) : activeTab === "body" && bodyType === "file" ? (
        <FileBodyEditor ds={dataSource} />
      ) : activeTab === "body" && bodyType === "form" ? (
        <KeyValueTable
          entries={form.rows}
          onChange={changeFormRows}
          keyPlaceholder="Field"
          valuePlaceholder="Value"
        />
      ) : activeTab === "body" && bodyType === "text" ? (
        <JsonEditor
          value={formInput.body}
          onChange={(val) => setFormInput((prev) => ({ ...prev, body: val }))}
          placeholder="Raw request body"
          tab="body"
          label="body text"
          error={null}
        />
      ) : viewMode === "table" ? (
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
    </div>
  );
}
