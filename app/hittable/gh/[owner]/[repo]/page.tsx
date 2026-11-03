"use client";
import { Suspense, useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";
import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { AlertCircle, GitBranch, Sparkles } from "lucide-react";
import { useWorkspace } from "@/context/workspaceContext";
import { findNode } from "@/services/workspace/backend";
import WorkspaceShell from "@/components/workspace/WorkspaceShell";
import RepoHeader, { repoUrl } from "@/components/github/RepoHeader";
import ChangesView, { readSides } from "@/components/github/ChangesView";
import DiffDialog, { type DiffSides } from "@/components/github/DiffView";
import { createGitHubBackend, type GitHubBackend } from "@/services/github/githubBackend";
import { api, describeError, GitHubError, signInUrl } from "@/utils/github/api";
import { rememberRecent } from "@/utils/github/local";

type State = { kind: "loading" } | { kind: "error"; error: unknown } | { kind: "ready"; backend: GitHubBackend };

const PRIMARY =
  "inline-flex min-h-10 items-center justify-center gap-2 rounded-lg bg-(--brand) px-4 py-2 text-sm font-semibold text-(--ink-950) hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--brand)";
const SECONDARY =
  "inline-flex min-h-10 items-center justify-center gap-2 rounded-lg border border-white/15 px-4 py-2 text-sm text-white/80 hover:bg-white/5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--brand)";

function Message({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="flex min-h-[calc(100dvh-44px)] w-full items-center justify-center bg-(--surface-0) px-4">
      <section aria-labelledby="gh-message" className="flex max-w-md flex-col gap-4 rounded-xl border border-white/10 bg-(--surface-1) p-6">
        <h1 id="gh-message" className="flex items-center gap-2 text-lg font-semibold text-white/90">
          <AlertCircle size={18} className="shrink-0 text-(--warning)" aria-hidden="true" /> {title}
        </h1>
        {children}
      </section>
    </div>
  );
}

function ErrorView({ error, owner, repo, retry }: { error: unknown; owner: string; repo: string; retry: () => void }) {
  const code = error instanceof GitHubError ? error.code : "";
  const name = `${owner}/${repo}`;
  const back = (
    <Link href="/hittable" className={SECONDARY}>
      Choose another repository
    </Link>
  );
  if (code === "signed_out") {
    return (
      <Message title={`Sign in to open ${name}`}>
        <p className="text-sm leading-6 text-white/70">Hittable reads and commits this repository with your GitHub account.</p>
        <a href={signInUrl()} className={PRIMARY}>
          <GitBranch size={16} aria-hidden="true" /> Sign in with GitHub
        </a>
      </Message>
    );
  }
  if (code === "not_found" || code === "forbidden") {
    return (
      <Message title={`Can't open ${name}`}>
        <p className="text-sm leading-6 text-white/70">
          It doesn&apos;t exist, or your account can&apos;t see it. An organisation&apos;s repositories may need an owner to approve
          Hittable first.
        </p>
        <a href="https://github.com/settings/applications" target="_blank" rel="noopener noreferrer" className="text-sm text-(--brand) hover:underline">
          Grant access on GitHub →
        </a>
        {back}
      </Message>
    );
  }
  if (code === "empty_repo") {
    return (
      <Message title={`${name} is empty`}>
        <p className="text-sm leading-6 text-white/70">
          There is nothing to open yet. Add a first commit on GitHub (a README is enough), then open it here again.
        </p>
        <a href={`https://github.com/${name}`} target="_blank" rel="noopener noreferrer" className={PRIMARY}>
          Open {name} on GitHub
        </a>
        {back}
      </Message>
    );
  }
  if (code === "branch_not_found") {
    return (
      <Message title="Branch not found">
        <p className="text-sm leading-6 text-white/70">{describeError(error)}</p>
        <Link href={repoUrl(owner, repo)} className={PRIMARY}>
          Open the default branch
        </Link>
      </Message>
    );
  }
  return (
    <Message title={`Couldn't open ${name}`}>
      <p className="text-sm leading-6 text-white/70">{describeError(error)}</p>
      <button type="button" className={PRIMARY} onClick={retry}>
        Try again
      </button>
      {back}
    </Message>
  );
}

/** FR-G7: a repository without hittable/ can get one, staged as changes to review. */
function InitializeHere({ backend }: { backend: GitHubBackend }) {
  const { refreshTree, setActiveFile } = useWorkspace();
  const [error, setError] = useState<string | null>(null);
  return (
    <div className="m-auto flex max-w-sm flex-col items-center gap-3 p-8 text-center">
      <Sparkles size={24} className="text-(--brand)" aria-hidden="true" />
      <h2 className="text-sm font-semibold text-white/90">No Hittable requests here yet</h2>
      <p className="text-sm leading-6 text-white/60">
        Add a <code className="text-white/80">hittable/</code> folder with an environment file and a sample request. They&apos;re
        listed as changes for you to review and commit.
      </p>
      <button
        type="button"
        className={PRIMARY}
        onClick={async () => {
          try {
            await backend.initialize();
            await refreshTree();
            setActiveFile(["hittable", "requests", "example.hit"]);
          } catch (err) {
            setError(describeError(err));
          }
        }}
      >
        Initialize Hittable here
      </button>
      {error && <p role="alert" className="text-xs text-(--danger)">{error}</p>}
    </div>
  );
}

function useDiff(backend: GitHubBackend | null) {
  const [diff, setDiff] = useState<{ path: string; theirs: boolean; sides: DiffSides | null; error?: string } | null>(null);
  const open = useCallback(
    async (path: string, theirs: boolean) => {
      if (!backend) return;
      setDiff({ path, theirs, sides: null });
      try {
        const sides = await readSides(backend, path, theirs);
        setDiff((d) => (d?.path === path ? { ...d, sides } : d));
      } catch (err) {
        setDiff((d) => (d?.path === path ? { ...d, error: describeError(err) } : d));
      }
    },
    [backend],
  );
  return { diff, open, close: () => setDiff(null) };
}

function Workspace({ backend }: { backend: GitHubBackend }) {
  const { tree, readOnly, activeFile, setActiveFile } = useWorkspace();
  const [panel, setPanel] = useState(false);
  const { diff, open, close } = useDiff(backend);
  const baseSha = useSyncExternalStore(backend.subscribe, backend.baseSha, backend.baseSha);
  const shownBase = useRef(baseSha);

  // FR-G6: check the branch head on focus and every minute (the backend throttles; 304s are free).
  useEffect(() => {
    const tick = () => document.visibilityState === "visible" && void backend.checkRemote();
    const id = setInterval(tick, 60_000);
    window.addEventListener("focus", tick);
    return () => {
      clearInterval(id);
      window.removeEventListener("focus", tick);
    };
  }, [backend]);

  // The base moved (a pull of their commits, or your commit): reload the open
  // file unless it is one of your changes, so the editor never shows stale text.
  useEffect(() => {
    if (shownBase.current === baseSha) return;
    shownBase.current = baseSha;
    if (activeFile && !backend.status()[activeFile.path.join("/")]) setActiveFile(activeFile.path);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only when the base moves
  }, [baseSha]);

  const hasHittable = !!findNode(tree, ["hittable"]);
  return (
    <>
      <WorkspaceShell
        header={<RepoHeader backend={backend} panelOpen={panel} onTogglePanel={() => setPanel((p) => !p)} onCompare={(p) => void open(p, true)} />}
        changes={panel && <ChangesView backend={backend} onClose={() => setPanel(false)} />}
        emptyState={
          tree.length === 0 || hasHittable ? undefined : readOnly ? (
            <p className="m-auto max-w-sm p-8 text-center text-sm leading-6 text-white/60">
              This repository has no <code className="text-white/80">hittable/</code> folder, and it&apos;s read-only here.
            </p>
          ) : (
            <InitializeHere backend={backend} />
          )
        }
      />
      {diff && <DiffDialog path={diff.path} sides={diff.sides} error={diff.error} onClose={close} />}
    </>
  );
}

function GitHubWorkspace() {
  const { owner, repo } = useParams<{ owner: string; repo: string }>();
  const ref = useSearchParams().get("ref");
  const { openBackend } = useWorkspace();
  const [state, setState] = useState<State>({ kind: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let live = true;
    setState({ kind: "loading" }); // eslint-disable-line react-hooks/set-state-in-effect -- a new repo or branch starts loading
    (async () => {
      const lower = `${owner}/${repo}`.toLowerCase();
      const meta = (await api.repos(attempt > 0)).find((r) => r.fullName.toLowerCase() === lower);
      if (!meta) throw new GitHubError("not_found", 404);
      const backend = await createGitHubBackend({ owner: meta.owner, repo: meta.name, branch: ref || meta.defaultBranch, meta });
      if (!live) return;
      await openBackend(backend);
      rememberRecent(meta.fullName);
      if (live) setState({ kind: "ready", backend });
    })().catch((error) => live && setState({ kind: "error", error }));
    return () => {
      live = false;
    };
  }, [owner, repo, ref, attempt, openBackend]);

  if (state.kind === "error") return <ErrorView error={state.error} owner={owner} repo={repo} retry={() => setAttempt((n) => n + 1)} />;
  if (state.kind === "loading") {
    return (
      <div role="status" className="flex min-h-[calc(100dvh-44px)] w-full items-center justify-center bg-(--surface-0) text-sm text-white/70">
        Opening {owner}/{repo}
        {ref ? ` on ${ref}` : ""}…
      </div>
    );
  }
  return <Workspace key={`${state.backend.label}@${state.backend.branch}`} backend={state.backend} />;
}

export default function GitHubWorkspacePage() {
  return (
    <Suspense fallback={null}>
      <GitHubWorkspace />
    </Suspense>
  );
}
