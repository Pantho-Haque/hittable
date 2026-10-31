import type { Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { test, expect, API, blankRequest, urlInput, send, response, statusBadge } from "./fixtures/test";

// FR-13 JSONPath filter and FR-14 Compare with the previous response.

const filterButton = (page: Page) => response(page).getByRole("button", { name: /Filter with JSONPath/ });
const filterInput = (page: Page) => response(page).getByRole("textbox", { name: "JSONPath filter" });
const compareButton = (page: Page) => response(page).getByRole("button", { name: "Compare with previous response" });
const compareView = (page: Page) => response(page).getByRole("region", { name: "Comparison with previous response" });

// Colour contrast stays on (NFR-1); only the response panel is scanned, the
// rest of the page has its own axe test.
async function blockingViolations(page: Page) {
  const results = await new AxeBuilder({ page })
    .include('section[aria-label="Response"]')
    .withTags(["wcag2a", "wcag2aa"])
    .analyze();
  return results.violations
    .filter((v) => v.impact === "serious" || v.impact === "critical")
    .map((v) => `${v.id}: ${v.help} (${v.nodes.length}× e.g. ${v.nodes[0]?.target.join(" ")})`);
}

async function sendTo(page: Page, path: string) {
  await urlInput(page).fill(`${API}${path}`);
  await send(page);
  await expect(statusBadge(page)).toHaveText("200 OK");
}

test.beforeEach(({ page }) => page.emulateMedia({ reducedMotion: "reduce" }));

test.describe("JSONPath filter", () => {
  test("filters the tree to the matches, with feedback for bad paths and no results", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await sendTo(page, "/users");
    const r = response(page);
    await filterButton(page).click();
    await expect(filterButton(page)).toHaveAttribute("aria-pressed", "true");

    await filterInput(page).fill("$[*].name");
    await expect(r.getByText('"Ada Lovelace"')).toBeVisible();
    await expect(r.getByText('"Grace Hopper"')).toBeVisible();
    await expect(r.getByText('"ada@example.com"')).toBeHidden();
    await expect(r.getByText("3 matches")).toBeVisible();
    await expect(filterButton(page)).toHaveAccessibleName("Filter with JSONPath (active)");
    expect(await blockingViolations(page)).toEqual([]);

    await filterInput(page).fill("$[");
    await expect(r.getByText(/^Invalid JSONPath: /)).toBeVisible();
    await expect(r.getByText('"ada@example.com"')).toBeVisible(); // the whole body again

    await filterInput(page).fill("$.nope");
    await expect(r.getByText("No results for $.nope")).toBeVisible();

    await filterInput(page).fill("$..tags[0]");
    await expect(r.getByText('"math"')).toBeVisible();
    await expect(r.getByText('"cobol"')).toBeVisible();

    // A new response clears it.
    await send(page);
    await expect(filterInput(page)).toHaveValue("");
    await expect(r.getByText('"ada@example.com"')).toBeVisible();
  });
});

test.describe("Compare with the previous response", () => {
  test("is disabled before there is an earlier response, then lists what changed", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await sendTo(page, "/users/1");
    await expect(compareButton(page)).toHaveAttribute("aria-disabled", "true");
    await expect(compareButton(page)).toHaveAttribute("title", "No earlier response for this request yet");

    // Same request (same route), different URL: still "the previous response".
    await urlInput(page).fill(`${API}/users/2`);
    await send(page);
    await expect(response(page).getByText('"Alan Turing"')).toBeVisible();
    await expect(compareButton(page)).toHaveAttribute("aria-disabled", "false");
    await compareButton(page).click();

    const view = compareView(page);
    await expect(view).toBeVisible();
    await expect(view.getByText(/\d+ changes · status 200/)).toBeVisible();
    const body = view.getByRole("list", { name: "Body changes" });
    for (const path of ["$['id']", "$['name']", "$['email']", "$['active']", "$['tags'][0]", "$['tags'][1]"]) {
      await expect(body.getByText(path, { exact: true })).toBeVisible();
    }
    await expect(body.getByText('"Ada Lovelace"')).toBeVisible();
    await expect(body.getByText('"Alan Turing"')).toBeVisible();
    await expect(body.getByText("removed").first()).toBeVisible();
    expect(await blockingViolations(page)).toEqual([]);

    await view.getByRole("textbox", { name: "Ignore paths" }).fill("$.tags, $.email");
    await expect(body.getByText("$['tags'][0]", { exact: true })).toBeHidden();
    await expect(body.getByText("$['email']", { exact: true })).toBeHidden();
    await expect(body.getByText("$['name']", { exact: true })).toBeVisible();

    await view.getByRole("button", { name: "Close compare" }).click();
    await expect(compareView(page)).toBeHidden();
    await expect(response(page).getByText('"Alan Turing"')).toBeVisible();
    await expect(compareButton(page)).toBeFocused();
  });

  test("two identical responses show no differences", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await sendTo(page, "/users/3");
    await send(page);
    await expect(compareButton(page)).toHaveAttribute("aria-disabled", "false");
    await compareButton(page).click();
    // The fixture server's Date header can tick between sends.
    await compareView(page).getByRole("textbox", { name: "Ignore paths" }).fill("date");
    await expect(compareView(page).getByText("No differences", { exact: true })).toBeVisible();
  });

  test("fits a phone screen", async ({ page, seed }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await blankRequest(page, seed);
    await sendTo(page, "/users/1");
    await urlInput(page).fill(`${API}/users/2`);
    await send(page);
    await expect(response(page).getByText('"Alan Turing"')).toBeVisible();
    await compareButton(page).click();
    const box = (await compareView(page).boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(390);
    const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    expect(scrollWidth).toBeLessThanOrEqual(390);
  });
});
