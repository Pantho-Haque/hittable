"use client";
import { useState, useSyncExternalStore } from "react";
import { GitCommitHorizontal, GitPullRequest, Lock, Undo2, X } from "lucide-react";
import { ModalActions, ModalShell } from "@/components/ui/SharedModal";
import { useNotification } from "@/hooks/useNotify";
import { useWorkspace } from "@/context/workspaceContext";
import { describeError, isGitHubError, signInUrl } from "@/utils/github/api";
import {
  LOCAL_ENV,
  branchSlug,
  nextBranchName,
  prefillMessage,
  type Change,
  type GitHubBackend,
} from "@/services/github/githubBackend";

/** Re-render on every backend change. */
export function useBackend(b: GitHubBackend) {
  useSyncExternalStore(b.subscribe, b.version, b.version);
  return b;
}

const STATUS: Record<Change["status"] | "C", { label: string; className: string }> = {
  M: { label: "Modified", className: "text-(--git-modified)" },
  A: { label: "Added", className: "text-(--git-added)" },
  D: { label: "Deleted", className: "text-(--git-deleted)" },
  C: { label: "Conflict", className: "text-(--danger)" },
};

const BUTTON =
  "inline-flex min-h-9 w-full items-center justify-center gap-2 rounded-md px-3 text-xs font-semibold disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--brand)";
const FIELD =
  "w-full rounded-md border border-(--border-strong) bg-(--surface-0) px-2 py-1.5 text-xs text-(--text) placeholder:text-(--muted) focus-visible:outline-2 focus-visible:outline-(--brand)";

const splitMessage = (message: string) => {
  const nl = message.indexOf("\n");
  return { subject: (nl < 0 ? message : message.slice(0, nl)).trim(), body: nl < 0 ? "" : message.slice(nl + 1).trim() };
};

type Confirm = { paths: string[]; title: string; subtitle: string };

