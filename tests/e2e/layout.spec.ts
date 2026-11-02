import { test, expect, API, openRequest, FOLDER_ONLY, urlInput, send, response } from "./fixtures/test";
import type { Locator, Page } from "@playwright/test";
import { openFile, openWorkspace } from "./fixtures/directory";

const box = async (l: Locator) => (await l.boundingBox())!;
const separator = (page: Page, name: string) => page.getByRole("separator", { name });

/** Drag a separator by (dx, dy) with the mouse, as a user would. */
async function drag(page: Page, handle: Locator, dx: number, dy: number) {
  const b = await box(handle);
  const x = b.x + b.width / 2;
  const y = b.y + b.height / 2;
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x + dx / 2, y + dy / 2, { steps: 4 });
  await page.mouse.move(x + dx, y + dy, { steps: 4 });
  await page.mouse.up();
}

test.describe("adjustable layout", () => {
  test.skip(({ browserName }) => browserName !== "chromium", FOLDER_ONLY);
  test.skip(({ isMobile }) => isMobile, "drag layouts are for pointer screens; phones stack");

  test("request and response: drag to resize, put side by side, collapse and restore; all remembered", async ({ page }) => {
    await openRequest(page);
    const split = separator(page, "Resize Request and Response");
    await expect(split).toHaveAttribute("aria-orientation", "horizontal"); // stacked by default

    const request = page.locator("#req-tabpanel");
    const before = (await box(request)).height;
    await drag(page, split, 0, 120);
    await expect.poll(async () => (await box(request)).height).toBeGreaterThan(before + 80);

    // Side by side.
    await page.getByRole("button", { name: "Show the response beside the request" }).click();
    await expect(split).toHaveAttribute("aria-orientation", "vertical");
    const req = await box(request);
    const res = await box(response(page));
    expect(res.x).toBeGreaterThan(req.x + req.width - 1);

    // Keyboard: Enter collapses the request, the restore bar brings it back.
    await split.focus();
    await page.keyboard.press("Enter");
    await expect(request).toBeHidden();
    await page.getByRole("button", { name: "Show Request" }).click();
    await expect(request).toBeVisible();

    // A reload keeps the layout.
    const width = (await box(request)).width;
    await page.reload();
    await openWorkspace(page);
    await openFile(page, "hittable/req.hit");
    await expect(separator(page, "Resize Request and Response")).toHaveAttribute("aria-orientation", "vertical");
    await expect.poll(async () => Math.round((await box(page.locator("#req-tabpanel"))).width)).toBe(Math.round(width));
  });

  test("the explorer resizes by drag, collapses past its minimum, and Ctrl/Cmd+B brings it back", async ({ page }) => {
    await openRequest(page);
    const explorer = page.locator("#directory-explorer");
    const split = separator(page, "Resize Explorer and Editor");
    const start = (await box(explorer)).width;
    await drag(page, split, 100, 0);
    await expect.poll(async () => (await box(explorer)).width).toBeGreaterThan(start + 80);
    await expect(page.getByRole("button", { name: "Explorer", exact: true })).toHaveAttribute("aria-expanded", "true");

    // Dragged nearly shut: it collapses, and the toolbar toggle knows.
    await drag(page, split, -((await box(explorer)).width - 20), 0);
    await expect(explorer).toBeHidden();
    await expect(page.getByRole("button", { name: "Explorer", exact: true })).toHaveAttribute("aria-expanded", "false");
    await page.keyboard.press("ControlOrMeta+b");
    await expect(explorer).toBeVisible();
  });

  test("HTML preview and source: resize over the live frame, put side by side, collapse the preview", async ({ page }) => {
    await openRequest(page);
    await urlInput(page).fill(`${API}/html`);
    await send(page);
    const frame = page.locator('iframe[title="HTML response preview"]');
    await expect(page.frameLocator('iframe[title="HTML response preview"]').getByRole("heading")).toBeVisible();
    const split = separator(page, "Resize Preview and Source");

    const before = (await box(frame)).height;
    await drag(page, split, 0, 60);
    // It stops where the source reaches its minimum height, so expect growth, not the full 60px.
    await expect.poll(async () => (await box(frame)).height).toBeGreaterThan(before + 20);
    // Dragging back up crosses the live frame and still tracks: the frame does not swallow the pointer.
    const grown = (await box(frame)).height;
    await drag(page, split, 0, -50);
    await expect.poll(async () => (await box(frame)).height).toBeLessThan(grown - 20);

    await response(page).getByRole("button", { name: "Show the source beside the preview" }).click();
    await expect(split).toHaveAttribute("aria-orientation", "vertical");

    await split.focus();
    await page.keyboard.press("Enter");
    await expect(frame).toBeHidden();
    await response(page).getByRole("button", { name: "Show Preview" }).click();
    await expect(frame).toBeVisible();
    // The preview kept its page (not reloaded into a blank frame).
    await expect(page.frameLocator('iframe[title="HTML response preview"]').getByRole("heading")).toBeVisible();
  });

  test("the source scrolls as one block: no scrollbar per line", async ({ page }) => {
    await openRequest(page);
    await urlInput(page).fill(`${API}/html`);
    await send(page);
    const source = response(page).getByRole("textbox", { name: "Response HTML source" });
    await expect(source).toBeVisible();
    expect(await response(page).locator(".cm-scroller").count()).toBe(1);
    // Every line is laid out without its own overflow.
    const scrollers = await source.locator(".cm-line").evaluateAll((lines) => lines.filter((l) => getComputedStyle(l).overflowX !== "visible").length);
    expect(scrollers).toBe(0);
  });
});
