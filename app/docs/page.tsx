import { notFound } from "next/navigation";
import DocsPage from "@/components/docs/DocsPage";
import { getDoc, getDocsOrder } from "@/utils/docs";

export default function DocsIndexPage() {
  const doc = getDoc("index");
  if (!doc) notFound();
  const order = getDocsOrder();
  const i = order.findIndex((d) => d.slug === "index");
  return <DocsPage doc={doc} next={order[i + 1]} />;
}
