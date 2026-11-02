"use client";

// The Body tab's Multipart and File editors. Bodies use the `@path` convention
// shared with the terminal app (docs/file-uploads.md).

import { useEffect, useMemo, useRef, useState } from "react";
import { ChevronDown, FileUp, FolderOpen, Laptop, Paperclip, Plus, Trash2 } from "lucide-react";
import type { DataSource } from "@/types/workspace";
import { ModalShell } from "@/components/ui/SharedModal";
import { useNotification } from "@/hooks/useNotify";
import { fuzzyScore } from "@/utils/fuzzy";
import { guessType, parseFileSpec, relativeRefPath, resolveRefPath } from "@/utils/fileBody";
import { getContentType } from "@/utils/bodyType";

type Row = { name: string; file: boolean; value: string };

/** Multipart rows from a body, or null when the body isn't a JSON object. */
export function rowsFromBody(body: string): Row[] | null {
  if (!body.trim()) return [];
  let obj: unknown;
  try {
    obj = JSON.parse(body);
  } catch {
    return null;
  }
  if (!obj || typeof obj !== "object" || Array.isArray(obj)) return null;
  return Object.entries(obj).map(([name, v]) => {
    if (typeof v !== "string") return { name, file: false, value: v === null ? "" : JSON.stringify(v) };
    if (v.startsWith("@@")) return { name, file: false, value: v.slice(1) };
    if (v.startsWith("@")) return { name, file: true, value: v.slice(1) };
    return { name, file: false, value: v };
  });
}

/** The body for multipart rows; a text value starting with @ is escaped as @@. */
export function rowsToBody(rows: Row[]): string {
  const obj: Record<string, string> = {};
  for (const r of rows) {
    if (!r.name.trim()) continue;
    obj[r.name] = r.file ? `@${r.value}` : r.value.startsWith("@") ? `@${r.value}` : r.value;
  }
  return JSON.stringify(obj, null, 2);
}

const fill = (path: string, env: Record<string, string>) => path.replace(/<<(\w+)>>/g, (m, k) => (Object.prototype.hasOwnProperty.call(env, k) ? env[k] : m));

/** Where a file spec points, and whether the workspace has it. */
function fileState(spec: string, ds: DataSource): { ok: boolean; message: string } | null {
  if (!spec.trim()) return null;
  try {
    const { path } = parseFileSpec(spec);
    const at = resolveRefPath(ds.hitPath ?? [], fill(path, ds.env ?? {})).join("/");
    const found = ds.workspaceFiles?.some((p) => p.join("/") === at);
    return found ? { ok: true, message: at } : { ok: false, message: `Not found: ${at}` };
  } catch (err) {
    return { ok: false, message: (err as Error).message };
  }
}

