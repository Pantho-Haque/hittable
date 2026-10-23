import type { Page, Locator } from "@playwright/test";
import {
  test,
  expect,
  API,
  sidebar,
  sidebarItem,
  dialog,
  urlInput,
  response,
  statusBadge,
  collection,
  route,
} from "./fixtures/test";

test.skip(({ isMobile }) => !isMobile, "phone layout only");

const noHorizontalScroll = (page: Page) =>
  expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)).toBeLessThanOrEqual(0);

async function fitsViewport(page: Page, el: Locator) {
  const box = (await el.boundingBox())!;
  const vw = page.viewportSize()!.width;
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(vw);
}

test.describe("on a phone @mobile", () => {
  test("the landing page fits and opens the app", async ({ page }) => {
    await page.goto("/");
    await noHorizontalScroll(page);
    await page.getByRole("link", { name: "Launch App" }).first().tap();
    await expect(page).toHaveURL(/\/hittable$/);
    await noHorizontalScroll(page);
  });

  test("docs pages fit the screen", async ({ page }) => {
    for (const p of ["/docs", "/docs/web-app", "/docs/import-export", "/docs/shellapp-keyboard"]) {
      await page.goto(p);
      await noHorizontalScroll(page);
    }
  });

  test("the sidebar starts collapsed, every tool is reachable, and a request can be sent", async ({ page, seed }) => {
    await seed({ collections: [collection("Phone", [route("me", `curl -X GET ${API}/users/1`)])] });
    await page.goto("/hittable");
    const expand = sidebar(page).getByRole("button", { name: "Expand sidebar" });
    await expect(expand).toBeVisible();
    for (const name of ["Import", "Auth", "History", "Notes", "Info"]) {
      await expect(sidebar(page).getByRole("button", { name, exact: true })).toBeVisible();
    }
    await noHorizontalScroll(page);

    await expand.tap();
    await sidebarItem(page, "Phone").tap();
    await sidebarItem(page, "me").tap();
    await expect(urlInput(page)).toHaveValue(`${API}/users/1`);
    // The open sidebar overlays the runner; the backdrop closes it.
    await page.mouse.click(page.viewportSize()!.width - 10, page.viewportSize()!.height - 10);
    await expect(expand).toBeVisible();

    await page.getByRole("button", { name: "Send (Ctrl/Cmd+Enter)" }).tap();
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText('"Ada Lovelace"')).toBeVisible();
    await noHorizontalScroll(page);
  });

  test("create a collection and a route from the phone", async ({ page, seed }) => {
    await seed({ collections: [] });
    await page.goto("/hittable");
    await sidebar(page).getByRole("button", { name: "Expand sidebar" }).tap();
    await sidebar(page).getByRole("button", { name: "New Collection" }).tap();
    const create = dialog(page, "Create collection");
    await fitsViewport(page, create);
    await create.getByRole("textbox").fill("Mobile API");
    await create.getByRole("button", { name: "Create" }).tap();
    await sidebar(page).getByRole("button", { name: "New Route" }).tap();
    const r = dialog(page, "Create route");
    await fitsViewport(page, r);
    await r.getByRole("textbox", { name: "get-users" }).fill("echo");
    await r.getByPlaceholder("Paste you curl here").fill(`curl -X GET ${API}/echo`);
    await r.getByRole("button", { name: "Create" }).tap();
    await expect(urlInput(page)).toHaveValue(`${API}/echo`);
  });

  test("the dialogs fit on a phone", async ({ page, seed }) => {
    await seed({ collections: [collection("Fit", [route("r", `curl -X GET ${API}/echo`)], { A: "1" })] });
    await page.goto("/hittable?c=Fit&r=r");
    await sidebar(page).getByRole("button", { name: "Environment Variables" }).tap();
    const env = dialog(page, "Environment: Fit");
    await expect(env).toBeVisible();
    await fitsViewport(page, env);
    await env.getByRole("button", { name: "Cancel" }).tap();

    for (const [button, name] of [
      ["Import", "Import"],
      ["Auth", "Authentication"],
      ["History", "Request History"],
      ["Info", "Hittable Info"],
      ["Notes", "Notebook"],
    ] as const) {
      await sidebar(page).getByRole("button", { name: button, exact: true }).tap();
      const d = dialog(page, name);
      await expect(d).toBeVisible();
      await fitsViewport(page, d);
      await page.keyboard.press("Escape");
      await expect(d).toBeHidden();
    }
  });
});
