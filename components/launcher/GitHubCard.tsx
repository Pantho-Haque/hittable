"use client";
import { useEffect, useId, useMemo, useState, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import { AlertCircle, ExternalLink, GitBranch, Lock, RotateCcw, Search } from "lucide-react";
import { api, describeError, isGitHubError, type RepoMeta } from "@/utils/github/api";
import { readRecent, setSignedInHint, signedInHint } from "@/utils/github/local";
import { fuzzyScore } from "@/utils/fuzzy";

const noSubscribe = () => () => {};
const CARD = "flex flex-col gap-4 rounded-xl border border-white/10 bg-(--surface-1) p-6";

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

function SignInPitch() {
  return (
    <section aria-labelledby="github-card-title" className={CARD}>
      <div className="flex h-11 w-11 items-center justify-center rounded-lg border border-white/10 bg-(--surface-2)">
        <GitBranch size={20} className="text-(--brand)" aria-hidden="true" />
      </div>
      <div className="space-y-2">
        <h2 id="github-card-title" className="text-lg font-semibold text-white/90">A GitHub repository</h2>
        <p className="text-sm leading-6 text-white/70">
          Edit the requests in your repositories, review the changes and commit them, from any browser.
        </p>
        <p className="text-xs leading-5 text-white/60">
          Hittable asks for access to your repositories so it can read and commit files. Your token stays on the
          server in an encrypted cookie and never reaches this page.
        </p>
      </div>
      <a
        href="/api/github/login?next=/hittable"
        className="mt-auto inline-flex min-h-10 items-center justify-center gap-2 rounded-lg bg-(--brand) px-4 py-2 text-sm font-semibold text-(--ink-950) hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--brand)"
      >
        <GitBranch size={16} aria-hidden="true" /> Sign in with GitHub
      </a>
    </section>
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
    <section aria-labelledby="github-card-title" className={`${CARD} min-w-0`}>
      <div className="flex items-center gap-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-white/10 bg-(--surface-2)">
          <GitBranch size={18} className="text-(--brand)" aria-hidden="true" />
        </div>
        <h2 id="github-card-title" className="text-lg font-semibold text-white/90">Your repositories</h2>
      </div>

      <div className="relative">
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
          className="min-h-10 w-full rounded-lg border border-(--border-strong) bg-(--surface-0) pl-8 pr-3 text-sm text-(--text) placeholder:text-(--muted) focus-visible:outline-2 focus-visible:outline-(--brand)"
        />
      </div>

      {owners.length > 1 && (
        <div role="group" aria-label="Filter by owner" className="flex flex-wrap gap-1.5">
          {[null, ...owners].map((o) => (
            <button
              key={o ?? "all"}
              type="button"
              aria-pressed={owner === o}
              onClick={() => {
                setOwner(o);
                setActive(0);
              }}
              className="min-h-8 rounded-full border border-(--border-strong) px-3 text-xs text-(--muted) hover:text-(--text) aria-pressed:border-(--brand) aria-pressed:bg-(--accent-soft) aria-pressed:text-(--brand) focus-visible:outline-2 focus-visible:outline-(--brand)"
            >
              {o ?? "All"}
            </button>
          ))}
        </div>
      )}

      {error ? (
        <div role="alert" className="flex flex-col items-start gap-3 rounded-lg border border-(--danger)/40 p-3">
          <p className="flex items-start gap-2 text-sm text-white/80">
            <AlertCircle size={16} className="mt-0.5 shrink-0 text-(--danger)" aria-hidden="true" /> {error}
          </p>
          <button type="button" className="workspace-button border border-(--border-strong)" onClick={() => load(true)}>
            <RotateCcw size={14} aria-hidden="true" /> Retry
          </button>
        </div>
      ) : !repos ? (
        <div aria-busy="true" className="space-y-2">
          <span role="status" className="sr-only">Loading repositories…</span>
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="h-12 animate-pulse rounded-lg bg-(--surface-2)" aria-hidden="true" />
          ))}
        </div>
      ) : options.length === 0 ? (
        <p className="rounded-lg border border-(--border) p-4 text-center text-sm text-(--muted)">
          {repos.length === 0 ? "No repositories yet. Create one on GitHub, then reload." : `No repositories match “${query}”.`}
        </p>
      ) : (
        <div id={listId} role="listbox" aria-label="Repositories" className="-mx-2 max-h-96 overflow-y-auto px-2">
          {sections.map((s) =>
            s.repos.length === 0 ? null : (
              <div key={s.title} role="group" aria-label={s.title}>
                <p className="px-2 pb-1 pt-2 text-[11px] font-semibold uppercase tracking-wide text-(--muted)" aria-hidden="true">
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
                      className="flex cursor-pointer flex-col gap-0.5 rounded-lg px-2 py-2 aria-selected:bg-(--surface-2)"
                    >
                      <span className="flex min-w-0 flex-wrap items-center gap-1.5">
                        <span className="truncate text-sm font-medium text-(--text)">{r.fullName}</span>
                        {r.private && <Badge><Lock size={10} className="mr-0.5 inline" aria-hidden="true" />Private</Badge>}
                        {r.fork && <Badge>Fork</Badge>}
                        {r.archived && <Badge tone="warning">Archived</Badge>}
                        {!r.canPush && !r.archived && <Badge tone="warning">Read-only</Badge>}
                      </span>
                      <span className="truncate text-xs text-(--muted)">
                        {r.description ? `${r.description} · ` : ""}
                        {r.pushedAt ? `pushed ${relativeTime(r.pushedAt)}` : "empty"}
                      </span>
                    </div>
                  );
                })}
              </div>
            ),
          )}
        </div>
      )}

      <p className="mt-auto text-xs leading-5 text-(--muted)">
        <a
          href="https://github.com/settings/applications"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 text-(--brand) hover:underline"
        >
          Missing an organisation&apos;s repos? Grant access on GitHub <ExternalLink size={12} aria-hidden="true" />
        </a>{" "}
        Organisations can restrict apps like Hittable until an owner approves them.
      </p>
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
