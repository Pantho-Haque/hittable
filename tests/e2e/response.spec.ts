import { test, expect, API, blankRequest, urlInput, send, response, statusBadge, clipboardText } from "./fixtures/test";

const view = (page: import("@playwright/test").Page, name: "JSON tree view" | "Raw text view" | "HTML preview") =>
  response(page).getByRole("button", { name });

test.describe("response viewer", () => {
  test("before any send it invites you to send", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await expect(response(page).getByText("Send a request to see the response")).toBeVisible();
  });

  test("JSON responses open as a collapsible tree", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/users`);
    await send(page);
    const r = response(page);
    await expect(r.getByText('"Ada Lovelace"')).toBeVisible();
    await expect(r.getByText('"math"')).toBeHidden(); // third level starts collapsed
    await expect(r.getByText("2 items").first()).toBeVisible();
    // Collapse the root array, then open it again.
    await r.getByRole("button", { name: "Collapse" }).first().click();
    await expect(r.getByText("3 items")).toBeVisible();
    await expect(r.getByText('"Ada Lovelace"')).toBeHidden();
    await r.getByRole("button", { name: "Expand" }).first().click();
    await expect(r.getByText('"Ada Lovelace"')).toBeVisible();
  });

  test("text responses show raw text with whitespace kept", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/text`);
    await send(page);
    const pre = response(page).locator("pre");
    await expect(pre).toHaveText("plain text line one\n  indented line two\nhello world");
    await expect(pre).toHaveCSS("white-space", "pre-wrap");
  });

  test("HTML responses render in a preview beside the source", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/html`);
    await send(page);
    const frame = page.frameLocator('iframe[title="HTML response preview"]');
    await expect(frame.getByRole("heading", { name: "Hello from the fixture" })).toBeVisible();
    await expect(response(page).getByText("Source", { exact: true })).toBeVisible();
    await expect(response(page).getByText("<h1", { exact: false }).first()).toBeVisible();
    // Relative links resolve against the request URL.
    await expect(frame.locator("base")).toHaveAttribute("href", `${API}/`);
  });

  test("Inspect picks an element in the preview and shows its tag", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/html`);
    await send(page);
    const frame = page.frameLocator('iframe[title="HTML response preview"]');
    await expect(frame.getByRole("heading")).toBeVisible();
    await response(page).getByRole("button", { name: "Inspect elements" }).click();
    // Click where the heading is drawn, as a user would: in inspect mode a layer
    // over the preview takes the click and finds the element underneath.
    const box = (await frame.getByRole("heading").boundingBox())!;
    await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await expect(response(page).getByText(/Line \d+/)).toBeVisible();
    await expect(response(page).getByText("greeting").first()).toBeVisible();
    await response(page).getByRole("button", { name: "Exit inspect mode" }).click();
  });

  test("the view can be overridden until the next response", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/users/1`);
    await send(page);
    await expect(response(page).getByText('"Ada Lovelace"')).toBeVisible();
    await view(page, "Raw text view").click();
    await expect(response(page).locator("pre")).toContainText('"name": "Ada Lovelace"');
    await view(page, "HTML preview").click();
    await expect(page.locator('iframe[title="HTML response preview"]')).toBeVisible();
    await view(page, "JSON tree view").click();
    await expect(response(page).getByText('"Ada Lovelace"')).toBeVisible();

    await view(page, "Raw text view").click();
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    // A new response goes back to the detected view.
    await expect(response(page).locator("pre")).toHaveCount(0);
    await expect(response(page).getByText('"Ada Lovelace"')).toBeVisible();
  });

  test("the headers tab lists, counts, filters and copies headers", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/users`);
    await send(page);
    const tab = response(page).getByRole("button", { name: /^headers\s*\(\d+\)$/ });
    await expect(tab).toBeVisible();
    await tab.click();
    await expect(response(page).getByText("x-total-count")).toBeVisible();
    await expect(response(page).getByText("application/json; charset=utf-8")).toBeVisible();

    await response(page).getByRole("button", { name: "Copy all headers" }).click();
    expect(JSON.parse(await clipboardText(page))).toMatchObject({ "x-total-count": "3" });

    const row = response(page).locator("div.group", { hasText: "x-total-count" });
    await row.hover();
    await row.getByRole("button", { name: "Copy x-total-count" }).click();
    expect(await clipboardText(page)).toBe("x-total-count: 3");

    await page.keyboard.press("ControlOrMeta+f");
    await page.getByPlaceholder("Search keys & values…").fill("total");
    await expect(response(page).getByText("x-total-count")).toBeVisible();
    await expect(response(page).getByText("content-type")).toBeHidden();
    await page.getByPlaceholder("Search keys & values…").fill("no-such-header");
    await expect(response(page).getByText('No headers matching "no-such-header"')).toBeVisible();
  });

  test("Set-Cookie headers are shown", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/cookie`);
    await send(page);
    await response(page).getByRole("button", { name: /^headers/ }).click();
    await expect(response(page).getByText("set-cookie")).toBeVisible();
    await expect(response(page).getByText(/session=abc123; Path=\/; HttpOnly/)).toBeVisible();
    await expect(response(page).getByText(/theme=dark/)).toBeVisible();
  });

  test("search counts matches, steps through them and expands to show them", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/users`);
    await send(page);
    await expect(response(page).getByText('"Ada Lovelace"')).toBeVisible();
    await response(page).getByRole("button", { name: "Search (⌘F)" }).click();
    const box = page.getByPlaceholder("Search keys & values…");
    await expect(box).toBeFocused();
    await box.fill("example.com");
    await expect(response(page).getByText("1/3")).toBeVisible();
    await box.press("Enter");
    await expect(response(page).getByText("2/3")).toBeVisible();
    await box.press("Shift+Enter");
    await expect(response(page).getByText("1/3")).toBeVisible();
    // A match in a collapsed node opens it.
    await box.fill("cobol");
    await expect(response(page).getByText("1/1")).toBeVisible();
    await expect(response(page).getByText('"cobol"')).toBeVisible();
    await box.fill("nothing-matches");
    await expect(response(page).getByText("0/0")).toBeVisible();
    await box.press("Escape");
    await expect(box).toBeHidden();
  });

  test("Copy puts the JSON body on the clipboard", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/users/2`);
    await send(page);
    await response(page).getByTitle("Copy JSON").click();
    await expect(response(page).getByTitle("Copy JSON")).toHaveText("Copied");
    expect(JSON.parse(await clipboardText(page))).toMatchObject({ id: 2, name: "Alan Turing" });
  });

  test("a large response renders and reports its size in KB", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/big`);
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText(/^\d+\.\d KB$/)).toBeVisible();
    await expect(response(page).getByText("3 keys")).toHaveCount(2000);
  });
});
