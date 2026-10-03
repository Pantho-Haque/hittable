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
    // Log the error to an error reporting service
    console.error(error);
  }, [error]);

  return (
    <div className="flex min-h-[calc(100dvh-44px)] flex-col items-center justify-center bg-(--ink-900) p-8 font-sans text-slate-100">
      <div className="w-full max-w-md rounded-2xl border border-white/10 bg-(--ink-800) p-8 text-center shadow-2xl">
        <h1 className="mb-4 text-2xl font-bold text-red-400">Something went wrong!</h1>
        <p className="mb-6 text-slate-300">
          {error.message || 'An unexpected error has occurred.'}
        </p>
        <div className="mb-6 text-sm text-slate-500">
          {error.digest && <p>Error ID: {error.digest}</p>}
        </div>
        <button
          className="rounded-full bg-(--brand) px-4 py-2 text-sm font-semibold text-(--ink-950) transition-all hover:brightness-110"
          onClick={() => reset()}
        >
          Try again
        </button>
      </div>
    </div>
  );
}