/** A file reference: an editable path, its state, Choose… and drag-and-drop. */
export function FileCell({ value, onChange, ds, label, large }: { value: string; onChange: (spec: string) => void; ds: DataSource; label: string; large?: boolean }) {
  const notify = useNotification();
  const [menu, setMenu] = useState(false);
  const [picker, setPicker] = useState(false);
  const [over, setOver] = useState(false);
  const [busy, setBusy] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  const root = useRef<HTMLDivElement>(null);
  const state = fileState(value, ds);
  const canCopy = !!ds.addUploadFile && !ds.readOnly;

  useEffect(() => {
    if (!menu) return;
    const close = (e: MouseEvent) => !root.current?.contains(e.target as Node) && setMenu(false);
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [menu]);

  const pickPath = (path: string[]) => {
    // Options after the path (;type=…) are kept when the file changes.
    const opts = /;(?:type|filename)=.*$/.exec(value)?.[0] ?? "";
    onChange(relativeRefPath(ds.hitPath ?? [], path) + opts);
  };
  const copyIn = async (file: File | undefined) => {
    if (!file || !ds.addUploadFile) return;
    setBusy(true);
    try {
      const path = await ds.addUploadFile(file);
      pickPath(path);
      notify.success({ title: `Added ${path.at(-1)}`, desc: `Copied to ${path.join("/")}.` });
    } catch (err) {
      notify.error({ title: `Couldn't add ${file.name}`, desc: (err as Error).message, timeout: 8000 });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div
      ref={root}
      className={`relative flex min-w-0 items-center gap-1.5 ${large ? "rounded-lg border-2 border-dashed p-4" : "pl-2.5 pr-1"} ${
        over ? "border-(--brand) bg-(--accent-soft)" : large ? "border-(--border-strong)" : ""
      }`}
      onDragOver={(e) => {
        if (!canCopy || !e.dataTransfer.types.includes("Files")) return;
        e.preventDefault();
        setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(e) => {
        if (!canCopy || !e.dataTransfer.files.length) return;
        e.preventDefault();
        setOver(false);
        void copyIn(e.dataTransfer.files[0]);
      }}
    >
      <Paperclip size={12} className="shrink-0 text-(--muted)" aria-hidden="true" />
      <input
        type="text"
        aria-label={label}
        value={value}
        readOnly={ds.readOnly}
        placeholder={canCopy ? "Drop a file, or choose one" : "path/to/file"}
        onChange={(e) => onChange(e.target.value)}
        spellCheck={false}
        className="h-8 min-w-0 flex-1 bg-transparent font-mono text-xs text-(--text) outline-none placeholder:text-(--muted)/50"
      />
      {state && (
        <span
          title={state.message}
          className={`max-w-40 shrink-0 truncate text-[11px] ${state.ok ? "text-(--success)" : "text-(--danger)"}`}
          role={state.ok ? undefined : "status"}
        >
          {state.ok ? "✓" : state.message}
        </span>
      )}
      {!ds.readOnly && (
        <button
          type="button"
          aria-haspopup="menu"
          aria-expanded={menu}
          disabled={busy}
          onClick={() => setMenu((m) => !m)}
          className="flex h-6 shrink-0 items-center gap-1 rounded border border-(--border) px-1.5 text-[11px] text-(--text) hover:bg-(--surface-2) disabled:opacity-50"
        >
          {busy ? "Adding…" : "Choose"}
          <ChevronDown size={11} aria-hidden="true" />
        </button>
      )}
      {menu && (
        <div role="menu" aria-label="Choose a file" className="absolute right-0 top-full z-50 mt-1 min-w-48 rounded-md border border-(--border-strong) bg-(--surface-1) py-1 shadow-xl shadow-black/40">
          {canCopy && (
            <button type="button" role="menuitem" className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs text-(--text) hover:bg-(--surface-2)"
              onClick={() => { setMenu(false); input.current?.click(); }}>
              <Laptop size={12} aria-hidden="true" /> From this computer…
            </button>
          )}
          <button type="button" role="menuitem" className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs text-(--text) hover:bg-(--surface-2)"
            onClick={() => { setMenu(false); setPicker(true); }}>
            <FolderOpen size={12} aria-hidden="true" /> From the workspace…
          </button>
        </div>
      )}
      <input ref={input} type="file" className="hidden" tabIndex={-1} aria-hidden="true"
        onChange={(e) => { void copyIn(e.target.files?.[0]); e.target.value = ""; }} />
      {picker && <WorkspaceFilePicker files={ds.workspaceFiles ?? []} onPick={(p) => { setPicker(false); pickPath(p); }} onClose={() => setPicker(false)} />}
    </div>
  );
}

function WorkspaceFilePicker({ files, onPick, onClose }: { files: string[][]; onPick: (path: string[]) => void; onClose: () => void }) {
  const [q, setQ] = useState("");
  const [active, setActive] = useState(0);
  const results = useMemo(() => {
    const all = files.map((p) => ({ path: p, text: p.join("/") }));
    if (!q.trim()) return all.slice(0, 200);
    return all
      .map((f) => ({ ...f, score: fuzzyScore(q, f.text) }))
      .filter((f) => f.score !== null)
      .sort((a, b) => b.score! - a.score!)
      .slice(0, 200);
  }, [files, q]);

  return (
    <ModalShell title="Choose a file from the workspace" subtitle="Its path is stored relative to this request." onClose={onClose}>
      <input
        autoFocus
        aria-label="Search files"
        aria-controls="workspace-file-results"
        value={q}
        onChange={(e) => { setQ(e.target.value); setActive(0); }}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") { e.preventDefault(); setActive((i) => Math.min(i + 1, results.length - 1)); }
          if (e.key === "ArrowUp") { e.preventDefault(); setActive((i) => Math.max(i - 1, 0)); }
          if (e.key === "Enter" && results[active]) { e.preventDefault(); onPick(results[active].path); }
        }}
        placeholder="Type part of a file name"
        className="h-9 w-full rounded-md border border-(--border-strong) bg-(--surface-1) px-3 text-sm text-(--text) outline-none focus:border-(--brand)"
      />
      <ul id="workspace-file-results" role="listbox" aria-label="Files" className="max-h-72 overflow-y-auto rounded-md border border-(--border)">
        {results.length === 0 && <li className="px-3 py-2 text-xs text-(--muted)">No files match.</li>}
        {results.map((f, i) => (
          <li key={f.text} role="option" aria-selected={i === active}>
            <button type="button" onClick={() => onPick(f.path)} onMouseEnter={() => setActive(i)}
              className={`block w-full truncate px-3 py-1.5 text-left font-mono text-xs ${i === active ? "bg-(--surface-2) text-(--brand)" : "text-(--text)"}`}>
              {f.text}
            </button>
          </li>
        ))}
      </ul>
    </ModalShell>
  );
}

/** Multipart fields: text or file per row. */
export function MultipartEditor({ ds }: { ds: DataSource }) {
  const { formInput, setFormInput } = ds;
  const body = formInput.body ?? "";
  // Rows live here as well as in the body: a row without a name isn't in the body yet.
  const [state, setState] = useState(() => ({ rows: rowsFromBody(body), rowsFor: body }));
  if (body !== state.rowsFor) setState({ rows: rowsFromBody(body), rowsFor: body });
  const rows = state.rows;

  if (!rows) {
    return (
      <p role="status" className="m-3 rounded-md border border-(--danger)/25 bg-(--danger)/10 px-3 py-2 text-xs text-(--danger)">
        A multipart body is a JSON object of fields, like {'{"name": "x", "file": "@files/a.png"}'}. Fix it in Text view, or pick another body type and back.
      </p>
    );
  }
  // An empty body shows one blank row; adding a row replaces it rather than leaving it above.
  const shown = rows.length ? rows : [{ name: "", file: false, value: "" }];
  const change = (next: Row[]) => {
    const text = rowsToBody(next);
    setState({ rows: next, rowsFor: text });
    setFormInput((prev) => ({ ...prev, body: text }));
  };
  const set = (i: number, patch: Partial<Row>) => change(shown.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  const cell = "h-8 w-full min-w-0 bg-transparent px-2.5 font-mono text-xs text-(--text) placeholder:text-(--muted)/50 outline-none focus:bg-(--surface-2)";

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto p-2 md:p-3">
      <div className="rounded-md border border-(--border)">
        <div className="grid grid-cols-[minmax(0,1fr)_5rem_minmax(0,1.6fr)_2rem] border-b border-(--border) bg-(--surface-3) text-[11px] font-medium text-(--muted)" aria-hidden="true">
          <span className="px-2.5 py-1.5">Field</span>
          <span className="border-l border-(--border) px-2.5 py-1.5">Type</span>
          <span className="border-l border-(--border) px-2.5 py-1.5">Value</span>
          <span />
        </div>
        {shown.map((r, i) => (
          <div key={i} className="group grid grid-cols-[minmax(0,1fr)_5rem_minmax(0,1.6fr)_2rem] border-b border-(--border) hover:bg-(--surface-2)/40">
            <input type="text" aria-label={`Field ${i + 1}`} placeholder="Field" value={r.name} readOnly={ds.readOnly}
              onChange={(e) => set(i, { name: e.target.value })} className={cell} />
            <select aria-label={`Field ${i + 1} type`} value={r.file ? "file" : "text"} disabled={ds.readOnly}
              onChange={(e) => set(i, { file: e.target.value === "file", value: "" })}
              className="h-8 border-l border-(--border) bg-transparent px-1.5 text-xs text-(--text) outline-none focus:bg-(--surface-2)">
              <option value="text" className="bg-(--surface-1)">Text</option>
              <option value="file" className="bg-(--surface-1)">File</option>
            </select>
            <div className="min-w-0 border-l border-(--border)">
              {r.file ? (
                <FileCell value={r.value} onChange={(v) => set(i, { value: v })} ds={ds} label={`Field ${i + 1} file`} />
              ) : (
                <input type="text" aria-label={`Field ${i + 1} value`} placeholder="Value" value={r.value} readOnly={ds.readOnly}
                  onChange={(e) => set(i, { value: e.target.value })} className={cell} />
              )}
            </div>
            {!ds.readOnly && (
              <button type="button" aria-label="Remove field" title="Remove field" onClick={() => change(shown.filter((_, j) => j !== i))}
                className="flex items-center justify-center text-(--muted) opacity-0 transition-opacity hover:text-(--danger) focus-visible:opacity-100 group-hover:opacity-100 group-focus-within:opacity-100">
                <Trash2 size={12} aria-hidden="true" />
              </button>
            )}
          </div>
        ))}
        {!ds.readOnly && (
          <div className="flex">
            <button type="button" onClick={() => change([...rows, { name: "", file: false, value: "" }])}
              className="flex flex-1 items-center gap-1.5 px-2.5 py-2 text-xs text-(--muted) hover:bg-(--surface-2)/60 hover:text-(--brand)">
              <Plus size={12} aria-hidden="true" /> Add field
            </button>
            <button type="button" onClick={() => change([...rows, { name: "", file: true, value: "" }])}
              className="flex flex-1 items-center gap-1.5 border-l border-(--border) px-2.5 py-2 text-xs text-(--muted) hover:bg-(--surface-2)/60 hover:text-(--brand)">
              <FileUp size={12} aria-hidden="true" /> Add file
            </button>
          </div>
        )}
      </div>
      <p className="mt-2 text-[11px] leading-4 text-(--muted)">
        Sent as <code className="font-mono">multipart/form-data</code>. Files are read from the workspace when you send; paths are relative to this request.
      </p>
    </div>
  );
}

/** The whole body is one file. */
export function FileBodyEditor({ ds }: { ds: DataSource }) {
  const { formInput, setFormInput } = ds;
  const body = (formInput.body ?? "").trim();
  const spec = body.startsWith("@") ? body.slice(1) : body;
  const header = getContentType(formInput.headers ?? "");
  let type = header;
  if (!type && spec) {
    try {
      const f = parseFileSpec(spec);
      type = f.type || guessType(f.path);
    } catch {
      type = "";
    }
  }
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-3">
      <FileCell large value={spec} ds={ds} label="Body file" onChange={(v) => setFormInput((prev) => ({ ...prev, body: v ? `@${v}` : "" }))} />
      <p className="text-[11px] leading-4 text-(--muted)">
        The file&apos;s bytes are the whole body.
        {type && (
          <> Content-Type: <code className="font-mono text-(--text)">{type}</code>{header ? " (from Headers)" : " (from the file name; set one in Headers to override)"}.</>
        )}
      </p>
    </div>
  );
}
