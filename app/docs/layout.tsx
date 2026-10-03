import type { Metadata } from "next";
import DocsSidebar from "@/components/docs/DocsSidebar";
import { getDocsIndex, getDocsNav } from "@/utils/docs";

export const metadata: Metadata = {
  title: { default: "Docs | Hittable", template: "%s | Hittable Docs" },
  description: "Documentation for Hittable: the browser API client, the hittable terminal app, and the on-disk project format they share.",
  alternates: { canonical: "/docs" },
};

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  const nav = getDocsNav();
  const index = getDocsIndex();
  return (
    <main className="min-h-[calc(100dvh-44px)] bg-(--ink-900) font-sans text-slate-100">
      <div className="mx-auto flex max-w-7xl flex-col px-4 sm:px-6 lg:flex-row lg:gap-10">
        <DocsSidebar nav={nav} index={index} />
        {children}
      </div>
    </main>
  );
}
