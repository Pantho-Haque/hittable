"use client";
import { useEffect, useState, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import { AlertCircle, Archive, FileText, FolderOpen, Layers, RotateCcw, Zap } from "lucide-react";
import { useWorkspace } from "@/context/workspaceContext";
import { loadDirectoryHandle } from "@/services/workspace/HandleStore";
import WorkspaceConnecting from "@/components/workspace/WorkspaceConnecting";
import GitHubCard from "@/components/launcher/GitHubCard";
import {
  dismissLegacy,
  downloadLegacyCollections,
  readLegacyCollections,
  type LegacyCollection,
} from "@/utils/legacyExport";

const AUTH_ERRORS: Record<string, string> = {
  state_mismatch: "Sign-in didn't finish because the request couldn't be verified. It may have expired; please try again.",
  missing_verifier: "Sign-in took too long and expired. Please try again.",
  exchange_failed: "GitHub didn't complete the sign-in. Please try again in a moment.",
  denied: "Sign-in was cancelled on GitHub. Nothing was changed.",
  config: "GitHub sign-in isn't set up on this server yet. You can still open a local folder.",
};

const noSubscribe = () => () => {};
const folderSupported = () => "showDirectoryPicker" in window;
const search = () => window.location.search;

const primary =
  "inline-flex min-h-10 items-center justify-center gap-2 rounded-lg bg-(--brand) px-4 py-2 text-sm font-semibold text-(--ink-950) hover:opacity-90 disabled:cursor-progress disabled:opacity-60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--brand)";
const secondary =
  "inline-flex min-h-10 items-center justify-center gap-2 rounded-lg border border-white/15 px-4 py-2 text-sm text-white/80 hover:bg-white/5 disabled:cursor-progress disabled:opacity-60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--brand)";

function MigrationBanner() {
  const [legacy, setLegacy] = useState<LegacyCollection[] | null>(null);
  useEffect(() => {
    setLegacy(readLegacyCollections()); // eslint-disable-line react-hooks/set-state-in-effect -- browser storage, after hydration
  }, []);
  if (!legacy) return null;

  return (
    <section
      aria-labelledby="legacy-title"
      className="flex flex-col gap-3 rounded-xl border border-(--warning)/40 bg-(--surface-1) p-4 sm:flex-row sm:items-center"
    >
      <Archive size={20} className="shrink-0 text-(--warning)" aria-hidden="true" />
      <div className="flex-1 space-y-1">
        <h2 id="legacy-title" className="text-sm font-semibold text-white/90">
          Your old browser collections are still here — download them before they&apos;re gone
        </h2>
        <p className="text-xs leading-5 text-white/70">
          {legacy.length === 1 ? "1 collection" : `${legacy.length} collections`} from the old Local mode, as Postman
          collections you can import anywhere. Secret values are not included.
        </p>
      </div>
      <div className="flex shrink-0 gap-2">
        <button type="button" className={primary} onClick={() => downloadLegacyCollections(legacy)}>
          Download
        </button>
        <button
          type="button"
          className={secondary}
          onClick={() => {
            dismissLegacy();
            setLegacy(null);
          }}
        >
          Dismiss
        </button>
      </div>
    </section>
  );
}

function FolderCard() {
  const { openFolder, reconnectFolder, backendKind, connectingStage } = useWorkspace();
  const router = useRouter();
  const supported = useSyncExternalStore(noSubscribe, folderSupported, () => true);
  const [saved, setSaved] = useState<string | null>(null);
  const busy = connectingStage !== null;

  useEffect(() => {
    void loadDirectoryHandle().then((h) => setSaved(h?.name ?? null));
  }, []);

  const open = async () => {
    if (await openFolder()) router.push("/hittable/folder");
  };
  // A click is the user gesture the folder picker needs.
  const reconnect = async () => {
    if (backendKind === "folder" || (await reconnectFolder(true))) router.push("/hittable/folder");
  };

  return (
    <section aria-labelledby="folder-card-title" className="flex flex-col gap-4 rounded-xl border border-(--border) bg-(--surface-1) p-5">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-(--border) bg-(--surface-2)">
          <FolderOpen size={18} className="text-(--brand)" aria-hidden="true" />
        </div>
        <div>
          <h2 id="folder-card-title" className="text-sm font-semibold text-(--text)">A folder on this computer</h2>
          <p className="text-xs text-(--muted)">No account needed</p>
        </div>
      </div>
      <p className="text-sm leading-6 text-white/70">
        Edits save straight to your files. A <code className="rounded bg-white/5 px-1 py-0.5 text-white/80">hittable/</code>{" "}
        folder is added if it isn&apos;t there.
      </p>
      {supported ? (
        <div className="flex flex-col gap-2">
          <button type="button" className={`${primary} w-full`} onClick={open} disabled={busy} aria-busy={busy}>
            <FolderOpen size={16} aria-hidden="true" /> Open a local folder
          </button>
          {saved && (
            <button type="button" className={`${secondary} w-full`} onClick={reconnect} disabled={busy} title="Open the last folder again">
              <RotateCcw size={16} aria-hidden="true" /> <span className="truncate">Reconnect {saved}</span>
            </button>
          )}
        </div>
      ) : (
        <p role="note" className="rounded-lg border border-(--border) bg-(--surface-2) p-3 text-xs leading-5 text-white/70">
          Opening a local folder needs Chrome or Edge on a computer (the File System Access API). In this browser, use a
          GitHub repository instead.
        </p>
      )}
    </section>
  );
}

const WORKSPACE = [
  { icon: Zap, name: ".hit files", text: "One request each: method, URL, headers, body" },
  { icon: Layers, name: "env.json", text: "Environments and <<KEY>> values" },
  { icon: FileText, name: "Markdown", text: "Notes that live next to the requests" },
];

/** What a workspace holds: the same files in the browser, the terminal app and your editor. */
function WorkspaceFacts() {
  return (
    <section aria-labelledby="facts-title" className="rounded-xl border border-(--border) p-5">
      <h2 id="facts-title" className="text-[11px] font-semibold uppercase tracking-wide text-(--muted)">
        Inside hittable/
      </h2>
      <ul className="mt-3 space-y-3">
        {WORKSPACE.map(({ icon: Icon, name, text }) => (
          <li key={name} className="flex items-start gap-3">
            <Icon size={14} className="mt-0.5 shrink-0 text-(--muted)" aria-hidden="true" />
            <span className="text-xs leading-5 text-(--muted)">
              <span className="font-mono text-(--text)">{name}</span> · {text}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** /hittable: choose where to work. */
export default function Launcher() {
  const { connectingStage } = useWorkspace();
  const query = useSyncExternalStore(noSubscribe, search, () => "");
  const authError = new URLSearchParams(query).get("auth_error");

  return (
    <div className="relative min-h-[calc(100dvh-44px)] w-full overflow-auto bg-(--surface-0)">
      {connectingStage && <WorkspaceConnecting stage={connectingStage} />}
      <div className="mx-auto flex max-w-5xl flex-col gap-6 px-4 py-10 sm:py-14">
        <header className="space-y-2 border-b border-(--border) pb-6">
          <h1 className="text-2xl font-semibold tracking-tight text-white/95">Open a workspace</h1>
          <p className="max-w-xl text-sm leading-6 text-(--muted)">
            Hittable keeps your requests as <code className="font-mono text-white/80">.hit</code> files next to your code.
            Pick where they live.
          </p>
        </header>

        {authError && (
          <div role="alert" className="flex items-start gap-3 rounded-xl border border-(--danger)/40 bg-(--surface-1) p-4">
            <AlertCircle size={18} className="mt-0.5 shrink-0 text-(--danger)" aria-hidden="true" />
            <p className="text-sm leading-6 text-white/80">
              {AUTH_ERRORS[authError] ?? "Sign-in with GitHub didn't work. Please try again."}
            </p>
          </div>
        )}

        <MigrationBanner />

        <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
          <GitHubCard />
          <aside aria-label="Local folder" className="flex flex-col gap-4">
            <FolderCard />
            <WorkspaceFacts />
          </aside>
        </div>
      </div>
    </div>
  );
}
