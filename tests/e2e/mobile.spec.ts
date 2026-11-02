import type { Page, Locator } from "@playwright/test";
import { test, expect, dialog, openLauncher } from "./fixtures/test";

test.skip(({ isMobile }) => !isMobile, "phone layout only");

const noHorizontalScroll = (page: Page) =>
  expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)).toBeLessThanOrEqual(0);

async function fitsViewport(page: Page, el: Locator) {
  const box = (await el.boundingBox())!;
  const vw = page.viewportSize()!.width;
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(vw);
}

// Phone browsers have no File System Access API (Android Chrome included;
// the Pixel project is desktop Chromium, so it is removed there).
const likeAPhone = (page: Page) =>
  page.addInitScript(() => {
    delete (Window.prototype as unknown as { showDirectoryPicker?: unknown }).showDirectoryPicker;
    delete (window as unknown as { showDirectoryPicker?: unknown }).showDirectoryPicker;
  });

test.describe("on a phone @mobile", () => {
  test("the landing page fits and opens the app", async ({ page }) => {
    await page.goto("/");
    await noHorizontalScroll(page);
    await page.getByRole("main").getByRole("link", { name: "Open a local folder" }).tap();
    await expect(page).toHaveURL(/\/hittable$/);
    await expect(page.getByRole("heading", { level: 1, name: "Open a workspace" })).toBeVisible();
    await noHorizontalScroll(page);
  });

  test("docs pages fit the screen", async ({ page }) => {
    for (const p of ["/docs", "/docs/web-app", "/docs/import-export", "/docs/shellapp-keyboard"]) {
      await page.goto(p);
      await noHorizontalScroll(page);
    }
  });

  test("the launcher explains that folders need a computer, and GitHub sign-in is in reach", async ({ page }) => {
    await likeAPhone(page);
    await openLauncher(page);
    const note = page.getByRole("note");
    await expect(note).toContainText("Chrome or Edge on a computer");
    await expect(note).toContainText("use a GitHub repository instead");
    await expect(page.getByRole("button", { name: "Open a local folder" })).toHaveCount(0);
    await expect(page.getByRole("banner").getByRole("link", { name: "Sign in" })).toBeVisible();
    await noHorizontalScroll(page);
  });

  test("the top bar fits, and its search button opens the palette", async ({ page }) => {
    await likeAPhone(page);
    await openLauncher(page);
    await page.getByRole("banner").getByRole("button", { name: "Search and commands" }).tap();
    const palette = dialog(page, "Command palette");
    await expect(palette).toBeVisible();
    await fitsViewport(page, palette);
    await noHorizontalScroll(page);
  });

  test("the old-collections banner fits and can be dismissed", async ({ page, seed }) => {
    await likeAPhone(page);
    await seed({ hittable: JSON.stringify([{ collectionName: "Phone", items: [] }]) });
    await openLauncher(page);
    const banner = page.getByRole("region", { name: /Your old browser collections/ });
    await expect(banner).toBeVisible();
    await fitsViewport(page, banner);
    await noHorizontalScroll(page);
    await banner.getByRole("button", { name: "Dismiss" }).tap();
    await expect(banner).toHaveCount(0);
  });
});
