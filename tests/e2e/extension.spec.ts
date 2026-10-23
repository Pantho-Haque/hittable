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
  collection,
  route,
  openRoute,
} from "./fixtures/test";

test.describe("without the browser extension", () => {
  test.use({ extension: false });

  test("the app explains how to install it and can be dismissed", async ({ page, seed }) => {
    await seed({ collections: [collection("Local", [route("r", `curl -X GET ${LOCAL_API}/users`)])] });
    await openRoute(page, "Local", "r");
    // The app pings for ~5 s before deciding the extension is missing.
    const modal = dialog(page, "No Extension Found");
    await expect(modal).toBeVisible({ timeout: 10_000 });
    await expect(modal.getByText(/install Hittable Extension/i)).toBeVisible();
    await expect(modal.getByText("chrome://extensions")).toBeVisible();
    await expect(modal.getByRole("button", { name: "Download" })).toBeVisible();
    await modal.getByRole("button", { name: "Close" }).click();
    await expect(modal).toBeHidden();
  });

  test("Download opens the extension release", async ({ page, seed, context }) => {
    await seed({ collections: [collection("Local", [route("r", `curl -X GET ${LOCAL_API}/users`)])] });
    await context.route("https://github.com/**", (r) => r.fulfill({ status: 200, body: "ok" }));
    await openRoute(page, "Local", "r");
    const modal = dialog(page, "No Extension Found");
    await expect(modal).toBeVisible({ timeout: 10_000 });
    const [popup] = await Promise.all([context.waitForEvent("page"), modal.getByRole("button", { name: "Download" }).click()]);
    expect(popup.url()).toBe("https://github.com/Pantho-Haque/hittable/releases/latest/download/hittable-extension.zip");
  });

  for (const host of ["localhost", "127.0.0.1", "0.0.0.0"]) {
    test(`a request to ${host} says to install the extension and never hits the proxy`, async ({ page, seed }) => {
      await seed({ collections: [collection("Local", [route("r", `curl -X GET http://${host}:4100/users`)])] });
      await openRoute(page, "Local", "r");
      const modal = dialog(page, "No Extension Found");
      await expect(modal).toBeVisible({ timeout: 10_000 });
      await modal.getByRole("button", { name: "Close" }).click();
      let proxied = false;
      page.on("request", (r) => r.url().endsWith("/api/proxy") && (proxied = true));
      await send(page);
      await expect(response(page).getByText("Install the Hittable Extension to make localhost requests")).toBeVisible();
      expect(proxied).toBe(false);
    });
  }

  test("remote URLs still work without it", async ({ page, seed }) => {
    await seed({ collections: [collection("Remote", [route("r", `curl -X GET ${API}/users/1`)])] });
    await openRoute(page, "Remote", "r");
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
  });
});

test.describe("with the browser extension", () => {
  test("localhost requests go through the extension, not the proxy", async ({ page, seed }) => {
    await seed({ collections: [collection("Local", [route("r", "")])] });
    await openRoute(page, "Local", "r");
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

  test("an extension failure is shown as an error", async ({ page, seed }) => {
    await seed({ collections: [collection("Local", [route("r", "curl -X GET http://localhost:1/nothing")])] });
    await openRoute(page, "Local", "r");
    await page.waitForFunction(() => window.__hittableExtension === true);
    await send(page);
    // Chrome: "Failed to fetch", Safari: "Load failed", Firefox: "NetworkError…".
    await expect(response(page).getByText(/Failed to fetch|Load failed|NetworkError/)).toBeVisible();
  });
});
