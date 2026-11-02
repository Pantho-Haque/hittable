import { test, expect, API, dialog, openRequest, response, send, statusBadge, urlInput, FOLDER_ONLY } from "./fixtures/test";
import { readFile, openFile } from "./fixtures/directory";

// Round 2: the production guard (FR-16) and git-safe secrets (FR-17), both
// driven by hittable/env.json in a workspace.
test.skip(({ browserName }) => browserName !== "chromium", FOLDER_ONLY);

const PROD = { HITTABLE_PRODUCTION: "true" };

test.describe("production guard (FR-16)", () => {
  test("a POST waits for confirmation: Cancel sends nothing, Send sends once", async ({ page }) => {
    await openRequest(page, { method: "POST", url: `${API}/echo`, headers: { "Content-Type": "application/json" }, body: '{"a":1}', env: PROD });
    await expect(page.getByText("PRODUCTION", { exact: true })).toBeVisible();
    let proxied = 0;
    page.on("request", (r) => r.url().endsWith("/api/proxy") && proxied++);

    await send(page);
    const confirm = dialog(page, "Send POST to production?");
    await expect(confirm).toContainText("[::1]:4100");
    await confirm.getByRole("button", { name: "Cancel" }).click();
    await expect(confirm).toBeHidden();
    expect(proxied).toBe(0);

    await urlInput(page).press("ControlOrMeta+Enter"); // the shortcut is guarded too
    await dialog(page, "Send POST to production?").getByRole("button", { name: "Send POST" }).click();
    await expect(statusBadge(page)).toHaveText("200 OK");
    expect(proxied).toBe(1);
  });

  test("a GET is not asked about", async ({ page }) => {
    await openRequest(page, { url: `${API}/users`, env: PROD });
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });

  test("marking env.json as production turns the guard on for the open request", async ({ page }) => {
    await openRequest(page, { method: "DELETE", url: `${API}/users/1` });
    await expect(page.getByText("PRODUCTION", { exact: true })).toHaveCount(0);
    await openFile(page, "hittable/env.json");
    const env = page.getByRole("textbox", { name: "Edit env.json" });
    await env.click();
    await page.keyboard.press("ControlOrMeta+a");
    await page.keyboard.insertText(JSON.stringify(PROD));
    await expect.poll(async () => JSON.parse(await readFile(page, "hittable/env.json"))).toEqual(PROD);
    await openFile(page, "hittable/req.hit");
    await expect(page.getByText("PRODUCTION", { exact: true })).toBeVisible();
    await send(page);
    await expect(dialog(page, "Send DELETE to production?")).toBeVisible();
  });
});

test.describe("git-safe secrets (FR-17)", () => {
  test("env.local.json overrides env.json and is listed in hittable/.gitignore", async ({ page }) => {
    await openRequest(page, {
      url: "<<BASE>>/echo",
      headers: { Authorization: "Bearer <<TOKEN>>" },
      env: { BASE: API, TOKEN: "placeholder" },
      envLocal: { TOKEN: "from-local" },
    });
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText('"Bearer from-local"')).toBeVisible();
    expect(await readFile(page, "hittable/.gitignore")).toBe("env.local.json\n");
  });
});
