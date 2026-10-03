import type { MetadataRoute } from "next";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: ["/", "/hittable", "/docs", "/sitemap.xml", "/favicon.ico", "/assets/", "/docs/"],
      disallow: ["/private/", "/api/"],
    },
    sitemap: `https://hittable.vercel.app/sitemap.xml`,
  };
}