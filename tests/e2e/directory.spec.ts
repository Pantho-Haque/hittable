import { test, expect, API, urlInput, send, response, statusBadge } from "./fixtures/test";
import { seedPrivateFolder, readFile, tree, node, expand, openWorkspace } from "./fixtures/directory";

test.describe("directory mode", () => {
  // Safari has no showDirectoryPicker (and no main-thread createWritable), so its
  // users get Local mode plus an explanation, tested in tools.spec.ts.
  test.skip(({ browserName }) => browserName !== "chromium", "Directory mode needs the File System Access API (Chromium only)");

  test("picking a folder scaffolds hittable/ and lists it in the explorer", async ({ page }) => {
    await seedPrivateFolder(page);
    await openWorkspace(page);
    await expect(page.getByRole("heading", { name: "Your workspace is ready" })).toBeVisible();
    await expand(page, "hittable");
    await expect(node(page, "hittable/env.json")).toBeVisible();
    await expect(node(page, "hittable/notes")).toBeVisible();
    await expect(node(page, "hittable/testcollection")).toBeVisible();
    expect(JSON.parse(await readFile(page, "hittable/env.json"))).toEqual({});
    expect(JSON.parse(await readFile(page, "hittable/testcollection/test.hit"))).toMatchObject({ method: "GET", response: null });
    expect(await readFile(page, "hittable/notes/sample.md")).toContain("# Sample Note");
  });

  test("a .hit file runs in the Runner with env.json variables and keeps its response", async ({ page }) => {
    await seedPrivateFolder(page, {
      "hittable/env.json": JSON.stringify({ BASE: API }),
      "hittable/users/get.hit": JSON.stringify({ method: "GET", url: "<<BASE>>/users/2", headers: { "X-Dir": "1" }, params: {}, body: "", response: null }),
    });
    await openWorkspace(page);
    await expand(page, "hittable");
    await expand(page, "hittable/users");
    await node(page, "hittable/users/get.hit").click();
    // A valid .hit opens in the Runner, as in the terminal app.
    await expect(page.getByRole("button", { name: "Runner" })).toHaveAttribute("aria-pressed", "true");
    await expect(urlInput(page)).toHaveValue("<<BASE>>/users/2");
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText('"Alan Turing"')).toBeVisible();

    // Sending writes the response into the file; the template stays.
    await expect.poll(async () => JSON.parse(await readFile(page, "hittable/users/get.hit")).response?.data?.name).toBe("Alan Turing");
    const saved = JSON.parse(await readFile(page, "hittable/users/get.hit"));
    expect(saved.url).toBe("<<BASE>>/users/2");
    expect(saved.headers).toEqual({ "X-Dir": "1" });
  });

  test("an invalid .hit file blocks the Runner until fixed", async ({ page }) => {
    await seedPrivateFolder(page, { "hittable/broken.hit": "{ not json" });
    await openWorkspace(page);
    await expand(page, "hittable");
    await node(page, "hittable/broken.hit").click();
    await page.getByRole("button", { name: "Runner" }).click();
    await expect(page.getByText("This file isn't valid Hittable JSON right now — fix it in Text mode first")).toBeVisible();
    await page.getByRole("button", { name: "Switch to Text mode to fix" }).click();
    await expect(page.getByRole("button", { name: "Text" })).toHaveAttribute("aria-pressed", "true");
  });

  test("the workspace reconnects after a reload, and can be switched back to Local", async ({ page }) => {
    await seedPrivateFolder(page);
    await openWorkspace(page);
    await page.reload();
    await expect(tree(page)).toBeVisible();
    await expand(page, "hittable");
    await expect(node(page, "hittable/env.json")).toBeVisible();

    await page.getByRole("button", { name: "Switch to local workspace" }).click();
    await expect(page.getByRole("complementary", { name: "Collections" })).toBeVisible();
    await page.reload();
    await expect(page.getByRole("complementary", { name: "Collections" })).toBeVisible();
  });

  test("New File creates a .hit seeded with an empty GET, and the context menu deletes it", async ({ page }) => {
    await seedPrivateFolder(page);
    await openWorkspace(page);
    await tree(page).page().getByRole("button", { name: "New File" }).click();
    const name = page.getByPlaceholder("File name");
    await expect(name).toBeFocused();
    await name.fill("ping.hit");
    await name.press("Enter");
    await expect(node(page, "ping.hit")).toBeVisible();
    expect(JSON.parse(await readFile(page, "ping.hit"))).toEqual({ method: "GET", url: "", headers: {}, params: {}, body: "", response: null });

    await node(page, "ping.hit").click({ button: "right" });
    await page.getByRole("button", { name: "Delete" }).click();
    await expect(node(page, "ping.hit")).toHaveCount(0);
    await expect(readFile(page, "ping.hit")).rejects.toThrow();
  });

  test("Ctrl/Cmd+B hides and shows the explorer", async ({ page }) => {
    await seedPrivateFolder(page);
    await openWorkspace(page);
    const toggle = page.getByRole("button", { name: "Explorer" });
    await expect(toggle).toHaveAttribute("aria-expanded", "true");
    await page.keyboard.press("ControlOrMeta+b");
    await expect(tree(page)).toBeHidden();
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await toggle.click();
    await expect(tree(page)).toBeVisible();
  });
});
