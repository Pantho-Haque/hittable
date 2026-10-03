// Docs content pipeline. Pages are markdown files in content/docs with a
// small frontmatter block; they are read and rendered at build time (server
// components only — this module uses fs). There is no versioning: the docs
// describe whatever is on main.
import fs from "node:fs";
import path from "node:path";
import { Marked, type Tokens } from "marked";
import { DOCS_NAV } from "@/constants/docs";

export type DocHeading = { id: string; text: string; depth: number };

export type DocMeta = {
  slug: string;
  title: string;
  description: string;
  group: string;
};

export type Doc = DocMeta & {
  html: string;
  headings: DocHeading[];
  updated: string; // ISO date of the file's last modification
};

const DOCS_DIR = path.join(process.cwd(), "content", "docs");

function parseFrontmatter(raw: string): { data: Record<string, string>; body: string } {
  const m = raw.match(/^---\n([\s\S]*?)\n---\n?/);
  if (!m) return { data: {}, body: raw };
  const data: Record<string, string> = {};
  for (const line of m[1].split("\n")) {
    const i = line.indexOf(":");
    if (i > 0) data[line.slice(0, i).trim()] = line.slice(i + 1).trim().replace(/^"(.*)"$/, "$1");
  }
  return { data, body: raw.slice(m[0].length) };
}

const escapeHtml = (s: string) =>
  s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

const stripTags = (s: string) => s.replace(/<[^>]*>/g, "");

const slugify = (s: string) =>
  s
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s-]/gu, "")
    .trim()
    .replace(/\s+/g, "-");

function render(md: string): { html: string; headings: DocHeading[] } {
  const headings: DocHeading[] = [];
  const seen = new Map<string, number>();
  const marked = new Marked({ gfm: true });
  marked.use({
    renderer: {
      heading({ tokens, depth }: Tokens.Heading) {
        const inner = this.parser.parseInline(tokens);
        const text = stripTags(inner);
        let id = slugify(text) || `section-${headings.length + 1}`;
        const n = seen.get(id) ?? 0;
        seen.set(id, n + 1);
        if (n > 0) id = `${id}-${n + 1}`;
        if (depth === 2 || depth === 3) headings.push({ id, text, depth });
        return `<h${depth} id="${id}"><a class="docs-anchor" href="#${id}" aria-label="Link to ${escapeHtml(text)}">#</a>${inner}</h${depth}>\n`;
      },
      code({ text, lang }: Tokens.Code) {
        const language = (lang || "").split(/\s+/)[0];
        return `<figure class="docs-code" data-lang="${escapeHtml(language)}"><pre><code class="language-${escapeHtml(language)}">${escapeHtml(text)}</code></pre></figure>\n`;
      },
      table({ header, rows }: Tokens.Table) {
        // Wrapped so wide reference tables scroll instead of breaking the layout.
        const cell = (c: Tokens.TableCell, tag: "th" | "td") =>
          `<${tag}${c.align ? ` style="text-align:${c.align}"` : ""}>${this.parser.parseInline(c.tokens)}</${tag}>`;
        const head = `<tr>${header.map((c) => cell(c, "th")).join("")}</tr>`;
        const body = rows.map((r) => `<tr>${r.map((c) => cell(c, "td")).join("")}</tr>`).join("");
        return `<div class="docs-table"><table><thead>${head}</thead><tbody>${body}</tbody></table></div>\n`;
      },
    },
  });
  const html = marked.parse(md, { async: false }) as string;
  return { html, headings };
}

export function getDocSlugs(): string[] {
  return fs
    .readdirSync(DOCS_DIR)
    .filter((f) => f.endsWith(".md"))
    .map((f) => f.replace(/\.md$/, ""));
}

export function getDoc(slug: string): Doc | null {
  const file = path.join(DOCS_DIR, `${slug}.md`);
  if (!/^[a-z0-9-]+$/.test(slug) || !fs.existsSync(file)) return null;
  const raw = fs.readFileSync(file, "utf8");
  const { data, body } = parseFrontmatter(raw);
  const { html, headings } = render(body);
  const group = DOCS_NAV.find((g) => g.slugs.includes(slug))?.title ?? "";
  return {
    slug,
    title: data.title ?? slug,
    description: data.description ?? "",
    group,
    html,
    headings,
    updated: fs.statSync(file).mtime.toISOString().slice(0, 10),
  };
}

export type NavGroup = { title: string; items: DocMeta[] };

// getDocsNav lists every page in the order the manifest gives, grouped;
// pages missing from the manifest land in a trailing "More" group so a new
// file is never invisible.
export function getDocsNav(): NavGroup[] {
  const all = new Map<string, DocMeta>();
  for (const slug of getDocSlugs()) {
    const d = getDoc(slug);
    if (d) all.set(slug, { slug, title: d.title, description: d.description, group: d.group });
  }
  const groups: NavGroup[] = DOCS_NAV.map((g) => ({
    title: g.title,
    items: g.slugs.map((s) => all.get(s)).filter((d): d is DocMeta => Boolean(d)),
  }));
  const listed = new Set(DOCS_NAV.flatMap((g) => g.slugs));
  const rest = [...all.values()].filter((d) => !listed.has(d.slug));
  if (rest.length) groups.push({ title: "More", items: rest });
  return groups.filter((g) => g.items.length);
}

// Flat reading order, for prev / next links.
export function getDocsOrder(): DocMeta[] {
  return getDocsNav().flatMap((g) => g.items);
}

// Search index: title + headings per page, shipped to the sidebar filter.
export function getDocsIndex(): { slug: string; title: string; headings: string[] }[] {
  return getDocsOrder().map((d) => {
    const doc = getDoc(d.slug);
    return { slug: d.slug, title: d.title, headings: doc?.headings.map((h) => h.text) ?? [] };
  });
}
