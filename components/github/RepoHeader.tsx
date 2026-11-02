"use client";
import { useEffect, useId, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { AlertTriangle, Check, ChevronDown, GitBranch, GitCompareArrows, Github, Lock, Plus, RefreshCw, X } from "lucide-react";
import { ModalShell } from "@/components/ui/SharedModal";
import { useWorkspace } from "@/context/workspaceContext";
import { api, describeError, signInUrl, type Branch } from "@/utils/github/api";
import type { GitHubBackend } from "@/services/github/githubBackend";
import { fuzzyScore } from "@/utils/fuzzy";
import { useBackend } from "./SourceControlPanel";

const POPOVER = "absolute left-0 top-[calc(100%+6px)] z-40 w-72 max-w-[calc(100vw-2rem)] rounded-md border border-(--border-strong) bg-(--ink-800) shadow-lg";

export const repoUrl = (owner: string, repo: string, branch?: string) =>
  `/hittable/gh/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}${branch ? `?ref=${encodeURIComponent(branch)}` : ""}`;

/** Close the popover on a click outside `root` or on Escape (focus back on the trigger). */
function useDismiss(open: boolean, close: () => void, root: React.RefObject<HTMLElement | null>) {
  const closeRef = useRef(close);
  useEffect(() => {
    closeRef.current = close;
  });
  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => !root.current?.contains(e.target as Node) && closeRef.current();
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && !document.querySelector("[data-modal]") && closeRef.current();
    document.addEventListener("pointerdown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open, root]);
}

/** FR-G3: search and switch branches, or create one from the current commit. */
export function BranchSwitcher({ backend }: { backend: GitHubBackend }) {
  const router = useRouter();
  const listId = useId();
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const [branches, setBranches] = useState<Branch[] | null>(null);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const close = () => {
    setOpen(false);
    trigger.current?.focus();
  };
  useDismiss(open, () => setOpen(false), root);

  useEffect(() => {
    if (!open) return;
    let live = true;
    api.branches(backend.owner, backend.repo).then(
      (b) => live && setBranches(b),
      (err) => live && setError(describeError(err)),
    );
    return () => {
      live = false;
    };
  }, [open, backend.owner, backend.repo]);

  const q = query.trim();
  const matches = (branches ?? [])
    .map((b) => ({ b, s: q ? fuzzyScore(q, b.name) : 0 }))
    .filter((x) => x.s !== null)
    .sort((a, b) => (b.s ?? 0) - (a.s ?? 0))
    .map((x) => x.b);
  const canCreate = !backend.readOnly && !!q && !!branches && !branches.some((b) => b.name === q);
  const count = matches.length + (canCreate ? 1 : 0);
  const current = Math.min(active, Math.max(count - 1, 0));
  const optionId = (i: number) => `${listId}-${i}`;

  const switchTo = (name: string) => {
    setOpen(false);
    if (name !== backend.branch) router.push(repoUrl(backend.owner, backend.repo, name));
  };
  const create = async () => {
    setBusy(true);
    setError(null);
    try {
      await backend.createBranchFromCurrent(q);
      setOpen(false);
      router.push(repoUrl(backend.owner, backend.repo, q));
    } catch (err) {
      setError(describeError(err));
    } finally {
      setBusy(false);
    }
  };
  const choose = (i: number) => (i < matches.length ? switchTo(matches[i].name) : void create());

  return (
    <div ref={root} className="relative">
      <button
        ref={trigger}
        type="button"
        className="workspace-button max-w-48 border border-(--border-strong)"
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-label={`Branch: ${backend.branch}. Switch branch`}
        onClick={() => {
          setOpen((o) => !o);
          setQuery("");
          setActive(0);
          setError(null);
        }}
      >
        <GitBranch size={14} aria-hidden="true" />
        <span className="truncate font-mono">{backend.branch}</span>
        <ChevronDown size={14} aria-hidden="true" />
      </button>
      {open && (
        <div role="dialog" aria-label="Switch branch" className={POPOVER}>
          <div className="border-b border-(--border) p-2">
            <input
              autoFocus
              role="combobox"
              aria-label={backend.readOnly ? "Find a branch" : "Find or create a branch"}
              aria-expanded="true"
              aria-controls={listId}
              aria-activedescendant={count ? optionId(current) : undefined}
              placeholder={backend.readOnly ? "Find a branch" : "Find or create a branch"}
              value={query}
              spellCheck={false}
              onChange={(e) => {
                setQuery(e.target.value);
                setActive(0);
              }}
              onKeyDown={(e) => {
                if (e.key === "ArrowDown" || e.key === "ArrowUp") {
                  e.preventDefault();
                  setActive(Math.max(0, Math.min(count - 1, current + (e.key === "ArrowDown" ? 1 : -1))));
                } else if (e.key === "Enter" && count) {
                  e.preventDefault();
                  choose(current);
                } else if (e.key === "Escape") {
                  e.preventDefault();
                  close();
                }
              }}
              className="min-h-8 w-full rounded border border-(--border-strong) bg-(--surface-0) px-2 font-mono text-xs text-(--text) placeholder:text-(--muted) focus-visible:outline-2 focus-visible:outline-(--brand)"
            />
          </div>
          {error && <p role="alert" className="px-3 py-2 text-xs text-(--danger)">{error}</p>}
          {!branches && !error ? (
            <p role="status" className="px-3 py-2 text-xs text-(--muted)">Loading branches…</p>
          ) : (
            <div id={listId} role="listbox" aria-label="Branches" className="max-h-64 overflow-y-auto py-1">
              {matches.map((b, i) => (
                <div
                  key={b.name}
                  id={optionId(i)}
                  role="option"
                  aria-selected={i === current}
                  onClick={() => switchTo(b.name)}
                  onMouseMove={() => i !== current && setActive(i)}
                  className="flex min-h-8 cursor-pointer items-center gap-2 px-3 text-xs text-(--text) aria-selected:bg-(--ink-500)"
                >
                  <Check size={14} className={b.name === backend.branch ? "text-(--brand)" : "invisible"} aria-hidden="true" />
                  <span className="truncate font-mono">{b.name}</span>
                  {b.name === backend.branch && <span className="sr-only">(current)</span>}
                  {b.protected && <Lock size={12} className="ml-auto shrink-0 text-(--muted)" aria-label="Protected" />}
                </div>
              ))}
              {canCreate && (
                <div
                  id={optionId(matches.length)}
                  role="option"
                  aria-selected={current === matches.length}
                  aria-disabled={busy}
                  onClick={() => !busy && void create()}
                  onMouseMove={() => setActive(matches.length)}
                  className="flex min-h-8 cursor-pointer items-center gap-2 border-t border-(--border) px-3 text-xs text-(--text) aria-selected:bg-(--ink-500)"
                >
                  <Plus size={14} className="shrink-0 text-(--brand)" aria-hidden="true" />
                  <span className="truncate">
                    {busy ? "Creating…" : <>Create branch <span className="font-mono">{q}</span> from {backend.branch}</>}
                  </span>
                </div>
              )}
              {count === 0 && <p className="px-3 py-2 text-xs text-(--muted)">No branch matches.</p>}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/** FR-G6: up to date, checking, or the files changed on both sides. */
export function SyncPill({ backend, onCompare }: { backend: GitHubBackend; onCompare: (path: string) => void }) {
  useBackend(backend);
  const { activeFile, setActiveFile } = useWorkspace();
  const root = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  useDismiss(open, () => setOpen(false), root);
  const { checking, error, remote } = backend.sync();

  if (remote) {
    const n = remote.conflicts.length;
    const takeTheirs = async () => {
      setOpen(false);
      const open = activeFile?.path.join("/");
      await backend.takeTheirs();
      if (open && remote.conflicts.includes(open)) setActiveFile(activeFile!.path);
    };
    return (
      <div ref={root} className="relative">
        <button
          type="button"
          className="workspace-button border border-(--danger)/50 text-(--danger)"
          aria-expanded={open}
          aria-haspopup="dialog"
          onClick={() => setOpen((o) => !o)}
        >
          <AlertTriangle size={14} aria-hidden="true" /> {n === 1 ? "1 conflict" : `${n} conflicts`}
        </button>
        {open && (
          <div role="dialog" aria-label="Remote changes" className={`${POPOVER} p-3`}>
            <p className="text-xs leading-5 text-(--text)">
              Someone pushed to <span className="font-mono">{backend.branch}</span> and changed {n === 1 ? "a file" : "files"} you
              changed too:
            </p>
            <ul className="my-2 space-y-1">
              {remote.conflicts.map((p) => (
                <li key={p} className="flex items-center gap-2 text-xs">
                  <span className="min-w-0 flex-1 truncate font-mono text-(--muted)" title={p}>
                    {p}
                  </span>
                  <button
                    type="button"
                    className="workspace-button"
                    aria-label={`Compare ${p}`}
                    onClick={() => {
                      setOpen(false);
                      onCompare(p);
                    }}
                  >
                    <GitCompareArrows size={14} aria-hidden="true" /> Compare
                  </button>
                </li>
              ))}
            </ul>
            <div className="flex flex-wrap justify-end gap-2">
              <button type="button" className="workspace-button border border-(--border-strong)" onClick={() => void takeTheirs()}>
                Take theirs
              </button>
              <button
                type="button"
                className="workspace-button border border-(--brand) text-(--brand)"
                onClick={() => {
                  setOpen(false);
                  void backend.keepMine();
                }}
              >
                Keep mine
              </button>
            </div>
          </div>
        )}
      </div>
    );
  }

  if (error?.code === "signed_out") {
    return (
      <a href={signInUrl()} className="workspace-button border border-(--warning)/50 text-(--warning)">
        Signed out — sign in
      </a>
    );
  }
  return (
    <button
      type="button"
      className="workspace-button"
      title={error ? describeError(error) : "Check GitHub for new commits"}
      onClick={() => void backend.checkRemote({ force: true })}
      disabled={checking}
    >
      <RefreshCw size={14} className={checking ? "animate-spin motion-reduce:animate-none" : ""} aria-hidden="true" />
      <span aria-live="polite">{checking ? "Checking…" : error ? "Sync failed — retry" : "Up to date"}</span>
    </button>
  );
}

export default function RepoHeader({
  backend,
  panelOpen,
  onTogglePanel,
  onCompare,
}: {
  backend: GitHubBackend;
  panelOpen: boolean;
  onTogglePanel: () => void;
  onCompare: (path: string) => void;
}) {
  useBackend(backend);
  const router = useRouter();
  const { closeWorkspace } = useWorkspace();
  const [confirmClose, setConfirmClose] = useState(false);
  const count = backend.changes().length;

  const close = () => {
    closeWorkspace();
    router.push("/hittable");
  };

  return (
    <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
      <a
        href={`https://github.com/${backend.owner}/${backend.repo}`}
        target="_blank"
        rel="noopener noreferrer"
        className="flex min-w-0 items-center gap-1.5 text-xs text-(--text) hover:underline"
        title={`Open ${backend.label} on GitHub`}
      >
        <Github size={14} className="shrink-0 text-(--brand)" aria-hidden="true" />
        <span className="truncate">{backend.label}</span>
      </a>
      <BranchSwitcher backend={backend} />
      <SyncPill backend={backend} onCompare={onCompare} />
      <span className="ml-auto" />
      <button
        type="button"
        className="workspace-button"
        aria-pressed={panelOpen}
        aria-controls="source-control"
        onClick={onTogglePanel}
        aria-label={`Source control, ${count === 1 ? "1 change" : `${count} changes`}`}
        title="Source control"
      >
        <GitBranch size={14} aria-hidden="true" />
        <span className="hidden sm:inline" aria-hidden="true">Changes</span>
        <span
          className={`rounded-full px-1.5 text-[11px] font-semibold ${count ? "bg-(--brand) text-(--ink-950)" : "bg-(--surface-2) text-(--muted)"}`}
          aria-hidden="true"
        >
          {count}
        </span>
      </button>
      <button type="button" className="workspace-button" onClick={() => (count ? setConfirmClose(true) : close())}>
        <X size={14} aria-hidden="true" /> <span className="hidden sm:inline">Close repository</span>
        <span className="sr-only sm:hidden">Close repository</span>
      </button>
      {confirmClose && (
        <ModalShell title="Close this repository?" onClose={() => setConfirmClose(false)}>
          <p className="text-xs leading-5 text-(--muted)">
            Your {count === 1 ? "uncommitted change stays" : `${count} uncommitted changes stay`} saved on this device. Open{" "}
            {backend.label} on {backend.branch} again to review and commit {count === 1 ? "it" : "them"}.
          </p>
          <div className="flex justify-end gap-2">
            <button type="button" className="workspace-button border border-(--border-strong)" onClick={() => setConfirmClose(false)}>
              Cancel
            </button>
            <button type="button" className="workspace-button border border-(--brand) text-(--brand)" onClick={close}>
              Close repository
            </button>
          </div>
        </ModalShell>
      )}
    </div>
  );
}
