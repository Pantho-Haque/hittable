import type { Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import {
  test,
  expect,
  API,
  sidebar,
  dialog,
  urlInput,
  openRoute,
  collection,
  route,
  folder,
} from "./fixtures/test";

// FR-1 command palette, FR-2 cheatsheet, NFR-3 keyboard access to the sidebar.

const shop = () =>
  collection(
    "Shop",
    [
      route("list products", `curl -X GET ${API}/products`),
      folder("Admin", [route("create widget", `curl -X POST ${API}/widgets -d '{"a":1}'`)]),
    ],
    { BASE: API },
  );

const palette = (page: Page) => dialog(page, "Command palette");
const search = (page: Page) => palette(page).getByRole("combobox");

test.describe("command palette", () => {
  test.beforeEach(async ({ seed }) => {
    await seed({ collections: [shop()] });
  });

  test("Ctrl/Cmd+K from the URL bar, type part of a request name, Enter opens it", async ({ page }) => {
    await openRoute(page, "Shop", "list products");
    await urlInput(page).focus();
    await page.keyboard.press("ControlOrMeta+k");
    await expect(palette(page)).toBeVisible();
    await expect(search(page)).toBeFocused();

    await page.keyboard.type("widg");
    const first = palette(page).getByRole("option").first();
    await expect(first).toContainText("create widget");
    await expect(first).toContainText("POST");
    await expect(first).toContainText("Shop / Admin");
    await page.keyboard.press("Enter");

    await expect(palette(page)).toBeHidden();
    await expect(urlInput(page)).toHaveValue(`${API}/widgets`);
    await expect(page).toHaveURL(/r=create(\+|%20)widget/);
  });

  test('typing "env" then Enter opens the Environment dialog', async ({ page }) => {
    await openRoute(page, "Shop", "list products");
    await page.keyboard.press("ControlOrMeta+k");
    await expect(search(page)).toBeFocused();
    await page.keyboard.type("env");
    await expect(palette(page).getByRole("option").first()).toContainText("Environment variables");
    await page.keyboard.press("Enter");
    await expect(dialog(page, "Environment: Shop")).toBeVisible();
  });

  test("Esc closes it and focus returns to where it was; the top bar button opens it too", async ({ page }) => {
    await openRoute(page, "Shop", "list products");
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

  test("action rows show their shortcut, arrows move the selection", async ({ page }) => {
    await page.goto("/hittable");
    await page.keyboard.press("ControlOrMeta+k");
    await expect(search(page)).toBeFocused();
    await page.keyboard.type("toggle sidebar");
    const row = palette(page).getByRole("option", { name: /Toggle sidebar/ });
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
    await page.goto("/hittable");
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
    await page.goto("/hittable");
    await page.locator("body").press("?");
    const sheet = dialog(page, "Keyboard shortcuts");
    await expect(sheet).toBeVisible();
    for (const group of ["General", "Request", "Response", "Sidebar", "Editor"]) {
      await expect(sheet.getByRole("heading", { name: group })).toBeVisible();
    }
    await expect(sheet.getByText("Beautify JSON")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(sheet).toBeHidden();
  });

  test("? in a text field types a ?, Ctrl/Cmd+/ opens it", async ({ page, seed }) => {
    await seed({ collections: [shop()] });
    await openRoute(page, "Shop", "list products");
    await urlInput(page).focus();
    await page.keyboard.press("End");
    await page.keyboard.type("?");
    await expect(urlInput(page)).toHaveValue(`${API}/products?`);
    await expect(dialog(page, "Keyboard shortcuts")).toBeHidden();
    await page.keyboard.press("ControlOrMeta+/");
    await expect(dialog(page, "Keyboard shortcuts")).toBeVisible();
  });
});

test.describe("keyboard-only sidebar", () => {
  test("Tab reaches a request row and Enter opens it", async ({ page, seed, browserName }) => {
    // Safari's Tab skips buttons unless "Press Tab to highlight each item" is
    // on (Option+Tab otherwise); WebKit in Playwright follows that default.
    test.skip(browserName === "webkit", "Safari's default Tab skips buttons");
    await seed({ collections: [shop()], sidebarCollapsed: false });
    await page.goto("/hittable?c=Shop");
    const row = sidebar(page).getByRole("button", { name: "GET list products", exact: true });
    await sidebar(page).getByRole("button", { name: "New Folder" }).focus();
    for (let i = 0; i < 10 && !(await row.evaluate((el) => el === document.activeElement)); i++) {
      await page.keyboard.press("Tab");
    }
    await expect(row).toBeFocused();
    // The row's ⋮ menu shows while the row has focus, so it is reachable too.
    await expect(sidebar(page).getByRole("button", { name: "Actions for list products" })).toBeVisible();
    await page.keyboard.press("Enter");
    await expect(urlInput(page)).toHaveValue(`${API}/products`);
  });
});

test.describe("on a phone @mobile", () => {
  test("the palette fits a 390×844 screen and opens a request", async ({ page, seed }) => {
    await seed({ collections: [shop()] });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/hittable");
    await page.getByRole("button", { name: "Search and commands" }).click();
    const d = palette(page);
    await expect(d).toBeVisible();
    const box = (await d.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(390);
    expect(box.y + box.height).toBeLessThanOrEqual(844);
    expect(await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)).toBeLessThanOrEqual(0);

    await search(page).fill("list prod");
    const option = d.getByRole("option", { name: /list products/ });
    expect((await option.boundingBox())!.height).toBeGreaterThanOrEqual(44);
    await option.click();
    await expect(urlInput(page)).toHaveValue(`${API}/products`);
  });
});
