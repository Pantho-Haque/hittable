import fs from "node:fs";
import type { Page } from "@playwright/test";
import { test, expect, openLauncher, FOLDER_ONLY } from "./fixtures/test";
import { seedPrivateFolder, tree } from "./fixtures/directory";

// /hittable: choose a local folder or a GitHub repository (FR-G9), see why
// sign-in failed, and rescue the old Local mode's collections (FR-G10).

const folderButton = (page: Page) => page.getByRole("button", { name: "Open a local folder" });

test.describe("workspaces launcher", () => {
  test("offers a local folder and GitHub sign-in", async ({ page, browserName }) => {
    test.skip(browserName !== "chromium", FOLDER_ONLY);
    await openLauncher(page);
    await expect(page.getByRole("heading", { level: 1, name: "Open a workspace" })).toBeVisible();
    await expect(folderButton(page)).toBeEnabled();
    // The top bar's account menu offers the same sign-in.
    await expect(page.locator('a[href^="/api/github/login"]').first()).toBeVisible();
  });

  test("Open a local folder opens the folder workspace", async ({ page, browserName }) => {
    test.skip(browserName !== "chromium", FOLDER_ONLY);
    await seedPrivateFolder(page, { "hittable/env.json": "{}" });
    await openLauncher(page);
    await folderButton(page).click();
    await expect(page).toHaveURL(/\/hittable\/folder$/);
    await expect(tree(page)).toBeVisible();
  });

  test("a cancelled folder picker stays on the launcher", async ({ page, browserName }) => {
    test.skip(browserName !== "chromium", FOLDER_ONLY);
    await page.addInitScript(() => {
      (window as unknown as { showDirectoryPicker: () => Promise<never> }).showDirectoryPicker = () =>
        Promise.reject(new DOMException("The user aborted a request.", "AbortError"));
    });
    await openLauncher(page);
    await folderButton(page).click();
    await expect(folderButton(page)).toBeEnabled();
    await expect(page).toHaveURL(/\/hittable$/);
  });

  test("browsers without the File System Access API get an explanation", async ({ page }) => {
    // WebKit has no showDirectoryPicker; elsewhere it is removed, like Firefox.
    await page.addInitScript(() => {
      delete (Window.prototype as unknown as { showDirectoryPicker?: unknown }).showDirectoryPicker;
      delete (window as unknown as { showDirectoryPicker?: unknown }).showDirectoryPicker;
    });
    await openLauncher(page);
    await expect(page.getByRole("note")).toContainText("Chrome or Edge");
    await expect(page.getByRole("note")).toContainText("use a GitHub repository instead");
    await expect(folderButton(page)).toHaveCount(0);
  });

  test("a sign-in that was cancelled on GitHub says so", async ({ page }) => {
    await page.goto("/hittable?auth_error=denied");
    await expect(page.getByRole("alert").filter({ hasText: "Sign-in was cancelled on GitHub. Nothing was changed." })).toBeVisible();
  });

  test("an unknown sign-in error still gets a message", async ({ page }) => {
    await page.goto("/hittable?auth_error=something_new");
    await expect(page.getByRole("alert").filter({ hasText: "Sign-in with GitHub didn't work. Please try again." })).toBeVisible();
  });

  test("no error, no alert", async ({ page }) => {
    await openLauncher(page);
    await expect(page.getByRole("heading", { name: "Open a workspace" })).toBeVisible();
    await expect(page.getByRole("alert").filter({ hasText: /Sign-in/ })).toHaveCount(0);
  });
});

test.describe("old Local mode collections (FR-G10)", () => {
  const OLD = [
    {
      collectionName: "My API",
      items: [
        { type: "route", name: "list users", curl: "curl -X GET <<BASE>>/users -H 'Authorization: Bearer <<TOKEN>>'" },
        { type: "folder", name: "Admin", items: [{ type: "route", name: "create", curl: `curl -X POST <<BASE>>/users -d '{"name":"Ada"}'` }] },
      ],
      env: { BASE: "https://api.example.com" },
      secrets: { TOKEN: "super-secret-value" },
    },
  ];
  const banner = (page: Page) => page.getByRole("region", { name: /Your old browser collections are still here/ });

  test("the banner offers a Postman download, without secret values", async ({ page, seed }) => {
    await seed({ hittable: JSON.stringify(OLD) });
    await openLauncher(page);
    await expect(banner(page)).toContainText("1 collection from the old Local mode");
    const [download] = await Promise.all([page.waitForEvent("download"), banner(page).getByRole("button", { name: "Download" }).click()]);
    expect(download.suggestedFilename()).toBe("My-API.postman_collection.json");
    const text = fs.readFileSync((await download.path())!, "utf8");
    expect(text).not.toContain("super-secret-value");
    const postman = JSON.parse(text);
    expect(postman.info).toMatchObject({ name: "My API", schema: expect.stringContaining("v2.1.0") });
    expect(postman.item[0]).toMatchObject({
      name: "list users",
      request: { method: "GET", url: "{{BASE}}/users", header: [{ key: "Authorization", value: "Bearer {{TOKEN}}" }] },
    });
    expect(postman.item[1]).toMatchObject({ name: "Admin", item: [{ name: "create", request: { method: "POST" } }] });
    expect(postman.variable).toEqual([
      { key: "BASE", value: "https://api.example.com", type: "string" },
      { key: "TOKEN", value: "", type: "string" },
    ]);
  });

  test("Dismiss hides it for good", async ({ page, seed }) => {
    await seed({ hittable: JSON.stringify(OLD) });
    await openLauncher(page);
    await banner(page).getByRole("button", { name: "Dismiss" }).click();
    await expect(banner(page)).toHaveCount(0);
    await page.reload();
    await expect(page.getByRole("heading", { name: "Open a workspace" })).toBeVisible();
    await expect(banner(page)).toHaveCount(0);
  });

  test("without old data there is no banner", async ({ page }) => {
    await openLauncher(page);
    await expect(page.getByRole("heading", { name: "Open a workspace" })).toBeVisible();
    await expect(banner(page)).toHaveCount(0);
  });

  test("corrupt old data shows no banner and no error", async ({ page, seed }) => {
    await seed({ hittable: "{not json" });
    await openLauncher(page);
    await expect(page.getByRole("heading", { name: "Open a workspace" })).toBeVisible();
    await expect(banner(page)).toHaveCount(0);
  });
});
