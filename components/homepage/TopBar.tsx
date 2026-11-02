"use client";

import Link from "next/link";
import { usePathname, useSelectedLayoutSegment } from "next/navigation";
import { useSyncExternalStore } from "react";
import { Search } from "lucide-react";
import { openTool } from "@/utils/uiEvents";
import AccountMenu from "@/components/auth/AccountMenu";

const noSubscribe = () => () => {};
const isMac = () => /Mac|iPhone|iPad/.test(navigator.platform);

// Opens the command palette (Ctrl/Cmd+K); ⌘ on a Mac, decided after hydration.
function PaletteButton() {
  const mac = useSyncExternalStore(noSubscribe, isMac, () => false);
  return (
    <button
      type="button"
      // Safari does not focus a button on click; focus it so the palette
      // can hand focus back here when it closes.
      onClick={(e) => {
        e.currentTarget.focus();
        openTool("palette");
      }}
      aria-label="Search and commands"
      aria-keyshortcuts="Control+K Meta+K"
      title={`Search and commands (${mac ? "⌘K" : "Ctrl+K"})`}
      className="flex min-h-8 items-center gap-2 rounded-md border border-(--border-strong) bg-(--ink-800) px-2.5 text-xs text-(--muted) transition-colors hover:bg-(--ink-600) hover:text-(--text)"
    >
      <Search size={14} aria-hidden="true" />
      <span className="hidden sm:inline">Search…</span>
      <kbd className="hidden rounded border border-(--border-strong) px-1.5 font-mono text-[11px] sm:inline">
        {mac ? "⌘K" : "Ctrl K"}
      </kbd>
    </button>
  );
}

// The mark: the slanted "H" on a rounded tile. `size` is the glyph; the tile is twice that.
export function BrandMark({ size = 16 }: { size?: number }) {
  return (
    <span
      aria-hidden="true"
      style={{ width: size * 2, height: size * 2 }}
      className="relative flex shrink-0 items-center justify-center rounded-xl border border-white/10 bg-linear-to-b from-(--ink-500) to-(--ink-800) transition-all duration-300 group-hover:border-white/20 group-hover:shadow-[0_4px_16px_rgba(255,255,255,0.1)]"
    >
      <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="text-slate-100 transition-transform duration-300 group-hover:scale-110">
        <path d="M10 3L6 21M18 3l-4 18M4 12h16" />
      </svg>
    </span>
  );
}

type NavLink = { label: string; href: string; external?: boolean };
const GITHUB: NavLink = { label: "GitHub", href: "https://github.com/Pantho-Haque/hittable", external: true };

const HOME_NAV_LINKS: NavLink[] = [
  { label: "How it works", href: "/#how" },
  { label: "Features", href: "/#features" },
  { label: "Privacy", href: "/#privacy" },
  { label: "Terminal", href: "/#terminal" },
  { label: "Docs", href: "/docs" },
  GITHUB,
];
const DOCS_NAV_LINKS: NavLink[] = [
  { label: "Overview", href: "/docs" },
  { label: "Web app", href: "/docs/web-app" },
  { label: "Terminal app", href: "/docs/shellapp" },
  GITHUB,
];

export default function Topbar() {
  const pathname = usePathname();
  // The segment, not the pathname: a 404 page is one prerendered file served
  // for every unknown URL, so its pathname differs between server and client
  // and would break hydration. Its segment is "_not-found" on both.
  const segment = useSelectedLayoutSegment();
  const isApp = segment === "hittable";
  const isDocs = segment === "docs";
  const links = isDocs ? DOCS_NAV_LINKS : HOME_NAV_LINKS;

  return (
    <header className="relative z-50 flex h-11 shrink-0 items-center justify-between gap-3 border-b border-(--border) bg-(--ink-950) px-4 sm:px-6">
      <div className="flex min-w-0 items-center gap-3">
        <Link href="/" className="group flex items-center gap-2 rounded-sm text-sm font-semibold tracking-tight text-(--text)">
          <BrandMark />
          Hittable
        </Link>

        {(isApp || isDocs) && (
          <span className="font-mono text-xs text-(--muted)">
            <span aria-hidden="true">/ </span>
            {isApp ? "app" : "docs"}
          </span>
        )}

        {!isApp && (
          <nav aria-label="Site" className={`${isDocs ? "md:flex" : "lg:flex"} ml-6 hidden items-center gap-5`}>
            {links.map((item) => {
              const current =
                (item.href === "/docs" && pathname === "/docs") ||
                (item.href !== "/docs" && !item.external && !item.href.includes("#") && pathname.startsWith(item.href));
              return (
                <a
                  key={item.label}
                  href={item.href}
                  target={item.external ? "_blank" : undefined}
                  rel={item.external ? "noopener noreferrer" : undefined}
                  aria-current={current ? "page" : undefined}
                  className={`text-[13px] transition-colors hover:text-(--text) ${current ? "text-(--text)" : "text-(--muted)"}`}
                >
                  {item.label}
                </a>
              );
            })}
          </nav>
        )}
      </div>

      <div className="flex shrink-0 items-center gap-2">
        {isApp ? (
          <PaletteButton />
        ) : (
          <Link
            href="/hittable"
            className="hidden min-h-8 items-center rounded-md px-2.5 text-[13px] font-medium text-(--brand) transition-colors hover:bg-(--accent-soft) sm:flex"
          >
            Open app
          </Link>
        )}
        <AccountMenu />
      </div>
    </header>
  );
}
