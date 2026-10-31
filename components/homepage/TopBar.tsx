"use client";

import Link from "next/link";
import { usePathname, useSelectedLayoutSegment } from "next/navigation";
import WorkspaceSwitcher from "@/components/workspace/WorkspaceSwitcher";
import { useSyncExternalStore } from "react";
import { Search } from "lucide-react";
import { openTool } from "@/utils/uiEvents";

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
      className="flex min-h-8 items-center gap-2 rounded-lg border border-white/15 bg-white/5 px-2.5 py-1.5 text-xs text-white/70 transition-colors hover:bg-white/10 hover:text-white focus-visible:outline-2 focus-visible:outline-(--brand)"
    >
      <Search size={14} aria-hidden="true" />
      <span className="hidden sm:inline">Search…</span>
      <kbd className="hidden rounded border border-white/15 px-1.5 font-mono text-[11px] text-white/70 sm:inline">
        {mac ? "⌘K" : "Ctrl K"}
      </kbd>
    </button>
  );
}

type NavLink = { label: string; href: string; external?: boolean };
const GITHUB: NavLink = { label: "GitHub", href: "https://github.com/Pantho-Haque/hittable", external: true };

// Landing page: sections of the page. Docs: the three places a reader goes.
const HOME_NAV_LINKS: NavLink[] = [
  { label: "Features", href: "/#features" },
  { label: "Shortcuts", href: "/#shortcuts" },
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

  return (
    <header className="relative z-50 flex items-center justify-between px-6 h-11 border-b border-white/5 bg-(--ink-950)/90 backdrop-blur-md shrink-0">
      {/* Left — logo + breadcrumb */}
      <div className="flex items-center gap-3">
        <Link href="/" className="flex items-center gap-3 group outline-none">
          <div className="relative flex items-center justify-center w-8 h-8 rounded-xl bg-linear-to-b from-(--ink-500) to-(--ink-800) border border-white/10 transition-all duration-300 group-hover:shadow-[0_4px_16px_rgba(255,255,255,0.1)] group-hover:border-white/20">
            <div className="absolute inset-0 rounded-xl bg-white/5 opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="text-slate-100 transition-transform duration-300 group-hover:scale-110">
              <path d="M10 3L6 21M18 3l-4 18M4 12h16" />
            </svg>
          </div>
          <span className="text-[11px] font-bold tracking-[0.18em] uppercase text-white/50 group-hover:text-white/80 transition-colors">
            Hittable
          </span>
        </Link>

        {(isApp || isDocs) && (
          <>
            <span className="text-white/50 text-xs">/</span>
            <span className="text-[11px] tracking-[0.15em] uppercase text-cyan-400/70">
              {isApp ? "App" : "Docs"}
            </span>
          </>
        )}
      </div>

      {/* Center — nav links (hidden on app page to save space) */}
      {!isApp && (
        <nav className="hidden sm:flex items-center gap-6 absolute left-1/2 -translate-x-1/2">
          {(isDocs ? DOCS_NAV_LINKS : HOME_NAV_LINKS).map((item) => (
            <a
              key={item.label}
              href={item.href}
              target={item.external ? "_blank" : undefined}
              rel={item.external ? "noopener noreferrer" : undefined}
              className={`text-[11px] tracking-[0.2em] uppercase transition-colors hover:text-cyan-400 ${
                (item.href === "/docs" && pathname === "/docs") || (item.href !== "/docs" && !item.external && pathname.startsWith(item.href))
                  ? "text-cyan-400"
                  : "text-white/50"
              }`}
            >
              {item.label}
            </a>
          ))}
        </nav>
      )}

      {/* Right — CTA */}
      <div className="flex items-center gap-3">
        {isApp ? (
          <>
            <PaletteButton />
            <WorkspaceSwitcher />
          </>
        ) : (
          <Link
            href="/hittable"
            className="flex items-center gap-1.5 px-3 py-1.5 text-[11px] font-bold tracking-[0.15em] uppercase rounded text-black transition-all hover:scale-105 active:scale-95"
            style={{
              background: "var(--brand)",
              boxShadow: "0 0 12px rgba(0,229,204,0.3)",
            }}
          >
            Open App
            <svg
              width="10"
              height="10"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.5"
            >
              <path d="M5 12h14M12 5l7 7-7 7" />
            </svg>
          </Link>
        )}
      </div>

      {/* Bottom glow line */}
      <div className="absolute bottom-0 left-0 right-0 h-px bg-linear-to-r from-transparent via-cyan-500/20 to-transparent" />
    </header>
  );
}
