import type { Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { test, expect, API, dialog, urlInput, methodSelect, openRequest, openLauncher, clipboardText, FOLDER_ONLY } from "./fixtures/test";
import { tree, node } from "./fixtures/directory";

// FR-1 command palette, FR-2 cheatsheet, NFR-3 keyboard access to the explorer.

const widget = JSON.stringify({ method: "POST", url: `${API}/widgets`, headers: {}, params: {}, body: '{"a":1}', response: null });
/** hittable/req.hit (GET /products) open, with hittable/admin/create-widget.hit beside it. */
const openShop = (page: Page) => openRequest(page, { url: `${API}/products`, files: { "hittable/admin/create-widget.hit": widget } });

const palette = (page: Page) => dialog(page, "Command palette");
const search = (page: Page) => palette(page).getByRole("combobox");

test.describe("command palette in a workspace", () => {
  test.skip(({ browserName }) => browserName !== "chromium", FOLDER_ONLY);

  test("Ctrl/Cmd+K from the URL bar, type part of a file name, Enter opens it", async ({ page }) => {
    await openShop(page);
    await urlInput(page).focus();
    await page.keyboard.press("ControlOrMeta+k");
    await expect(palette(page)).toBeVisible();
    await expect(search(page)).toBeFocused();

    await page.keyboard.type("widg");
    const first = palette(page).getByRole("option").first();
    await expect(first).toContainText("create-widget.hit");
    await expect(first).toContainText("hittable / admin");
    await page.keyboard.press("Enter");

    await expect(palette(page)).toBeHidden();
    await expect(urlInput(page)).toHaveValue(`${API}/widgets`);
    await expect(methodSelect(page)).toHaveValue("POST");
  });

  test('typing "env.json" then Enter opens the environment file', async ({ page }) => {
    await openShop(page);
    await page.keyboard.press("ControlOrMeta+k");
    await expect(search(page)).toBeFocused();
    await page.keyboard.type("env.json");
    await expect(palette(page).getByRole("option").first()).toContainText("env.json");
    await page.keyboard.press("Enter");
    await expect(page.getByRole("textbox", { name: "Edit env.json" })).toBeVisible();
  });

  test("Copy request as cURL copies the open request", async ({ page }) => {
    await openShop(page);
    await page.keyboard.press("ControlOrMeta+k");
    await page.keyboard.type("copy request as curl");
    await page.keyboard.press("Enter");
    await expect(page.getByText("Copied as cURL")).toBeVisible();
    expect(await clipboardText(page)).toMatch(new RegExp(`^curl -X GET\\b.*${API.replace(/[[\]]/g, "\\$&")}/products`));
  });

  test("Esc closes it and focus returns to where it was; the top bar button opens it too", async ({ page }) => {
    await openShop(page);
    await urlInput(page).focus();
    await page.keyboard.press("ControlOrMeta+k");
    await expect(palette(page)).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(palette(page)).toBeHidden();
    await expect(urlInput(page)).toBeFocused();

    const trigger = page.getByRole("button", { name: "Search and commands" });
    await trigger.click();
    await expect(palette(page)).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(trigger).toBeFocused();
  });
});

test.describe("command palette on the launcher", () => {
  test("action rows show their shortcut, arrows move the selection", async ({ page }) => {
    await openLauncher(page);
    await page.keyboard.press("ControlOrMeta+k");
    await expect(search(page)).toBeFocused();
    await page.keyboard.type("toggle explorer");
    const row = palette(page).getByRole("option", { name: /Toggle explorer/ });
    await expect(row).toHaveAttribute("aria-selected", "true");
    await expect(row.locator("kbd")).toHaveText([/Ctrl|⌘/, "B"]);
    await search(page).fill("");
    await page.keyboard.press("ArrowDown");
    const second = palette(page).getByRole("option").nth(1);
    await expect(second).toHaveAttribute("aria-selected", "true");
    await expect(search(page)).toHaveAttribute("aria-activedescendant", (await second.getAttribute("id"))!);
  });
});

test.describe("palette accessibility", () => {
  test("axe finds no serious problems in the open palette or the cheatsheet (colour contrast included)", async ({ page }) => {
    await openLauncher(page);
    await page.keyboard.press("ControlOrMeta+k");
    await expect(search(page)).toBeFocused();
    await page.keyboard.type("e");
    for (const name of ["Command palette", "Keyboard shortcuts"]) {
      if (name === "Keyboard shortcuts") {
        await page.keyboard.press("Escape");
        await page.locator("body").press("?");
      }
      await expect(dialog(page, name)).toBeVisible();
      const results = await new AxeBuilder({ page }).include('[role="dialog"]')
        .withTags(["wcag2a", "wcag2aa"]).analyze();
      const blocking = results.violations
        .filter((v) => v.impact === "serious" || v.impact === "critical")
        .map((v) => `${v.id}: ${v.nodes[0]?.target.join(" ")}`);
      expect(blocking, name).toEqual([]);
    }
  });
});

test.describe("keyboard shortcuts cheatsheet", () => {
  test("? opens it outside text fields, grouped by area, and Esc closes it", async ({ page }) => {
    await openLauncher(page);
    await page.locator("body").press("?");
    const sheet = dialog(page, "Keyboard shortcuts");
    await expect(sheet).toBeVisible();
    for (const group of ["General", "Request", "Response", "Explorer", "Editor"]) {
      await expect(sheet.getByRole("heading", { name: group })).toBeVisible();
    }
    await expect(sheet.getByText("Beautify JSON")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(sheet).toBeHidden();
  });

  test("? in a text field types a ?, Ctrl/Cmd+/ opens it", async ({ page, browserName }) => {
    test.skip(browserName !== "chromium", FOLDER_ONLY);
    await openShop(page);
    await urlInput(page).focus();
    await page.keyboard.press("End");
    await page.keyboard.type("?");
    await expect(urlInput(page)).toHaveValue(`${API}/products?`);
    await expect(dialog(page, "Keyboard shortcuts")).toBeHidden();
    await page.keyboard.press("ControlOrMeta+/");
    await expect(dialog(page, "Keyboard shortcuts")).toBeVisible();
  });
});

test.describe("keyboard-only explorer", () => {
  test.skip(({ browserName }) => browserName !== "chromium", FOLDER_ONLY);

  test("Tab reaches the file tree, arrows move through it and Enter opens a file", async ({ page }) => {
    await openShop(page);
    // Start from a fresh page with nothing selected, as someone arriving by keyboard would.
    await page.reload();
    await expect(tree(page)).toBeVisible();
    await page.getByRole("button", { name: "Refresh" }).focus();
    await page.keyboard.press("Tab");
    await expect(tree(page)).toBeFocused();
    // hittable/ is open by default: admin/, then its siblings.
    for (let i = 0; i < 10 && !(await tree(page).getAttribute("aria-activedescendant"))?.endsWith(encodeURIComponent("hittable/req.hit")); i++) {
      await page.keyboard.press("ArrowDown");
    }
    await expect(tree(page)).toHaveAttribute("aria-activedescendant", `file-${encodeURIComponent("hittable/req.hit")}`);
    await page.keyboard.press("Enter");
    await expect(urlInput(page)).toHaveValue(`${API}/products`);
    await expect(node(page, "hittable/req.hit")).toBeVisible();
  });
});

test.describe("on a phone @mobile", () => {
  test("the palette fits a 390×844 screen and runs an action", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await openLauncher(page);
    await page.getByRole("button", { name: "Search and commands" }).click();
    const d = palette(page);
    await expect(d).toBeVisible();
    const box = (await d.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(390);
    expect(box.y + box.height).toBeLessThanOrEqual(844);
    expect(await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)).toBeLessThanOrEqual(0);

    await search(page).fill("keyboard short");
    const option = d.getByRole("option", { name: /Keyboard shortcuts/ });
    expect((await option.boundingBox())!.height).toBeGreaterThanOrEqual(44);
    await option.click();
    await expect(dialog(page, "Keyboard shortcuts")).toBeVisible();
  });
});
