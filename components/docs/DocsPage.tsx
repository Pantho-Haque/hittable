import Link from "next/link";
import { ArrowLeft, ArrowRight, Pencil } from "lucide-react";
import type { Doc, DocMeta } from "@/utils/docs";
import { DOCS_REPO_EDIT_BASE } from "@/constants/docs";
import DocsToc from "./DocsToc";
import CopyCodeButtons from "./CopyCodeButtons";

const href = (slug: string) => (slug === "index" ? "/docs" : `/docs/${slug}`);

export default function DocsPage({ doc, prev, next }: { doc: Doc; prev?: DocMeta; next?: DocMeta }) {
  return (
    <div className="flex min-w-0 flex-1 gap-10">
      <article className="min-w-0 flex-1 py-10 lg:py-12">
        <header className="mb-8">
          {doc.group && (
            <div className="mb-2 text-[10px] font-bold uppercase tracking-[0.2em] text-cyan-400/70">{doc.group}</div>
          )}
          <h1 className="text-3xl font-bold tracking-tight text-slate-50 sm:text-4xl">{doc.title}</h1>
          {doc.description && <p className="mt-3 max-w-2xl text-[15px] leading-relaxed text-white/55">{doc.description}</p>}
        </header>

        <CopyCodeButtons>
          <div className="docs-prose" dangerouslySetInnerHTML={{ __html: doc.html }} />
        </CopyCodeButtons>

        <footer className="mt-14 border-t border-white/5 pt-6">
          <div className="mb-6 flex flex-wrap items-center justify-between gap-3 text-[12px] text-white/35">
            <span>Last updated {doc.updated} · describes the current main branch (docs are not versioned)</span>
            <a
              href={`${DOCS_REPO_EDIT_BASE}${doc.slug}.md`}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 hover:text-cyan-300 transition-colors"
            >
              <Pencil size={12} /> Edit this page
            </a>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            {prev ? (
              <Link href={href(prev.slug)} className="group rounded-xl border border-white/5 bg-white/2 p-4 hover:bg-white/4 transition-colors">
                <div className="mb-1 flex items-center gap-1 text-[10px] uppercase tracking-[0.2em] text-white/30">
                  <ArrowLeft size={12} /> Previous
                </div>
                <div className="text-sm text-slate-100 group-hover:text-cyan-300 transition-colors">{prev.title}</div>
              </Link>
            ) : (
              <span />
            )}
            {next && (
              <Link href={href(next.slug)} className="group rounded-xl border border-white/5 bg-white/2 p-4 text-right hover:bg-white/4 transition-colors">
                <div className="mb-1 flex items-center justify-end gap-1 text-[10px] uppercase tracking-[0.2em] text-white/30">
                  Next <ArrowRight size={12} />
                </div>
                <div className="text-sm text-slate-100 group-hover:text-cyan-300 transition-colors">{next.title}</div>
              </Link>
            )}
          </div>
        </footer>
      </article>

      <aside className="hidden xl:block w-56 shrink-0">
        <div className="sticky top-11 max-h-[calc(100dvh-44px)] overflow-y-auto py-12">
          <DocsToc headings={doc.headings} />
        </div>
      </aside>
    </div>
  );
}
