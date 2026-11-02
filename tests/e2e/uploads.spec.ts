import crypto from "node:crypto";
import { test, expect, API, openRequest, FOLDER_ONLY, send, response, sendButton, statusBadge } from "./fixtures/test";
import { readBytes } from "./fixtures/directory";
import type { Page } from "@playwright/test";

// File uploads from a local folder (docs/file-uploads.md). The fixture's /upload
// answers with each part's name, type, size and SHA-256 as received.
const PNG = Buffer.from("89504e470d0a1a0a0000000d4948445200000001000000010806000000ff00", "hex");
const sha = (b: Buffer) => crypto.createHash("sha256").update(b).digest("hex");
const b64 = (b: Buffer) => `base64:${b.toString("base64")}`;
const bodyTab = (page: Page) => page.getByRole("tab", { name: "Body", exact: true });
/** The response as raw text: the JSON tree collapses nested parts. */
async function raw(page: Page) {
  await expect(statusBadge(page)).toHaveText("200 OK");
  await response(page).getByRole("button", { name: "Raw text view" }).click();
  return response(page);
}

test.describe("file uploads", () => {
  test.skip(({ browserName }) => browserName !== "chromium", FOLDER_ONLY);

  test("multipart: a text field and a file from the workspace arrive byte for byte", async ({ page }) => {
    await openRequest(page, {
      method: "POST",
      url: `${API}/upload`,
      headers: { "Content-Type": "multipart/form-data" },
      body: JSON.stringify({ name: "Tanvir", avatar: "@files/avatar.png" }),
      files: { "hittable/files/avatar.png": b64(PNG) },
    });
    await bodyTab(page).click();
    await expect(page.getByRole("radio", { name: "Multipart" })).toHaveAttribute("aria-checked", "true");
    await expect(page.getByLabel("Field 2 file")).toHaveValue("files/avatar.png");
    await send(page);
    const res = await raw(page);
    await expect(res).toContainText(sha(PNG));
    await expect(res).toContainText("multipart/form-data; boundary=");
    await expect(res).toContainText("avatar.png");
    await expect(res).toContainText("Tanvir");
  });

  test("choose a file from the computer: copied to hittable/files, stored relative, sent", async ({ page }) => {
    await openRequest(page, { method: "POST", url: `${API}/upload`, headers: { "Content-Type": "multipart/form-data" }, body: "{}" });
    await bodyTab(page).click();
    await page.getByRole("button", { name: "Add file" }).click();
    await page.getByLabel("Field 1", { exact: true }).fill("photo");
    await page.locator('input[type="file"]').setInputFiles({ name: "me.png", mimeType: "image/png", buffer: PNG });
    await expect(page.getByLabel("Field 1 file")).toHaveValue("files/me.png");
    expect(await readBytes(page, "hittable/files/me.png")).toBe(PNG.toString("base64"));
    await expect(page.getByRole("treeitem", { name: "files", exact: true })).toBeVisible(); // the new folder shows in the explorer
    await send(page);
    await expect(await raw(page)).toContainText(sha(PNG));
  });

  test("a file body: the bytes are the body, with a Content-Type from the file name", async ({ page }) => {
    const pdf = Buffer.from("%PDF-1.7\n\0\x01binary");
    await openRequest(page, { method: "PUT", url: `${API}/upload`, body: "@files/report.pdf", files: { "hittable/files/report.pdf": b64(pdf) } });
    await bodyTab(page).click();
    await expect(page.getByRole("radio", { name: "File" })).toHaveAttribute("aria-checked", "true");
    await expect(page.getByText("application/pdf", { exact: true })).toBeVisible();
    await send(page);
    const res = await raw(page);
    await expect(res).toContainText(sha(pdf));
    await expect(res).toContainText('"application/pdf"');
  });

  test("a missing file stops the send and names it", async ({ page }) => {
    await openRequest(page, { method: "PUT", url: `${API}/upload`, body: "@files/nope.bin" });
    await bodyTab(page).click();
    await expect(page.getByText(/Not found: hittable\/files\/nope\.bin/)).toBeVisible();
    await send(page);
    await expect(response(page)).toContainText("File not found: files/nope.bin");
    await expect(sendButton(page)).toBeEnabled();
  });

  test("an upload over the proxy limit says how big it was", async ({ page }) => {
    await openRequest(page, { method: "PUT", url: `${API}/upload`, body: "" });
    await bodyTab(page).click();
    await page.getByRole("radio", { name: "File" }).click();
    await page.locator('input[type="file"]').setInputFiles({ name: "big.bin", mimeType: "application/octet-stream", buffer: Buffer.alloc(4 * 1024 * 1024 + 10) });
    await expect(page.getByLabel("Body file")).toHaveValue("files/big.bin");
    await send(page);
    await expect(response(page)).toContainText(/The upload is 4\.0 MB/, { timeout: 20_000 });
  });

  test("Copy as cURL writes -F for files", async ({ page, context }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    await openRequest(page, {
      method: "POST", url: `${API}/upload`, headers: { "Content-Type": "multipart/form-data" },
      body: JSON.stringify({ name: "x", avatar: "@files/avatar.png" }),
    });
    await page.getByRole("button", { name: /Copy as/ }).click();
    await page.getByRole("menuitem", { name: "cURL" }).click();
    const copied = await page.evaluate(() => navigator.clipboard.readText());
    expect(copied).toContain("-F 'avatar=@files/avatar.png'");
    expect(copied).toContain("--form-string 'name=x'");
  });
});
