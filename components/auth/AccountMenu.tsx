"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { Github } from "lucide-react";
import { ModalShell } from "@/components/ui/SharedModal";
import { clearWorkingCopies, hasWorkingCopies } from "@/services/github/workingCopy";
import { setSignedInHint } from "@/utils/github/local";

type User = { login: string; name: string | null; avatarUrl: string };

// undefined while loading; null when signed out. A 404/500/network error (the
// server side not configured or not deployed yet) also reads as signed out.
async function fetchUser(): Promise<User | null> {
  try {
    const res = await fetch("/api/github/session", { cache: "no-store" });
    if (!res.ok) return null;
    const body = (await res.json()) as { user?: User | null };
    return body.user?.login ? body.user : null;
  } catch {
    return null;
  }
}

const ITEM =
  "flex w-full min-h-9 items-center px-3 text-left text-[13px] text-(--text) hover:bg-(--ink-500) focus-visible:bg-(--ink-500) focus-visible:outline-none";

export default function AccountMenu() {
  const pathname = usePathname();
  const [user, setUser] = useState<User | null | undefined>(undefined);
  const [open, setOpen] = useState(false);
  const [askKeep, setAskKeep] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let live = true;
    fetchUser().then((u) => live && setUser(u));
    return () => {
      live = false;
    };
  }, []);

  // Focus the first item on open; close on a click outside.
  useEffect(() => {
    if (!open) return;
    menuRef.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
    const onDown = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", onDown);
    return () => document.removeEventListener("pointerdown", onDown);
  }, [open]);

  if (user === undefined) return <span className="block h-7 w-7" aria-hidden="true" />;

  if (user === null) {
    // Only rendered client-side (after the fetch), so reading location is safe.
    const next = pathname + window.location.search;
    return (
      <a
        href={`/api/github/login?next=${encodeURIComponent(next)}`}
        className="flex min-h-8 items-center gap-2 rounded-md border border-(--border-strong) px-2.5 text-[13px] font-medium text-(--text) transition-colors hover:border-(--muted) hover:bg-(--ink-600)"
      >
        <Github size={14} aria-hidden="true" />
        <span className="sm:hidden">Sign in</span>
        <span className="hidden sm:inline">Sign in with GitHub</span>
      </a>
    );
  }

  const close = (refocus = true) => {
    setOpen(false);
    if (refocus) buttonRef.current?.focus();
  };

  const onMenuKey = (e: React.KeyboardEvent) => {
    const items = [...(menuRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])];
    const i = items.indexOf(document.activeElement as HTMLElement);
    const go = (n: number) => {
      e.preventDefault();
      items[(n + items.length) % items.length]?.focus();
    };
    if (e.key === "Escape") {
      e.preventDefault();
      close();
    } else if (e.key === "ArrowDown") go(i + 1);
    else if (e.key === "ArrowUp") go(i - 1);
    else if (e.key === "Home") go(0);
    else if (e.key === "End") go(items.length - 1);
    else if (e.key === "Tab") close(false);
  };

  const signOut = async (discard = false) => {
    try {
      if (discard) await clearWorkingCopies();
      setSignedInHint(false);
      await fetch("/api/github/logout", { method: "POST", headers: { "X-Hittable": "1" } });
    } finally {
      window.location.assign("/");
    }
  };

  // FR-G1: uncommitted GitHub changes on this device are the user's call.
  const requestSignOut = async () => {
    if (!(await hasWorkingCopies())) return signOut();
    setOpen(false);
    setAskKeep(true);
  };

  return (
    <div ref={rootRef} className="relative">
      <button
        ref={buttonRef}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Account: ${user.login}`}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") {
            e.preventDefault();
            setOpen(true);
          }
        }}
        className="flex h-8 w-8 items-center justify-center rounded-full border border-(--border-strong) hover:border-(--muted)"
      >
        {/* A GitHub avatar URL; next/image would need remotePatterns for it. */}
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img src={user.avatarUrl} alt="" width={24} height={24} className="h-6 w-6 rounded-full bg-(--ink-600)" />
      </button>

      {open && (
        <div
          ref={menuRef}
          role="menu"
          aria-label="Account"
          onKeyDown={onMenuKey}
          className="absolute right-0 top-[calc(100%+6px)] z-50 w-60 overflow-hidden rounded-md border border-(--border-strong) bg-(--ink-800) py-1 shadow-lg"
        >
          <div className="border-b border-(--border) px-3 pb-2 pt-1.5">
            {user.name && <p className="truncate text-[13px] font-medium text-(--text)">{user.name}</p>}
            <p className="truncate font-mono text-xs text-(--muted)">@{user.login}</p>
          </div>
          <div className="py-1">
            <a role="menuitem" tabIndex={-1} href="/hittable" className={ITEM}>
              Your repositories
            </a>
            <a
              role="menuitem"
              tabIndex={-1}
              href="https://github.com/settings/applications"
              target="_blank"
              rel="noopener noreferrer"
              className={ITEM}
            >
              Manage access on GitHub
            </a>
          </div>
          <div className="border-t border-(--border) pt-1">
            <button role="menuitem" tabIndex={-1} type="button" onClick={() => void requestSignOut()} className={ITEM}>
              Sign out
            </button>
          </div>
        </div>
      )}

      {askKeep && (
        <ModalShell title="You have uncommitted changes" onClose={() => setAskKeep(false)}>
          <p className="text-xs leading-5 text-(--muted)">
            Changes you haven&apos;t committed to GitHub are saved in this browser. Keep them here for when you sign in again, or
            discard them now (for example on a shared computer).
          </p>
          <div className="flex flex-wrap justify-end gap-2">
            <button type="button" className="workspace-button border border-(--border-strong)" onClick={() => setAskKeep(false)}>
              Cancel
            </button>
            <button type="button" className="workspace-button border border-(--danger)/50 text-(--danger)" onClick={() => void signOut(true)}>
              Discard
            </button>
            <button type="button" className="workspace-button border border-(--brand) text-(--brand)" onClick={() => void signOut()}>
              Keep them on this device
            </button>
          </div>
        </ModalShell>
      )}
    </div>
  );
}
