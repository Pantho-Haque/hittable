import type { ReactNode } from "react";
import Link from "next/link";
import { Github } from "lucide-react";
import { RequestCommitDemo } from "@/components/homepage/RequestCommitDemo";
import { CopyCommand } from "@/components/homepage/CopyCommand";
import { BrandMark } from "@/components/homepage/TopBar";
import { SHORTCUTS } from "@/constants/shortcuts";
import {
  AUTHOR_URL,
  FEATURES,
  GITHUB_REPO_URL,
  INSTALL_COMMAND,
  NOT_REQUESTED,
  PRIVACY_FACTS,
  SCOPES,
  SIGN_IN_URL,
  STEPS,
} from "@/constants/landing";

const external = { target: "_blank", rel: "noopener noreferrer" } as const;
const LINK = "text-(--brand) underline decoration-(--border-strong) underline-offset-4 hover:decoration-(--brand)";

// The page is a git log: every section is a commit on one branch line (the
// left rail), and its node sits next to the short SHA above the heading.
function Section({ id, sha, title, lede, children }: { id: string; sha: string; title: string; lede?: ReactNode; children: ReactNode }) {
  return (
    <section id={id} aria-labelledby={`${id}-title`} className="scroll-mt-14 py-16 sm:py-20">
      <header className="mb-10 max-w-2xl">
        <p className="relative font-mono text-xs text-(--muted)">
          <span
            aria-hidden="true"
            className="absolute -left-[30px] top-[2.5px] h-[11px] w-[11px] rounded-full border-2 border-(--brand) bg-(--ink-900) sm:-left-[46px]"
          />
          {sha}
        </p>
        <h2 id={`${id}-title`} className="mt-2 text-2xl font-semibold tracking-[-0.02em] text-(--text) sm:text-[32px] sm:leading-tight">
          {title}
        </h2>
        {lede && <p className="mt-3 text-base leading-relaxed text-(--muted)">{lede}</p>}
      </header>
      {children}
    </section>
  );
}

function Keys({ keys }: { keys: string[] }) {
  return (
    <span className="flex shrink-0 items-center gap-1">
      {keys.map((k) => (
        <kbd
          key={k}
          className="min-w-7 rounded border border-(--border-strong) border-b-2 bg-(--ink-800) px-1.5 py-0.5 text-center font-mono text-xs text-(--text)"
        >
          {k}
        </kbd>
      ))}
    </span>
  );
}

