"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Search, X, Menu } from "lucide-react";
import { cn } from "@/utils/cn";
import type { NavGroup } from "@/utils/docs";

type IndexEntry = { slug: string; title: string; headings: string[] };

export default function DocsSidebar({ nav, index }: { nav: NavGroup[]; index: IndexEntry[] }) {
  const pathname = usePathname();
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const active = pathname === "/docs" ? "index" : pathname.replace(/^\/docs\//, "");

  // Filter pages by title or any heading; the matching heading is shown as
  // a hint so the result explains itself.
  const results = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return null;
    return index
      .map((e) => {
        const inTitle = e.title.toLowerCase().includes(q);
        const heading = e.headings.find((h) => h.toLowerCase().includes(q));
        return inTitle || heading ? { ...e, hint: inTitle ? "" : heading ?? "" } : null;
      })
      .filter((e): e is IndexEntry & { hint: string } => Boolean(e));
  }, [query, index]);

  const href = (slug: string) => (slug === "index" ? "/docs" : `/docs/${slug}`);

  const list = (
    <nav aria-label="Documentation" className="flex flex-col gap-6 text-sm">
      <label className="relative block">
        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-white/30" />
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search docs…"
          className="w-full rounded-lg border border-white/10 bg-white/5 py-2 pl-9 pr-3 text-[13px] text-slate-100 placeholder:text-white/30 focus:border-cyan-500/40 focus:bg-white/8 outline-none transition-colors"
        />
      </label>

      {results ? (
        <ul className="flex flex-col gap-1">
          {results.length === 0 && <li className="px-3 py-2 text-white/40">No matches</li>}
          {results.map((r) => (
            <li key={r.slug}>
              <Link
                href={href(r.slug)}
                onClick={() => setOpen(false)}
                className="block rounded-md px-3 py-2 hover:bg-white/5 transition-colors"
              >
                <div className="text-slate-100">{r.title}</div>
                {r.hint && <div className="text-[12px] text-white/40 truncate">§ {r.hint}</div>}
              </Link>
            </li>
          ))}
        </ul>
      ) : (
        nav.map((g) => (
          <div key={g.title}>
            <div className="mb-2 px-3 text-[10px] font-bold uppercase tracking-[0.2em] text-white/30">{g.title}</div>
            <ul className="flex flex-col gap-0.5 border-l border-white/5">
              {g.items.map((d) => {
                const isActive = d.slug === active;
                return (
                  <li key={d.slug}>
                    <Link
                      href={href(d.slug)}
                      onClick={() => setOpen(false)}
                      aria-current={isActive ? "page" : undefined}
                      className={cn(
                        "-ml-px block border-l py-1.5 pl-3 pr-2 text-[13px] transition-colors",
                        isActive
                          ? "border-cyan-400 text-cyan-300"
                          : "border-transparent text-white/55 hover:border-white/20 hover:text-slate-100",
                      )}
                    >
                      {d.title}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        ))
      )}
    </nav>
  );

  return (
    <>
      {/* Mobile: toggle bar under the top bar */}
      <div className="lg:hidden sticky top-11 z-30 flex items-center justify-between border-b border-white/5 bg-(--ink-900)/90 px-4 py-2 backdrop-blur-md">
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          className="flex items-center gap-2 text-[11px] font-bold uppercase tracking-[0.18em] text-white/60 hover:text-cyan-300 transition-colors"
        >
          {open ? <X size={14} /> : <Menu size={14} />}
          Docs menu
        </button>
        <span className="text-[11px] text-white/30">{nav.flatMap((g) => g.items).find((d) => d.slug === active)?.title}</span>
      </div>
      {open && (
        <div className="lg:hidden border-b border-white/5 bg-(--ink-800) px-4 py-5">{list}</div>
      )}

      {/* Desktop: sticky column */}
      <aside className="hidden lg:block w-64 shrink-0">
        <div className="sticky top-11 max-h-[calc(100dvh-44px)] overflow-y-auto py-10 pr-6">{list}</div>
      </aside>
    </>
  );
}
