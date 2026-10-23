import type { Page } from "@playwright/test";
import { test, expect, API, dialog, editor, send, response, collection, route, openRoute } from "./fixtures/test";

async function applyAuth(page: Page, preset: RegExp, fill: (d: ReturnType<typeof dialog>) => Promise<void>) {
  await page.getByRole("button", { name: "Auth", exact: true }).click();
  const d = dialog(page, "Authentication");
  await d.getByRole("button", { name: preset }).click();
  await fill(d);
  await d.getByRole("button", { name: "Apply" }).click();
  await expect(d).toBeHidden();
}

const headersJson = async (page: Page) => {
  await page.getByRole("button", { name: "Headers", exact: true }).click();
  return JSON.parse((await editor(page, "headers").inputValue()) || "{}");
};

test.describe("auth presets", () => {
  test.beforeEach(async ({ page, seed }) => {
    await seed({ collections: [collection("Presets", [route("r", `curl -X GET ${API}/echo -H "Accept: application/json"`)])] });
    await openRoute(page, "Presets", "r");
  });

  test("Bearer token writes an Authorization header that is sent", async ({ page }) => {
    await applyAuth(page, /^Bearer Token/, (d) => d.getByPlaceholder("eyJhbGciOiJIUzI1NiIs...").fill("abc.def"));
    expect(await headersJson(page)).toEqual({ Accept: "application/json", Authorization: "Bearer abc.def" });
    await send(page);
    await expect(response(page).getByText('"Bearer abc.def"')).toBeVisible();
  });

  test("Basic auth is base64 of user:pass", async ({ page }) => {
    await applyAuth(page, /^Basic Auth/, async (d) => {
      await d.getByPlaceholder("admin").fill("ada");
      await d.getByPlaceholder("••••••••").fill("s3cret:x");
    });
    expect((await headersJson(page)).Authorization).toBe(`Basic ${Buffer.from("ada:s3cret:x").toString("base64")}`);
  });

  test("API key goes in a custom header, X-API-Key by default", async ({ page }) => {
    await applyAuth(page, /^API Key/, (d) => d.getByPlaceholder("your-api-key").fill("k-1"));
    expect((await headersJson(page))["X-API-Key"]).toBe("k-1");
    await applyAuth(page, /^API Key/, async (d) => {
      await d.getByPlaceholder("X-API-Key").fill("X-Custom-Key");
      await d.getByPlaceholder("your-api-key").fill("k-2");
    });
    expect((await headersJson(page))["X-Custom-Key"]).toBe("k-2");
  });

  test("switching preset replaces the old Authorization, and Clear Auth removes it", async ({ page }) => {
    await applyAuth(page, /^Bearer Token/, (d) => d.getByPlaceholder("eyJhbGciOiJIUzI1NiIs...").fill("one"));
    await applyAuth(page, /^Basic Auth/, (d) => d.getByPlaceholder("admin").fill("u"));
    expect((await headersJson(page)).Authorization).toMatch(/^Basic /);
    await page.getByRole("button", { name: "Auth", exact: true }).click();
    await dialog(page, "Authentication").getByRole("button", { name: "Clear Auth" }).click();
    expect(await headersJson(page)).toEqual({ Accept: "application/json" });
    // Only the header is stored: the request is now unsaved, not the preset.
    await expect(page.getByText("Unsaved · Ctrl/Cmd+S")).toBeHidden();
  });

  test("Cancel and Escape leave the headers alone", async ({ page }) => {
    await page.getByRole("button", { name: "Auth", exact: true }).click();
    const d = dialog(page, "Authentication");
    await d.getByRole("button", { name: /^Bearer Token/ }).click();
    await d.getByPlaceholder("eyJhbGciOiJIUzI1NiIs...").fill("nope");
    await d.getByRole("button", { name: "Cancel" }).click();
    await expect(d).toBeHidden();
    await page.getByRole("button", { name: "Auth", exact: true }).click();
    await d.getByRole("button", { name: /^Bearer Token/ }).click();
    await d.getByPlaceholder("eyJhbGciOiJIUzI1NiIs...").fill("nope");
    await page.keyboard.press("Escape");
    await expect(d).toBeHidden();
    expect(await headersJson(page)).toEqual({ Accept: "application/json" });
  });
});

test.describe("info", () => {
  test("lists the keyboard shortcuts and the privacy notes", async ({ page }) => {
    await page.goto("/hittable");
    await page.getByRole("button", { name: "Info" }).click();
    const d = dialog(page, "Hittable Info");
    await d.getByRole("button", { name: "Keybindings" }).click();
    for (const text of ["Send request", "Save route", "Beautify JSON", "Search in response", "Create new route", "Toggle sidebar"]) {
      await expect(d.getByText(text)).toBeVisible();
    }
    await d.getByRole("button", { name: "Data & Privacy" }).click();
    await expect(d.getByText(/localStorage/i).first()).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(d).toBeHidden();
  });
});

test.describe("workspace mode", () => {
  test("the Local/Directory switch says Local and offers a folder", async ({ page }) => {
    await page.goto("/hittable");
    const toggle = page.getByRole("button", { name: "Open a directory workspace" });
    await expect(toggle).toHaveText("Local");
    await expect(toggle).toBeEnabled();
  });

  test("browsers without the File System Access API get an explanation", async ({ page }) => {
    await page.addInitScript(() => {
      // Like Safari or Firefox.
      delete (Window.prototype as unknown as { showDirectoryPicker?: unknown }).showDirectoryPicker;
      delete (window as unknown as { showDirectoryPicker?: unknown }).showDirectoryPicker;
    });
    await page.goto("/hittable");
    await page.getByRole("button", { name: "Open a directory workspace" }).click();
    await expect(page.getByRole("status")).toContainText("Browser Not Supported");
    await expect(page.getByRole("status")).toContainText("Chromium-based browsers");
  });
});
