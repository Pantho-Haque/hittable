"use client";
import { useEffect, useId, useMemo, useState, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import { AlertCircle, Check, ChevronRight, ExternalLink, Github, Lock, RefreshCw, RotateCcw, Search } from "lucide-react";
import { api, describeError, isGitHubError, type RepoMeta } from "@/utils/github/api";
import { readRecent, setSignedInHint, signedInHint } from "@/utils/github/local";
import { fuzzyScore } from "@/utils/fuzzy";

const noSubscribe = () => () => {};
const CARD = "flex min-w-0 flex-col rounded-xl border border-(--border) bg-(--surface-1)";

const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 365 * 86400],
  ["month", 30 * 86400],
  ["week", 7 * 86400],
  ["day", 86400],
  ["hour", 3600],
  ["minute", 60],
];

/** "3 days ago", "yesterday", "just now". */
export function relativeTime(iso: string, now = Date.now()): string {
  const seconds = Math.round((new Date(iso).getTime() - now) / 1000);
  const fmt = new Intl.RelativeTimeFormat("en", { numeric: "auto" });
  for (const [unit, size] of UNITS) if (Math.abs(seconds) >= size) return fmt.format(Math.round(seconds / size), unit);
  return "just now";
}

const PITCH = [
  "Edit the requests in any repository you can see, from any browser",
  "Review every change as a diff, revert one change at a time",
  "Commit to the branch, or to a new branch with a pull request",
];

function SignInPitch() {
  return (
    <section aria-labelledby="github-card-title" className={`${CARD} gap-6 p-6 sm:p-8`}>
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-(--border) bg-(--surface-2)">
          <Github size={20} className="text-(--text)" aria-hidden="true" />
        </div>
        <div>
          <h2 id="github-card-title" className="text-base font-semibold text-(--text)">A GitHub repository</h2>
          <p className="text-xs text-(--muted)">Your requests, versioned with your code</p>
        </div>
      </div>
      <ul className="space-y-2.5">
        {PITCH.map((line) => (
          <li key={line} className="flex items-start gap-2.5 text-sm leading-6 text-white/80">
            <Check size={16} className="mt-1 shrink-0 text-(--brand)" aria-hidden="true" /> {line}
          </li>
        ))}
      </ul>
      <div className="mt-auto space-y-3">
        <a
          href="/api/github/login?next=/hittable"
          className="inline-flex min-h-10 w-full items-center justify-center gap-2 rounded-lg bg-(--brand) px-4 py-2 text-sm font-semibold text-(--ink-950) hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--brand) sm:w-auto"
        >
          <Github size={16} aria-hidden="true" /> Sign in with GitHub
        </a>
        <p className="flex items-start gap-2 text-xs leading-5 text-(--muted)">
          <Lock size={12} className="mt-1 shrink-0" aria-hidden="true" />
          Hittable asks for access to your repositories so it can read and commit files. Your token stays on the server in
          an encrypted cookie and never reaches this page.
        </p>
      </div>
    </section>
  );
}

/** The owner's GitHub avatar over a letter tile (shown while it loads, or if it can't). */
function Avatar({ owner }: { owner: string }) {
  const [failed, setFailed] = useState(false);
  return (
    <span
      className="relative flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-md border border-(--border) bg-(--surface-2) text-xs font-semibold uppercase text-(--muted)"
      aria-hidden="true"
    >
      {owner[0]}
      {!failed && (
        // eslint-disable-next-line @next/next/no-img-element -- a 32 px avatar from GitHub, no optimisation needed
        <img
          src={`https://github.com/${encodeURIComponent(owner)}.png?size=64`}
          alt=""
          width={32}
          height={32}
          loading="lazy"
          onError={() => setFailed(true)}
          className="absolute inset-0 h-full w-full object-cover"
        />
      )}
    </span>
  );
}

