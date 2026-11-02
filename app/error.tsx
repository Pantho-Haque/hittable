'use client';

import { useEffect } from 'react';

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <main className="flex min-h-[calc(100dvh-44px)] items-center bg-(--ink-900) px-4 py-16 text-(--text) sm:px-6">
      <div className="mx-auto w-full max-w-md">
        <p className="font-mono text-sm text-(--danger)">Error</p>
        <h1 className="mt-3 text-2xl font-semibold tracking-[-0.02em]">Something went wrong</h1>
        <p className="mt-3 break-words text-sm leading-relaxed text-(--muted)">{error.message || 'An unexpected error occurred.'}</p>
        {error.digest && <p className="mt-2 font-mono text-xs text-(--muted)">Error ID {error.digest}</p>}
        <button
          type="button"
          onClick={() => reset()}
          className="mt-8 flex min-h-10 items-center rounded-md bg-(--brand) px-4 text-sm font-semibold text-(--accent-ink) hover:brightness-110"
        >
          Try again
        </button>
      </div>
    </main>
  );
}
