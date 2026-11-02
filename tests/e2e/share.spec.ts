import AxeBuilder from "@axe-core/playwright";
import type { Browser, Page } from "@playwright/test";
import { encodeShare } from "@/utils/shareLink";
import { test, expect, API, openRequest, urlInput, methodSelect, clipboardText, dialog, fillTab, editor, FOLDER_ONLY } from "./fixtures/test";
import { readFile, node, expand, seedPrivateFolder } from "./fixtures/directory";

// FR-15: a request travels in the URL fragment; opening the link previews it
// and never sends it. Importing writes hittable/shared/<name>.hit into the
// open workspace.

const shared = {
  method: "POST",
  url: `${API}/users?team=core`,
  headers: JSON.stringify({ Authorization: "Bearer <<TOKEN>>" }),
  params: JSON.stringify({ team: "core" }),
  body: '{"name": "Zoë"}',
};

/** A share link made the way the app makes one, so engines without folder mode can open it too. */
function linkFragment() {
  const res = encodeShare(shared);
  if (!res.ok) throw new Error(res.error);
  return res.fragment;
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

async function expectPreview(p: Page) {
  const d = dialog(p, "Shared request");
  await expect(d).toBeVisible();
  await expect(d.getByText("POST", { exact: true })).toBeVisible();
  await expect(d.getByText(`${API}/users?team=core`)).toBeVisible();
  await expect(d.getByText("Bearer <<TOKEN>>")).toBeVisible();
  await expect(d.getByText('{"name": "Zoë"}')).toBeVisible();
  await expect(d.getByText(/Nothing has been sent/)).toBeVisible();
  return d;
}

test.describe("making a share link", () => {
  test.skip(({ browserName }) => browserName !== "chromium", FOLDER_ONLY);

  test("Copy as → Share link hides the token, and the link previews the request without sending it", async ({ page, browser }) => {
    await openRequest(page, { method: "POST", url: `${API}/users?team=core`, headers: { Authorization: "Bearer abc" }, env: { TOKEN: "env-secret" } });
    await fillTab(page, "body", '{"name": "Zoë"}');
    await page.getByRole("button", { name: /Copy as/ }).click();
    await page.getByRole("menuitem", { name: "Share link" }).click();
    await expect(page.getByText("Share link copied")).toBeVisible();
    await expect(page.getByText(/Authorization was replaced with Bearer <<TOKEN>>/)).toBeVisible();
    const link = await clipboardText(page);
    expect(link).toMatch(/\/hittable#share=[A-Za-z0-9_-]+$/);
    expect(link).not.toContain("abc");
    expect(link).not.toContain("env-secret");

    const { p, sent, close } = await freshPage(browser);
    await p.goto(link);
    await expectPreview(p);
    expect(sent).toEqual([]);
    expect(await p.evaluate(() => (window as unknown as { __sent?: boolean }).__sent ?? false)).toBe(false);
    await close();
  });
});

test.describe("opening a share link", () => {
  test("without a workspace it previews the request, sends nothing and says how to import it", async ({ browser }) => {
    const { p, sent, close } = await freshPage(browser);
    await p.goto(`/hittable#${linkFragment()}`);
    const d = await expectPreview(p);
    await expect(d.getByText(/open a local folder or a GitHub repository — this preview comes back there/)).toBeVisible();
    await expect(d.getByRole("button", { name: "Import" })).toHaveCount(0);
    // Chrome/Edge can open a folder from right here; WebKit has no folder access.
    await expect(d.getByRole("button", { name: "Open a local folder" })).toHaveCount(
      (await p.evaluate(() => "showDirectoryPicker" in window)) ? 1 : 0,
    );
    await d.getByRole("button", { name: "Dismiss" }).click();
    await expect(d).toBeHidden();
    expect(new URL(p.url()).hash).toBe("");
    await expect(p.getByRole("heading", { name: "Open a workspace" })).toBeVisible();
    expect(sent).toEqual([]);
    await close();
  });

  // A link arrives on the launcher with nothing open. Opening a folder from
  // the launcher afterwards brings the preview back there, ready to import.
  test("the link comes back once a folder is open, and imports there", async ({ page, browserName }) => {
    test.skip(browserName !== "chromium", "File System Access API (Chromium only)");
    await seedPrivateFolder(page, { "hittable/env.json": "{}" });
    await page.goto(`/hittable#${linkFragment()}`);
    const d = await expectPreview(page);
    await d.getByRole("button", { name: "Dismiss" }).click();
    // Dismiss forgets it; open the link again, then open the folder.
    await page.goto(`/hittable#${linkFragment()}`);
    await (await expectPreview(page)).getByRole("button", { name: "Open a local folder" }).click();
    await expect(page).toHaveURL(/\/hittable\/folder/);
    const back = await expectPreview(page);
    await back.getByRole("button", { name: "Import" }).click();
    // It opens in the Runner with the shared request.
    await expect(urlInput(page)).toHaveValue(shared.url);
  });

  test("a damaged link shows an error, not a crash", async ({ page }) => {
    await page.goto("/hittable#share=eJzLSM3JyVcozy_KSQEAGgQEXQ-broken");
    const d = dialog(page, "Can't open share link");
    await expect(d.getByText(/damaged or too large/)).toBeVisible();
    await d.getByRole("button", { name: "Dismiss" }).click();
    await expect(d).toBeHidden();
    expect(new URL(page.url()).hash).toBe("");
    await expect(page.getByRole("heading", { name: "Open a workspace" })).toBeVisible(); // the app is still there
  });

  test("the open preview has no serious accessibility violations", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto(`/hittable#${linkFragment()}`);
    await expect(dialog(page, "Shared request")).toBeVisible();
    const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
    const blocking = results.violations
      .filter((v) => v.impact === "serious" || v.impact === "critical")
      .map((v) => `${v.id}: ${v.help} (${v.nodes.length}× e.g. ${v.nodes[0]?.target.join(" ")})`);
    expect(blocking).toEqual([]);
  });

  test("the preview fits a 390×844 phone @mobile", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/hittable#${linkFragment()}`);
    const d = dialog(page, "Shared request");
    await expect(d).toBeVisible();
    const box = (await d.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(390);
    expect(box.y + box.height).toBeLessThanOrEqual(844);
    expect(await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)).toBeLessThanOrEqual(0);
    await expect(d.getByRole("button", { name: "Dismiss" })).toBeInViewport();
  });
});