export default function HomePage() {
  return (
    <main className="overflow-x-hidden bg-(--ink-900) text-(--text)">
      <div className="mx-auto max-w-6xl px-4 sm:px-6">
        {/* ── Hero ── */}
        <section className="grid items-start gap-12 pb-16 pt-14 sm:pt-20 lg:grid-cols-[minmax(0,1fr)_minmax(0,520px)] lg:gap-16 lg:pb-24 lg:pt-24">
          <div className="min-w-0 max-w-xl">
            <h1 className="text-[40px] font-semibold leading-[1.05] tracking-[-0.035em] text-(--text) sm:text-5xl lg:text-[56px]">
              API requests that live in your repo.
            </h1>
            <p className="mt-6 text-lg leading-relaxed text-(--muted)">
              Hittable is an open-source API client. Sign in with GitHub, open a repository, and your requests are plain{" "}
              <code className="font-mono text-base text-(--text)">.hit</code> files in its{" "}
              <code className="font-mono text-base text-(--text)">hittable/</code> folder. Send them, edit them, and commit the change straight
              back. No database, no cloud workspace, no account beyond GitHub.
            </p>
            <div className="mt-8 flex flex-wrap items-center gap-3">
              <a
                href={SIGN_IN_URL}
                className="flex min-h-11 items-center gap-2 rounded-md bg-(--brand) px-5 text-[15px] font-semibold text-(--accent-ink) transition-[filter] hover:brightness-110"
              >
                <Github size={18} aria-hidden="true" />
                Sign in with GitHub
              </a>
              <Link
                href="/hittable"
                className="flex min-h-11 items-center rounded-md border border-(--border-strong) px-5 text-[15px] font-medium text-(--text) transition-colors hover:bg-(--ink-800)"
              >
                Open a local folder
              </Link>
            </div>
            <p className="mt-4 text-sm text-(--muted)">
              Local folders work in Chrome and Edge, with no account. <a href="#privacy" className={LINK}>What sign-in can access</a>
            </p>
          </div>
          <div className="flex min-w-0 justify-center lg:justify-end">
            <RequestCommitDemo />
          </div>
        </section>

        {/* ── The log ── */}
        <div className="ml-[5px] border-l border-(--border) pl-6 sm:pl-10">
          <Section
            id="how"
            sha="e81d4b6"
            title="From sign-in to commit"
            lede="Your requests are files in your repository, so they are reviewed, branched and versioned like the code they call."
          >
            <ol className="relative grid gap-10 sm:grid-cols-3 sm:gap-8">
              <span aria-hidden="true" className="absolute left-0 right-0 top-[5px] hidden h-px bg-(--border-strong) sm:block" />
              {STEPS.map((s) => (
                <li key={s.sha} className="relative min-w-0 sm:pt-8">
                  <span
                    aria-hidden="true"
                    className="absolute left-0 top-0 hidden h-[11px] w-[11px] rounded-full border-2 border-(--muted) bg-(--ink-900) sm:block"
                  />
                  <p className="font-mono text-xs text-(--muted)">{s.sha}</p>
                  <h3 className="mt-1 text-base font-semibold text-(--text)">{s.title}</h3>
                  <p className="mt-2 text-sm leading-relaxed text-(--muted)">{s.desc}</p>
                  <p className="mt-4 inline-block max-w-full truncate rounded border border-(--border) bg-(--ink-800) px-2 py-1 font-mono text-xs text-(--text)">
                    {s.artifact}
                  </p>
                </li>
              ))}
            </ol>
          </Section>

          <Section id="features" sha="b20f7c3" title="A full client, in plain files" lede="Everything below works on a GitHub repository and on a local folder.">
            <dl className="grid border-t border-(--border) md:grid-cols-2 md:gap-x-12">
              {FEATURES.map((f) => (
                <div key={f.title} className="min-w-0 border-b border-(--border) py-5">
                  <dt className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
                    <span className="font-medium text-(--text)">{f.title}</span>
                    <code className="max-w-full truncate rounded border border-(--border) bg-(--ink-800) px-1.5 py-0.5 font-mono text-xs text-(--text)">
                      {f.token}
                    </code>
                  </dt>
                  <dd className="mt-1.5 text-sm leading-relaxed text-(--muted)">{f.desc}</dd>
                </div>
              ))}
            </dl>
          </Section>

          <Section
            id="privacy"
            sha="7a4e19d"
            title="Permissions and privacy"
            lede="Sign-in uses a GitHub OAuth App. This is everything it asks for, and where the token goes."
          >
            <div className="grid gap-12 lg:grid-cols-2">
              <div className="min-w-0">
                <h3 className="text-sm font-medium text-(--muted)">Requested</h3>
                <ul className="mt-3 border-t border-(--border)">
                  {SCOPES.map((s) => (
                    <li key={s.name} className="grid grid-cols-[88px_minmax(0,1fr)] gap-4 border-b border-(--border) py-4">
                      <code className="font-mono text-sm text-(--brand)">{s.name}</code>
                      <span className="text-sm leading-relaxed text-(--text)">{s.why}</span>
                    </li>
                  ))}
                </ul>
                <h3 className="mt-8 text-sm font-medium text-(--muted)">Not requested</h3>
                <ul className="mt-3 flex flex-wrap gap-2" aria-label="Scopes not requested">
                  {NOT_REQUESTED.map((n) => (
                    <li key={n} className="rounded border border-(--border) px-2 py-1 font-mono text-xs text-(--muted) line-through">
                      {n}
                    </li>
                  ))}
                </ul>
                <p className="mt-3 text-sm leading-relaxed text-(--muted)">No organisation admin, workflow, webhook or delete access.</p>
              </div>
              <dl className="min-w-0 border-t border-(--border)">
                {PRIVACY_FACTS.map((p) => (
                  <div key={p.term} className="grid gap-1 border-b border-(--border) py-4 sm:grid-cols-[120px_minmax(0,1fr)] sm:gap-4">
                    <dt className="text-sm font-medium text-(--text)">{p.term}</dt>
                    <dd className="text-sm leading-relaxed text-(--muted)">{p.detail}</dd>
                  </div>
                ))}
              </dl>
              <p className="pt-4 text-sm">
                <a href="https://github.com/settings/applications" {...external} className={LINK}>
                  Review authorised apps on GitHub
                </a>
              </p>
            </div>
          </Section>

          <Section
            id="terminal"
            sha="0c95e2a"
            title="The same files in your terminal"
            lede={
              <>
                <code className="font-mono text-[15px] text-(--text)">hittable</code> is a terminal app that reads the same{" "}
                <code className="font-mono text-[15px] text-(--text)">hittable/</code> folder: browse, send and edit requests, with git and an
                editor built in. Open source, like the web app.
              </>
            }
          >
            <div className="max-w-3xl">
              <CopyCommand command={INSTALL_COMMAND} />
              <p className="mt-4 text-sm leading-relaxed text-(--muted)">
                <Link href="/docs/shellapp" className={LINK}>
                  Terminal app docs
                </Link>
                <span aria-hidden="true"> · </span>
                Calling <code className="font-mono text-(--text)">localhost</code> from the web app?{" "}
                <Link href="/docs/browser-extension" className={LINK}>
                  Add the browser extension
                </Link>
              </p>
            </div>
          </Section>

          <Section id="shortcuts" sha="4d1b8f0" title="Keyboard first" lede="Every action has a key. Press ? in the app for the full list.">
            <dl className="grid border-t border-(--border) md:grid-cols-2 md:gap-x-12">
              {SHORTCUTS.map((s) => (
                <div key={s.id} className="flex min-h-12 items-center justify-between gap-4 border-b border-(--border) py-2">
                  <dt className="text-sm text-(--muted)">{s.description}</dt>
                  <dd>
                    <Keys keys={s.keys} />
                  </dd>
                </div>
              ))}
            </dl>
          </Section>
        </div>

        {/* ── Footer ── */}
        <footer className="flex flex-col gap-4 border-t border-(--border) py-8 text-[13px] text-(--muted) sm:flex-row sm:items-center sm:justify-between">
          <span className="flex items-center gap-2 text-(--text)">
            <BrandMark size={12} />
            Hittable
            <span className="text-(--muted)">· open source</span>
          </span>
          <nav aria-label="Footer" className="flex flex-wrap items-center gap-x-6 gap-y-2">
            <Link href="/docs" className="hover:text-(--text)">
              Docs
            </Link>
            <a href={GITHUB_REPO_URL} {...external} className="hover:text-(--text)">
              GitHub
            </a>
            <a href={AUTHOR_URL} {...external} className="hover:text-(--text)">
              Made by Pantho Haque
            </a>
          </nav>
        </footer>
      </div>
    </main>
  );
}
