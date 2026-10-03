import type { Metadata } from "next";
import { notFound } from "next/navigation";
import DocsPage from "@/components/docs/DocsPage";
import { getDoc, getDocSlugs, getDocsOrder } from "@/utils/docs";

type Props = { params: Promise<{ slug: string }> };

export const dynamicParams = false;

export function generateStaticParams() {
  return getDocSlugs()
    .filter((s) => s !== "index")
    .map((slug) => ({ slug }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const doc = getDoc(slug);
  if (!doc) return {};
  return {
    title: doc.title,
    description: doc.description,
    alternates: { canonical: `/docs/${slug}` },
    openGraph: { title: `${doc.title} | Hittable Docs`, description: doc.description, url: `/docs/${slug}` },
  };
}

export default async function DocPage({ params }: Props) {
  const { slug } = await params;
  const doc = getDoc(slug);
  if (!doc) notFound();
  const order = getDocsOrder();
  const i = order.findIndex((d) => d.slug === slug);
  return <DocsPage doc={doc} prev={i > 0 ? order[i - 1] : undefined} next={order[i + 1]} />;
}
