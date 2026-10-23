import { test, expect, API, blankRequest, urlInput, send, response, statusBadge } from "./fixtures/test";

type Flags = { __xss?: number; __htmlScriptRan?: boolean; __htmlOnerror?: boolean };
const flags = (page: import("@playwright/test").Page) =>
  page.evaluate(() => {
    const w = window as unknown as Flags;
    return { xss: w.__xss, script: w.__htmlScriptRan, onerror: w.__htmlOnerror };
  });

test.describe("untrusted responses never run code", () => {
  test("HTML inside a JSON response renders as text in the tree", async ({ page, seed, dialogs }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/xss`);
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    // As a value and as a key.
    await expect(response(page).getByText('"<img src=x onerror="window.__xss=1;alert(1)">"')).toHaveCount(2);
    await expect(response(page).getByText('"<script>window.__xss=2</script>"')).toBeVisible();
    await expect(response(page).locator("img")).toHaveCount(0);
    await expect(response(page).locator("script")).toHaveCount(0);
    // Search highlighting must not turn the text into markup either.
    await page.keyboard.press("ControlOrMeta+f");
    await page.getByPlaceholder("Search keys & values…").fill("onerror");
    await expect(response(page).locator("mark").first()).toBeVisible();
    await expect(response(page).locator("img")).toHaveCount(0);
    expect(await flags(page)).toEqual({});
    expect(dialogs).toEqual([]);
  });

  test("the same payload in raw text and as a forced HTML preview stays inert", async ({ page, seed, dialogs }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/xss-text`);
    await send(page);
    await expect(response(page).locator("pre")).toHaveText('before <img src=x onerror="window.__xss=1;alert(1)"> after');
    await response(page).getByRole("button", { name: "HTML preview" }).click();
    await expect(page.locator('iframe[title="HTML response preview"]')).toBeVisible();
    await response(page).getByRole("button", { name: "JSON tree view" }).click();
    await response(page).getByRole("button", { name: "Raw text view" }).click();
    await expect(response(page).locator("img")).toHaveCount(0);
    expect(await flags(page)).toEqual({});
    expect(dialogs).toEqual([]);
  });

  test("the HTML preview is sandboxed: the page's scripts and handlers never run", async ({ page, seed, dialogs }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/html`);
    await send(page);
    const iframe = page.locator('iframe[title="HTML response preview"]');
    await expect(page.frameLocator('iframe[title="HTML response preview"]').getByRole("heading")).toBeVisible();
    const sandbox = await iframe.getAttribute("sandbox");
    expect(sandbox).not.toBeNull();
    expect(sandbox).not.toContain("allow-scripts");
    const inner = await iframe.evaluate((el: HTMLIFrameElement) => (el.contentWindow as unknown as { __inner?: boolean }).__inner);
    expect(inner).toBeUndefined();
    expect(await flags(page)).toEqual({});
    expect(dialogs).toEqual([]);
  });
});

test.describe("proxy API input checks", () => {
  test("rejects a body that is not a JSON object", async ({ request }) => {
    for (const data of ["not json", "[1,2]", "null"]) {
      const res = await request.post("/api/proxy", { data, headers: { "content-type": "application/json" } });
      expect(res.status()).toBe(400);
      expect(await res.json()).toMatchObject({ ok: false, status: 0, error: expect.stringContaining("Invalid request body") });
    }
  });

  test("rejects forbidden methods and malformed headers", async ({ request }) => {
    let res = await request.post("/api/proxy", { data: { url: `${API}/echo`, method: "TRACE" } });
    expect(res.status()).toBe(400);
    expect((await res.json()).error).toContain("not allowed");
    res = await request.post("/api/proxy", { data: { url: `${API}/echo`, method: "GET", headers: "[1]" } });
    expect(res.status()).toBe(400);
    expect((await res.json()).error).toBe("Headers must be a JSON object");
  });

  test("forwards a normal request with the documented reply shape", async ({ request }) => {
    const res = await request.post("/api/proxy", {
      data: { url: `${API}/cookie`, method: "GET", headers: '{"X-A":"1"}', body: "" },
    });
    expect(res.status()).toBe(200);
    expect(res.headers()["cache-control"]).toBe("no-store");
    const json = await res.json();
    expect(json).toMatchObject({ status: 200, ok: true, data: { cookies: "set" }, cookies: { session: "abc123", theme: "dark" } });
    expect(json.headers["set-cookie"]).toContain("session=abc123");
  });

  test("hop-by-hop headers from the client are not forwarded", async ({ request }) => {
    const res = await request.post("/api/proxy", {
      data: { url: `${API}/echo`, method: "GET", headers: { "X-Keep": "yes", Connection: "close", "Proxy-Authorization": "secret" } },
    });
    const json = await res.json();
    expect(json.data.headers["x-keep"]).toBe("yes");
    expect(json.data.headers["proxy-authorization"]).toBeUndefined();
  });
});