function Badge({ children, tone = "muted" }: { children: React.ReactNode; tone?: "muted" | "warning" }) {
  return (
    <span
      className={`shrink-0 rounded-full border px-1.5 py-px text-[11px] leading-4 ${
        tone === "warning" ? "border-(--warning)/40 text-(--warning)" : "border-(--border-strong) text-(--muted)"
      }`}
    >
      {children}
    </span>
  );
}

/** FR-G2: every repository the account can see, with search, owner chips and Recent. */
export function RepoPicker({ onSignedOut }: { onSignedOut: () => void }) {
  const router = useRouter();
  const listId = useId();
  const [repos, setRepos] = useState<RepoMeta[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [owner, setOwner] = useState<string | null>(null);
  const [active, setActive] = useState(0);
  const [recent, setRecent] = useState<string[]>([]);

  const load = (fresh = false) => {
    setError(null);
    setRepos(null);
    api.repos(fresh).then(setRepos, (err) => {
      if (isGitHubError(err, "signed_out")) onSignedOut();
      else setError(describeError(err));
    });
  };
  useEffect(() => {
    load();
    setRecent(readRecent());
    // eslint-disable-next-line react-hooks/exhaustive-deps -- once
  }, []);

  const owners = useMemo(() => {
    const count = new Map<string, number>();
    for (const r of repos ?? []) count.set(r.owner, (count.get(r.owner) ?? 0) + 1);
    return [...count.keys()].sort((a, b) => count.get(b)! - count.get(a)! || a.localeCompare(b));
  }, [repos]);

  // Recent first (only with no search or filter), then every match.
  const sections = useMemo(() => {
    const pool = (repos ?? []).filter((r) => !owner || r.owner === owner);
    const q = query.trim();
    if (q) {
      const matches = pool
        .map((r) => ({ r, s: fuzzyScore(q, `${r.fullName} ${r.description ?? ""}`) }))
        .filter((x): x is { r: RepoMeta; s: number } => x.s !== null)
        .sort((a, b) => b.s - a.s)
        .map((x) => x.r);
      return [{ title: "Results", repos: matches }];
    }
    const byName = new Map(pool.map((r) => [r.fullName.toLowerCase(), r]));
    const recentRepos = owner ? [] : recent.map((n) => byName.get(n.toLowerCase())).filter((r): r is RepoMeta => !!r);
    return [
      ...(recentRepos.length ? [{ title: "Recent", repos: recentRepos }] : []),
      { title: owner ? `${owner}'s repositories` : "All repositories", repos: pool },
    ];
  }, [repos, owner, query, recent]);

  const options = sections.flatMap((s) => s.repos.map((r) => ({ r, id: `${listId}-${s.title}-${r.fullName}` })));
  const current = Math.min(active, Math.max(options.length - 1, 0));
  const open = (r: RepoMeta) => router.push(`/hittable/gh/${encodeURIComponent(r.owner)}/${encodeURIComponent(r.name)}`);

  const activeId = options[current]?.id;
  useEffect(() => {
    if (activeId) document.getElementById(activeId)?.scrollIntoView?.({ block: "nearest" });
  }, [activeId]);

  const onKeyDown = (e: React.KeyboardEvent) => {
    const go = (i: number) => {
      e.preventDefault();
      setActive(Math.max(0, Math.min(options.length - 1, i)));
    };
    if (e.key === "ArrowDown") go(current + 1);
    else if (e.key === "ArrowUp") go(current - 1);
    else if (e.key === "Home" && e.ctrlKey) go(0);
    else if (e.key === "End" && e.ctrlKey) go(options.length - 1);
    else if (e.key === "Enter" && options[current]) {
      e.preventDefault();
      open(options[current].r);
    }
  };

  let index = -1;
  return (
    <section aria-labelledby="github-card-title" className={CARD}>
      <div className="flex items-center gap-3 border-b border-(--border) px-5 py-4">
        <Github size={18} className="shrink-0 text-(--text)" aria-hidden="true" />
        <h2 id="github-card-title" className="text-sm font-semibold text-(--text)">Your repositories</h2>
        {repos && (
          <span className="rounded-full bg-(--surface-2) px-2 py-px text-[11px] font-medium tabular-nums text-(--muted)">
            {repos.length}
          </span>
        )}
        <button
          type="button"
          className="workspace-button ml-auto"
          onClick={() => load(true)}
          disabled={!repos && !error}
          aria-label="Refresh repositories"
          title="Refresh the list from GitHub"
        >
          <RefreshCw size={14} className={!repos && !error ? "animate-spin motion-reduce:animate-none" : ""} aria-hidden="true" />
        </button>
      </div>

      <div className="flex flex-col gap-2 px-5 pt-4 sm:flex-row">
        <div className="relative min-w-0 flex-1">
          <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-(--muted)" aria-hidden="true" />
          <input
            type="search"
            role="combobox"
            aria-label="Search repositories"
            aria-expanded="true"
            aria-controls={listId}
            aria-autocomplete="list"
            aria-activedescendant={activeId}
            placeholder="Search repositories"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            onKeyDown={onKeyDown}
            className="min-h-9 w-full rounded-lg border border-(--border-strong) bg-(--surface-0) pl-8 pr-3 text-sm text-(--text) placeholder:text-(--muted) focus-visible:outline-2 focus-visible:outline-(--brand)"
          />
        </div>
        {owners.length > 1 && (
          <select
            aria-label="Filter by owner"
            value={owner ?? ""}
            onChange={(e) => {
              setOwner(e.target.value || null);
              setActive(0);
            }}
            className="min-h-9 max-w-full rounded-lg border border-(--border-strong) bg-(--surface-0) px-3 text-sm text-(--text) focus-visible:outline-2 focus-visible:outline-(--brand) sm:w-48"
          >
            <option value="">All owners</option>
            {owners.map((o) => (
              <option key={o} value={o}>
                {o}
              </option>
            ))}
          </select>
        )}
      </div>

      <div className="px-5 py-4">
        {error ? (
          <div role="alert" className="flex flex-col items-start gap-3 rounded-lg border border-(--danger)/40 p-4">
            <p className="flex items-start gap-2 text-sm text-white/80">
              <AlertCircle size={16} className="mt-0.5 shrink-0 text-(--danger)" aria-hidden="true" /> {error}
            </p>
            <button type="button" className="workspace-button border border-(--border-strong)" onClick={() => load(true)}>
              <RotateCcw size={14} aria-hidden="true" /> Retry
            </button>
          </div>
        ) : !repos ? (
          <div aria-busy="true" className="divide-y divide-(--border) rounded-lg border border-(--border)">
            <span role="status" className="sr-only">Loading repositories…</span>
            {[0, 1, 2, 3, 4].map((i) => (
              <div key={i} className="flex items-center gap-3 px-3 py-3" aria-hidden="true">
                <div className="h-8 w-8 animate-pulse rounded-md bg-(--surface-2) motion-reduce:animate-none" />
                <div className="flex-1 space-y-1.5">
                  <div className="h-3 w-2/5 animate-pulse rounded bg-(--surface-2) motion-reduce:animate-none" />
                  <div className="h-2.5 w-3/5 animate-pulse rounded bg-(--surface-2) motion-reduce:animate-none" />
                </div>
              </div>
            ))}
          </div>
        ) : options.length === 0 ? (
          <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-(--border-strong) px-4 py-10 text-center">
            <Search size={18} className="text-(--muted)" aria-hidden="true" />
            <p className="text-sm text-(--muted)">
              {repos.length === 0 ? "No repositories yet. Create one on GitHub, then refresh." : `No repositories match “${query}”.`}
            </p>
          </div>
        ) : (
          <div
            id={listId}
            role="listbox"
            aria-label="Repositories"
            className="max-h-[26rem] overflow-y-auto overscroll-contain rounded-lg border border-(--border)"
          >
            {sections.map((s) =>
              s.repos.length === 0 ? null : (
                <div key={s.title} role="group" aria-label={s.title}>
                  <p
                    className="sticky top-0 z-10 border-b border-(--border) bg-(--surface-1)/95 px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wide text-(--muted) backdrop-blur"
                    aria-hidden="true"
                  >
                    {s.title}
                  </p>
                  {s.repos.map((r) => {
                    const i = ++index;
                    return (
                      <div
                        key={r.fullName}
                        id={options[i].id}
                        role="option"
                        aria-selected={i === current}
                        onClick={() => open(r)}
                        onMouseMove={() => i !== current && setActive(i)}
                        className="group flex cursor-pointer items-center gap-3 border-b border-(--border) px-3 py-2.5 last:border-b-0 aria-selected:bg-(--surface-2)"
                      >
                        <Avatar owner={r.owner} />
                        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                          <span className="flex min-w-0 items-center gap-1.5">
                            <span className="truncate text-sm">
                              <span className="text-(--muted)">{r.owner}/</span>
                              <span className="font-medium text-(--text)">{r.name}</span>
                            </span>
                            {r.private && <Badge><Lock size={10} className="mr-0.5 inline" aria-hidden="true" />Private</Badge>}
                            {r.fork && <Badge>Fork</Badge>}
                            {r.archived && <Badge tone="warning">Archived</Badge>}
                            {!r.canPush && !r.archived && <Badge tone="warning">Read-only</Badge>}
                          </span>
                          {r.description && <span className="truncate text-xs text-(--muted)">{r.description}</span>}
                        </span>
                        <span className="hidden shrink-0 text-xs tabular-nums text-(--muted) sm:inline">
                          {r.pushedAt ? `pushed ${relativeTime(r.pushedAt)}` : "empty"}
                        </span>
                        <ChevronRight
                          size={16}
                          className="shrink-0 text-(--muted) opacity-0 transition-opacity group-aria-selected:opacity-100 motion-reduce:transition-none"
                          aria-hidden="true"
                        />
                      </div>
                    );
                  })}
                </div>
              ),
            )}
          </div>
        )}
      </div>

      <div className="mt-auto flex flex-col gap-2 border-t border-(--border) px-5 py-3 text-xs text-(--muted) sm:flex-row sm:items-center sm:justify-between">
        <span className="hidden sm:inline">
          <kbd className="rounded border border-(--border-strong) px-1 font-mono">↑</kbd>{" "}
          <kbd className="rounded border border-(--border-strong) px-1 font-mono">↓</kbd> to move ·{" "}
          <kbd className="rounded border border-(--border-strong) px-1 font-mono">Enter</kbd> to open
        </span>
        <a
          href="https://github.com/settings/applications"
          target="_blank"
          rel="noopener noreferrer"
          title="Organisations can restrict apps like Hittable until an owner approves them."
          className="inline-flex items-center gap-1 text-(--brand) hover:underline"
        >
          Missing an organisation&apos;s repos? Grant access on GitHub <ExternalLink size={12} aria-hidden="true" />
        </a>
      </div>
    </section>
  );
}

/** The launcher's GitHub card: the sign-in pitch, or the repository picker once signed in. */
export default function GitHubCard() {
  const hinted = useSyncExternalStore(noSubscribe, signedInHint, () => false);
  const [signedIn, setSignedIn] = useState<boolean | null>(null);

  // Once: the picker finds out by listing repositories when this device was signed in.
  useEffect(() => {
    if (signedInHint()) return;
    let live = true;
    void api.session().then((u) => {
      if (!live) return;
      setSignedIn(!!u);
      if (u) setSignedInHint(true);
    });
    return () => {
      live = false;
    };
  }, []);

  if (!(signedIn ?? hinted)) return <SignInPitch />;
  return (
    <RepoPicker
      onSignedOut={() => {
        setSignedInHint(false);
        setSignedIn(false);
      }}
    />
  );
}
