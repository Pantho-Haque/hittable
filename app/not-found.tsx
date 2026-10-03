import Link from "next/link";

export default function NotFound() {
  return (
    <main className="flex min-h-[calc(100dvh-44px)] flex-col items-center justify-center bg-(--ink-900) p-8 font-sans text-slate-100">
      <div className="w-full max-w-md rounded-2xl border border-white/10 bg-(--ink-800) p-8 text-center shadow-2xl">
        <h1 className="mb-2 text-5xl font-bold tracking-tight">404</h1>
        <h2 className="mb-4 text-lg text-slate-300">Page not found</h2>
        <p className="mb-8 text-sm text-slate-400">The page you are looking for doesn&apos;t exist or has been moved.</p>
        <div className="flex items-center justify-center gap-3">
          <Link href="/" className="rounded-full bg-(--brand) px-4 py-2 text-sm font-semibold text-(--ink-950) transition-all hover:brightness-110">
            Home
          </Link>
          <Link href="/docs" className="rounded-full border border-white/10 px-4 py-2 text-sm font-medium text-slate-300 transition-colors hover:bg-white/5">
            Docs
          </Link>
        </div>
      </div>
    </main>
  );
}
