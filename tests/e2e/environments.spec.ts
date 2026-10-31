import {
  test,
  expect,
  API,
  collection,
  dialog,
  openRoute,
  response,
  route,
  send,
  statusBadge,
  urlInput,
} from "./fixtures/test";
import { seedPrivateFolder, readFile, node, expand, openWorkspace } from "./fixtures/directory";

// Round 2: the production guard (FR-16) and git-safe secrets (FR-17).

test.describe("production guard (FR-16)", () => {
  const prodShop = () =>
    collection(
      "Shop",
      [route("create", `curl -X POST ${API}/echo -H 'Content-Type: application/json' -d '{"a":1}'`), route("list", `curl -X GET ${API}/users`)],
      { HITTABLE_PRODUCTION: "true" },
    );

  test("a POST waits for confirmation: Cancel sends nothing, Send sends once", async ({ page, seed }) => {
    await seed({ collections: [prodShop()] });
    await openRoute(page, "Shop", "create");
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

  test("a GET is not asked about", async ({ page, seed }) => {
    await seed({ collections: [prodShop()] });
    await openRoute(page, "Shop", "list");
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });

  test("the Environment toggle marks a collection as production", async ({ page, seed }) => {
    await seed({ collections: [collection("Shop", [route("list", `curl -X GET ${API}/users`)])] });
    await openRoute(page, "Shop", "list");
    await expect(page.getByText("PRODUCTION", { exact: true })).toHaveCount(0);
    await page.keyboard.press("ControlOrMeta+k");
    await page.keyboard.type("environment");
    await page.keyboard.press("Enter");
    const env = dialog(page, /Environment/);
    await env.getByLabel(/Production environment/).check();
    await env.getByRole("button", { name: /Save/ }).click();
    await expect(page.getByText("PRODUCTION", { exact: true })).toBeVisible();
    const stored = await page.evaluate(() => JSON.parse(localStorage.getItem("hittable")!));
    expect(stored[0].env.HITTABLE_PRODUCTION).toBe("true");
  });
});

test.describe("git-safe secrets (FR-17)", () => {
  test.skip(({ browserName }) => browserName !== "chromium", "Directory mode needs the File System Access API (Chromium only)");

  test("env.local.json overrides env.json and is listed in hittable/.gitignore", async ({ page }) => {
    await seedPrivateFolder(page, {
      "hittable/env.json": JSON.stringify({ BASE: API, TOKEN: "placeholder" }),
      "hittable/env.local.json": JSON.stringify({ TOKEN: "from-local" }),
      "hittable/me.hit": JSON.stringify({ method: "GET", url: "<<BASE>>/echo", headers: { Authorization: "Bearer <<TOKEN>>" }, params: {}, body: "", response: null }),
    });
    await openWorkspace(page);
    await expand(page, "hittable");
    await node(page, "hittable/me.hit").click();
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText('"Bearer from-local"')).toBeVisible();
    expect(await readFile(page, "hittable/.gitignore")).toBe("env.local.json\n");
  });
});
