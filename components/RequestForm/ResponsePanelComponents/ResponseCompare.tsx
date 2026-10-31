'use client';
import { useMemo } from "react";
import { X } from "lucide-react";
import type { TResponseJson } from "@/types";
import { diffJson, diffHeaders, type DiffKind } from "@/utils/jsonDiff";
import { parseJsonPath } from "@/utils/jsonPath";

type Response = NonNullable<TResponseJson>;

const KIND_CLASS: Record<DiffKind, string> = {
  added: "text-(--success) border-(--success)/30 bg-(--success)/10",
  removed: "text-(--danger) border-(--danger)/30 bg-(--danger)/10",
  changed: "text-(--warning) border-(--warning)/30 bg-(--warning)/10",
};

function parseBody(data: unknown): { json: true; value: unknown } | { json: false; text: string } {
  if (data === undefined) return { json: false, text: "" };
  if (typeof data !== "string") return { json: true, value: data };
  try {
    return { json: true, value: JSON.parse(data) };
  } catch {
    return { json: false, text: data };
  }
}

const asText = (b: ReturnType<typeof parseBody>) => (b.json ? JSON.stringify(b.value, null, 2) : b.text);
const lineCount = (s: string) => (s === "" ? 0 : s.split("\n").length);

/** A value as JSON, cut short; the full text (up to 2,000 characters) is in the tooltip. */
function Val({ v, className }: { v: unknown; className: string }) {
  const s = JSON.stringify(v) ?? String(v);
  const long = s.length > 120;
  return (
    <span className={className} title={long ? s.slice(0, 2000) : undefined}>
      {long ? `${s.slice(0, 119)}…` : s}
    </span>
  );
}

function Values({ kind, before, after }: { kind: DiffKind; before?: unknown; after?: unknown }) {
  return (
    <span className="w-full font-mono text-xs break-all">
      {kind !== "added" && <Val v={before} className="text-white/70" />}
      {kind === "changed" && <span className="text-white/60"> → </span>}
      {kind !== "removed" && <Val v={after} className="text-white/90" />}
    </span>
  );
}

function Row({ kind, label, before, after }: { kind: DiffKind; label: string; before?: unknown; after?: unknown }) {
  return (
    <li className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5 border-b border-white/5 px-3 py-1.5">
      <span className={`rounded border px-1.5 text-xs ${KIND_CLASS[kind]}`}>{kind}</span>
      <code className="font-mono text-xs text-white/90 break-all">{label}</code>
      <Values kind={kind} before={before} after={after} />
    </li>
  );
}

/**
 * The current response against the one before it for the same request
 * (FR-14). `ignore` is comma-separated: `$…` entries are JSONPath patterns
 * for the body, anything else is a header name.
 */
export default function ResponseCompare({
  previous,
  current,
  ignore,
  onIgnoreChange,
  onClose,
}: {
  previous: Response;
  current: Response;
  ignore: string;
  onIgnoreChange: (v: string) => void;
  onClose: () => void;
}) {
  const d = useMemo(() => {
    const items = ignore.split(",").map((s) => s.trim()).filter(Boolean);
    const paths = items.filter((s) => s.startsWith("$"));
    const a = parseBody(previous.data);
    const b = parseBody(current.data);
    const body = a.json && b.json ? diffJson(a.value, b.value, { ignore: paths }) : null;
    const textBefore = asText(a);
    const textAfter = asText(b);
    const textDiffers = !body && textBefore !== textAfter;
    const headers = diffHeaders(previous.headers, current.headers, items.filter((s) => !s.startsWith("$")));
    return {
      body,
      textDiffers,
      lines: [lineCount(textBefore), lineCount(textAfter)],
      headers,
      badIgnores: paths.filter((p) => !parseJsonPath(p).ok),
      count: (body ? body.length : textDiffers ? 1 : 0) + headers.length,
    };
  }, [previous, current, ignore]);

  const st = (s?: number) => (s == null ? "—" : String(s));
  const statusChanged = previous.status !== current.status;
  const same = d.count === 0 && !statusChanged;
  const status = statusChanged ? `status ${st(previous.status)} → ${st(current.status)}` : `status ${st(current.status)}`;

  return (
    <div
      role="region"
      aria-label="Comparison with previous response"
      className="flex-1 min-h-0 overflow-auto"
      onKeyDown={(e) => e.key === "Escape" && onClose()}
    >
      <div className="sticky top-0 z-10 flex flex-wrap items-center gap-x-3 gap-y-1.5 border-b border-white/10 bg-(--surface-2) px-3 py-2">
        <h3 className="text-xs font-semibold text-white/90">Compared with the previous response</h3>
        <p className="text-xs text-white/70">
          {same ? status : `${d.count} ${d.count === 1 ? "change" : "changes"} · ${status}`}
        </p>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close compare"
          title="Close compare (Esc)"
          className="ml-auto p-1 rounded text-white/70 hover:text-white hover:bg-white/5"
        >
          <X className="h-3.5 w-3.5" aria-hidden="true" />
        </button>
        <label className="flex w-full items-center gap-2 text-xs text-white/70">
          <span className="shrink-0">Ignore paths</span>
          <input
            type="text"
            value={ignore}
            onChange={(e) => onIgnoreChange(e.target.value)}
            spellCheck={false}
            autoComplete="off"
            placeholder="$.updatedAt, $.items[*].id, date"
            className="min-w-0 flex-1 rounded border border-white/10 bg-(--surface-1) px-2 py-1 font-mono text-xs text-white/85 placeholder:text-white/60"
          />
        </label>
        {d.badIgnores.length > 0 && (
          <p aria-live="polite" className="w-full text-xs text-(--danger)">
            Invalid ignore path: {d.badIgnores.join(", ")}
          </p>
        )}
      </div>

      {same ? (
        <p className="p-6 text-center text-sm text-white/70">No differences</p>
      ) : (
        <>
          <h4 className="px-3 pt-3 pb-1 text-xs font-semibold uppercase tracking-wider text-white/70">Body</h4>
          {d.body ? (
            d.body.length === 0 ? (
              <p className="px-3 py-1.5 text-xs text-white/70">Same body</p>
            ) : (
              <ul aria-label="Body changes">
                {d.body.map((e) => (
                  <Row key={e.path} kind={e.kind} label={e.path} before={e.before} after={e.after} />
                ))}
              </ul>
            )
          ) : (
            <p className="px-3 py-1.5 text-xs text-white/70">
              {d.textDiffers ? `Body differs · ${d.lines[0]} → ${d.lines[1]} lines` : "Same body"}
            </p>
          )}
          {d.headers.length > 0 && (
            <>
              <h4 className="px-3 pt-3 pb-1 text-xs font-semibold uppercase tracking-wider text-white/70">Headers</h4>
              <ul aria-label="Header changes">
                {d.headers.map((h) => (
                  <Row key={h.name} kind={h.kind} label={h.name} before={h.before} after={h.after} />
                ))}
              </ul>
            </>
          )}
        </>
      )}
    </div>
  );
}
