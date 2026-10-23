import fs from "node:fs";
import path from "node:path";
import { test, expect, clipboardText } from "./fixtures/test";

// Ground truth comes from the markdown files themselves.
const DOCS_DIR = path.join(process.cwd(), "content", "docs");
const docs = fs
  .readdirSync(DOCS_DIR)
  .filter((f) => f.endsWith(".md"))
  .map((f) => {
    const slug = f.replace(/\.md$/, "");
    const raw = fs.readFileSync(path.join(DOCS_DIR, f), "utf8");
    const title = raw.match(/^title:\s*(.+)$/m)?.[1].trim() ?? slug;
    return { slug, title, url: slug === "index" ? "/docs" : `/docs/${slug}` };
  });

test.describe("docs", () => {
  for (const doc of docs) {
    test(`${doc.url} renders "${doc.title}" and marks itself in the sidebar`, async ({ page }) => {
      const res = await page.goto(doc.url);
      expect(res?.status()).toBe(200);
      await expect(page.getByRole("heading", { level: 1 })).toHaveText(doc.title);
      // Pages use "<title> | Hittable Docs"; the index keeps the section title.
      await expect(page).toHaveTitle(doc.slug === "index" ? "Docs | Hittable" : `${doc.title} | Hittable Docs`);
      const nav = page.getByRole("navigation", { name: "Documentation" }).last();
      await expect(nav.getByRole("link", { name: doc.title, exact: true })).toHaveAttribute("aria-current", "page");
      await expect(page.getByRole("link", { name: "Edit this page" })).toHaveAttribute(
        "href",
        `https://github.com/Pantho-Haque/hittable/edit/main/content/docs/${doc.slug}.md`,
      );
      // Every image on the page actually loads.
      for (const img of await page.locator("article img").all()) {
        await img.scrollIntoViewIfNeeded();
        await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.complete && el.naturalWidth > 0)).toBe(true);
      }
    });
  }

  test("the sidebar walks through every page in order", async ({ page }) => {
    await page.goto("/docs");
    const nav = page.getByRole("navigation", { name: "Documentation" }).last();
    const titles = await nav.getByRole("link").allTextContents();
    expect(titles.sort()).toEqual(docs.map((d) => d.title).sort());
    for (const title of titles) {
      await nav.getByRole("link", { name: title, exact: true }).click();
      await expect(page.getByRole("heading", { level: 1 })).toHaveText(title);
    }
  });

  test("previous and next links chain the pages", async ({ page }) => {
    await page.goto("/docs");
    await expect(page.getByRole("link", { name: /Previous/ })).toHaveCount(0);
    await page.getByRole("link", { name: /Next Getting started/ }).click();
    await expect(page).toHaveURL(/\/docs\/getting-started$/);
    await page.getByRole("link", { name: /Previous Hittable documentation/ }).click();
    await expect(page).toHaveURL(/\/docs$/);
  });

  test("no internal link or anchor in the docs is broken", async ({ page, request }) => {
    const targets = new Set<string>();
    for (const doc of docs) {
      await page.goto(doc.url);
      const hrefs = await page.locator("article a[href]").evaluateAll((as) => as.map((a) => a.getAttribute("href")!));
      for (const href of hrefs) {
        if (href.startsWith("#")) {
          await expect(page.locator(`[id="${href.slice(1)}"]`), `${doc.url} → ${href}`).toHaveCount(1);
        } else if (href.startsWith("/")) {
          targets.add(href);
        }
      }
    }
    expect(targets.size).toBeGreaterThan(5);
    for (const target of targets) {
      const [p, hash] = target.split("#");
      const res = await request.get(p);
      expect(res.status(), target).toBe(200);
      if (hash) expect(await res.text(), target).toContain(`id="${hash}"`);
    }
  });

  test("the table of contents jumps to a section", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/docs/web-app");
    const toc = page.getByRole("navigation", { name: "On this page" });
    await expect(toc).toBeVisible();
    await toc.getByRole("link", { name: "Keyboard shortcuts" }).click();
    await expect(page).toHaveURL(/#keyboard-shortcuts$/);
    await expect(page.locator("#keyboard-shortcuts")).toBeInViewport();
  });

  test("code blocks have a working copy button", async ({ page }) => {
    await page.goto("/docs/getting-started");
    const block = page.locator("figure.docs-code").first();
    const code = await block.locator("code").textContent();
    // Not located by name: the name changes to "Copied" on click.
    const button = block.getByRole("button");
    await expect(button).toHaveText("Copy");
    await button.click();
    await expect(button).toHaveText("Copied");
    expect(await clipboardText(page)).toBe(code);
    await expect(button).toHaveText("Copy");
  });

  test("search finds pages by title and by heading", async ({ page }) => {
    await page.goto("/docs");
    const nav = page.getByRole("navigation", { name: "Documentation" }).last();
    const search = nav.getByRole("searchbox");
    await search.fill("keyboard shortcuts");
    await expect(nav.getByRole("link", { name: /Web app.*Keyboard shortcuts/ })).toBeVisible();
    await search.fill("FAQ");
    await nav.getByRole("link", { name: /FAQ and troubleshooting/ }).click();
    await expect(page).toHaveURL(/\/docs\/faq$/);
    await search.fill("zzzz-no-such-thing");
    await expect(nav.getByText("No matches")).toBeVisible();
  });

  test("an unknown docs page is a 404", async ({ page, request }) => {
    const res = await page.goto("/docs/does-not-exist");
    expect(res?.status()).toBe(404);
    await expect(page.getByText("Page not found")).toBeVisible();
    expect((await request.get("/docs/..%2Fpackage")).status()).toBe(404);
  });

  test("docs menu works on a phone @mobile", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/docs");
    const toggle = page.getByRole("button", { name: "Docs menu" });
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-expanded", "true");
    await page.getByRole("navigation", { name: "Documentation" }).first().getByRole("link", { name: "FAQ and troubleshooting" }).click();
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("FAQ and troubleshooting");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  });
});
