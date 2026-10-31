import AxeBuilder from "@axe-core/playwright";
import type { Browser, Page } from "@playwright/test";
import { test, expect, API, collection, route, openRoute, urlInput, methodSelect, clipboardText, dialog, sidebar, fillTab, editor } from "./fixtures/test";

// FR-15: a request travels in the URL fragment; opening the link previews it
// and never sends it.

async function copyShareLink(page: Page, seed: (d: { collections: ReturnType<typeof collection>[] }) => Promise<void>) {
  await seed({ collections: [collection("Share", [route("src", `curl -X POST ${API}/users?team=core -H "Authorization: Bearer abc"`)], { TOKEN: "env-secret" })] });
  await openRoute(page, "Share", "src");
  await fillTab(page, "body", '{"name": "Zoë"}');
  await page.getByRole("button", { name: /Copy as/ }).click();
  await page.getByRole("menuitem", { name: "Share link" }).click();
  await expect(page.getByText("Share link copied")).toBeVisible();
  await expect(page.getByText(/Authorization was replaced with Bearer <<TOKEN>>/)).toBeVisible();
  const link = await clipboardText(page);
  expect(link).toMatch(/\/hittable#share=[A-Za-z0-9_-]+$/);
  expect(link).not.toContain("abc");
  return link;
}

/** A new browser profile, as the person the link was sent to; records anything that would send. */
async function freshPage(browser: Browser) {
  const ctx = await browser.newContext();
  const p = await ctx.newPage();
  const sent: string[] = [];
  p.on("request", (r) => {
    if (r.url().includes("/api/proxy") || r.url().startsWith(API)) sent.push(r.url());
  });
  await p.addInitScript(() => {
    window.addEventListener("message", (e) => {
      if ((e.data as { type?: string })?.type === "HITTABLE_REQUEST") (window as unknown as { __sent: boolean }).__sent = true;
    });
  });
  return { p, sent, close: () => ctx.close() };
}

test.describe("share link", () => {
  test("round-trips a request, hides the token, sends nothing, and imports into Shared", async ({ page, seed, browser }) => {
    const link = await copyShareLink(page, seed);
    const { p, sent, close } = await freshPage(browser);
    await p.goto(link);
    const d = dialog(p, "Shared request");
    await expect(d).toBeVisible();
    await expect(d.getByText("POST", { exact: true })).toBeVisible();
    await expect(d.getByText(`${API}/users?team=core`)).toBeVisible();
    await expect(d.getByText("Bearer <<TOKEN>>")).toBeVisible();
    await expect(d.getByText('{"name": "Zoë"}')).toBeVisible();
    await expect(d.getByText(/Nothing has been sent/)).toBeVisible();

    await d.getByRole("button", { name: "Import" }).click();
    await expect(d).toBeHidden();
    await expect(urlInput(p)).toHaveValue(`${API}/users?team=core`);
    await expect(methodSelect(p)).toHaveValue("POST");
    await expect(p).toHaveURL(/\/hittable\?c=Shared&r=/);
    await p.getByRole("tab", { name: "Headers", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(p, "headers").inputValue())).toMatchObject({ Authorization: "Bearer <<TOKEN>>" });
    const stored = await p.evaluate(() => JSON.parse(localStorage.getItem("hittable")!));
    expect(stored.find((c: { collectionName: string }) => c.collectionName === "Shared")).toMatchObject({ env: {}, secrets: {} });

    expect(sent).toEqual([]);
    expect(await p.evaluate(() => (window as unknown as { __sent?: boolean }).__sent ?? false)).toBe(false);
    await close();
  });

  test("Dismiss removes the fragment and keeps the workspace as it was", async ({ page, seed }) => {
    const link = await copyShareLink(page, seed);
    await page.goto(link);
    const d = dialog(page, "Shared request");
    await d.getByRole("button", { name: "Dismiss" }).click();
    await expect(d).toBeHidden();
    expect(new URL(page.url()).hash).toBe("");
    const stored = await page.evaluate(() => JSON.parse(localStorage.getItem("hittable")!));
    expect(stored.map((c: { collectionName: string }) => c.collectionName)).toEqual(["Share"]);
  });

  test("a damaged link shows an error, not a crash", async ({ page }) => {
    await page.goto("/hittable#share=eJzLSM3JyVcozy_KSQEAGgQEXQ-broken");
    const d = dialog(page, "Can't open share link");
    await expect(d.getByText(/damaged or too large/)).toBeVisible();
    await d.getByRole("button", { name: "Dismiss" }).click();
    await expect(d).toBeHidden();
    expect(new URL(page.url()).hash).toBe("");
    await expect(sidebar(page)).toBeVisible(); // the app is still there
  });

  test("the open preview has no serious accessibility violations", async ({ page, seed }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    const link = await copyShareLink(page, seed);
    await page.goto(link);
    await expect(dialog(page, "Shared request")).toBeVisible();
    const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
    const blocking = results.violations
      .filter((v) => v.impact === "serious" || v.impact === "critical")
      .map((v) => `${v.id}: ${v.help} (${v.nodes.length}× e.g. ${v.nodes[0]?.target.join(" ")})`);
    expect(blocking).toEqual([]);
  });

  test("the preview fits a 390×844 phone @mobile", async ({ page, seed }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const link = await copyShareLink(page, seed);
    await page.goto(link);
    const d = dialog(page, "Shared request");
    await expect(d).toBeVisible();
    const box = (await d.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(390);
    expect(box.y + box.height).toBeLessThanOrEqual(844);
    expect(await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)).toBeLessThanOrEqual(0);
    await expect(d.getByRole("button", { name: "Import" })).toBeInViewport();
  });
});
