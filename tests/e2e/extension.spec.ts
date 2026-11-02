import {
  test,
  expect,
  LOCAL_API,
  API,
  dialog,
  urlInput,
  methodSelect,
  send,
  fillTab,
  response,
  statusBadge,
  openRequest,
  FOLDER_ONLY,
} from "./fixtures/test";

// The extension status lives in the Runner, which needs a folder workspace.
test.skip(({ browserName }) => browserName !== "chromium", FOLDER_ONLY);

test.describe("without the browser extension", () => {
  test.use({ extension: false });

  // FR-10: the "No Extension Found" dialog used to open by itself on every
  // visit, even for people who never call localhost. Now nothing pops up; a
  // status button explains on demand, and a localhost send says what to do.
  const extensionButton = (page: import("@playwright/test").Page) =>
    page.getByRole("button", { name: "Extension not installed" });

  test("nothing pops up; the status button explains how to install it", async ({ page }) => {
    await openRequest(page, { url: `${LOCAL_API}/users` });
    // The app pings for ~5 s before deciding the extension is missing.
    await expect(extensionButton(page)).toBeVisible({ timeout: 10_000 });
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await extensionButton(page).click();
    const modal = dialog(page, "Localhost extension");
    await expect(modal.getByText("chrome://extensions")).toBeVisible();
    await expect(modal.getByText(/Only needed for localhost/)).toBeVisible();
    await modal.getByRole("button", { name: "Close" }).click();
    await expect(modal).toBeHidden();
  });

  test("Download opens the extension release", async ({ page, context }) => {
    await context.route("https://github.com/**", (r) => r.fulfill({ status: 200, body: "ok" }));
    await openRequest(page, { url: `${LOCAL_API}/users` });
    await extensionButton(page).click({ timeout: 10_000 });
    const modal = dialog(page, "Localhost extension");
    const [popup] = await Promise.all([context.waitForEvent("page"), modal.getByRole("button", { name: "Download" }).click()]);
    expect(popup.url()).toBe("https://github.com/Pantho-Haque/hittable/releases/latest/download/hittable-extension.zip");
  });

  for (const host of ["localhost", "127.0.0.1", "0.0.0.0"]) {
    test(`a request to ${host} says to install the extension and never hits the proxy`, async ({ page }) => {
      await openRequest(page, { url: `http://${host}:4100/users` });
      await expect(extensionButton(page)).toBeVisible({ timeout: 10_000 });
      let proxied = false;
      page.on("request", (r) => r.url().endsWith("/api/proxy") && (proxied = true));
      await send(page);
      await expect(response(page).getByText("localhost needs the Hittable browser extension")).toBeVisible();
      await expect(response(page).getByRole("link", { name: "Install the extension" })).toBeVisible();
      expect(proxied).toBe(false);
    });
  }

  test("remote URLs still work without it", async ({ page }) => {
    await openRequest(page, { url: `${API}/users/1` });
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
  });
});

test.describe("with the browser extension", () => {
  test("localhost requests go through the extension, not the proxy", async ({ page }) => {
    await openRequest(page);
    await page.waitForFunction(() => window.__hittableExtension === true);
    let proxied = false;
    page.on("request", (r) => r.url().endsWith("/api/proxy") && (proxied = true));
    await methodSelect(page).selectOption("POST");
    await urlInput(page).fill(`${LOCAL_API}/echo?via=extension`);
    await fillTab(page, "headers", '{"Content-Type": "application/json", "X-Local": "1"}');
    await fillTab(page, "body", '{"local": true}');
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText('"extension"')).toBeVisible();
    await expect(response(page).getByText('"x-local"')).toBeVisible();
    expect(proxied).toBe(false);
    const sent = await page.evaluate(() => (window as unknown as { __extensionRequests: unknown[] }).__extensionRequests);
    expect(sent).toEqual([
      { url: `${LOCAL_API}/echo?via=extension`, method: "POST", headers: { "Content-Type": "application/json", "X-Local": "1" }, body: '{"local": true}' },
    ]);
    await expect(dialog(page, "No Extension Found")).toHaveCount(0);
  });

  test("an extension failure is shown as an error", async ({ page }) => {
    await openRequest(page, { url: "http://localhost:1/nothing" });
    await page.waitForFunction(() => window.__hittableExtension === true);
    await send(page);
    // Chrome: "Failed to fetch", Safari: "Load failed", Firefox: "NetworkError…".
    await expect(response(page).getByText(/Failed to fetch|Load failed|NetworkError/)).toBeVisible();
  });
});
