import Link from "next/link";

export default function NotFound() {
  return (
    <main className="flex min-h-[calc(100dvh-44px)] items-center bg-(--ink-900) px-4 py-16 text-(--text) sm:px-6">
      <div className="mx-auto w-full max-w-md">
        <p className="font-mono text-sm text-(--muted)">
          <span className="text-(--danger)">404</span> Not Found
        </p>
        <h1 className="mt-3 text-5xl font-semibold tracking-[-0.03em]">404</h1>
        <h2 className="mt-2 text-lg text-(--text)">Page not found</h2>
        <p className="mt-3 text-sm leading-relaxed text-(--muted)">Nothing lives at this path. It may have moved, or the link has a typo.</p>
        <div className="mt-8 flex flex-wrap items-center gap-3">
          <Link href="/" className="flex min-h-10 items-center rounded-md bg-(--brand) px-4 text-sm font-semibold text-(--accent-ink) hover:brightness-110">
            Home
          </Link>
          <Link href="/docs" className="flex min-h-10 items-center rounded-md border border-(--border-strong) px-4 text-sm font-medium text-(--text) hover:bg-(--ink-800)">
            Docs
          </Link>
        </div>
      </div>
    </main>
  );
}
