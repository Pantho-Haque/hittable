"use client";
import { useEffect, useMemo, useState, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import { ModalActions, ModalShell } from "@/components/ui/SharedModal";
import { useWorkspace } from "@/context/workspaceContext";
import { useNotification } from "@/hooks/useNotify";
import { METHOD_COLORS } from "@/constants/hittable";
import type { THittableCurlJson } from "@/types";
import { decodeShare } from "@/utils/shareLink";
import { findNode, HITTABLE_DIR } from "@/services/workspace/backend";
import { serializeHitFile } from "@/utils/workspace/hitFileParser";

// FR-15: opening /hittable#share=… shows what the link holds and lets the
// user import it. Nothing here sends a request.

const subscribe = (cb: () => void) => {
  window.addEventListener("hashchange", cb);
  return () => window.removeEventListener("hashchange", cb);
};

/** Remove the fragment, keeping Next's history state, and let readers of the hash know. */
function clearHash() {
  window.history.replaceState(window.history.state, "", window.location.pathname + window.location.search);
  window.dispatchEvent(new HashChangeEvent("hashchange"));
}

function defaultName({ method, url }: THittableCurlJson) {
  let where = url.split(/[?#]/)[0];
  try {
    const u = new URL(url);
    where = u.host + u.pathname;
  } catch {
    // a <<BASE>> template or a partial URL: use it as written
  }
  return `${method} ${where.replace(/\/$/, "")}`.trim().slice(0, 120);
}

function uniqueName(base: string, taken: (n: string) => boolean) {
  let name = base;
  for (let i = 2; taken(name); i++) name = `${base}-${i}`;
  return name;
}

const parseMap = (s: string): Record<string, string> => JSON.parse(s || "{}");

function KeyValues({ label, map }: { label: string; map: Record<string, string> }) {
  const rows = Object.entries(map);
  if (!rows.length) return null;
  return (
    <section>
      <h3 className="mb-1 text-[11px] uppercase tracking-wider text-white/60">{label}</h3>
      <table className="w-full text-left text-xs">
        <tbody>
          {rows.map(([k, v]) => (
            <tr key={k} className="border-t border-white/5 align-top">
              <th scope="row" className="py-1 pr-3 font-normal text-white/70 break-all">{k}</th>
              <td className="py-1 text-white/90 break-all">{v}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

// A link opened with no workspace (it lands on the launcher) is remembered for
// this tab; when a folder or repository opens, the preview comes back there.
const PENDING_KEY = "hittable_pending_share";
const pending = {
  get: () => { try { return sessionStorage.getItem(PENDING_KEY); } catch { return null; } },
  set: (v: string) => { try { sessionStorage.setItem(PENDING_KEY, v); } catch { /* storage blocked */ } },
  clear: () => { try { sessionStorage.removeItem(PENDING_KEY); } catch { /* storage blocked */ } },
};
const folderSupported = () => typeof window !== "undefined" && "showDirectoryPicker" in window;

export default function SharePreview() {
  const hash = useSyncExternalStore(subscribe, () => window.location.hash, () => "");
  const shared = useMemo(() => (hash.startsWith("#share=") ? decodeShare(hash.slice(1)) : null), [hash]);
  const [busy, setBusy] = useState(false);
  const { backendKind, readOnly, tree, createEntry, setActiveFile, openFolder } = useWorkspace();
  const notify = useNotification();
  const router = useRouter();

  // Remember a link that arrived with nothing open; replay it once a
  // writable workspace is open.
  useEffect(() => {
    if (!backendKind && hash.startsWith("#share=")) pending.set(hash);
    if (backendKind && !readOnly && !hash) {
      const saved = pending.get();
      if (saved) {
        pending.clear();
        window.location.hash = saved.slice(1);
      }
    }
  }, [backendKind, readOnly, hash]);
  const dismiss = () => {
    pending.clear();
    clearHash();
  };

  if (!shared) return null;

  if (!shared.ok) {
    return (
      <ModalShell title="Can't open share link" onClose={dismiss}>
        <p className="text-xs text-white/80">{shared.error}</p>
        <div className="flex justify-end">
          <button
            type="button"
            onClick={dismiss}
            className="rounded-md border border-white/10 px-4 py-1.5 text-xs text-white/80 hover:bg-white/5 focus-visible:outline-2 focus-visible:outline-(--brand)"
          >
            Dismiss
          </button>
        </div>
      </ModalShell>
    );
  }

  const { request, name } = shared;
  const headers = parseMap(request.headers);
  const params = parseMap(request.params);

  const importFolder = async () => {
    const dir = [HITTABLE_DIR, "shared"];
    const base = (name.trim() || defaultName(request)).replace(/[^\w.-]+/g, "-").replace(/^[-.]+|-+$/g, "").slice(0, 60) || "request";
    const file = `${uniqueName(base, (n) => !!findNode(tree, [...dir, `${n}.hit`]))}.hit`;
    await createEntry(
      [...dir, file],
      "file",
      serializeHitFile({ method: request.method, url: request.url, headers, params, body: request.body, response: null }),
    );
    pending.clear();
    clearHash();
    setActiveFile([...dir, file]);
    notify.success({ title: "Imported into hittable/shared", desc: file });
  };

  // Import needs a workspace it can write to.
  const blocked = !backendKind
    ? "To import it, open a local folder or a GitHub repository — this preview comes back there."
    : readOnly
      ? "This workspace is read-only, so the request can't be imported here."
      : null;

  const onImport = async () => {
    setBusy(true);
    try {
      await importFolder();
    } catch (err) {
      notify.error({ title: "Import failed", desc: (err as Error).message || "The request could not be saved." });
    } finally {
      setBusy(false);
    }
  };

  return (
    <ModalShell title="Shared request" size="wide" onClose={dismiss}>
      <div
        role="region"
        aria-label="Shared request details"
        tabIndex={0}
        className="flex min-h-0 flex-col gap-3 overflow-auto font-mono focus-visible:outline-2 focus-visible:outline-(--brand)"
      >
        {name && <p className="text-xs text-white/80 break-all">{name}</p>}
        <p className="text-xs break-all">
          <span className="mr-2 font-bold" style={{ color: METHOD_COLORS[request.method] }}>{request.method}</span>
          <span className="text-white/90">{request.url}</span>
        </p>
        <KeyValues label="Headers" map={headers} />
        <KeyValues label="Params" map={params} />
        {request.body && (
          <section>
            <h3 className="mb-1 text-[11px] uppercase tracking-wider text-white/60">Body</h3>
            <pre className="whitespace-pre-wrap break-all rounded-md bg-(--surface-0) p-2 text-xs text-white/90">{request.body}</pre>
          </section>
        )}
      </div>
      <p className="text-xs text-white/70">
        Nothing has been sent. Variables like {"<<TOKEN>>"} need values in your Environment.
      </p>
      {blocked ? (
        <>
          <p className="text-xs text-(--warning)">{blocked}</p>
          <div className="flex justify-end gap-2">
            {!backendKind && folderSupported() && (
              <button
                type="button"
                onClick={async () => {
                  // The click is the user gesture the folder picker needs.
                  if (await openFolder()) router.push(`/hittable/folder${window.location.hash}`);
                }}
                className="rounded-md bg-(--brand) px-4 py-1.5 text-xs font-semibold text-(--accent-ink) hover:opacity-90 focus-visible:outline-2 focus-visible:outline-(--brand)"
              >
                Open a local folder
              </button>
            )}
            <button
              type="button"
              onClick={dismiss}
              className="rounded-md border border-white/10 px-4 py-1.5 text-xs text-white/80 hover:bg-white/5 focus-visible:outline-2 focus-visible:outline-(--brand)"
            >
              Dismiss
            </button>
          </div>
        </>
      ) : (
        <ModalActions onCancel={dismiss} onConfirm={onImport} confirmLabel="Import" cancelLabel="Dismiss" loading={busy} />
      )}
    </ModalShell>
  );
}
