"use client";
import { useMemo, useState, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import { ModalActions, ModalShell } from "@/components/ui/SharedModal";
import { useDataContext } from "@/context/dataContext";
import { useWorkspace } from "@/context/workspaceContext";
import { useNotification } from "@/hooks/useNotify";
import { METHOD_COLORS } from "@/constants/hittable";
import type { THittableCurlJson, THittableItem } from "@/types";
import { decodeShare } from "@/utils/shareLink";
import { jsonToCurl } from "@/utils/curlConverter";
import { createCollectionName, createCurlName } from "@/utils/hittableCollectionModifier";
import { selectionHref } from "@/utils/selectionHref";
import { createFile, fileExists } from "@/utils/workspace/fsHelpers";
import { serializeHitFile } from "@/utils/workspace/hitFileParser";

// FR-15: opening /hittable#share=… shows what the link holds and lets the
// user import it. Nothing here sends a request.

const SHARED = "Shared";

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

async function uniqueName(base: string, taken: (n: string) => boolean | Promise<boolean>, suffix = (i: number) => ` (${i})`) {
  let name = base;
  for (let i = 2; await taken(name); i++) name = base + suffix(i);
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

export default function SharePreview() {
  const hash = useSyncExternalStore(subscribe, () => window.location.hash, () => "");
  const shared = useMemo(() => (hash.startsWith("#share=") ? decodeShare(hash.slice(1)) : null), [hash]);
  const [busy, setBusy] = useState(false);
  const { collections, setCollections } = useDataContext();
  const { mode, hittableDir, directoryHandle, refreshTree, setActiveFile } = useWorkspace();
  const notify = useNotification();
  const router = useRouter();

  if (!shared) return null;

  if (!shared.ok) {
    return (
      <ModalShell title="Can't open share link" onClose={clearHash}>
        <p className="text-xs text-white/80">{shared.error}</p>
        <div className="flex justify-end">
          <button
            type="button"
            onClick={clearHash}
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

  const importLocal = async () => {
    const col = collections.find((c) => c.collectionName === SHARED);
    const items: THittableItem[] = col?.items ?? [];
    const routeName = await uniqueName(name.trim() || defaultName(request), (n) => items.some((i) => i.name === n));
    setCollections((prev) => {
      const withShared = prev.some((c) => c.collectionName === SHARED) ? prev : createCollectionName(prev, SHARED);
      return createCurlName(withShared, SHARED, routeName, jsonToCurl(request));
    });
    clearHash();
    router.push(selectionHref({ collectionName: SHARED, curlName: routeName, folderPath: [] }));
    notify.success({ title: `Imported into ${SHARED}`, desc: routeName });
  };

  const importFolder = async () => {
    if (!hittableDir || !directoryHandle) throw new Error("No folder is open");
    const dir = await hittableDir.getDirectoryHandle("shared", { create: true });
    const base = (name.trim() || defaultName(request)).replace(/[^\w.-]+/g, "-").replace(/^[-.]+|-+$/g, "").slice(0, 60) || "request";
    const stem = await uniqueName(base, (n) => fileExists(dir, `${n}.hit`), (i) => `-${i}`);
    const file = `${stem}.hit`;
    const handle = await createFile(
      dir,
      file,
      serializeHitFile({ method: request.method, url: request.url, headers, params, body: request.body, response: null }),
    );
    await refreshTree();
    clearHash();
    setActiveFile({ path: [hittableDir.name, "shared", file], handle });
    notify.success({ title: "Imported into hittable/shared", desc: file });
  };

  const onImport = async () => {
    setBusy(true);
    try {
      await (mode === "directory" ? importFolder() : importLocal());
    } catch (err) {
      notify.error({ title: "Import failed", desc: (err as Error).message || "The request could not be saved." });
    } finally {
      setBusy(false);
    }
  };

  return (
    <ModalShell title="Shared request" size="wide" onClose={clearHash}>
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
      <ModalActions onCancel={clearHash} onConfirm={onImport} confirmLabel="Import" cancelLabel="Dismiss" loading={busy} />
    </ModalShell>
  );
}