test.describe("importing a share link into a workspace", () => {
  test.skip(({ browserName }) => browserName !== "chromium", FOLDER_ONLY);

  // The link arrives while the workspace is open (same document), the only
  // way the preview sees an open workspace today.
  const arrive = (page: Page) => page.evaluate((f) => (window.location.hash = f), linkFragment());

  test("Import writes hittable/shared/<name>.hit, opens it, and a second import gets its own name", async ({ page }) => {
    await openRequest(page, { url: `${API}/users/1` });
    await arrive(page);
    let d = await expectPreview(page);
    await d.getByRole("button", { name: "Import" }).click();
    await expect(d).toBeHidden();
    expect(new URL(page.url()).hash).toBe("");
    await expect(page.getByText("Imported into hittable/shared")).toBeVisible();

    const file = "hittable/shared/POST-1-4100-users.hit";
    await expand(page, "hittable/shared");
    await expect(node(page, file)).toBeVisible();
    await expect(urlInput(page)).toHaveValue(`${API}/users?team=core`);
    await expect(methodSelect(page)).toHaveValue("POST");
    await page.getByRole("tab", { name: "Headers", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "headers").inputValue())).toEqual({ Authorization: "Bearer <<TOKEN>>" });
    expect(JSON.parse(await readFile(page, file))).toEqual({
      method: "POST",
      url: `${API}/users?team=core`,
      headers: { Authorization: "Bearer <<TOKEN>>" },
      params: { team: "core" },
      body: '{"name": "Zoë"}',
      response: null,
    });

    await arrive(page);
    d = await expectPreview(page);
    await d.getByRole("button", { name: "Import" }).click();
    await expect(node(page, "hittable/shared/POST-1-4100-users-2.hit")).toBeVisible();
    expect(JSON.parse(await readFile(page, "hittable/shared/POST-1-4100-users-2.hit")).method).toBe("POST");
  });

  test("Dismiss removes the fragment and leaves the workspace as it was", async ({ page }) => {
    await openRequest(page, { url: `${API}/users/1` });
    await arrive(page);
    const d = dialog(page, "Shared request");
    await d.getByRole("button", { name: "Dismiss" }).click();
    await expect(d).toBeHidden();
    expect(new URL(page.url()).hash).toBe("");
    await expect(urlInput(page)).toHaveValue(`${API}/users/1`);
    await expect(readFile(page, "hittable/shared/POST-1-4100-users.hit")).rejects.toThrow();
  });
});
