"use client";

import { useEffect, useState } from "react";
import { cn } from "@/utils/cn";
import type { DocHeading } from "@/utils/docs";

// "On this page": h2/h3 of the current doc, the one in view highlighted.
export default function DocsToc({ headings }: { headings: DocHeading[] }) {
  const [active, setActive] = useState(headings[0]?.id ?? "");

  useEffect(() => {
    if (!headings.length) return;
    const els = headings.map((h) => document.getElementById(h.id)).filter((e): e is HTMLElement => Boolean(e));
    const observer = new IntersectionObserver(
      (entries) => {
        const hit = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)[0];
        if (hit) setActive(hit.target.id);
      },
      { rootMargin: "-56px 0px -70% 0px", threshold: 0 },
    );
    els.forEach((el) => observer.observe(el));
    return () => observer.disconnect();
  }, [headings]);

  if (headings.length < 2) return null;
  return (
    <nav aria-label="On this page" className="text-[12px]">
      <div className="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-white/30">On this page</div>
      <ul className="flex flex-col gap-1 border-l border-white/5">
        {headings.map((h) => (
          <li key={h.id}>
            <a
              href={`#${h.id}`}
              className={cn(
                "-ml-px block border-l py-1 pr-2 leading-snug transition-colors",
                h.depth === 3 ? "pl-6" : "pl-3",
                active === h.id ? "border-cyan-400 text-cyan-300" : "border-transparent text-white/45 hover:text-slate-100",
              )}
            >
              {h.text}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
