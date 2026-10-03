import type { MetadataRoute } from "next";
import { getDocSlugs } from "@/utils/docs";

export default function sitemap(): MetadataRoute.Sitemap {
  const base = "https://hittable.vercel.app";
  const docs: MetadataRoute.Sitemap = getDocSlugs().map((slug) => ({
    url: slug === "index" ? `${base}/docs` : `${base}/docs/${slug}`,
    lastModified: new Date(),
    changeFrequency: "weekly",
    priority: slug === "index" ? 0.8 : 0.6,
  }));
  return [
    {
      url: base,
      lastModified: new Date(),
      changeFrequency: "monthly",
      priority: 1,
    },
    {
      url: `${base}/hittable`,
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 0.8,
    },
    ...docs,
  ];
}
