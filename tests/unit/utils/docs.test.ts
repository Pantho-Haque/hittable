import { describe, it, expect } from "vitest";
import { getDoc, getDocSlugs, getDocsNav, getDocsOrder, getDocsIndex } from "@/utils/docs";
import { DOCS_NAV } from "@/constants/docs";

const slugs = getDocSlugs();

describe("docs pages", () => {
  it("every markdown file is a page with a title and description", () => {
    expect(slugs.length).toBeGreaterThan(5);
    for (const slug of slugs) {
      const doc = getDoc(slug)!;
      expect(doc, slug).not.toBeNull();
      expect(doc.title, slug).not.toBe(slug);
      expect(doc.description, slug).not.toBe("");
      expect(doc.updated).toMatch(/^\d{4}-\d{2}-\d{2}$/);
      expect(doc.html).not.toContain("---\ntitle:");
    }
  });

  it("every page is in the sidebar manifest, and the manifest names no missing page", () => {
    const listed = DOCS_NAV.flatMap((g) => g.slugs);
    expect([...slugs].sort()).toEqual([...listed].sort());
    expect(getDocsNav().map((g) => g.title)).toEqual(DOCS_NAV.map((g) => g.title));
    expect(getDocsOrder().map((d) => d.slug)).toEqual(listed);
  });

  it("rejects slugs that could escape the docs folder", () => {
    for (const bad of ["../package", "..%2Fpackage", "index.md", "Index", "a/b", "", "nope"]) expect(getDoc(bad)).toBeNull();
  });

  it("heading ids are unique per page and the table of contents lists h2/h3", () => {
    for (const slug of slugs) {
      const { html, headings } = getDoc(slug)!;
      const ids = [...html.matchAll(/<h[1-6] id="([^"]+)"/g)].map((m) => m[1]);
      expect(new Set(ids).size, slug).toBe(ids.length);
      for (const h of headings) {
        expect([2, 3]).toContain(h.depth);
        expect(ids).toContain(h.id);
      }
    }
  });

  it("links between pages point at pages that exist, and #anchors at headings that exist", () => {
    for (const slug of slugs) {
      for (const [, target, hash] of getDoc(slug)!.html.matchAll(/href="\/docs\/([a-z0-9-]*)(?:#([^"]+))?"/g)) {
        const page = getDoc(target || "index");
        expect(page, `${slug} → /docs/${target}`).not.toBeNull();
        if (hash) expect(page!.html, `${slug} → /docs/${target}#${hash}`).toContain(`id="${hash}"`);
      }
    }
  });

  it("code blocks are HTML-escaped and carry their language", () => {
    const web = getDoc("project-format")!;
    expect(web.html).toContain('<figure class="docs-code" data-lang="json">');
    expect(web.html).toContain("&quot;method&quot;");
    expect(web.html).not.toMatch(/<code[^>]*>[^<]*"method"/);
    expect(web.html).toContain("&lt;&lt;BASE_URL&gt;&gt;");
  });

  it("tables are wrapped so wide tables scroll, and the keyboard can scroll them", () => {
    expect(getDoc("web-app")!.html).toContain('<div class="docs-table" tabindex="0" role="region" aria-label="Table"><table>');
  });

  it("code blocks can be focused, so keyboard users can scroll long lines", () => {
    expect(getDoc("getting-started")!.html).toMatch(/<figure class="docs-code"[^>]*><pre tabindex="0">/);
  });

  it("the search index lists each page's title and headings", () => {
    const index = getDocsIndex();
    expect(index.map((i) => i.slug)).toEqual(getDocsOrder().map((d) => d.slug));
    expect(index.find((i) => i.slug === "web-app")!.headings).toContain("Two workspace modes");
  });
});