function PullRequestDialog({
  backend,
  message,
  reason,
  onDone,
  onClose,
}: {
  backend: GitHubBackend;
  message: string;
  reason?: string;
  onDone: () => void;
  onClose: () => void;
}) {
  const { success } = useNotification();
  const { subject, body: initialBody } = splitMessage(message);
  const [branch, setBranch] = useState(() => branchSlug(subject));
  const [title, setTitle] = useState(subject);
  const [body, setBody] = useState(initialBody);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    if (!branch.trim() || !title.trim()) return setError("A branch name and a title are needed.");
    setBusy(true);
    setError(null);
    try {
      const { pr } = await backend.commitToNewBranch({
        branch: branch.trim(),
        message: body.trim() ? `${title.trim()}\n\n${body.trim()}` : title.trim(),
        title: title.trim(),
        body,
      });
      success({
        title: `Opened pull request #${pr.number}`,
        desc: `${branch.trim()} → ${backend.branch}`,
        timeout: 15000,
        action: { label: "View pull request", onClick: () => window.open(pr.url, "_blank", "noopener,noreferrer") },
      });
      onDone();
    } catch (err) {
      if (isGitHubError(err, "branch_exists")) {
        const suggestion = nextBranchName(branch.trim());
        setError(`A branch named “${branch.trim()}” already exists. Try “${suggestion}”, filled in below.`);
        setBranch(suggestion);
      } else setError(describeError(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <ModalShell title="Commit to a new branch & open a pull request" onClose={onClose} size="wide">
      {reason && <p className="text-xs leading-5 text-(--warning)">{reason}</p>}
      <form
        className="flex flex-col gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <label className="flex flex-col gap-1 text-xs text-(--muted)">
          New branch (from {backend.baseSha().slice(0, 7)} on {backend.branch})
          <input className={FIELD} value={branch} onChange={(e) => setBranch(e.target.value)} autoFocus spellCheck={false} />
        </label>
        <label className="flex flex-col gap-1 text-xs text-(--muted)">
          Pull request title
          <input className={FIELD} value={title} onChange={(e) => setTitle(e.target.value)} />
        </label>
        <label className="flex flex-col gap-1 text-xs text-(--muted)">
          Description
          <textarea className={`${FIELD} min-h-24 font-mono`} value={body} onChange={(e) => setBody(e.target.value)} />
        </label>
        {error && (
          <p role="alert" className="text-xs leading-5 text-(--danger)">
            {error}
          </p>
        )}
        <div className="flex justify-end gap-2">
          <button type="button" className="workspace-button border border-(--border-strong)" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button type="submit" className={`${BUTTON} w-auto bg-(--brand) text-(--ink-950)`} disabled={busy} aria-busy={busy}>
            <GitPullRequest size={14} aria-hidden="true" /> {busy ? "Opening…" : "Commit & open pull request"}
          </button>
        </div>
      </form>
    </ModalShell>
  );
}

/** FR-G5: the changes, discard, and the commit box. */
export default function SourceControlPanel({
  backend,
  onOpenDiff,
  onClose,
}: {
  backend: GitHubBackend;
  onOpenDiff: (path: string) => void;
  onClose: () => void;
}) {
  useBackend(backend);
  const { activeFile, setActiveFile } = useWorkspace();
  const { success } = useNotification();
  const changes = backend.changes();
  const { remote } = backend.sync();
  const [draft, setDraft] = useState<string | null>(null); // null: follow the prefill
  const message = draft ?? prefillMessage(changes);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<React.ReactNode>(null);
  const [confirm, setConfirm] = useState<Confirm | null>(null);
  const [pr, setPr] = useState<{ reason?: string } | null>(null);
  const branch = backend.branch;
  const blocked = changes.length === 0 || !!remote || busy || !message.trim();

  // The editor holds the file's text: reload (or close) it when its change is dropped.
  const refreshOpen = (paths: string[]) => {
    const open = activeFile?.path.join("/");
    if (!open || !paths.includes(open)) return;
    const added = changes.find((c) => c.path === open)?.status === "A";
    setActiveFile(added ? null : activeFile!.path);
  };

  const discard = async () => {
    if (!confirm) return;
    const { paths } = confirm;
    setConfirm(null);
    if (paths.length === changes.length) await backend.discardAll();
    else await backend.discard(paths);
    refreshOpen(paths);
  };

  const commit = async () => {
    setBusy(true);
    setError(null);
    const attempt = async (retried: boolean): Promise<void> => {
      try {
        const res = await backend.commit(message);
        setDraft(null);
        success({
          title: `Committed ${res.sha.slice(0, 7)} to ${branch}`,
          desc: message.split("\n")[0],
          timeout: 15000,
          action: { label: "View on GitHub", onClick: () => window.open(res.url, "_blank", "noopener,noreferrer") },
        });
      } catch (err) {
        if (isGitHubError(err, "head_moved")) {
          // No overlap: the base moved under the changes; commit them on top once.
          if (!backend.sync().remote && !retried) return attempt(true);
          setError(
            backend.sync().remote
              ? `Someone else changed the same files on ${branch}. Nothing was committed and nothing is lost: they're marked C. Keep yours or take theirs from the sync button, then commit.`
              : `${branch} moved on GitHub. Your changes are on top of the latest commit now; review them and commit again.`,
          );
        } else if (isGitHubError(err, "protected")) {
          setPr({ reason: `${branch} is protected: GitHub only accepts changes to it through a pull request. Your changes are kept.` });
        } else if (isGitHubError(err, "signed_out")) {
          setError(
            <>
              Your GitHub session ended; your changes are kept on this device.{" "}
              <a className="text-(--brand) underline" href={signInUrl()}>
                Sign in again
              </a>
            </>,
          );
        } else setError(describeError(err));
      }
    };
    await attempt(false);
    setBusy(false);
  };

  return (
    <aside
      id="source-control"
      aria-label="Source control"
      className="absolute inset-y-0 right-0 z-30 flex w-full max-w-80 flex-col border-l border-(--border) bg-(--surface-1) shadow-xl shadow-black/60 md:static md:z-auto md:h-full md:w-full md:max-w-none md:border-l-0 md:shadow-none"
    >
      <div className="flex items-center gap-2 border-b border-(--border) px-3 py-2">
        <h2 className="text-xs font-semibold text-(--text)">Source control</h2>
        <span className="ml-auto" />
        {changes.length > 0 && !backend.readOnly && (
          <button
            type="button"
            className="workspace-button"
            onClick={() =>
              setConfirm({
                paths: changes.map((c) => c.path),
                title: `Discard all ${changes.length} changes?`,
                subtitle: "Your edits, new files and deletions go back to the last commit. This can't be undone.",
              })
            }
          >
            <Undo2 size={14} aria-hidden="true" /> Discard all
          </button>
        )}
        <button type="button" className="workspace-button" aria-label="Close source control" onClick={onClose}>
          <X size={14} aria-hidden="true" />
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        <h3 className="flex items-center gap-2 px-3 pb-1 pt-3 text-[11px] font-semibold uppercase tracking-wide text-(--muted)">
          Changes
          <span className="rounded-full bg-(--surface-2) px-1.5 text-[11px] text-(--text)">
            {changes.length}
          </span>
        </h3>
        {changes.length === 0 ? (
          <p className="px-3 py-2 text-xs leading-5 text-(--muted)">
            No changes. Edits are saved on this device as you type and listed here until you commit them.
          </p>
        ) : (
          <ul aria-label="Changed files">
            {changes.map((c) => {
              const s = STATUS[c.conflict ? "C" : c.status];
              return (
                <li key={c.path} className="group flex items-center gap-1 pr-1 hover:bg-(--surface-2)">
                  <button
                    type="button"
                    onClick={() => onOpenDiff(c.path)}
                    // An explicit name: the browser separates the flex items
                    // with a space, which read as "get.hit , Modified".
                    aria-label={`${c.path}, ${s.label}${c.binary ? ", binary" : ""}`}
                    title={`${c.path} (${s.label}) — open the diff`}
                    className="flex min-h-8 min-w-0 flex-1 items-center gap-2 px-3 text-left text-xs text-(--text)"
                  >
                    <span className={`w-3 shrink-0 font-mono font-bold ${s.className}`} aria-hidden="true">
                      {c.conflict ? "C" : c.status}
                    </span>
                    <span className={`truncate ${c.status === "D" ? "line-through" : ""}`}>{c.path}</span>
                    {c.binary && <span className="shrink-0 rounded bg-(--surface-2) px-1 text-[11px] text-(--muted)">binary</span>}
                  </button>
                  {!backend.readOnly && (
                    <button
                      type="button"
                      className="workspace-button"
                      aria-label={`Discard changes to ${c.path}`}
                      title="Discard"
                      onClick={() =>
                        setConfirm({
                          paths: [c.path],
                          title: `Discard changes to ${c.path.split("/").at(-1)}?`,
                          subtitle:
                            c.status === "A"
                              ? "The new file is removed. This can't be undone."
                              : c.status === "D"
                                ? "The file is restored from the last commit."
                                : "The file goes back to the last commit. This can't be undone.",
                        })
                      }
                    >
                      <Undo2 size={14} aria-hidden="true" />
                    </button>
                  )}
                </li>
              );
            })}
          </ul>
        )}
        {backend.hasLocalEnv() && (
          <p className="mx-3 mt-3 flex items-start gap-2 rounded-md border border-(--border) p-2 text-xs leading-5 text-(--muted)">
            <Lock size={14} className="mt-0.5 shrink-0" aria-hidden="true" />
            <span>
              <span className="font-mono text-(--text)">{LOCAL_ENV}</span> is local only: kept on this device, never committed.
            </span>
          </p>
        )}
      </div>

      {backend.readOnly ? (
        <p className="border-t border-(--border) p-3 text-xs leading-5 text-(--muted)">
          {backend.meta.archived
            ? "This repository is archived on GitHub, so it's read-only. You can still run its requests."
            : "You can't push to this repository, so it's read-only. You can still run its requests."}
        </p>
      ) : (
        <form
          className="flex flex-col gap-2 border-t border-(--border) p-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (!blocked) void commit();
          }}
        >
          <label htmlFor="commit-message" className="text-[11px] font-semibold uppercase tracking-wide text-(--muted)">
            Commit message
          </label>
          <textarea
            id="commit-message"
            className={`${FIELD} min-h-20 font-mono`}
            value={message}
            placeholder="Describe your changes"
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && !blocked) {
                e.preventDefault();
                void commit();
              }
            }}
          />
          {remote && (
            <p className="text-xs leading-5 text-(--warning)">Resolve the conflicts with the remote changes before committing.</p>
          )}
          {error && (
            <p role="alert" className="text-xs leading-5 text-(--danger)">
              {error}
            </p>
          )}
          <button type="submit" className={`${BUTTON} bg-(--brand) text-(--ink-950)`} disabled={blocked} aria-busy={busy}>
            <GitCommitHorizontal size={14} aria-hidden="true" />
            {busy ? "Committing…" : (
              <span className="truncate">
                Commit to <span className="font-mono">{branch}</span>
              </span>
            )}
          </button>
          <button
            type="button"
            className={`${BUTTON} border border-(--border-strong) text-(--text) hover:bg-(--surface-2)`}
            disabled={blocked}
            onClick={() => setPr({})}
          >
            <GitPullRequest size={14} aria-hidden="true" /> Commit to new branch &amp; open pull request
          </button>
        </form>
      )}

      {confirm && (
        <ModalShell title={confirm.title} onClose={() => setConfirm(null)}>
          <p className="text-xs leading-5 text-(--muted)">{confirm.subtitle}</p>
          <ModalActions onCancel={() => setConfirm(null)} onConfirm={() => void discard()} confirmLabel="Discard" confirmDanger />
        </ModalShell>
      )}
      {pr && (
        <PullRequestDialog
          backend={backend}
          message={message}
          reason={pr.reason}
          onClose={() => setPr(null)}
          onDone={() => {
            const paths = changes.map((c) => c.path);
            setPr(null);
            setDraft(null);
            setError(null);
            refreshOpen(paths);
          }}
        />
      )}
    </aside>
  );
}
